package native

import (
	"fmt"

	"gocompare/internal/textcompare"
)

// compareIgnoringSpace 只比较当前缓冲区，不写回磁盘。
func (a *workspaceState) compareIgnoringSpace() {
	if a.panes[0] == nil || a.panes[1] == nil {
		a.err = "先打开两个文本文件"
		return
	}
	result, err := textcompare.Compare(a.panes[0].buffer.String(), a.panes[1].buffer.String(), textcompare.Rules{IgnoreSpace: true}, textcompare.Myers, nil)
	if err != nil {
		a.err = err.Error()
		return
	}
	a.err = ""
	if result.Equal {
		a.message = "按规则相同"
		return
	}
	a.message = fmt.Sprintf("按规则 %d 个差异", len(result.Hunks))
}
