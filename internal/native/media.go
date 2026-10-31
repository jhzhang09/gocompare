package native

import (
	"context"

	"github.com/egoist/mygo/ui"
	"gocompare/internal/media"
)

// loadMedia 在后台读取两侧 ID3v2 标签。失败时清空标签，不交给文本比较。
// 这里不解码音频，视图只展示已经读到的文本。
func (a *workspaceState) loadMedia() {
	paths := a.paths
	a.mediaGen++
	gen := a.mediaGen
	a.job(func(ctx context.Context) {
		left, err := media.Open(ctx, paths[0])
		var right media.Tags
		if err == nil {
			right, err = media.Open(ctx, paths[1])
		}
		if err == nil && ctx.Err() != nil {
			err = ctx.Err()
		}
		a.publish(func() {
			if gen != a.mediaGen {
				return
			}
			a.busy = false
			if err != nil {
				a.err = err.Error()
				a.clearMedia()
				return
			}
			a.mediaLeft = left
			a.mediaRight = right
			a.mediaReady = true
			a.err = ""
			a.message = "已读取媒体标签"
		})
	})
}

func (a *workspaceState) clearMedia() {
	a.mediaLeft = media.Tags{}
	a.mediaRight = media.Tags{}
	a.mediaReady = false
}

// mediaView 只显示已加载的标签。没有播放控件，时间推进不会改这些文本。
func (a *workspaceState) mediaView(c *ui.Context) {
	ui.Text(c, "不能播放").Padding(8, 20)
	if !a.mediaReady {
		if a.err == "" {
			ui.Text(c, "选择两个含 ID3v2 标签的文件").Padding(8, 20)
		}
		return
	}
	ui.Text(c, "左侧标题 "+a.mediaLeft.Title).Padding(4, 20)
	ui.Text(c, "左侧艺术家 "+a.mediaLeft.Artist).Padding(4, 20)
	ui.Text(c, "左侧专辑 "+a.mediaLeft.Album).Padding(4, 20)
	ui.Text(c, "右侧标题 "+a.mediaRight.Title).Padding(4, 20)
	ui.Text(c, "右侧艺术家 "+a.mediaRight.Artist).Padding(4, 20)
	ui.Text(c, "右侧专辑 "+a.mediaRight.Album).Padding(4, 20)
}
