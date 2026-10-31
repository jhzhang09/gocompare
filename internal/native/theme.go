package native

import "github.com/egoist/mygo/ui"

// 预解析矢量 SVG 图标（极简设计风格）
var (
	iconTextDiff   = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="#ffffff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="2"/><path d="M9 3v18"/><line x1="13" y1="8" x2="18" y2="8"/><line x1="13" y1="12" x2="18" y2="12"/><line x1="13" y1="16" x2="16" y2="16"/><line x1="5" y1="8" x2="7" y2="8"/><line x1="5" y1="12" x2="7" y2="12"/></svg>`))
	iconFileDiff   = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="#ffffff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="9" y1="13" x2="15" y2="13"/><line x1="9" y1="17" x2="13" y2="17"/></svg>`))
	iconFolderDiff = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="#ffffff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/><line x1="9" y1="14" x2="15" y2="14"/></svg>`))
	iconGitMerge   = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="#ffffff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="18" cy="18" r="3"/><circle cx="6" cy="6" r="3"/><path d="M6 21V9a9 9 0 0 0 9 9"/></svg>`))
	iconImageDiff  = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="#ffffff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="2" ry="2"/><circle cx="8.5" cy="8.5" r="1.5"/><polyline points="21 15 16 10 5 21"/></svg>`))
	iconMediaTag   = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="#ffffff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/></svg>`))
	iconVersionReg = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="#ffffff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><polyline points="9 12 11 14 15 10"/></svg>`))

	iconCompare     = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="#ffffff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="5 3 19 12 5 21 5 3"/></svg>`))
	iconSwap        = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="#ffffff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="17 1 21 5 17 9"/><path d="M3 11V9a4 4 0 0 1 4-4h14"/><polyline points="7 23 3 19 7 15"/><path d="M21 13v2a4 4 0 0 1-4 4H3"/></svg>`))
	iconChevronUp   = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="#ffffff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="18 15 12 9 6 15"/></svg>`))
	iconChevronDown = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="#ffffff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="6 9 12 15 18 9"/></svg>`))
	iconSearch      = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="#ffffff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>`))
	iconSliders     = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="#ffffff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="4" y1="21" x2="4" y2="14"/><line x1="4" y1="10" x2="4" y2="3"/><line x1="12" y1="21" x2="12" y2="12"/><line x1="12" y1="8" x2="12" y2="3"/><line x1="20" y1="21" x2="20" y2="16"/><line x1="20" y1="12" x2="20" y2="3"/><line x1="1" y1="14" x2="7" y2="14"/><line x1="9" y1="8" x2="15" y2="8"/><line x1="17" y1="16" x2="23" y2="16"/></svg>`))
	iconFolderOpen  = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="#ffffff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>`))
	iconSave        = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="#ffffff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z"/><polyline points="17 21 17 13 7 13 7 21"/><polyline points="7 3 7 8 15 8"/></svg>`))
	iconFile        = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="#ffffff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M13 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"/><polyline points="13 2 13 9 20 9"/></svg>`))
	iconCheck       = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="#ffffff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>`))
	iconAlert       = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="#ffffff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>`))
	iconExternal    = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="#ffffff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/><polyline points="15 3 21 3 21 9"/><line x1="10" y1="14" x2="21" y2="3"/></svg>`))
	iconSync        = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="#ffffff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="23 4 23 10 17 10"/><polyline points="1 20 1 14 7 14"/><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/></svg>`))
	iconUndo        = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="#ffffff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="1 4 1 10 7 10"/><path d="M3.51 15a9 9 0 1 0 2.13-9.36L1 10"/></svg>`))
	iconReset       = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="#ffffff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/><polyline points="3 3 3 8 8 8"/></svg>`))
	iconAnchor      = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="#ffffff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="5" r="3"/><line x1="12" y1="22" x2="12" y2="8"/><path d="M5 12H2a10 10 0 0 0 20 0h-3"/></svg>`))
)

var modeIcons = []*ui.SVG{
	iconFileDiff,
	iconFolderDiff,
	iconGitMerge,
	iconImageDiff,
	iconMediaTag,
	iconVersionReg,
}

// uiPalette 定义极简高级调色板
type uiPalette struct {
	// 侧边栏（与系统环境紧密融合）
	sidebarBg         ui.Color
	sidebarBorder     ui.Color
	sidebarItemHover  ui.Color
	sidebarItemActive ui.Color
	sidebarText       ui.Color
	sidebarMuted      ui.Color
	sidebarBtnBg      ui.Color
	sidebarBtnBorder  ui.Color

	// 容器与卡片
	appBg         ui.Color
	headerBg      ui.Color
	subtleSurface ui.Color
	cardBg        ui.Color
	cardBorder    ui.Color

	// 语义高亮色
	leftAccent  ui.Color
	leftBg      ui.Color
	rightAccent ui.Color
	rightBg     ui.Color
	baseAccent  ui.Color
	baseBg      ui.Color

	sameFg    ui.Color
	sameBg    ui.Color
	diffFg    ui.Color
	diffBg    ui.Color
	dangerFg  ui.Color
	dangerBg  ui.Color
	successFg ui.Color
	successBg ui.Color

	pillBg     ui.Color
	pillBorder ui.Color
}

