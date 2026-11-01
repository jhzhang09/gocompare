package native

import (
	"context"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui/editor"
	"gocompare/internal/compare"
	"gocompare/internal/document"
	"gocompare/internal/i18n"
	"gocompare/internal/workspace"
	"path/filepath"
	"strings"
)

func (a *workspaceState) canSaveSession() bool {
	if a.kind == kindMerge {
		return strings.TrimSpace(a.paths[0]) != "" && strings.TrimSpace(a.paths[1]) != ""
	}
	return a.panes[0] != nil || a.directoryRows != nil
}

func (a *workspaceState) saveSession(path string) {
	if a.busy {
		return
	}
	s := workspace.Session{Version: 1, Wrap: a.wrap, IgnoreSpace: a.ignoreSpace}
	switch {
	case a.kind == kindMerge && strings.TrimSpace(a.paths[0]) != "" && strings.TrimSpace(a.paths[1]) != "":
		s.Kind = "merge"
		s.Left, s.Right = a.paths[0], a.paths[1]
		s.Base, s.Output = a.mergeBase, a.mergeOutput
	case a.directoryRows != nil:
		s.Kind = "directory"
		s.DirectoryFilter = a.directoryOptions.Filter
		s.CompareMode = a.directoryOptions.CompareMode
		s.CompareTime = a.directoryOptions.CompareTime
		s.FollowLinks = a.directoryOptions.Links == compare.LinkFollow
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
			a.message = i18n.T("session.saved")
		})
	})
}
func (a *workspaceState) openSession(path string) {
	if a.busy {
		return
	}
	auth := a.takeURLPasswords()
	a.job(func(ctx context.Context) {
		s, err := workspace.Read(path)
		var left, right *document.File
		if err == nil && s.Kind == "merge" {
			a.publish(func() {
				a.busy = false
				a.kind = kindMerge
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
			links := compare.LinkRecord
			if s.FollowLinks {
				links = compare.LinkFollow
			}
			options := compare.ScanOptions{Filter: s.DirectoryFilter, CompareMode: s.CompareMode, CompareTime: s.CompareTime, Links: links, KeepParents: true}
			rows, err = compare.CompareDirectories(ctx, s.Left, s.Right, options)
			a.publish(func() {
				a.busy = false
				if err != nil {
					a.err = err.Error()
					return
				}
				a.installDirectory([2]string{s.Left, s.Right}, rows)
				a.directoryOptions = options
				a.dirFilter = s.DirectoryFilter
				a.compareMode = s.CompareMode
				a.compareTime = s.CompareTime
				a.followLinks = s.FollowLinks
			})
			return
		}
		if err == nil {
			left, err = openInput(ctx, s.Left, false, auth)
		}
		if err == nil {
			right, err = openInput(ctx, s.Right, false, auth)
		}
		a.publish(func() {
			a.busy = false
			if err != nil {
				if !a.handleRemoteError(err, func() { a.openSession(path) }) {
					a.err = err.Error()
				}
				return
			}
			a.ignoreSpace = s.IgnoreSpace
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
		path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: a.win, Title: i18n.T("session.saveTitle"), DefaultPath: "comparison.gocompare", Filters: []mygo.FileFilter{{Name: i18n.T("session.filterName"), Extensions: []string{"gocompare"}}}})
		if err != nil {
			a.err = err.Error()
		} else if path != "" {
			a.saveSession(path)
		}
		return
	}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: i18n.T("session.openTitle"), Filters: []mygo.FileFilter{{Name: i18n.T("session.filterName"), Extensions: []string{"gocompare"}}}})
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
	defaultPath := a.panes[side].file.Path
	if a.panes[side].file.IsRemote() {
		// 远程文件另存为只写本地，默认只给出文件名。
		defaultPath = filepath.Base(defaultPath)
	}
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: a.win, Title: i18n.T("dialog.saveAsTitle"), DefaultPath: defaultPath})
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
			p.edit.Label = i18n.T([]string{"a11y.leftEditor", "a11y.rightEditor"}[side])
			p.buffer.SetLanguage(editor.LanguageFor(f.Path))
			a.paths[side] = f.Path
			if p.buffer.Version() == version {
				p.buffer.MarkSaved()
			}
			a.message = i18n.TArgs("status.savedAs", f.Path)
			a.updateTitle()
		})
	})
}
func (a *workspaceState) updateTitle() {
	if a.win != nil && a.panes[0] != nil {
		a.win.SetTitle(filepath.Base(a.panes[0].file.Path) + " ↔ " + filepath.Base(a.panes[1].file.Path) + " — GoCompare")
	}
}
