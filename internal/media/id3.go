// Package media 读取已声明的媒体标签。这里不解码音频，也不表示能够播放。
package media

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf16"
)

// maxTagBytes 是标签区的读取上限。只读文件开头的标签，音频本身的大小不受限制。
const maxTagBytes = 16 << 20

// Tags 是从一个容器里读到的文本与元数据标签。空字符串表示该帧不存在。
type Tags struct {
	Title   string
	Artist  string
	Album   string
	Year    string
	Track   string
	Genre   string
	Comment string
	Format  string
	HasPic  bool
	PicSize int
}

// ID3v2Title 读取 ID3v2 头里的 TIT2，并在同一遍扫描里带上已有的 TPE1 与 TALB。
// 没有 TIT2、TIT2 为空、头不合法或帧被截断时返回错误，文案与只读标题时相同。
func ID3v2Title(data []byte) (Tags, error) {
	tags, err := ReadID3v2(data)
	if err != nil {
		return Tags{}, err
	}
	if tags.Title == "" {
		return Tags{}, errors.New("TIT2 frame not found")
	}
	return tags, nil
}

// ReadID3v2 读取 ID3v2.3 或 ID3v2.4 中的文本标签与封面信息。
// v2.4 的帧大小是同步安全整数，v2.3 是普通大端整数；扩展头会被跳过。
// 整个标签使用不同步编码，或目标帧被压缩、加密时明确报错，不返回猜测的文本。
func ReadID3v2(data []byte) (Tags, error) {
	if len(data) < 10 || string(data[0:3]) != "ID3" {
		return Tags{}, errors.New("not an id3v2 header")
	}
	version, flags := data[3], data[5]
	if version != 3 && version != 4 {
		return Tags{}, fmt.Errorf("unsupported id3v2.%d tag", version)
	}
	if flags&0x80 != 0 {
		return Tags{}, errors.New("unsynchronised id3 tag is not supported")
	}
	size := synchsafe(data[6:10])
	if size < 0 || 10+size > len(data) {
		return Tags{}, errors.New("id3 size exceeds buffer")
	}
	body := data[10 : 10+size]
	if flags&0x40 != 0 {
		skip, err := extendedHeaderSize(body, version)
		if err != nil {
			return Tags{}, err
		}
		body = body[skip:]
	}
	tags := Tags{Format: fmt.Sprintf("ID3v2.%d", version)}
	var sawAny bool
	for len(body) >= 10 {
		if body[0] == 0 {
			break
		}
		id := string(body[0:4])
		frameSize := int(binary.BigEndian.Uint32(body[4:8]))
		if version == 4 {
			frameSize = synchsafe(body[4:8])
		}
		if frameSize < 0 || 10+frameSize > len(body) {
			return Tags{}, errors.New("truncated id3 frame")
		}
		payload := body[10 : 10+frameSize]
		formatFlag := body[9]
		body = body[10+frameSize:]

		switch id {
		case "TIT2", "TPE1", "TALB", "TYER", "TDRC", "TRCK", "TCON":
			if packedFrame(formatFlag, version) {
				return Tags{}, fmt.Errorf("compressed or encrypted %s is not supported", id)
			}
			if len(payload) == 0 {
				continue
			}
			text := decodeText(payload[0], payload[1:])
			if text == "" {
				continue
			}
			sawAny = true
			switch id {
			case "TIT2":
				if tags.Title == "" {
					tags.Title = text
				}
			case "TPE1":
				if tags.Artist == "" {
					tags.Artist = text
				}
			case "TALB":
				if tags.Album == "" {
					tags.Album = text
				}
			case "TYER", "TDRC":
				if tags.Year == "" {
					tags.Year = text
				}
			case "TRCK":
				if tags.Track == "" {
					tags.Track = text
				}
			case "TCON":
				if tags.Genre == "" {
					tags.Genre = text
				}
			}
		case "COMM":
			if !packedFrame(formatFlag, version) && len(payload) > 4 {
				text := decodeComment(payload)
				if text != "" && tags.Comment == "" {
					tags.Comment = text
					sawAny = true
				}
			}
		case "APIC":
			if !packedFrame(formatFlag, version) && len(payload) > 0 {
				tags.HasPic = true
				tags.PicSize = len(payload)
				sawAny = true
			}
		}
	}
	if !sawAny {
		return Tags{}, errors.New("TIT2 frame not found")
	}
	return tags, nil
}

