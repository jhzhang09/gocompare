package document

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// detectEncoding 识别 BOM、合法 UTF-8，或能按 GBK 原样往返的字节。
// Latin-1 与二进制没有可靠标记，返回空字符串，调用方必须拒绝。
func detectEncoding(data []byte) string {
	switch {
	case bytes.HasPrefix(data, []byte{0xff, 0xfe}):
		return "utf-16le"
	case bytes.HasPrefix(data, []byte{0xfe, 0xff}):
		return "utf-16be"
	}
	if utf8.Valid(data) {
		return "utf-8"
	}
	decoded, _, err := transform.Bytes(simplifiedchinese.GBK.NewDecoder(), data)
	if err != nil || bytes.IndexByte(decoded, 0) >= 0 {
		return ""
	}
	back, _, err := transform.Bytes(simplifiedchinese.GBK.NewEncoder(), decoded)
	if err != nil || !bytes.Equal(back, data) {
		return ""
	}
	return "gbk"
}

func decodeBytes(data []byte, encoding string) (string, bool, bool, error) {
	var bom bool
	switch encoding {
	case "utf-8":
		if bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
			bom = true
			data = data[3:]
		}
		if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
			return "", false, false, errors.New("unsupported encoding or binary file; UTF-8 required")
		}
		text, crlf, err := normalizeNewlines(string(data))
		return text, bom, crlf, err
	case "utf-16le", "utf-16be":
		if encoding == "utf-16le" && bytes.HasPrefix(data, []byte{0xff, 0xfe}) {
			bom = true
			data = data[2:]
		}
		if encoding == "utf-16be" && bytes.HasPrefix(data, []byte{0xfe, 0xff}) {
			bom = true
			data = data[2:]
		}
		if len(data)%2 != 0 {
			return "", false, false, errors.New("odd utf-16 length")
		}
		units := make([]uint16, len(data)/2)
		for i := range units {
			if encoding == "utf-16le" {
				units[i] = binary.LittleEndian.Uint16(data[i*2:])
			} else {
				units[i] = binary.BigEndian.Uint16(data[i*2:])
			}
		}
		text, crlf, err := normalizeNewlines(string(utf16.Decode(units)))
		if err != nil {
			return "", false, false, err
		}
		if strings.ContainsRune(text, 0) {
			return "", false, false, errors.New("unsupported encoding or binary file")
		}
		return text, bom, crlf, nil
	case "gbk":
		decoded, _, err := transform.Bytes(simplifiedchinese.GBK.NewDecoder(), data)
		if err != nil {
			return "", false, false, fmt.Errorf("gbk: %w", err)
		}
		if bytes.IndexByte(decoded, 0) >= 0 {
			return "", false, false, errors.New("unsupported encoding or binary file")
		}
		text, crlf, err := normalizeNewlines(string(decoded))
		return text, false, crlf, err
	case "latin1":
		runes := make([]rune, len(data))
		for i, b := range data {
			runes[i] = rune(b)
		}
		text, crlf, err := normalizeNewlines(string(runes))
		if err != nil {
			return "", false, false, err
		}
		if strings.ContainsRune(text, 0) {
			return "", false, false, errors.New("unsupported encoding or binary file")
		}
		return text, false, crlf, nil
	default:
		return "", false, false, fmt.Errorf("unsupported encoding %q", encoding)
	}
}

func normalizeNewlines(text string) (string, bool, error) {
	crlf := strings.Contains(text, "\r\n")
	norm := strings.ReplaceAll(text, "\r\n", "\n")
	if strings.ContainsRune(norm, '\r') || (crlf && strings.Count(text, "\n") != strings.Count(text, "\r\n")) {
		return "", false, errors.New("mixed or unsupported line endings")
	}
	return norm, crlf, nil
}

func encodeText(text, encoding string, bom bool) ([]byte, error) {
	switch encoding {
	case "", "utf-8":
		data := []byte(text)
		if bom {
			data = append([]byte{0xef, 0xbb, 0xbf}, data...)
		}
		return data, nil
	case "utf-16le", "utf-16be":
		return encodeUTF16(text, encoding, bom)
	case "gbk":
		data, _, err := transform.Bytes(simplifiedchinese.GBK.NewEncoder(), []byte(text))
		if err != nil {
			return nil, fmt.Errorf("gbk: %w", err)
		}
		return data, nil
	case "latin1":
		out := make([]byte, 0, len(text))
		for _, r := range text {
			if r > 0xff {
				return nil, fmt.Errorf("rune %q is not latin1", r)
			}
			out = append(out, byte(r))
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported encoding %q", encoding)
	}
}

func encodeUTF16(text, encoding string, bom bool) ([]byte, error) {
	units := utf16.Encode([]rune(text))
	out := make([]byte, 0, len(units)*2+2)
	if bom {
		if encoding == "utf-16le" {
			out = append(out, 0xff, 0xfe)
		} else {
			out = append(out, 0xfe, 0xff)
		}
	}
	for _, unit := range units {
		var buf [2]byte
		if encoding == "utf-16le" {
			binary.LittleEndian.PutUint16(buf[:], unit)
		} else {
			binary.BigEndian.PutUint16(buf[:], unit)
		}
		out = append(out, buf[:]...)
	}
	return out, nil
}
