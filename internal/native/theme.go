package native

import (
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
	"gocompare/internal/config"
)

// 矢量 SVG 图标（基于 Onevra Lucide 图标集）
var (
	iconTextDiff   = lucide.FileDiff()
	iconFileDiff   = lucide.FileText()
	iconFolderDiff = lucide.Folder()
	iconGitMerge   = lucide.GitMerge()
	iconImageDiff  = lucide.Image()
	iconMediaTag   = lucide.Music()
	iconVersionReg = lucide.ShieldCheck()
	iconRemote     = lucide.Server()
	iconTable      = lucide.Table()

	iconCompare      = lucide.Play()
	iconSwap         = lucide.ArrowLeftRight()
	iconChevronUp    = lucide.ChevronUp()
	iconChevronDown  = lucide.ChevronDown()
	iconChevronLeft  = lucide.ChevronLeft()
	iconChevronRight = lucide.ChevronRight()
	iconSearch       = lucide.Search()
	iconSliders      = lucide.SlidersHorizontal()
	iconFolderOpen   = lucide.FolderOpen()
	iconSave         = lucide.Save()
	iconFile         = lucide.File()
	iconCheck        = lucide.Check()
	iconAlert        = lucide.TriangleAlert()
	iconExternal     = lucide.ExternalLink()
	iconSync         = lucide.RefreshCw()
	iconUndo         = lucide.Undo2()
	iconRedo         = lucide.Redo2()
	iconReset        = lucide.RotateCcw()
	iconAnchor       = lucide.Anchor()
	iconDrop         = lucide.Download()
	iconInfo         = lucide.Info()
	iconPalette      = lucide.Palette()
	iconKeyboard     = lucide.Keyboard()
	iconGlobe        = lucide.Globe()
	iconUser         = lucide.UserRound()
)

var modeIcons = []*ui.SVG{
	iconFileDiff,
	iconFolderDiff,
	iconGitMerge,
	iconImageDiff,
	iconMediaTag,
	iconVersionReg,
}

// 预设主题配色标识
const (
	paletteClassic  = "classic"
	paletteEmerald  = "emerald"
	paletteViolet   = "violet"
	paletteGraphite = "graphite"
	paletteAmber    = "amber"
)

var paletteKeys = []string{
	paletteClassic,
	paletteEmerald,
	paletteViolet,
	paletteGraphite,
	paletteAmber,
}

func paletteIndexOf(name string) int {
	for i, k := range paletteKeys {
		if k == name {
			return i
		}
	}
	return 0
}

func paletteNameOf(idx int) string {
	if idx >= 0 && idx < len(paletteKeys) {
		return paletteKeys[idx]
	}
	return paletteClassic
}

// palettePreviewColor 返回各主题预设在设置面板中的代表色圆点。
func palettePreviewColor(idx int) ui.Color {
	switch paletteNameOf(idx) {
	case paletteEmerald:
		return ui.Hex("#059669")
	case paletteViolet:
		return ui.Hex("#6366f1")
	case paletteGraphite:
		return ui.Hex("#52525b")
	case paletteAmber:
		return ui.Hex("#d97706")
	default:
		return ui.Hex("#2563eb")
	}
}