// decodeComment 解析 COMM 帧：1 字节编码 + 3 字节语言 + 以 0 结尾的简短描述 + 正文。
func decodeComment(payload []byte) string {
	if len(payload) <= 4 {
		return ""
	}
	enc := payload[0]
	rest := payload[4:]
	if enc == 1 || enc == 2 {
		for i := 0; i+1 < len(rest); i += 2 {
			if rest[i] == 0 && rest[i+1] == 0 {
				return decodeText(enc, rest[i+2:])
			}
		}
		return decodeText(enc, rest)
	}
	if idx := bytes.IndexByte(rest, 0); idx >= 0 {
		return decodeText(enc, rest[idx+1:])
	}
	return decodeText(enc, rest)
}

// extendedHeaderSize 返回扩展头占用的字节数。v2.3 的大小不含自身 4 字节，v2.4 的同步安全大小含自身。
func extendedHeaderSize(body []byte, version byte) (int, error) {
	if len(body) < 4 {
		return 0, errors.New("truncated id3 extended header")
	}
	n := int(binary.BigEndian.Uint32(body[0:4])) + 4
	if version == 4 {
		n = synchsafe(body[0:4])
	}
	if n < 4 || n > len(body) {
		return 0, errors.New("truncated id3 extended header")
	}
	return n, nil
}

// packedFrame 判断帧格式标志里是否有压缩、加密或帧级不同步。
func packedFrame(format, version byte) bool {
	if version == 4 {
		return format&0x0e != 0
	}
	return format&0xc0 != 0
}

// Open 读取音频文件头部的元数据标签（支持 ID3v2、FLAC、M4A/MP4、WAV、OGG 及末尾 ID3v1），音频再大也不整体读入。
func Open(ctx context.Context, path string) (Tags, error) {
	if err := ctx.Err(); err != nil {
		return Tags{}, err
	}
	f, err := os.Open(path)
	if err != nil {
		return Tags{}, err
	}
	defer f.Close()
	header := make([]byte, 12)
	n, err := io.ReadFull(f, header)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		if errors.Is(err, io.EOF) {
			return Tags{}, errors.New("not an id3v2 header")
		}
		return Tags{}, err
	}
	header = header[:n]
	if len(header) < 10 {
		return Tags{}, errors.New("not an id3v2 header")
	}

	switch {
	case string(header[0:3]) == "ID3":
		size := synchsafe(header[6:10])
		if size > maxTagBytes {
			return Tags{}, fmt.Errorf("id3 tag in %s exceeds 16 MiB", path)
		}
		data := make([]byte, 10+size)
		copy(data, header[:10])
		if _, err = f.Seek(10, io.SeekStart); err != nil {
			return Tags{}, err
		}
		if _, err = io.ReadFull(f, data[10:]); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return Tags{}, errors.New("id3 size exceeds buffer")
			}
			return Tags{}, err
		}
		if err = ctx.Err(); err != nil {
			return Tags{}, err
		}
		return ReadID3v2(data)

	case string(header[0:4]) == "fLaC":
		return readFLAC(ctx, f)

	case len(header) >= 8 && string(header[4:8]) == "ftyp":
		return readMP4(ctx, f)

	case len(header) >= 12 && string(header[0:4]) == "RIFF" && string(header[8:12]) == "WAVE":
		return readWAV(ctx, f)

	case string(header[0:4]) == "OggS":
		return readOGG(ctx, f)
	}

	// 检查文件末尾是否有 128 字节的 ID3v1 标签
	if info, statErr := f.Stat(); statErr == nil && info.Size() >= 128 {
		tail := make([]byte, 128)
		if _, seekErr := f.Seek(-128, io.SeekEnd); seekErr == nil {
			if _, readErr := io.ReadFull(f, tail); readErr == nil && string(tail[0:3]) == "TAG" {
				return parseID3v1(tail), nil
			}
		}
	}

	return Tags{}, errors.New("not an id3v2 header")
}

