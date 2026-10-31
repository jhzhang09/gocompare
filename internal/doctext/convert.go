// Package doctext 把 Word、PDF 和 RTF 的已声明子集转成纯文本。转换不可逆。
package doctext

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

// DOCX 读取 word/document.xml 里的 w:t 文本，段落之间用换行分隔。
func DOCX(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	var file *zip.File
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			file = f
			break
		}
	}
	if file == nil {
		return "", errors.New("word/document.xml not found")
	}
	r, err := file.Open()
	if err != nil {
		return "", err
	}
	defer r.Close()
	body, err := io.ReadAll(io.LimitReader(r, 32<<20))
	if err != nil {
		return "", err
	}
	dec := xml.NewDecoder(bytes.NewReader(body))
	var b strings.Builder
	inT := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		switch el := tok.(type) {
		case xml.StartElement:
			if el.Name.Local == "t" {
				inT = true
			}
			if el.Name.Local == "p" && b.Len() > 0 {
				b.WriteByte('\n')
			}
		case xml.EndElement:
			if el.Name.Local == "t" {
				inT = false
			}
		case xml.CharData:
			if inT {
				b.Write(el)
			}
		}
	}
	return b.String(), nil
}

// RTF 去掉控制字和分组符号，保留可见文本。\'hh 按字节再解释为 Latin-1。
func RTF(data []byte) (string, error) {
	var b strings.Builder
	for i := 0; i < len(data); {
		switch data[i] {
		case '{', '}':
			i++
		case '\\':
			if i+1 < len(data) && data[i+1] == '\'' && i+3 < len(data) {
				n, err := strconv.ParseUint(string(data[i+2:i+4]), 16, 8)
				if err != nil {
					return "", err
				}
				b.WriteRune(rune(n))
				i += 4
				continue
			}
			i++
			for i < len(data) && ((data[i] >= 'a' && data[i] <= 'z') || (data[i] >= 'A' && data[i] <= 'Z')) {
				i++
			}
			for i < len(data) && (data[i] == '-' || (data[i] >= '0' && data[i] <= '9')) {
				i++
			}
			if i < len(data) && data[i] == ' ' {
				if bytes.HasSuffix([]byte(b.String()), []byte("\n")) {
					i++
					continue
				}
				i++
			}
		case '\r', '\n':
			i++
		default:
			r, size := utf8.DecodeRune(data[i:])
			b.WriteRune(r)
			i += size
		}
	}
	return b.String(), nil
}

// PDF 只抽取未压缩流里的圆括号文本。带 /Filter 的流会被跳过。
func PDF(data []byte) (string, error) {
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		return "", errors.New("not a pdf")
	}
	var b strings.Builder
	rest := data
	for {
		start := bytes.Index(rest, []byte("stream"))
		if start < 0 {
			break
		}
		header := rest[:start]
		rest = rest[start+len("stream"):]
		if len(rest) > 0 && rest[0] == '\r' {
			rest = rest[1:]
		}
		if len(rest) > 0 && rest[0] == '\n' {
			rest = rest[1:]
		}
		end := bytes.Index(rest, []byte("endstream"))
		if end < 0 {
			return "", errors.New("truncated pdf stream")
		}
		body := rest[:end]
		rest = rest[end+len("endstream"):]
		if bytes.Contains(header, []byte("/Filter")) {
			continue
		}
		b.WriteString(parenText(body))
	}
	if b.Len() == 0 {
		return "", errors.New("no pdf text")
	}
	return b.String(), nil
}

func parenText(body []byte) string {
	var b strings.Builder
	for i := 0; i < len(body); i++ {
		if body[i] != '(' {
			continue
		}
		i++
		for i < len(body) && body[i] != ')' {
			if body[i] == '\\' && i+1 < len(body) {
				i++
			}
			b.WriteByte(body[i])
			i++
		}
	}
	return b.String()
}
