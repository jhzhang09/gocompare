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

// Blocks 提取 h1、p、li 和 td 的文本。不执行脚本，也不加载外部资源。
func Blocks(source string) []Block {
	var out []Block
	lower := strings.ToLower(source)
	for _, tag := range []string{"h1", "p", "li", "td"} {
		from := 0
		open := "<" + tag
		close := "</" + tag + ">"
		for {
			rel := strings.Index(lower[from:], open)
			if rel < 0 {
				break
			}
			start := from + rel
			gt := strings.Index(source[start:], ">")
			if gt < 0 {
				break
			}
			content := start + gt + 1
			endRel := strings.Index(lower[content:], close)
			if endRel < 0 {
				break
			}
			text := strings.TrimSpace(stripTags(source[content : content+endRel]))
			if text != "" {
				out = append(out, Block{Tag: tag, Text: text})
			}
			from = content + endRel + len(close)
		}
	}
	return out
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