// readFLAC 解析 FLAC 元数据块中的 VORBIS_COMMENT (4) 与 PICTURE (6)。
func readFLAC(ctx context.Context, f *os.File) (Tags, error) {
	if _, err := f.Seek(4, io.SeekStart); err != nil {
		return Tags{}, err
	}
	tags := Tags{Format: "FLAC"}
	blockHeader := make([]byte, 4)
	for {
		if err := ctx.Err(); err != nil {
			return Tags{}, err
		}
		if _, err := io.ReadFull(f, blockHeader); err != nil {
			break
		}
		last := blockHeader[0]&0x80 != 0
		blockType := blockHeader[0] & 0x7f
		length := int(blockHeader[1])<<16 | int(blockHeader[2])<<8 | int(blockHeader[3])
		if length < 0 || length > maxTagBytes {
			return Tags{}, errors.New("flac metadata block exceeds limit")
		}
		if blockType == 4 || blockType == 6 {
			buf := make([]byte, length)
			if _, err := io.ReadFull(f, buf); err != nil {
				return Tags{}, err
			}
			if blockType == 4 {
				parseVorbisComments(buf, &tags)
			} else if blockType == 6 {
				tags.HasPic = true
				tags.PicSize = length
			}
		} else {
			if _, err := f.Seek(int64(length), io.SeekCurrent); err != nil {
				break
			}
		}
		if last {
			break
		}
	}
	return tags, nil
}

func parseVorbisComments(b []byte, tags *Tags) {
	if len(b) < 8 {
		return
	}
	vendorLen := int(binary.LittleEndian.Uint32(b[0:4]))
	if vendorLen < 0 || 4+vendorLen+4 > len(b) {
		return
	}
	pos := 4 + vendorLen
	count := int(binary.LittleEndian.Uint32(b[pos : pos+4]))
	pos += 4
	for i := 0; i < count && pos+4 <= len(b); i++ {
		n := int(binary.LittleEndian.Uint32(b[pos : pos+4]))
		pos += 4
		if n < 0 || pos+n > len(b) {
			break
		}
		entry := string(b[pos : pos+n])
		pos += n
		k, v, ok := strings.Cut(entry, "=")
		if !ok || v == "" {
			continue
		}
		switch strings.ToUpper(strings.TrimSpace(k)) {
		case "TITLE":
			if tags.Title == "" {
				tags.Title = v
			}
		case "ARTIST":
			if tags.Artist == "" {
				tags.Artist = v
			}
		case "ALBUM":
			if tags.Album == "" {
				tags.Album = v
			}
		case "DATE", "YEAR":
			if tags.Year == "" {
				tags.Year = v
			}
		case "TRACKNUMBER", "TRACK":
			if tags.Track == "" {
				tags.Track = v
			}
		case "GENRE":
			if tags.Genre == "" {
				tags.Genre = v
			}
		case "COMMENT", "DESCRIPTION":
			if tags.Comment == "" {
				tags.Comment = v
			}
		}
	}
}

// readOGG 在文件头部最多 256 KiB 内定位 VorbisComment 或 OpusTags。
func readOGG(ctx context.Context, f *os.File) (Tags, error) {
	if err := ctx.Err(); err != nil {
		return Tags{}, err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return Tags{}, err
	}
	buf, err := io.ReadAll(io.LimitReader(f, 256<<10))
	if err != nil {
		return Tags{}, err
	}
	tags := Tags{Format: "OGG"}
	if idx := bytes.Index(buf, []byte("\x03vorbis")); idx >= 0 {
		parseVorbisComments(buf[idx+7:], &tags)
	} else if idx := bytes.Index(buf, []byte("OpusTags")); idx >= 0 {
		tags.Format = "OGG/Opus"
		parseVorbisComments(buf[idx+8:], &tags)
	}
	return tags, nil
}

