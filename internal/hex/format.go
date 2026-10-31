package hex

import (
	"fmt"
	"strings"
)

// FormatDiff 把差异块写成稳定的文本。CLI 和脚本都使用这一输出。
func FormatDiff(chunks []Chunk) string {
	var b strings.Builder
	for _, c := range chunks {
		fmt.Fprintf(&b, "offset %d\n", c.Offset)
	}
	return b.String()
}
