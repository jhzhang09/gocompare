// Package htmlview 把 HTML 解析成文本块。展示不依赖 WebView；报告只是另存的 HTML 文件。
package htmlview

import (
	"fmt"
	"html"
	"io"
	"strings"
)

// Block 是一个可在原生界面逐行显示的块。
type Block struct {
	Tag  string
	Text string
}

// Blocks 按文档顺序提取 h1、p、li 和 td 的文本，并解码字符实体。不执行脚本，也不加载外部资源。
// 标签名必须完整匹配，<pre>、<link> 之类不会被当成 <p>、<li>。
func Blocks(source string) []Block {
	var out []Block
	lower := strings.ToLower(source)
	for from := 0; ; {
		rel := strings.IndexByte(lower[from:], '<')
		if rel < 0 {
			break
		}
		start := from + rel
		from = start + 1
		tag := blockTag(lower[start+1:])
		if tag == "" {
			continue
		}
		gt := strings.IndexByte(source[start:], '>')
		if gt < 0 {
			break
		}
		content := start + gt + 1
		close := "</" + tag + ">"
		endRel := strings.Index(lower[content:], close)
		if endRel < 0 {
			break
		}
		text := strings.TrimSpace(html.UnescapeString(stripTags(source[content : content+endRel])))
		if text != "" {
			out = append(out, Block{Tag: tag, Text: text})
		}
		from = content + endRel + len(close)
	}
	return out
}

// blockTag 返回紧跟在 < 之后的块级标签名；名字后必须是 >、空白或 /。
func blockTag(rest string) string {
	for _, tag := range []string{"h1", "p", "li", "td"} {
		if !strings.HasPrefix(rest, tag) || len(rest) == len(tag) {
			continue
		}
		switch rest[len(tag)] {
		case '>', ' ', '\t', '\n', '\r', '/':
			return tag
		}
	}
	return ""
}

// WriteReport 把块写成独立的 HTML 文件。调用方保存文件，界面仍使用 Blocks。
func WriteReport(w io.Writer, title string, blocks []Block) error {
	if _, err := fmt.Fprintf(w, "<!DOCTYPE html><html><head><meta charset=\"utf-8\"><title>%s</title></head><body>\n", html.EscapeString(title)); err != nil {
		return err
	}
	for _, block := range blocks {
		if _, err := fmt.Fprintf(w, "<%s>%s</%s>\n", block.Tag, html.EscapeString(block.Text), block.Tag); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "</body></html>\n")
	return err
}

func stripTags(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == '<':
			in = true
		case r == '>':
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	return b.String()
}
