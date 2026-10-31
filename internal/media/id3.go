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

// maxBytes 与文档读取上限一致，避免把整个容器读进内存。
const maxBytes = 16 << 20

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

// ReadID3v2 读取 ID3v2 中的 TIT2、TPE1 和 TALB。三者都没有时返回错误。
// 不处理扩展头和不同步；调用方应提供普通连续帧。不解码音频。
func ReadID3v2(data []byte) (Tags, error) {
	if len(data) < 10 || string(data[0:3]) != "ID3" {
		return Tags{}, errors.New("not an id3v2 header")
	}
	size := synchsafe(data[6:10])
	if size < 0 || 10+size > len(data) {
		return Tags{}, errors.New("id3 size exceeds buffer")
	}
	var tags Tags
	var sawTitle bool
	body := data[10 : 10+size]
	for len(body) >= 10 {
		if body[0] == 0 {
			break
		}
		id := string(body[0:4])
		frameSize := int(binary.BigEndian.Uint32(body[4:8]))
		if frameSize < 0 || 10+frameSize > len(body) {
			return Tags{}, errors.New("truncated id3 frame")
		}
		payload := body[10 : 10+frameSize]
		if id == "TIT2" || id == "TPE1" || id == "TALB" {
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

// Open 按 16 MiB 上限读取路径并解析 ID3v2 文本标签。取消时返回 ctx 的错误。
func Open(ctx context.Context, path string) (Tags, error) {
	if err := ctx.Err(); err != nil {
		return Tags{}, err
	}
	f, err := os.Open(path)
	if err != nil {
		return Tags{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil {
		return Tags{}, err
	}
	if len(data) > maxBytes {
		return Tags{}, fmt.Errorf("%s exceeds 16 MiB", path)
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

func decodeText(enc byte, payload []byte) string {
	switch enc {
	case 0, 3:
		return string(trimZero(payload))
	case 1:
		if len(payload) >= 2 && payload[0] == 0xff && payload[1] == 0xfe {
			payload = payload[2:]
		}
		return utf16le(payload)
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

func utf16le(b []byte) string {
	if len(b)%2 == 1 {
		b = b[:len(b)-1]
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(b[i*2:])
	}
	for len(u) > 0 && u[len(u)-1] == 0 {
		u = u[:len(u)-1]
	}
	return string(utf16.Decode(u))
}
