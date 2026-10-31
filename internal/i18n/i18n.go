package i18n

import (
	"fmt"
	"sync"
)

type Language string

const (
	ZhCN Language = "zh-CN"
	ZhTW Language = "zh-TW"
	EnUS Language = "en"
)

var (
	currentLang = ZhCN
	mu          sync.RWMutex
)

func SetLanguage(l Language) {
	mu.Lock()
	defer mu.Unlock()
	switch l {
	case ZhTW, EnUS:
		currentLang = l
	default:
		currentLang = ZhCN
	}
}

func CurrentLanguage() Language {
	mu.RLock()
	defer mu.RUnlock()
	return currentLang
}

var messages = map[string]map[Language]string{
	// App & Brand
	"app.title": {
		ZhCN: "GoCompare",
		ZhTW: "GoCompare",
		EnUS: "GoCompare",
	},
	"app.subtitle": {
		ZhCN: "Diff Workbench",
		ZhTW: "Diff Workbench",
		EnUS: "Diff Workbench",
	},
	"nav.modes": {
		ZhCN: "模式",
		ZhTW: "模式",
		EnUS: "MODES",
	},
	"nav.session": {
		ZhCN: "会话",
		ZhTW: "工作階段",
		EnUS: "SESSION",
	},
	"nav.settings": {
		ZhCN: "设置",
		ZhTW: "設定",
		EnUS: "Settings",
	},
	"nav.openSession": {
		ZhCN: "打开会话…",
		ZhTW: "開啟工作階段…",
		EnUS: "Open Session…",
	},
	"nav.saveSession": {
		ZhCN: "保存会话…",
		ZhTW: "儲存工作階段…",
		EnUS: "Save Session…",
	},

	// Modes
	"mode.scratch": {
		ZhCN: "文本比较",
		ZhTW: "文本比較",
		EnUS: "Text Compare",
	},
	"mode.file": {
		ZhCN: "文件比较",
		ZhTW: "文件比較",
		EnUS: "File Compare",
	},
	"mode.directory": {
		ZhCN: "目录比较",
		ZhTW: "目錄比較",
		EnUS: "Folder Compare",
	},
	"mode.merge": {
		ZhCN: "文本合并",
		ZhTW: "文本合併",
		EnUS: "Text Merge",
	},
	"mode.picture": {
		ZhCN: "图片比较",
		ZhTW: "圖片比較",
		EnUS: "Picture Compare",
	},
	"mode.media": {
		ZhCN: "媒体标签",
		ZhTW: "媒體標籤",
		EnUS: "Media Tags",
	},
	"mode.version": {
		ZhCN: "版本与注册表",
		ZhTW: "版本與登錄檔",
		EnUS: "Version & Registry",
	},

	// Actions & Buttons
	"action.compare": {
		ZhCN: "开始比较",
		ZhTW: "開始比較",
		EnUS: "Compare",
	},
	"action.swap": {
		ZhCN: "交换两侧",
		ZhTW: "交換兩側",
		EnUS: "Swap Sides",
	},
	"action.clear": {
		ZhCN: "清空两侧",
		ZhTW: "清空兩側",
		EnUS: "Clear Both",
	},
	"action.sample": {
		ZhCN: "载入示例",
		ZhTW: "載入範例",
		EnUS: "Load Sample",
	},
	"action.prevDiff": {
		ZhCN: "上一差异",
		ZhTW: "上一差異",
		EnUS: "Prev Diff",
	},
	"action.nextDiff": {
		ZhCN: "下一差异",
		ZhTW: "下一差異",
		EnUS: "Next Diff",
	},
	"action.applyLeft": {
		ZhCN: "← 应用到左侧",
		ZhTW: "← 套用到左側",
		EnUS: "← Apply to Left",
	},
	"action.applyRight": {
		ZhCN: "应用到右侧 →",
		ZhTW: "套用到右側 →",
		EnUS: "Apply to Right →",
	},
	"action.find": {
		ZhCN: "查找",
		ZhTW: "尋找",
		EnUS: "Find",
	},
	"action.options": {
		ZhCN: "比较选项",
		ZhTW: "比較選項",
		EnUS: "Diff Options",
	},
	"action.backToFolder": {
		ZhCN: "返回目录",
		ZhTW: "返回目錄",
		EnUS: "Back to Folder",
	},
	"action.openMerge": {
		ZhCN: "打开合并窗口",
		ZhTW: "開啟合併視窗",
		EnUS: "Open Merge Window",
	},
	"action.browse": {
		ZhCN: "浏览…",
		ZhTW: "瀏覽…",
		EnUS: "Browse…",
	},
	"action.recompute": {
		ZhCN: "重新计算",
		ZhTW: "重新計算",
		EnUS: "Recompute",
	},
	"action.closeAlert": {
		ZhCN: "关闭提示",
		ZhTW: "關閉提示",
		EnUS: "Dismiss",
	},
	"action.cancelTask": {
		ZhCN: "取消任务",
		ZhTW: "取消工作",
		EnUS: "Cancel Task",
	},

	// Options Drawer
	"option.ignoreSpace": {
		ZhCN: "忽略空白",
		ZhTW: "忽略空白",
		EnUS: "Ignore Whitespace",
	},
	"option.ignoreBlankLines": {
		ZhCN: "忽略空行",
		ZhTW: "忽略空行",
		EnUS: "Ignore Blank Lines",
	},
	"option.wrap": {
		ZhCN: "自动换行",
		ZhTW: "自動換行",
		EnUS: "Wrap Lines",
	},
	"option.inlineDiff": {
		ZhCN: "行内高亮",
		ZhTW: "行內醒目提示",
		EnUS: "Inline Diff",
	},
	"option.modeCompare": {
		ZhCN: "比较",
		ZhTW: "比較",
		EnUS: "Compare",
	},
	"option.modeEditLeft": {
		ZhCN: "编辑左侧",
		ZhTW: "編輯左側",
		EnUS: "Edit Left",
	},
	"option.modeEditRight": {
		ZhCN: "编辑右侧",
		ZhTW: "編輯右側",
		EnUS: "Edit Right",
	},
	"option.dirFilter": {
		ZhCN: "目录过滤",
		ZhTW: "目錄過濾",
		EnUS: "Folder Filter",
	},
	"option.dirFilterPlaceholder": {
		ZhCN: "*.go 或 nested/*.txt",
		ZhTW: "*.go 或 nested/*.txt",
		EnUS: "*.go or nested/*.txt",
	},
	"option.compareMode": {
		ZhCN: "比较权限",
		ZhTW: "比較權限",
		EnUS: "Compare Permissions",
	},
	"option.compareTime": {
		ZhCN: "比较时间",
		ZhTW: "比較時間",
		EnUS: "Compare Timestamps",
	},
	"option.followLinks": {
		ZhCN: "跟随链接",
		ZhTW: "跟隨符號連結",
		EnUS: "Follow Symlinks",
	},
	"option.anchors": {
		ZhCN: "行对齐锚点",
		ZhTW: "行對齊錨點",
		EnUS: "Alignment Anchors",
	},
	"option.markLeft": {
		ZhCN: "标记左侧",
		ZhTW: "標記左側",
		EnUS: "Mark Left",
	},
	"option.markRight": {
		ZhCN: "标记右侧",
		ZhTW: "標記右側",
		EnUS: "Mark Right",
	},
	"option.alignHere": {
		ZhCN: "在此对齐",
		ZhTW: "在此對齊",
		EnUS: "Align Here",
	},
	"option.clearAnchors": {
		ZhCN: "解除对齐",
		ZhTW: "解除對齊",
		EnUS: "Clear Anchors",
	},
	"option.anchorCount": {
		ZhCN: "%d 个锚点",
		ZhTW: "%d 個錨點",
		EnUS: "%d Anchors",
	},

	// Panes & Inputs
	"pane.leftPath": {
		ZhCN: "左侧路径",
		ZhTW: "左側路徑",
		EnUS: "Left Path",
	},
	"pane.rightPath": {
		ZhCN: "右侧路径",
		ZhTW: "右側路徑",
		EnUS: "Right Path",
	},
	"pane.pathPlaceholder": {
		ZhCN: "输入路径或浏览选择…",
		ZhTW: "輸入路徑或瀏覽選擇…",
		EnUS: "Enter path or browse…",
	},
	"pane.leftText": {
		ZhCN: "左侧文本",
		ZhTW: "左側文本",
		EnUS: "Left Text",
	},
	"pane.rightText": {
		ZhCN: "右侧文本",
		ZhTW: "右側文本",
		EnUS: "Right Text",
	},
	"pane.paste": {
		ZhCN: "📋 粘贴",
		ZhTW: "📋 貼上",
		EnUS: "📋 Paste",
	},
	"pane.pasteLeft": {
		ZhCN: "粘贴到左侧",
		ZhTW: "貼上到左側",
		EnUS: "Paste to Left",
	},
	"pane.pasteRight": {
		ZhCN: "粘贴到右侧",
		ZhTW: "貼上到右側",
		EnUS: "Paste to Right",
	},
	"pane.copy": {
		ZhCN: "复制",
		ZhTW: "複製",
		EnUS: "Copy",
	},
	"pane.clear": {
		ZhCN: "清空",
		ZhTW: "清空",
		EnUS: "Clear",
	},
	"pane.saveAs": {
		ZhCN: "另存为…",
		ZhTW: "另存為…",
		EnUS: "Save As…",
	},
	"pane.saveLeft": {
		ZhCN: "保存左侧",
		ZhTW: "儲存左側",
		EnUS: "Save Left",
	},
	"pane.saveRight": {
		ZhCN: "保存右侧",
		ZhTW: "儲存右側",
		EnUS: "Save Right",
	},
	"pane.unsaved": {
		ZhCN: "未保存",
		ZhTW: "未儲存",
		EnUS: "Unsaved",
	},
	"pane.blankIgnored": {
		ZhCN: "已忽略空行",
		ZhTW: "已忽略空行",
		EnUS: "Blank Lines Ignored",
	},
	"pane.scratchTip": {
		ZhCN: "💡 提示：可直接在左右两栏中键入或按 ⌘V 粘贴文本，支持实时差异高亮与双侧编辑",
		ZhTW: "💡 提示：可直接在左右兩欄中輸入或按 ⌘V 貼上文本，支援即時差異醒目提示與雙側編輯",
		EnUS: "💡 Tip: Type or press ⌘V to paste text into either pane. Real-time diff and dual-side editing supported.",
	},

	// Status Badges & Messages
	"badge.same": {
		ZhCN: "内容一致",
		ZhTW: "內容一致",
		EnUS: "Identical",
	},
	"badge.diffCount": {
		ZhCN: "%d 处差异",
		ZhTW: "%d 處差異",
		EnUS: "%d Diffs",
	},
	"badge.itemCount": {
		ZhCN: "%d 项",
		ZhTW: "%d 項",
		EnUS: "%d Items",
	},
	"badge.noConflict": {
		ZhCN: "无冲突",
		ZhTW: "無衝突",
		EnUS: "No Conflicts",
	},
	"badge.conflictCount": {
		ZhCN: "%d 处冲突",
		ZhTW: "%d 處衝突",
		EnUS: "%d Conflicts",
	},
	"status.saveShortcut": {
		ZhCN: "⌘S 保存",
		ZhTW: "⌘S 儲存",
		EnUS: "⌘S Save",
	},
	"status.scratchLoaded": {
		ZhCN: "纯文本比较 · 可直接在两栏中键入或粘贴文本",
		ZhTW: "純文本比較 · 可直接在兩欄中輸入或貼上文本",
		EnUS: "Text Compare · Type or paste text directly into both panes",
	},
	"status.selectSource": {
		ZhCN: "选择左右来源，然后开始比较",
		ZhTW: "選擇左右來源，然後開始比較",
		EnUS: "Select left and right sources, then start comparison",
	},
	"status.clearedBoth": {
		ZhCN: "已清空两栏文本",
		ZhTW: "已清空兩欄文本",
		EnUS: "Both panes cleared",
	},
	"status.sampleLoaded": {
		ZhCN: "已载入示例对比文本",
		ZhTW: "已載入範例對比文本",
		EnUS: "Sample text loaded",
	},
	"status.swappedSides": {
		ZhCN: "已交换左右两侧文本",
		ZhTW: "已交換左右兩側文本",
		EnUS: "Sides swapped",
	},

	// Settings Dialog
	"settings.title": {
		ZhCN: "首选项设置",
		ZhTW: "偏好設定",
		EnUS: "Preferences",
	},
	"settings.language": {
		ZhCN: "语言设置",
		ZhTW: "語言設定",
		EnUS: "Language",
	},
	"settings.languageHint": {
		ZhCN: "选择界面显示语言",
		ZhTW: "選擇介面顯示語言",
		EnUS: "Select interface display language",
	},
	"settings.langZhCN": {
		ZhCN: "简体中文",
		ZhTW: "簡體中文",
		EnUS: "Simplified Chinese",
	},
	"settings.langZhTW": {
		ZhCN: "繁体中文",
		ZhTW: "繁體中文",
		EnUS: "Traditional Chinese",
	},
	"settings.langEn": {
		ZhCN: "English",
		ZhTW: "English",
		EnUS: "English",
	},
	"settings.appearance": {
		ZhCN: "显示设置",
		ZhTW: "顯示設定",
		EnUS: "Appearance",
	},
	"settings.themeHint": {
		ZhCN: "跟随系统或自定日间/夜间外观",
		ZhTW: "跟隨系統或自訂日間/夜間外觀",
		EnUS: "Follow system or customize Light/Dark appearance",
	},
	"settings.themeLight": {
		ZhCN: "日间",
		ZhTW: "日間",
		EnUS: "Light",
	},
	"settings.themeDark": {
		ZhCN: "夜间",
		ZhTW: "夜間",
		EnUS: "Dark",
	},
	"settings.themeSystem": {
		ZhCN: "系统",
		ZhTW: "系統",
		EnUS: "System",
	},
	"settings.close": {
		ZhCN: "关闭",
		ZhTW: "關閉",
		EnUS: "Close",
	},

	// Dialogs
	"dialog.error": {
		ZhCN: "错误",
		ZhTW: "錯誤",
		EnUS: "Error",
	},
	"dialog.confirm": {
		ZhCN: "确定",
		ZhTW: "確定",
		EnUS: "OK",
	},

	// Dialogs
	"dialog.discardTitle": {
		ZhCN: "放弃未保存的修改？",
		ZhTW: "放棄未儲存的修改？",
		EnUS: "Discard unsaved changes?",
	},
	"dialog.discardDesc": {
		ZhCN: "可取消后先保存左右文件。",
		ZhTW: "可取消後先儲存左右文件。",
		EnUS: "You can cancel and save files before switching.",
	},
	"dialog.cancel": {
		ZhCN: "取消",
		ZhTW: "取消",
		EnUS: "Cancel",
	},
	"dialog.discard": {
		ZhCN: "放弃修改",
		ZhTW: "放棄修改",
		EnUS: "Discard",
	},
	"dialog.syncTitle": {
		ZhCN: "执行同步计划？",
		ZhTW: "執行同步計劃？",
		EnUS: "Execute synchronization plan?",
	},
	"dialog.syncDesc": {
		ZhCN: "预览中的复制和删除将修改目标目录；覆盖和删除的文件会先备份。",
		ZhTW: "預覽中的複製和刪除將修改目標目錄；覆蓋和刪除的文件會先備份。",
		EnUS: "Copy and delete operations in the preview will modify the target directory. Backups are created beforehand.",
	},
	"dialog.execute": {
		ZhCN: "执行",
		ZhTW: "執行",
		EnUS: "Execute",
	},
}

func T(key string) string {
	mu.RLock()
	defer mu.RUnlock()
	if m, ok := messages[key]; ok {
		if text, ok := m[currentLang]; ok && text != "" {
			return text
		}
		if text, ok := m[ZhCN]; ok && text != "" {
			return text
		}
	}
	return key
}

func TArgs(key string, args ...any) string {
	format := T(key)
	return fmt.Sprintf(format, args...)
}
