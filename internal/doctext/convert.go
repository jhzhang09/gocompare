// Package doctext 把 Word、PDF 和 RTF 的已声明子集转成纯文本。转换不可逆。
package doctext

import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"io"
	"regexp"
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

// PDF 抽取未压缩及 FlateDecode 压缩内容流里的文本。
// 只有 Tj、TJ、'、" 这些显示文本操作符的字符串操作数才算文本；
// 字体程序、图片、XRef 等非内容流直接跳过，避免二进制数据被当成文本。
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
		// 只看当前对象的字典，前面其他对象里的 /Subtype 等不能影响本流。
		if i := bytes.LastIndex(header, []byte("obj")); i >= 0 {
			header = header[i:]
		}
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
		if !contentStream(header) {
			continue
		}
		streamData := body
		if bytes.Contains(header, []byte("/Filter")) {
			// 只支持单一 FlateDecode；其他过滤器或解压失败的流跳过。
			if !bytes.Contains(header, []byte("/FlateDecode")) {
				continue
			}
			zr, err := zlib.NewReader(bytes.NewReader(body))
			if err != nil {
				continue
			}
			decomp, err := io.ReadAll(io.LimitReader(zr, 32<<20))
			_ = zr.Close()
			if err != nil {
				continue
			}
			streamData = decomp
		}
		b.WriteString(contentText(streamData))
	}
	if b.Len() == 0 {
		return "", errors.New("no pdf text")
	}
	return b.String(), nil
}

// nonContentStream 匹配已知不含页面文本的流：嵌入字体、图片、XRef、对象流、元数据和附件。
var nonContentStream = regexp.MustCompile(`/Length[123][\s/>]|/Subtype\s*/(Image|Type1C|CIDFontType0C|OpenType|XML)\b|/Type\s*/(XRef|ObjStm|Metadata|EmbeddedFile)\b`)

func contentStream(header []byte) bool {
	return !nonContentStream.Match(header)
}

// contentText 词法扫描内容流，把文本操作符之前的字符串操作数拼接输出。
func contentText(body []byte) string {
	var out strings.Builder
	var pending [][]byte
	isSpace := func(c byte) bool {
		return c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f' || c == 0
	}
	isDelim := func(c byte) bool { return strings.IndexByte("()<>[]{}/%", c) >= 0 }
	for i := 0; i < len(body); {
		c := body[i]
		switch {
		case isSpace(c):
			i++
		case c == '%':
			for i < len(body) && body[i] != '\n' && body[i] != '\r' {
				i++
			}
		case c == '(':
			var str []byte
			str, i = literalString(body, i+1)
			pending = append(pending, str)
		case c == '<' && i+1 < len(body) && body[i+1] == '<', c == '>' && i+1 < len(body) && body[i+1] == '>':
			i += 2
		case c == '<':
			j := bytes.IndexByte(body[i+1:], '>')
			if j < 0 {
				return out.String()
			}
			hexStr := string(bytes.Join(bytes.Fields(body[i+1:i+1+j]), nil))
			if len(hexStr)%2 != 0 {
				hexStr += "0"
			}
			if decoded, err := hex.DecodeString(hexStr); err == nil {
				pending = append(pending, decoded)
			}
			i += j + 2
		case c == '[' || c == ']' || c == '{' || c == '}' || c == '>' || c == ')':
			i++
		case c == '/':
			i++
			for i < len(body) && !isSpace(body[i]) && !isDelim(body[i]) {
				i++
			}
		default:
			j := i
			for j < len(body) && !isSpace(body[j]) && !isDelim(body[j]) {
				j++
			}
			tok := string(body[i:j])
			i = j
			if tok == "" || strings.ContainsAny(tok[:1], "+-.0123456789") {
				continue // 数字是 TJ 数组里的字距，保留待输出字符串
			}
			if tok == "Tj" || tok == "TJ" || tok == "'" || tok == "\"" {
				for _, p := range pending {
					out.WriteString(pdfString(p))
				}
			}
			pending = pending[:0]
		}
	}
	return out.String()
}

// literalString 解析圆括号字符串（支持嵌套括号与转义），返回内容和右括号之后的位置。
func literalString(body []byte, i int) ([]byte, int) {
	var out []byte
	depth := 1
	for i < len(body) {
		c := body[i]
		switch c {
		case '\\':
			i++
			if i >= len(body) {
				return out, i
			}
			e := body[i]
			switch e {
			case 'n':
				out = append(out, '\n')
			case 'r':
				out = append(out, '\r')
			case 't':
				out = append(out, '\t')
			case 'b', 'f':
				// 退格与换页没有可显示内容，忽略，避免整串被当成二进制丢弃。
			case '\r':
				if i+1 < len(body) && body[i+1] == '\n' {
					i++
				}
			case '\n':
				// 行尾续行，不输出字符。
			default:
				if e >= '0' && e <= '7' {
					v, n := 0, 0
					for n < 3 && i < len(body) && body[i] >= '0' && body[i] <= '7' {
						v = v*8 + int(body[i]-'0')
						i++
						n++
					}
					out = append(out, byte(v))
					continue
				}
				out = append(out, e)
			}
			i++
		case '(':
			depth++
			out = append(out, c)
			i++
		case ')':
			depth--
			i++
			if depth == 0 {
				return out, i
			}
			out = append(out, c)
		default:
			out = append(out, c)
			i++
		}
	}
	return out, i
}

// pdfString 把字符串字节转成 UTF-8。合法 UTF-8 原样保留，否则按 Latin-1 解释；
// 含控制字符的通常是 CID 字体的字形编号而不是文本，直接丢弃。
func pdfString(p []byte) string {
	for _, c := range p {
		if c < 0x20 && c != '\t' && c != '\n' && c != '\r' {
			return ""
		}
	}
	if utf8.Valid(p) {
		return string(p)
	}
	r := make([]rune, len(p))
	for i, c := range p {
		r[i] = rune(c)
	}
	return string(r)
}

// Convert 根据文件名后缀或内容特征自动转换文档为纯文本。
func Convert(path string, data []byte) (string, error) {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".docx") {
		return DOCX(data)
	}
	if strings.HasSuffix(lower, ".rtf") {
		return RTF(data)
	}
	if strings.HasSuffix(lower, ".pdf") {
		return PDF(data)
	}
	if bytes.HasPrefix(data, []byte("%PDF-")) {
		return PDF(data)
	}
	if bytes.HasPrefix(data, []byte("{\\rtf")) {
		return RTF(data)
	}
	if bytes.HasPrefix(data, []byte("PK\x03\x04")) {
		text, err := DOCX(data)
		if err == nil {
			return text, nil
		}
	}
	return "", errors.New("unsupported document format for conversion")
}

// IsRTF 按后缀或 {\rtf 文件头判断是否为 RTF。
func IsRTF(path string, data []byte) bool {
	return strings.HasSuffix(strings.ToLower(path), ".rtf") || bytes.HasPrefix(data, []byte("{\\rtf"))
}

// Supports 返回该文件是否支持文档文本转换。
func Supports(path string, data []byte) bool {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".docx") || strings.HasSuffix(lower, ".rtf") || strings.HasSuffix(lower, ".pdf") {
		return true
	}
	if bytes.HasPrefix(data, []byte("%PDF-")) || bytes.HasPrefix(data, []byte("{\\rtf")) {
		return true
	}
	return false
}