// buildPresetTheme 根据预设名和日间/夜间状态构造完整 ui.Theme。
func buildPresetTheme(preset string, dark bool) *ui.Theme {
	black := ui.RGB(0, 0, 0)
	white := ui.RGB(255, 255, 255)
	if dark {
		t := ui.DarkTheme()
		t.Dark = true
		t.Radius = 7
		switch preset {
		case paletteEmerald:
			accent := ui.Hex("#10b981")
			t.Background = ui.Hex("#0c1713")
			t.Surface = ui.Hex("#13241e")
			t.SurfaceHover = ui.Hex("#1b322a")
			t.SurfacePressed = ui.Hex("#234036")
			t.Border = ui.Hex("#254238")
			t.Text = ui.Hex("#ecfdf5")
			t.TextMuted = ui.Hex("#8fb8a8")
			t.Accent = accent
			t.AccentHover = accent.Mix(white, 0.14)
			t.AccentPressed = accent.Mix(black, 0.14)
			t.AccentText = ui.Hex("#062118")
		case paletteViolet:
			accent := ui.Hex("#818cf8")
			t.Background = ui.Hex("#11101d")
			t.Surface = ui.Hex("#1a182b")
			t.SurfaceHover = ui.Hex("#24223a")
			t.SurfacePressed = ui.Hex("#2e2b49")
			t.Border = ui.Hex("#322f4f")
			t.Text = ui.Hex("#f5f3ff")
			t.TextMuted = ui.Hex("#a5a1c2")
			t.Accent = accent
			t.AccentHover = accent.Mix(white, 0.14)
			t.AccentPressed = accent.Mix(black, 0.14)
			t.AccentText = ui.Hex("#131126")
		case paletteGraphite:
			accent := ui.Hex("#d4d4d8")
			t.Background = ui.Hex("#121214")
			t.Surface = ui.Hex("#1c1c1f")
			t.SurfaceHover = ui.Hex("#27272a")
			t.SurfacePressed = ui.Hex("#323236")
			t.Border = ui.Hex("#323238")
			t.Text = ui.Hex("#f4f4f5")
			t.TextMuted = ui.Hex("#a1a1aa")
			t.Accent = accent
			t.AccentHover = accent.Mix(white, 0.2)
			t.AccentPressed = accent.Mix(black, 0.15)
			t.AccentText = ui.Hex("#18181b")
		case paletteAmber:
			accent := ui.Hex("#f59e0b")
			t.Background = ui.Hex("#171410")
			t.Surface = ui.Hex("#221e18")
			t.SurfaceHover = ui.Hex("#2e2820")
			t.SurfacePressed = ui.Hex("#3a3228")
			t.Border = ui.Hex("#3d352a")
			t.Text = ui.Hex("#f7f2e8")
			t.TextMuted = ui.Hex("#b5a895")
			t.Accent = accent
			t.AccentHover = accent.Mix(white, 0.14)
			t.AccentPressed = accent.Mix(black, 0.14)
			t.AccentText = ui.Hex("#1f1608")
		default: // classic
			accent := ui.Hex("#3b82f6")
			t.Background = ui.Hex("#0f172a")
			t.Surface = ui.Hex("#1e293b")
			t.SurfaceHover = ui.Hex("#283548")
			t.SurfacePressed = ui.Hex("#334155")
			t.Border = ui.Hex("#334155")
			t.Text = ui.Hex("#f8fafc")
			t.TextMuted = ui.Hex("#94a3b8")
			t.Accent = accent
			t.AccentHover = accent.Mix(white, 0.14)
			t.AccentPressed = accent.Mix(black, 0.14)
			t.AccentText = ui.Hex("#ffffff")
		}
		t.Danger = ui.Hex("#f87171")
		t.Warning = ui.Hex("#fbbf24")
		t.Success = ui.Hex("#34d399")
		t.Selection = t.Accent.Alpha(0.26)
		t.Focus = t.Accent.Alpha(0.75)
		t.Scrollbar = t.Text.Alpha(0.32)
		return t
	}

	t := ui.LightTheme()
	t.Radius = 7
	switch preset {
	case paletteEmerald:
		accent := ui.Hex("#059669")
		t.Background = ui.Hex("#f4f9f6")
		t.Surface = ui.Hex("#ffffff")
		t.SurfaceHover = ui.Hex("#eaf3ee")
		t.SurfacePressed = ui.Hex("#dbe9e1")
		t.Border = ui.Hex("#d5e6dd")
		t.Text = ui.Hex("#0f291e")
		t.TextMuted = ui.Hex("#527364")
		t.Accent = accent
		t.AccentHover = accent.Mix(black, 0.12)
		t.AccentPressed = accent.Mix(black, 0.22)
		t.AccentText = ui.Hex("#ffffff")
	case paletteViolet:
		accent := ui.Hex("#6366f1")
		t.Background = ui.Hex("#f7f6fc")
		t.Surface = ui.Hex("#ffffff")
		t.SurfaceHover = ui.Hex("#eeecfa")
		t.SurfacePressed = ui.Hex("#e2def5")
		t.Border = ui.Hex("#e0dcf0")
		t.Text = ui.Hex("#1e1b4b")
		t.TextMuted = ui.Hex("#636087")
		t.Accent = accent
		t.AccentHover = accent.Mix(black, 0.12)
		t.AccentPressed = accent.Mix(black, 0.22)
		t.AccentText = ui.Hex("#ffffff")
	case paletteGraphite:
		accent := ui.Hex("#27272a")
		t.Background = ui.Hex("#f8f8f9")
		t.Surface = ui.Hex("#ffffff")
		t.SurfaceHover = ui.Hex("#f1f1f3")
		t.SurfacePressed = ui.Hex("#e4e4e7")
		t.Border = ui.Hex("#e4e4e7")
		t.Text = ui.Hex("#18181b")
		t.TextMuted = ui.Hex("#71717a")
		t.Accent = accent
		t.AccentHover = accent.Mix(white, 0.15)
		t.AccentPressed = accent.Mix(black, 0.2)
		t.AccentText = ui.Hex("#ffffff")
	case paletteAmber:
		accent := ui.Hex("#d97706")
		t.Background = ui.Hex("#fcfaf5")
		t.Surface = ui.Hex("#ffffff")
		t.SurfaceHover = ui.Hex("#f5efe2")
		t.SurfacePressed = ui.Hex("#ebe2ce")
		t.Border = ui.Hex("#e6dcc8")
		t.Text = ui.Hex("#292116")
		t.TextMuted = ui.Hex("#786b59")
		t.Accent = accent
		t.AccentHover = accent.Mix(black, 0.12)
		t.AccentPressed = accent.Mix(black, 0.22)
		t.AccentText = ui.Hex("#ffffff")
	default: // classic
		accent := ui.Hex("#2563eb")
		t.Background = ui.Hex("#f8fafc")
		t.Surface = ui.Hex("#ffffff")
		t.SurfaceHover = ui.Hex("#f1f5f9")
		t.SurfacePressed = ui.Hex("#e2e8f0")
		t.Border = ui.Hex("#e2e8f0")
		t.Text = ui.Hex("#0f172a")
		t.TextMuted = ui.Hex("#64748b")
		t.Accent = accent
		t.AccentHover = accent.Mix(black, 0.12)
		t.AccentPressed = accent.Mix(black, 0.22)
		t.AccentText = ui.Hex("#ffffff")
	}
	t.Danger = ui.Hex("#dc2626")
	t.Warning = ui.Hex("#d97706")
	t.Success = ui.Hex("#059669")
	t.Selection = t.Accent.Alpha(0.20)
	t.Focus = t.Accent.Alpha(0.55)
	t.Scrollbar = t.Text.Alpha(0.28)
	return t
}

