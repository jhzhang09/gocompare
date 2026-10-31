// Package paginate 把文本分成固定行数的页。它不调用操作系统打印对话框。
package paginate

import (
	"strings"
	"unicode/utf8"
)

// Pages 按 rune 宽度折行，再按 rows 行分页。width 与 rows 都必须大于 0。
func Pages(text string, width, rows int) []string {
	if width <= 0 || rows <= 0 {
		return nil
	}
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if line == "" {
			lines = append(lines, "")
			continue
		}
		for line != "" {
			n, count := 0, 0
			for n < len(line) && count < width {
				_, size := utf8.DecodeRuneInString(line[n:])
				n += size
				count++
			}
			lines = append(lines, line[:n])
			line = line[n:]
		}
	}
	if len(lines) == 0 {
		return []string{""}
	}
	var pages []string
	for i := 0; i < len(lines); i += rows {
		j := i + rows
		if j > len(lines) {
			j = len(lines)
		}
		pages = append(pages, strings.Join(lines[i:j], "\n"))
	}
	return pages
}
