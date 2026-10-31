package native

import "gocompare/internal/i18n"

// compareIgnoringSpace 将规则作为会话状态保存，并重算真实高亮。
func (a *workspaceState) compareIgnoringSpace() {
	if a.panes[0] == nil {
		a.err = i18n.T("err.openTwoTexts")
		return
	}
	a.ignoreSpace = !a.ignoreSpace
	a.recompute()
}
