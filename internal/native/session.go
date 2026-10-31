package native

import (
	"context"
	"fmt"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui/editor"
	"gocompare/internal/compare"
	"gocompare/internal/document"
	"gocompare/internal/workspace"
	"path/filepath"
	"strings"
)

func (a *workspaceState) canSaveSession() bool {
	if a.kind == 2 {
		return strings.TrimSpace(a.paths[0]) != "" && strings.TrimSpace(a.paths[1]) != ""
	}
	return a.panes[0] != nil || a.directoryRows != nil
}

func (a *workspaceState) saveSession(path string) {
	if a.busy {
		return
	}
	s := workspace.Session{Version: 1, Wrap: a.wrap}
	switch {
	case a.kind == 2 && strings.TrimSpace(a.paths[0]) != "" && strings.TrimSpace(a.paths[1]) != "":
		s.Kind = "merge"
		s.Left, s.Right = a.paths[0], a.paths[1]
		s.Base, s.Output = a.mergeBase, a.mergeOutput
	case a.directoryRows != nil:
		s.Kind = "directory"
		s.Left = a.directoryRoots[0]
		s.Right = a.directoryRoots[1]
	case a.panes[0] != nil:
		s.Left = a.panes[0].file.Path
		s.Right = a.panes[1].file.Path
	default:
		return
	}
	a.job(func(ctx context.Context) {
		err := workspace.Write(path, s)
		a.publish(func() {
			a.busy = false
			if err != nil {
				a.err = err.Error()
				return
			}
			a.message = "会话已保存（未保存内容仍需单独保存）"
		})
	})
}
func (a *workspaceState) openSession(path string) {
	if a.busy {
		return
	}
	a.job(func(ctx context.Context) {
		s, err := workspace.Read(path)
		var left, right *document.File
		if err == nil && s.Kind == "merge" {
			a.publish(func() {
				a.busy = false
				a.kind = 2
				a.paths = [2]string{s.Left, s.Right}
				a.mergeBase = s.Base
				a.mergeOutput = s.Output
				a.wrap = s.Wrap
				a.loadMerge()
			})
			return
		}
		if err == nil && s.Kind == "directory" {
			var rows []compare.Entry
			rows, err = compare.Directories(ctx, s.Left, s.Right)
			a.publish(func() {
				a.busy = false
				if err != nil {
					a.err = err.Error()
					return
				}
				a.installDirectory([2]string{s.Left, s.Right}, rows)
			})
			return
		}
		if err == nil {
			left, err = document.Read(ctx, s.Left)
		}
		if err == nil {
			right, err = document.Read(ctx, s.Right)
		}
		a.publish(func() {
			a.busy = false
			if err != nil {
				a.err = err.Error()
				return
			}
			a.install(left, right)
			a.wrap = s.Wrap
		})
	})
}
func (a *workspaceState) sessionDialog(save bool) {
	if a.busy {
		return
	}
	if save {
		path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: a.win, Title: "保存比较会话", DefaultPath: "comparison.gocompare", Filters: []mygo.FileFilter{{Name: "GoCompare 会话", Extensions: []string{"gocompare"}}}})
		if err != nil {
			a.err = err.Error()
		} else if path != "" {
			a.saveSession(path)
		}
		return
	}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: "打开比较会话", Filters: []mygo.FileFilter{{Name: "GoCompare 会话", Extensions: []string{"gocompare"}}}})
	if err != nil {
		a.err = err.Error()
	} else if len(paths) > 0 {
		a.guard(func() { a.openSession(paths[0]) })
	}
}
func (a *workspaceState) saveAs(side int) {
	if a.busy || a.panes[side] == nil {
		return
	}
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: a.win, Title: "另存为（选择新文件名）", DefaultPath: a.panes[side].file.Path})
	if err != nil {
		a.err = err.Error()
		return
	}
	if path != "" {
		a.saveAsPath(side, path)
	}
}
func (a *workspaceState) saveAsPath(side int, path string) {
	if a.busy || a.panes[side] == nil {
		return
	}
	p := a.panes[side]
	file, text, version := p.file, p.buffer.String(), p.buffer.Version()
	a.job(func(ctx context.Context) {
		f, err := file.SaveAs(ctx, path, text)
		a.publish(func() {
			a.busy = false
			if err != nil {
				a.err = err.Error()
				return
			}
			p.file = f
			p.edit.Label = filepath.Base(f.Path)
			p.buffer.SetLanguage(editor.LanguageFor(f.Path))
			a.paths[side] = f.Path
			if p.buffer.Version() == version {
				p.buffer.MarkSaved()
			}
			a.message = fmt.Sprintf("已另存为 %s", f.Path)
			a.updateTitle()
		})
	})
}
func (a *workspaceState) updateTitle() {
	if a.win != nil && a.panes[0] != nil {
		a.win.SetTitle(filepath.Base(a.panes[0].file.Path) + " ↔ " + filepath.Base(a.panes[1].file.Path) + " — GoCompare")
	}
}