// readWAV 解析 WAV RIFF 中的 LIST->INFO 块或内嵌 ID3 块。
func readWAV(ctx context.Context, f *os.File) (Tags, error) {
	if _, err := f.Seek(12, io.SeekStart); err != nil {
		return Tags{}, err
	}
	tags := Tags{Format: "WAV"}
	chunkHeader := make([]byte, 8)
	for {
		if err := ctx.Err(); err != nil {
			return Tags{}, err
		}
		if _, err := io.ReadFull(f, chunkHeader); err != nil {
			break
		}
		id := string(chunkHeader[0:4])
		size := int(binary.LittleEndian.Uint32(chunkHeader[4:8]))
		if size < 0 {
			break
		}
		if (id == "LIST" || id == "id3 " || id == "ID3 ") && size <= maxTagBytes {
			buf := make([]byte, size)
			if _, err := io.ReadFull(f, buf); err != nil {
				break
			}
			if id == "LIST" && len(buf) >= 4 && string(buf[0:4]) == "INFO" {
				parseRIFFInfo(buf[4:], &tags)
			} else if (id == "id3 " || id == "ID3 ") && len(buf) >= 10 && string(buf[0:3]) == "ID3" {
				if id3Tags, err := ReadID3v2(buf); err == nil {
					id3Tags.Format = "WAV (ID3v2)"
					return id3Tags, nil
				}
			}
		} else {
			if _, err := f.Seek(int64(size), io.SeekCurrent); err != nil {
				break
			}
		}
		if size%2 == 1 {
			_, _ = f.Seek(1, io.SeekCurrent)
		}
	}
	return tags, nil
}

func parseRIFFInfo(b []byte, tags *Tags) {
	for len(b) >= 8 {
		id := string(b[0:4])
		size := int(binary.LittleEndian.Uint32(b[4:8]))
		b = b[8:]
		if size < 0 || size > len(b) {
			break
		}
		val := strings.TrimRight(string(b[:size]), "\x00 ")
		b = b[size:]
		if size%2 == 1 && len(b) > 0 {
			b = b[1:]
		}
		if val == "" {
			continue
		}
		switch id {
		case "INAM":
			tags.Title = val
		case "IART":
			tags.Artist = val
		case "IPRD":
			tags.Album = val
		case "ICRD":
			tags.Year = val
		case "IPRT", "ITRK":
			tags.Track = val
		case "IGNR":
			tags.Genre = val
		case "ICMT":
			tags.Comment = val
		}
	}
}

// readMP4 解析 M4A/MP4 容器的 moov -> udta -> meta -> ilst 元数据原子。
func readMP4(ctx context.Context, f *os.File) (Tags, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return Tags{}, err
	}
	tags := Tags{Format: "M4A/MP4"}
	hdr := make([]byte, 8)
	for {
		if err := ctx.Err(); err != nil {
			return Tags{}, err
		}
		if _, err := io.ReadFull(f, hdr); err != nil {
			break
		}
		size := int64(binary.BigEndian.Uint32(hdr[0:4]))
		typ := string(hdr[4:8])
		headerLen := int64(8)
		if size == 1 {
			ext := make([]byte, 8)
			if _, err := io.ReadFull(f, ext); err != nil {
				break
			}
			size = int64(binary.BigEndian.Uint64(ext))
			headerLen = 16
		}
		if size < headerLen {
			break
		}
		bodySize := size - headerLen
		if typ == "moov" && bodySize <= maxTagBytes {
			buf := make([]byte, bodySize)
			if _, err := io.ReadFull(f, buf); err != nil {
				break
			}
			parseMP4Atoms(buf, &tags)
			break
		}
		if _, err := f.Seek(bodySize, io.SeekCurrent); err != nil {
			break
		}
	}
	return tags, nil
}