// activeTheme 返回当前配置下的主题（供 Shell.Theme 与无 Shell 视图共用）。
func activeTheme(c *ui.Context) *ui.Theme {
	preset := config.Current().Palette
	t := buildPresetTheme(preset, c.Theme().Dark)
	p := c.Preferences()
	if p.HighContrast {
		t.Border = t.Border.Mix(t.Text, 0.45)
		t.TextMuted = t.TextMuted.Mix(t.Text, 0.4)
		t.Focus.A = 255
		t.Scrollbar = t.Scrollbar.Alpha(1.6)
	}
	if s := p.TextScale; s > 0 && s != 1 {
		t.FontSize *= s
	}
	return t
}

// applyOnevraTheme 将当前活动主题应用到当前帧，并返回主题实例。
func applyOnevraTheme(c *ui.Context) *ui.Theme {
	t := activeTheme(c)
	c.SetTheme(t)
	return t
}

// uiPalette 定义融合 Onevra 基底与 GoCompare 语义差异色的调色板
type uiPalette struct {
	// 侧边栏（与 Onevra 表面层次融合）
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
			sidebarBg:         t.Surface,
			sidebarBorder:     t.Border,
			sidebarItemHover:  t.SurfaceHover,
			sidebarItemActive: t.Accent.Alpha(0.15),
			sidebarText:       t.Text,
			sidebarMuted:      t.TextMuted,
			sidebarBtnBg:      t.SurfaceHover,
			sidebarBtnBorder:  t.Border,

			appBg:         t.Background,
			headerBg:      t.Surface,
			subtleSurface: t.Background.Mix(t.Surface, 0.55),
			cardBg:        t.Surface,
			cardBorder:    t.Border,

			leftAccent:  ui.Hex("#4ade80"),
			leftBg:      ui.Hex("#162c20"),
			rightAccent: ui.Hex("#c084fc"),
			rightBg:     ui.Hex("#2b1c3b"),
			baseAccent:  t.Accent,
			baseBg:      t.Accent.Alpha(0.18),

			sameFg:    t.TextMuted,
			sameBg:    t.Text.Alpha(0.06),
			diffFg:    t.Warning,
			diffBg:    ui.Hex("#3b2a10"),
			dangerFg:  t.Danger,
			dangerBg:  ui.Hex("#3b1c22"),
			successFg: t.Success,
			successBg: ui.Hex("#152e20"),

			pillBg:     t.Text.Alpha(0.06),
			pillBorder: t.Border,
		}
	}
	return uiPalette{
		sidebarBg:         t.Background.Mix(t.Text, 0.025),
		sidebarBorder:     t.Border,
		sidebarItemHover:  t.SurfaceHover,
		sidebarItemActive: t.Accent.Alpha(0.10),
		sidebarText:       t.Text,
		sidebarMuted:      t.TextMuted,
		sidebarBtnBg:      t.Surface,
		sidebarBtnBorder:  t.Border,

		appBg:         t.Background,
		headerBg:      t.Surface,
		subtleSurface: t.Background.Mix(t.Text, 0.02),
		cardBg:        t.Surface,
		cardBorder:    t.Border,

		leftAccent:  ui.Hex("#15803d"),
		leftBg:      ui.Hex("#dcfce7"),
		rightAccent: ui.Hex("#7c3aed"),
		rightBg:     ui.Hex("#f3e8ff"),
		baseAccent:  t.Accent,
		baseBg:      t.Accent.Alpha(0.10),

		sameFg:    t.TextMuted,
		sameBg:    t.Text.Alpha(0.05),
		diffFg:    t.Warning,
		diffBg:    ui.Hex("#fef3c7"),
		dangerFg:  t.Danger,
		dangerBg:  ui.Hex("#ffe4e6"),
		successFg: t.Success,
		successBg: ui.Hex("#dcfce7"),

		pillBg:     t.Text.Alpha(0.04),
		pillBorder: t.Border,
	}
}

