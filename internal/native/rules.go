package native

// compareIgnoringSpace 将规则作为会话状态保存，并重算真实高亮。
func (a *workspaceState) compareIgnoringSpace() {
	if a.panes[0] == nil {
		a.err = "先打开两个文本文件"
		return
	}
	a.ignoreSpace = !a.ignoreSpace
	a.recompute()
}