func paletteFor(t *ui.Theme) uiPalette {
	if t.Dark {
		return uiPalette{
			// 暗色模式：采用深灰底与半透明分层，告别单调高饱和死黑
			sidebarBg:         ui.Hex("#18181b"),
			sidebarBorder:     ui.RGBA(255, 255, 255, 0.08),
			sidebarItemHover:  ui.RGBA(255, 255, 255, 0.06),
			sidebarItemActive: ui.Hex("#27272a"),
			sidebarText:       ui.Hex("#fafafa"),
			sidebarMuted:      ui.RGBA(255, 255, 255, 0.48),
			sidebarBtnBg:      ui.Hex("#202023"),
			sidebarBtnBorder:  ui.RGBA(255, 255, 255, 0.10),

			appBg:         ui.Hex("#121214"),
			headerBg:      ui.Hex("#18181b"),
			subtleSurface: ui.Hex("#1c1c1f"),
			cardBg:        ui.Hex("#202024"),
			cardBorder:    ui.RGBA(255, 255, 255, 0.09),

			leftAccent:  ui.Hex("#5ecc71"),
			leftBg:      ui.Hex("#1f3325"),
			rightAccent: ui.Hex("#c084fc"),
			rightBg:     ui.Hex("#2d1f3d"),
			baseAccent:  ui.Hex("#60a5fa"),
			baseBg:      ui.Hex("#172554"),

			sameFg:    ui.RGBA(255, 255, 255, 0.50),
			sameBg:    ui.RGBA(255, 255, 255, 0.06),
			diffFg:    ui.Hex("#fbbf24"),
			diffBg:    ui.Hex("#3d2c12"),
			dangerFg:  ui.Hex("#ff6762"),
			dangerBg:  ui.Hex("#3e1f23"),
			successFg: ui.Hex("#5ecc71"),
			successBg: ui.Hex("#173121"),

			pillBg:     ui.RGBA(255, 255, 255, 0.06),
			pillBorder: ui.RGBA(255, 255, 255, 0.10),
		}
	}
	return uiPalette{
		// 亮色模式：采用温润暖白 (#f8f8f6) 与原生灰阶 (#f0f0ee)
		sidebarBg:         ui.Hex("#f0f0ee"),
		sidebarBorder:     ui.RGBA(0, 0, 0, 0.07),
		sidebarItemHover:  ui.RGBA(0, 0, 0, 0.04),
		sidebarItemActive: ui.Hex("#ffffff"),
		sidebarText:       ui.Hex("#18181b"),
		sidebarMuted:      ui.RGBA(0, 0, 0, 0.50),
		sidebarBtnBg:      ui.Hex("#ffffff"),
		sidebarBtnBorder:  ui.RGBA(0, 0, 0, 0.09),

		appBg:         ui.Hex("#f8f8f6"),
		headerBg:      ui.Hex("#ffffff"),
		subtleSurface: ui.Hex("#f4f4f2"),
		cardBg:        ui.Hex("#ffffff"),
		cardBorder:    ui.RGBA(0, 0, 0, 0.08),

		leftAccent:  ui.Hex("#0dbe4e"),
		leftBg:      ui.Hex("#e6f9ed"),
		rightAccent: ui.Hex("#8b5cf6"),
		rightBg:     ui.Hex("#f5f3ff"),
		baseAccent:  ui.Hex("#2563eb"),
		baseBg:      ui.Hex("#eff6ff"),

		sameFg:    ui.RGBA(0, 0, 0, 0.55),
		sameBg:    ui.RGBA(0, 0, 0, 0.05),
		diffFg:    ui.Hex("#b45309"),
		diffBg:    ui.Hex("#fef3c7"),
		dangerFg:  ui.Hex("#e11d48"),
		dangerBg:  ui.Hex("#ffe4e6"),
		successFg: ui.Hex("#15803d"),
		successBg: ui.Hex("#dcfce7"),

		pillBg:     ui.RGBA(0, 0, 0, 0.04),
		pillBorder: ui.RGBA(0, 0, 0, 0.07),
	}
}

// statusPill 极简圆角药丸徽章
func statusPill(c *ui.Context, text string, fg, bg ui.Color) {
	ui.Row(c).Shrink(0).Padding(2, 7).Radius(12).Background(bg).Border(1, fg.Alpha(0.20)).Children(func() {
		ui.Text(c, text).FontSize(11).Bold().TextColor(fg).SingleLine()
	})
}

// sideBadge 左右侧专属高亮标记胶囊
func sideBadge(c *ui.Context, side int, pal uiPalette) {
	label := "LEFT"
	fg, bg := pal.leftAccent, pal.leftBg
	if side == 1 {
		label = "RIGHT"
		fg, bg = pal.rightAccent, pal.rightBg
	} else if side == 2 {
		label = "BASE"
		fg, bg = pal.baseAccent, pal.baseBg
	}
	ui.Row(c).Shrink(0).Padding(1, 5).Radius(4).Background(bg).Border(1, fg.Alpha(0.25)).Children(func() {
		ui.Text(c, label).FontSize(9).Bold().TextColor(fg).SingleLine()
	})
}