// statusPill 使用 Onevra Badge 组件渲染极简圆角药丸徽章（覆盖默认 AlignSelf(Start) 为 Center，避免在水平行中上浮）
func statusPill(c *ui.Context, text string, fg, bg ui.Color) {
	components.Badge(c, text).
		AlignSelf(ui.Center).
		Shrink(0).
		FontSize(11).
		Padding(2, 7).
		Background(bg).
		TextColor(fg).
		Border(1, fg.Alpha(0.22))
}

// sideBadge 左右侧专属高亮标记胶囊（覆盖默认 AlignSelf(Start) 为 Center，保证在行内垂直居中）
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
	components.Badge(c, label).
		AlignSelf(ui.Center).
		Shrink(0).
		FontSize(9).
		Padding(2, 5).
		Radius(4).
		Background(bg).
		TextColor(fg).
		Border(1, fg.Alpha(0.25))
}

// toolbarBtn 使用 Onevra Button 组件创建紧凑型工具栏按钮（适配 900px+ 视口）。
func toolbarBtn(c *ui.Context, label string, icon *ui.SVG, variant components.ButtonVariant, disabled bool) ui.Element {
	btn := components.Button(c, "", variant).
		Label(label).
		Padding(4, 9).
		Gap(4).
		Radius(6).
		Disabled(disabled)
	if disabled {
		btn.Opacity(0.45)
	}
	btn.Children(func() {
		if icon != nil {
			components.Icon(c, icon).Size(11, 11)
		}
		txt := components.Text(c, label).FontSize(11).SingleLine()
		if variant == components.ButtonPrimary {
			txt.Bold()
		}
	})
	return btn
}

