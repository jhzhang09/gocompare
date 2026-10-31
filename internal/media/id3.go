// Package media 读取已声明的媒体标签。这里不解码音频，也不表示能够播放。
package media

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"unicode/utf16"
)

// maxTagBytes 是 ID3v2 标签区的读取上限。只读文件开头的标签，音频本身的大小不受限制。
const maxTagBytes = 16 << 20

// Tags 是从一个容器里读到的文本标签。空字符串表示该帧不存在。
type Tags struct {
	Title  string
	Artist string
	Album  string
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

// ReadID3v2 读取 ID3v2.3 或 ID3v2.4 中的 TIT2、TPE1 和 TALB。三者都没有时返回错误。
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
	var tags Tags
	var sawTitle bool
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
		if id == "TIT2" || id == "TPE1" || id == "TALB" {
			if packedFrame(body[9], version) {
				return Tags{}, fmt.Errorf("compressed or encrypted %s is not supported", id)
			}
			if len(payload) == 0 {
				return Tags{}, errors.New("empty " + id)
			}
			text := decodeText(payload[0], payload[1:])
			switch id {
			case "TIT2":
				if !sawTitle {
					tags.Title = text
					sawTitle = true
				}
			case "TPE1":
				if tags.Artist == "" {
					tags.Artist = text
				}
			case "TALB":
				if tags.Album == "" {
					tags.Album = text
				}
			}
		}
		body = body[10+frameSize:]
	}
	if !sawTitle && tags.Artist == "" && tags.Album == "" {
		return Tags{}, errors.New("TIT2 frame not found")
	}
	return tags, nil
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

// Open 只读取文件开头的 ID3v2 标签区并解析文本标签，音频再大也不整体读入。取消时返回 ctx 的错误。
func Open(ctx context.Context, path string) (Tags, error) {
	if err := ctx.Err(); err != nil {
		return Tags{}, err
	}
	f, err := os.Open(path)
	if err != nil {
		return Tags{}, err
	}
	defer f.Close()
	header := make([]byte, 10)
	if _, err = io.ReadFull(f, header); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return Tags{}, errors.New("not an id3v2 header")
		}
		return Tags{}, err
	}
	if string(header[0:3]) != "ID3" {
		return Tags{}, errors.New("not an id3v2 header")
	}
	size := synchsafe(header[6:10])
	if size > maxTagBytes {
		return Tags{}, fmt.Errorf("id3 tag in %s exceeds 16 MiB", path)
	}
	data := make([]byte, 10+size)
	copy(data, header)
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
	if len(b) > 0 && b[len(b)-1] == 0 {
		return b[:len(b)-1]
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