func parseMP4Atoms(b []byte, tags *Tags) {
	for len(b) >= 8 {
		size := int(binary.BigEndian.Uint32(b[0:4]))
		typ := string(b[4:8])
		if size < 8 || size > len(b) {
			break
		}
		body := b[8:size]
		b = b[size:]
		switch typ {
		case "udta", "ilst":
			parseMP4Atoms(body, tags)
		case "meta":
			if len(body) > 4 {
				parseMP4Atoms(body[4:], tags)
			}
		case "\xa9nam", "\xa9ART", "\xa9alb", "\xa9day", "\xa9gen", "\xa9cmt", "trkn", "covr":
			val, raw := extractMP4Data(body)
			switch typ {
			case "\xa9nam":
				tags.Title = val
			case "\xa9ART":
				tags.Artist = val
			case "\xa9alb":
				tags.Album = val
			case "\xa9day":
				tags.Year = val
			case "\xa9gen":
				tags.Genre = val
			case "\xa9cmt":
				tags.Comment = val
			case "trkn":
				if len(raw) >= 4 {
					tr := binary.BigEndian.Uint16(raw[2:4])
					if tr > 0 {
						tags.Track = fmt.Sprintf("%d", tr)
					}
				}
			case "covr":
				if len(raw) > 0 {
					tags.HasPic = true
					tags.PicSize = len(raw)
				}
			}
		}
	}
}

func extractMP4Data(b []byte) (string, []byte) {
	for len(b) >= 16 {
		size := int(binary.BigEndian.Uint32(b[0:4]))
		typ := string(b[4:8])
		if size < 16 || size > len(b) {
			break
		}
		if typ == "data" {
			payload := b[16:size]
			return strings.TrimRight(string(payload), "\x00"), payload
		}
		b = b[size:]
	}
	return "", nil
}

func parseID3v1(tail []byte) Tags {
	clean := func(b []byte) string {
		if i := bytes.IndexByte(b, 0); i >= 0 {
			b = b[:i]
		}
		return strings.TrimSpace(decodeText(0, b))
	}
	tags := Tags{
		Format:  "ID3v1",
		Title:   clean(tail[3:33]),
		Artist:  clean(tail[33:63]),
		Album:   clean(tail[63:93]),
		Year:    clean(tail[93:97]),
		Comment: clean(tail[97:127]),
	}
	if tail[125] == 0 && tail[126] != 0 {
		tags.Comment = clean(tail[97:125])
		tags.Track = fmt.Sprintf("%d", tail[126])
	}
	return tags
}

func synchsafe(b []byte) int {
	if len(b) != 4 {
		return -1
	}
	return int(b[0]&0x7f)<<21 | int(b[1]&0x7f)<<14 | int(b[2]&0x7f)<<7 | int(b[3]&0x7f)
}

// decodeText 按 ID3 文本编码解码：0 是 Latin-1，1 是带 BOM 的 UTF-16，2 是无 BOM 的 UTF-16BE，3 是 UTF-8。
func decodeText(enc byte, payload []byte) string {
	switch enc {
	case 0:
		payload = trimZero(payload)
		runes := make([]rune, len(payload))
		for i, b := range payload {
			runes[i] = rune(b)
		}
		return string(runes)
	case 1:
		if len(payload) >= 2 && payload[0] == 0xfe && payload[1] == 0xff {
			return utf16Text(payload[2:], binary.BigEndian)
		}
		if len(payload) >= 2 && payload[0] == 0xff && payload[1] == 0xfe {
			payload = payload[2:]
		}
		return utf16Text(payload, binary.LittleEndian)
	case 2:
		return utf16Text(payload, binary.BigEndian)
	default:
		return string(trimZero(payload))
	}
}

func trimZero(b []byte) []byte {
	for len(b) > 0 && b[len(b)-1] == 0 {
		b = b[:len(b)-1]
	}
	return b
}

func utf16Text(b []byte, order binary.ByteOrder) string {
	if len(b)%2 == 1 {
		b = b[:len(b)-1]
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = order.Uint16(b[i*2:])
	}
	for len(u) > 0 && u[len(u)-1] == 0 {
		u = u[:len(u)-1]
	}
	return string(utf16.Decode(u))
}
