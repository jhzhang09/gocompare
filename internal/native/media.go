package native

import (
	"context"

	"github.com/egoist/mygo/ui"
	"gocompare/internal/i18n"
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
			a.message = i18n.T("media.loaded")
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
	t := c.Theme()
	pal := paletteFor(t)

	ui.Row(c).Height(36).Padding(0, 16).Gap(8).Background(pal.cardBg).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
		ui.Icon(c, iconMediaTag).Size(13, 13).TextColor(pal.baseAccent)
		ui.Text(c, i18n.T("media.header")).FontSize(11).Bold()
		statusPill(c, i18n.T("media.noPlayback"), pal.sameFg, pal.sameBg)
	})

	if !a.mediaReady {
		if a.err == "" {
			a.modeEmptyView(c, pal, 4)
		}
		return
	}

	ui.Column(c).Grow(1).FillWidth().Padding(14, 16).Gap(12).Children(func() {
		ui.Row(c).FillWidth().Gap(12).Children(func() {
			// 左侧卡片
			ui.Column(c).Grow(1).Basis(0).Padding(14, 16).Gap(8).Radius(8).Background(pal.cardBg).Border(1, pal.cardBorder).Children(func() {
				ui.Row(c).Gap(6).Margin(0, 0, 4).Children(func() {
					sideBadge(c, 0, pal)
					ui.Text(c, i18n.T("media.leftTags")).Bold().FontSize(12)
				})
				a.mediaFieldRow(c, pal, i18n.TArgs("media.leftTitle", a.mediaLeft.Title), a.mediaLeft.Title != a.mediaRight.Title)
				a.mediaFieldRow(c, pal, i18n.TArgs("media.leftArtist", a.mediaLeft.Artist), a.mediaLeft.Artist != a.mediaRight.Artist)
				a.mediaFieldRow(c, pal, i18n.TArgs("media.leftAlbum", a.mediaLeft.Album), a.mediaLeft.Album != a.mediaRight.Album)
			})
			// 右侧卡片
			ui.Column(c).Grow(1).Basis(0).Padding(14, 16).Gap(8).Radius(8).Background(pal.cardBg).Border(1, pal.cardBorder).Children(func() {
				ui.Row(c).Gap(6).Margin(0, 0, 4).Children(func() {
					sideBadge(c, 1, pal)
					ui.Text(c, i18n.T("media.rightTags")).Bold().FontSize(12)
				})
				a.mediaFieldRow(c, pal, i18n.TArgs("media.rightTitle", a.mediaRight.Title), a.mediaLeft.Title != a.mediaRight.Title)
				a.mediaFieldRow(c, pal, i18n.TArgs("media.rightArtist", a.mediaRight.Artist), a.mediaLeft.Artist != a.mediaRight.Artist)
				a.mediaFieldRow(c, pal, i18n.TArgs("media.rightAlbum", a.mediaRight.Album), a.mediaLeft.Album != a.mediaRight.Album)
			})
		})
	})
}

func (a *workspaceState) mediaFieldRow(c *ui.Context, pal uiPalette, text string, diff bool) {
	bg := pal.subtleSurface
	if diff {
		bg = pal.diffBg
	}
	ui.Row(c).FillWidth().Padding(6, 10).Gap(6).Radius(5).Background(bg).Children(func() {
		ui.Text(c, text).Grow(1).FontSize(12)
		if diff {
			statusPill(c, i18n.T("dir.different"), pal.diffFg, pal.cardBg)
		}
	})
}