// compactBtn 使用 Onevra Button 组件创建面板头部/子栏紧凑按钮。
func compactBtn(c *ui.Context, label string, icon *ui.SVG, variant components.ButtonVariant, disabled bool) ui.Element {
	btn := components.Button(c, "", variant).
		Label(label).
		Shrink(0).
		Padding(3, 8).
		Gap(3).
		Radius(5).
		Disabled(disabled)
	if disabled {
		btn.Opacity(0.45)
	}
	btn.Children(func() {
		if icon != nil {
			components.Icon(c, icon).Size(11, 11)
		}
		txt := components.Text(c, label).FontSize(11).SingleLine()
		if variant == components.ButtonPrimary {
			txt.Bold()
		}
	})
	return btn
}

// dangerBtn 使用 Onevra Button 组件创建危险/取消语义按钮。
func dangerBtn(c *ui.Context, label string, icon *ui.SVG, pal uiPalette, disabled bool) ui.Element {
	btn := components.Button(c, "", components.ButtonSecondary).
		Label(label).
		Shrink(0).
		Padding(4, 8).
		Gap(4).
		Radius(6).
		Background(pal.dangerBg).
		Border(1, pal.dangerFg.Alpha(0.28)).
		TextColor(pal.dangerFg).
		Disabled(disabled)
	if disabled {
		btn.Opacity(0.45)
	}
	btn.Children(func() {
		if icon != nil {
			components.Icon(c, icon).Size(11, 11).TextColor(pal.dangerFg)
		}
		components.Text(c, label).FontSize(11).Bold().TextColor(pal.dangerFg).SingleLine()
	})
	return btn
}

// toggleBtn 使用 Onevra Button 组件创建支持激活高亮状态的紧凑切换按钮。
func toggleBtn(c *ui.Context, label string, active, disabled bool) ui.Element {
	pal := paletteFor(c.Theme())
	btn := compactBtn(c, label, nil, components.ButtonSecondary, disabled)
	if active && !disabled {
		btn.Background(pal.baseBg).Border(1, pal.baseAccent.Alpha(0.35)).TextColor(pal.baseAccent)
	}
	return btn
}

// simpleSelect 使用 Onevra Select 组件渲染紧凑下拉选择框，返回其 Trigger 元素（可调用 .Changed() 或 .Grow()）。
func simpleSelect(c *ui.Context, selected *string, options []string, disabled bool) ui.Element {
	sel := components.Select(c, selected)
	sel.Trigger.Disabled(disabled).Padding(4, 22, 4, 8)
	sel.Trigger.Children(func() {
		components.Text(c, *selected).FontSize(11).SingleLine()
	})
	sel.Popup(func(panel ui.Element) {
		for _, opt := range options {
			opt := opt
			item := sel.Item(opt)
			item.Children(func() {
				components.Text(c, opt).FontSize(11).SingleLine()
			})
		}
	})
	return sel.Trigger
}
