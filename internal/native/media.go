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
	auth := a.takeURLPasswords()
	a.job(func(ctx context.Context) {
		leftPath, leftClean, err := openInputTemp(ctx, paths[0], 128<<20, auth)
		defer leftClean()
		var left media.Tags
		if err == nil {
			left, err = media.Open(ctx, leftPath)
		}
		var right media.Tags
		if err == nil {
			var rightPath string
			var rightClean func()
			rightPath, rightClean, err = openInputTemp(ctx, paths[1], 128<<20, auth)
			defer rightClean()
			if err == nil {
				right, err = media.Open(ctx, rightPath)
			}
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
				if !a.handleRemoteError(err, a.load) {
					a.err = err.Error()
				}
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
		if a.mediaReady {
			diffs := mediaDiffCount(a.mediaLeft, a.mediaRight)
			if diffs == 0 {
				statusPill(c, i18n.T("dir.equal"), pal.sameFg, pal.sameBg)
			} else {
				statusPill(c, i18n.TArgs("table.diffCount", diffs), pal.diffFg, pal.diffBg)
			}
		}
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
					if a.mediaLeft.Format != "" {
						statusPill(c, a.mediaLeft.Format, pal.baseAccent, pal.baseBg)
					}
				})
				a.mediaFieldRow(c, pal, i18n.TArgs("media.leftTitle", a.mediaLeft.Title), a.mediaLeft.Title != a.mediaRight.Title)
				a.mediaFieldRow(c, pal, i18n.TArgs("media.leftArtist", a.mediaLeft.Artist), a.mediaLeft.Artist != a.mediaRight.Artist)
				a.mediaFieldRow(c, pal, i18n.TArgs("media.leftAlbum", a.mediaLeft.Album), a.mediaLeft.Album != a.mediaRight.Album)
				a.mediaExtraRows(c, pal, a.mediaLeft, a.mediaRight)
			})
			// 右侧卡片
			ui.Column(c).Grow(1).Basis(0).Padding(14, 16).Gap(8).Radius(8).Background(pal.cardBg).Border(1, pal.cardBorder).Children(func() {
				ui.Row(c).Gap(6).Margin(0, 0, 4).Children(func() {
					sideBadge(c, 1, pal)
					ui.Text(c, i18n.T("media.rightTags")).Bold().FontSize(12)
					if a.mediaRight.Format != "" {
						statusPill(c, a.mediaRight.Format, pal.baseAccent, pal.baseBg)
					}
				})
				a.mediaFieldRow(c, pal, i18n.TArgs("media.rightTitle", a.mediaRight.Title), a.mediaLeft.Title != a.mediaRight.Title)
				a.mediaFieldRow(c, pal, i18n.TArgs("media.rightArtist", a.mediaRight.Artist), a.mediaLeft.Artist != a.mediaRight.Artist)
				a.mediaFieldRow(c, pal, i18n.TArgs("media.rightAlbum", a.mediaRight.Album), a.mediaLeft.Album != a.mediaRight.Album)
				a.mediaExtraRows(c, pal, a.mediaRight, a.mediaLeft)
			})
		})
	})
}

func mediaDiffCount(l, r media.Tags) int {
	diffs := 0
	for _, pair := range [][2]string{
		{l.Title, r.Title},
		{l.Artist, r.Artist},
		{l.Album, r.Album},
		{l.Year, r.Year},
		{l.Track, r.Track},
		{l.Genre, r.Genre},
		{l.Comment, r.Comment},
	} {
		if pair[0] != pair[1] {
			diffs++
		}
	}
	if l.HasPic != r.HasPic || l.PicSize != r.PicSize {
		diffs++
	}
	return diffs
}

func (a *workspaceState) mediaExtraRows(c *ui.Context, pal uiPalette, self, other media.Tags) {
	if self.Year != "" || other.Year != "" {
		a.mediaFieldRow(c, pal, i18n.TArgs("media.year", self.Year), self.Year != other.Year)
	}
	if self.Track != "" || other.Track != "" {
		a.mediaFieldRow(c, pal, i18n.TArgs("media.track", self.Track), self.Track != other.Track)
	}
	if self.Genre != "" || other.Genre != "" {
		a.mediaFieldRow(c, pal, i18n.TArgs("media.genre", self.Genre), self.Genre != other.Genre)
	}
	if self.Comment != "" || other.Comment != "" {
		a.mediaFieldRow(c, pal, i18n.TArgs("media.comment", self.Comment), self.Comment != other.Comment)
	}
	if self.HasPic || other.HasPic {
		coverText := i18n.T("media.noCover")
		if self.HasPic {
			coverText = i18n.TArgs("media.cover", self.PicSize)
		}
		a.mediaFieldRow(c, pal, coverText, self.HasPic != other.HasPic || self.PicSize != other.PicSize)
	}
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
