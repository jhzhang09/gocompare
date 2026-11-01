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
	"mode.table": {
		ZhCN: "表格比较",
		ZhTW: "表格比較",
		EnUS: "Table Compare",
	},
	"table.same": {
		ZhCN: "表格相同",
		ZhTW: "表格相同",
		EnUS: "Tables Identical",
	},
	"table.diffCount": {
		ZhCN: "%d 处差异",
		ZhTW: "%d 處差異",
		EnUS: "%d differences",
	},
	"dirmerge.action": {
		ZhCN: "目录合并",
		ZhTW: "目錄合併",
		EnUS: "Merge Folders",
	},
	"dirmerge.previewDone": {
		ZhCN: "目录合并预览：%d 个文件项",
		ZhTW: "目錄合併預覽：%d 個檔案項",
		EnUS: "Folder merge preview: %d items",
	},
	"dirmerge.items": {
		ZhCN: "合并项：%d",
		ZhTW: "合併項：%d",
		EnUS: "Items: %d",
	},
	"dirmerge.conflicts": {
		ZhCN: "冲突：%d",
		ZhTW: "衝突：%d",
		EnUS: "Conflicts: %d",
	},
	"dirmerge.clean": {
		ZhCN: "可干净合并",
		ZhTW: "可乾淨合併",
		EnUS: "Merges cleanly",
	},
	"dirmerge.empty": {
		ZhCN: "没有需要合并的文件",
		ZhTW: "沒有需要合併的檔案",
		EnUS: "Nothing to merge",
	},
	"dirmerge.kind.left": {
		ZhCN: "采用左侧",
		ZhTW: "採用左側",
		EnUS: "Take left",
	},
	"dirmerge.kind.right": {
		ZhCN: "采用右侧",
		ZhTW: "採用右側",
		EnUS: "Take right",
	},
	"dirmerge.kind.delete": {
		ZhCN: "删除",
		ZhTW: "刪除",
		EnUS: "Delete",
	},
	"dirmerge.kind.merged": {
		ZhCN: "已合并",
		ZhTW: "已合併",
		EnUS: "Merged",
	},
	"dirmerge.kind.conflict": {
		ZhCN: "冲突",
		ZhTW: "衝突",
		EnUS: "Conflict",
	},
	"dirmerge.reason.binary": {
		ZhCN: "二进制冲突，写出时保存为 .left / .right",
		ZhTW: "二進位衝突，寫出時保存為 .left / .right",
		EnUS: "Binary conflict, written as .left / .right",
	},
	"dirmerge.reason.deleteModify": {
		ZhCN: "删除/修改冲突，写出时保存现存一侧",
		ZhTW: "刪除/修改衝突，寫出時保存現存一側",
		EnUS: "Delete/modify conflict, surviving side written as .left / .right",
	},
	"table.sheetCount": {
		ZhCN: "工作表：左侧 %d 张，右侧 %d 张",
		ZhTW: "工作表：左側 %d 張，右側 %d 張",
		EnUS: "Sheets: %d left, %d right",
	},
	"table.details": {
		ZhCN: "差异明细",
		ZhTW: "差異明細",
		EnUS: "Differences",
	},
	"table.cell": {
		ZhCN: "行 %s 列 %d",
		ZhTW: "列 %s 欄 %d",
		EnUS: "Row %s, column %d",
	},
	"table.more": {
		ZhCN: "… 以及其他 %d 处差异",
		ZhTW: "… 以及其他 %d 處差異",
		EnUS: "… and %d more differences",
	},
	"table.kind.left_only": {
		ZhCN: "仅左侧",
		ZhTW: "僅左側",
		EnUS: "Left only",
	},
	"table.kind.right_only": {
		ZhCN: "仅右侧",
		ZhTW: "僅右側",
		EnUS: "Right only",
	},
	"table.kind.duplicate": {
		ZhCN: "重复键",
		ZhTW: "重複鍵",
		EnUS: "Duplicate key",
	},
	"table.kind.sheet_left_only": {
		ZhCN: "仅左侧工作表",
		ZhTW: "僅左側工作表",
		EnUS: "Left-only sheet",
	},
	"table.kind.sheet_right_only": {
		ZhCN: "仅右侧工作表",
		ZhTW: "僅右側工作表",
		EnUS: "Right-only sheet",
	},
	"settings.downloadURL": {
		ZhCN: "下载地址：%s",
		ZhTW: "下載位址：%s",
		EnUS: "Download: %s",
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
	"action.reset": {
		ZhCN: "重置",
		ZhTW: "重設",
		EnUS: "Reset",
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

	// Picture Compare
	"picture.modeSide": {
		ZhCN: "并排",
		ZhTW: "並排",
		EnUS: "Side by Side",
	},
	"picture.modeOverlay": {
		ZhCN: "叠加",
		ZhTW: "疊加",
		EnUS: "Overlay",
	},
	"picture.modeDifference": {
		ZhCN: "差值",
		ZhTW: "差值",
		EnUS: "Difference",
	},
	"picture.modeTolerance": {
		ZhCN: "容差",
		ZhTW: "容差",
		EnUS: "Tolerance",
	},
	"picture.toleranceValue": {
		ZhCN: "容差 %d",
		ZhTW: "容差 %d",
		EnUS: "Tolerance %d",
	},
	"picture.toleranceSlider": {
		ZhCN: "容差滑块",
		ZhTW: "容差滑桿",
		EnUS: "Tolerance slider",
	},
	"picture.exceedCount": {
		ZhCN: "超出容差 %d 像素 (%.2f%%)",
		ZhTW: "超出容差 %d 像素 (%.2f%%)",
		EnUS: "%d pixels beyond tolerance (%.2f%%)",
	},
	"picture.modeReplace": {
		ZhCN: "替换颜色",
		ZhTW: "取代顏色",
		EnUS: "Replace Color",
	},
	"picture.flipH": {
		ZhCN: "水平",
		ZhTW: "水平",
		EnUS: "Flip H",
	},
	"picture.flipV": {
		ZhCN: "垂直",
		ZhTW: "垂直",
		EnUS: "Flip V",
	},
	"picture.flipLeftH": {
		ZhCN: "左图水平翻转",
		ZhTW: "左圖水平翻轉",
		EnUS: "Flip left image horizontally",
	},
	"picture.flipLeftV": {
		ZhCN: "左图垂直翻转",
		ZhTW: "左圖垂直翻轉",
		EnUS: "Flip left image vertically",
	},
	"picture.flipRightH": {
		ZhCN: "右图水平翻转",
		ZhTW: "右圖水平翻轉",
		EnUS: "Flip right image horizontally",
	},
	"picture.flipRightV": {
		ZhCN: "右图垂直翻转",
		ZhTW: "右圖垂直翻轉",
		EnUS: "Flip right image vertically",
	},
	"picture.flippedH": {
		ZhCN: "已水平翻转",
		ZhTW: "已水平翻轉",
		EnUS: "flipped horizontally",
	},
	"picture.flippedV": {
		ZhCN: "已垂直翻转",
		ZhTW: "已垂直翻轉",
		EnUS: "flipped vertically",
	},
	"picture.flippedBoth": {
		ZhCN: "已水平和垂直翻转",
		ZhTW: "已水平和垂直翻轉",
		EnUS: "flipped both ways",
	},
	"picture.zoomFit": {
		ZhCN: "适应窗口",
		ZhTW: "適應視窗",
		EnUS: "Fit Window",
	},
	"picture.zoom100": {
		ZhCN: "1:1 实际",
		ZhTW: "1:1 實際",
		EnUS: "1:1 Actual",
	},
	"picture.zoom200": {
		ZhCN: "200%",
		ZhTW: "200%",
		EnUS: "200%",
	},
	"picture.zoomGrid": {
		ZhCN: "像素网格",
		ZhTW: "像素網格",
		EnUS: "Pixel Grid",
	},
	"picture.identical": {
		ZhCN: "两图完全一致",
		ZhTW: "兩圖完全一致",
		EnUS: "Identical",
	},
	"picture.identicalFlipped": {
		ZhCN: "翻转后完全一致",
		ZhTW: "翻轉後完全一致",
		EnUS: "Identical after flipping",
	},
	"picture.sizeMismatch": {
		ZhCN: "尺寸不同",
		ZhTW: "尺寸不同",
		EnUS: "Size Mismatch",
	},
	"picture.diffFound": {
		ZhCN: "存在差异 (首处: %d, %d)",
		ZhTW: "存在差異 (首處: %d, %d)",
		EnUS: "Differences (first at %d, %d)",
	},
	"picture.withinTolerance": {
		ZhCN: "容差内相同",
		ZhTW: "容差內相同",
		EnUS: "Within Tolerance",
	},
	"picture.outOfTolerance": {
		ZhCN: "超出容差",
		ZhTW: "超出容差",
		EnUS: "Out of Tolerance",
	},
	"picture.leftImage": {
		ZhCN: "左侧图片",
		ZhTW: "左側圖片",
		EnUS: "Left Image",
	},
	"picture.rightImage": {
		ZhCN: "右侧图片",
		ZhTW: "右側圖片",
		EnUS: "Right Image",
	},
	"picture.emptyTitle": {
		ZhCN: "选择两张 PNG、JPEG 或 GIF",
		ZhTW: "選擇兩張 PNG、JPEG 或 GIF",
		EnUS: "Select two PNG, JPEG or GIF images",
	},
	"picture.emptyDesc": {
		ZhCN: "支持并排、半透明叠加、像素差值、容差阈值校验，可分别翻转左右图后再比较",
		ZhTW: "支援並排、半透明疊加、像素差值、容差閾值校驗，可分別翻轉左右圖後再比較",
		EnUS: "Side-by-side, overlay, pixel difference and tolerance checks; flip either image before comparing",
	},
	"picture.loaded": {
		ZhCN: "已加载图片",
		ZhTW: "已載入圖片",
		EnUS: "Images loaded",
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
	"status.processing": {
		ZhCN: "正在处理…",
		ZhTW: "正在處理…",
		EnUS: "Processing…",
	},
	"status.diffRunning": {
		ZhCN: "正在后台计算差异，可继续编辑…",
		ZhTW: "正在背景計算差異，可繼續編輯…",
		EnUS: "Computing diff in background, you can continue editing…",
	},
	"status.diffStopped": {
		ZhCN: "差异计算已停止，请重新计算",
		ZhTW: "差異計算已停止，請重新計算",
		EnUS: "Diff calculation stopped, please recompute",
	},

	// Mode Metadata
	"meta.dir": {
		ZhCN: "文件树与属性比对",
		ZhTW: "檔案樹與屬性比對",
		EnUS: "File Tree & Attribute Comparison",
	},
	"meta.permission": {
		ZhCN: "权限",
		ZhTW: "權限",
		EnUS: "Permissions",
	},
	"meta.time": {
		ZhCN: "时间",
		ZhTW: "時間",
		EnUS: "Timestamps",
	},
	"meta.merge3": {
		ZhCN: "三方合并",
		ZhTW: "三方合併",
		EnUS: "3-Way Merge",
	},
	"meta.merge2": {
		ZhCN: "两方合并",
		ZhTW: "兩方合併",
		EnUS: "2-Way Merge",
	},
	"meta.picture": {
		ZhCN: "PNG · JPEG · GIF 像素级",
		ZhTW: "PNG · JPEG · GIF 像素級",
		EnUS: "PNG · JPEG · GIF Pixel-Level",
	},
	"meta.media": {
		ZhCN: "ID3v2 元数据标签",
		ZhTW: "ID3v2 元數據標籤",
		EnUS: "ID3v2 Metadata Tags",
	},
	"meta.version": {
		ZhCN: "PE FileVersion 与注册表",
		ZhTW: "PE FileVersion 與登錄檔",
		EnUS: "PE FileVersion & Registry",
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
	"status.resetDone": {
		ZhCN: "已重置当前比较",
		ZhTW: "已重設目前比較",
		EnUS: "Comparison reset",
	},

	// Mode Empty State Guides
	// Kind 0: File Compare
	"intro.fileTag": {
		ZhCN: "双栏代码/文本 · 词级高亮对比",
		ZhTW: "雙欄程式碼/文字 · 詞級高亮對比",
		EnUS: "Dual-Pane Code & Text · Intra-Line Diff",
	},
	"intro.fileTitle": {
		ZhCN: "文件比较",
		ZhTW: "檔案比較",
		EnUS: "File Compare",
	},
	"intro.fileDesc": {
		ZhCN: "在上方输入或浏览选择左右两侧文件，点击「开始比较」或按 Enter 查看完整差异。",
		ZhTW: "在上方輸入或瀏覽選擇左右兩側檔案，點擊「開始比較」或按 Enter 查看完整差異。",
		EnUS: "Enter or browse left and right files above, then click 'Start Compare' or press Enter.",
	},
	"intro.fileFeat1Title": {
		ZhCN: "行内词级对比",
		ZhTW: "行內詞級對比",
		EnUS: "Intra-Line Diff",
	},
	"intro.fileFeat1Desc": {
		ZhCN: "精确高亮每行差异内部字符插入、删除与变更",
		ZhTW: "精確高亮每行差異內部字元插入、刪除與變更",
		EnUS: "Pinpoints character-level insertions, deletions, and changes",
	},
	"intro.fileFeat2Title": {
		ZhCN: "双向安全编辑",
		ZhTW: "雙向安全編輯",
		EnUS: "Two-Way Editing",
	},
	"intro.fileFeat2Desc": {
		ZhCN: "支持两侧实时编辑、块级差异应用与快速保存",
		ZhTW: "支援兩側即時編輯、區塊差異套用與快速儲存",
		EnUS: "Live two-way editing, hunk applying, and fast saving",
	},
	"intro.fileFeat3Title": {
		ZhCN: "对齐锚点与规则",
		ZhTW: "對齊錨點與規則",
		EnUS: "Manual Anchors",
	},
	"intro.fileFeat3Desc": {
		ZhCN: "支持手动设置对齐标记、忽略空白与忽略空行",
		ZhTW: "支援手動設置對齊標記、忽略空白與忽略空行",
		EnUS: "Manual alignment anchors, ignore whitespace & blank lines",
	},
	"intro.fileScratchHint": {
		ZhCN: "无需本地文件？",
		ZhTW: "無需本機檔案？",
		EnUS: "No local files?",
	},
	"intro.toScratch": {
		ZhCN: "纯文本粘贴比较",
		ZhTW: "純文字貼上比較",
		EnUS: "Paste Text Compare",
	},

	// Kind 1: Directory Compare
	"intro.dirTag": {
		ZhCN: "目录树结构 · 递归比对与安全同步",
		ZhTW: "目錄樹結構 · 遞歸比對與安全同步",
		EnUS: "Directory Tree · Recursive Diff & Safe Sync",
	},
	"intro.dirTitle": {
		ZhCN: "目录比较",
		ZhTW: "目錄比較",
		EnUS: "Directory Compare",
	},
	"intro.dirDesc": {
		ZhCN: "在上方输入或浏览选择左右两个文件夹，点击「开始比较」递归扫描目录结构与文件差异。",
		ZhTW: "在上方輸入或瀏覽選擇左右兩個資料夾，點擊「開始比較」遞歸掃描目錄結構與檔案差異。",
		EnUS: "Enter or browse left and right folders above, then click 'Start Compare' to scan directories.",
	},
	"intro.dirFeat1Title": {
		ZhCN: "全树递归扫描",
		ZhTW: "全樹遞歸掃描",
		EnUS: "Recursive Scan",
	},
	"intro.dirFeat1Desc": {
		ZhCN: "快速遍历整棵目录树，支持按通配符与时间权限比对",
		ZhTW: "快速遍歷整棵目錄樹，支援按萬用字元與時間權限比對",
		EnUS: "Traverses directory trees with glob filter, time, and permissions",
	},
	"intro.dirFeat2Title": {
		ZhCN: "状态分类过滤",
		ZhTW: "狀態分類過濾",
		EnUS: "Status Filtering",
	},
	"intro.dirFeat2Desc": {
		ZhCN: "区分不同、仅在左侧、仅在右侧与相同文件，一目了然",
		ZhTW: "區分不同、僅在左側、僅在右側與相同檔案，一目了然",
		EnUS: "Filters different, left-only, right-only, and identical items",
	},
	"intro.dirFeat3Title": {
		ZhCN: "安全同步与回滚",
		ZhTW: "安全同步與回滾",
		EnUS: "Safe Sync & Rollback",
	},
	"intro.dirFeat3Desc": {
		ZhCN: "多向同步前自动备份目标文件，支持一键记录回滚",
		ZhTW: "多向同步前自動備份目標檔案，支援一鍵記錄回滾",
		EnUS: "Automatic backup before sync with one-click journal rollback",
	},

	// Kind 2: Text Merge
	"intro.mergeTag": {
		ZhCN: "三方/两方合并 · 智能冲突决策",
		ZhTW: "三方/兩方合併 · 智慧衝突決策",
		EnUS: "3-Way / 2-Way Merge · Conflict Resolution",
	},
	"intro.mergeTitle": {
		ZhCN: "文本合并",
		ZhTW: "文本合併",
		EnUS: "Text Merge",
	},
	"intro.mergeDesc": {
		ZhCN: "输入左右两侧文件，有共同祖先时可指定基准文件，进行逐块冲突比对与合并决策。",
		ZhTW: "輸入左右兩側檔案，有共同祖先時可指定基準檔案，進行逐塊衝突比對與合併決策。",
		EnUS: "Enter left and right files with optional base ancestor to resolve conflicts.",
	},
	"intro.mergeFeat1Title": {
		ZhCN: "共同祖先基准",
		ZhTW: "共同祖先基準",
		EnUS: "Base Ancestor",
	},
	"intro.mergeFeat1Desc": {
		ZhCN: "支持 Git 风格三方合并，精确定位两路改动冲突点",
		ZhTW: "支援 Git 風格三方合併，精確定位兩路改動衝突點",
		EnUS: "Git-style 3-way merge to isolate two-branch conflict hunks",
	},
	"intro.mergeFeat2Title": {
		ZhCN: "逐块采纳决策",
		ZhTW: "逐塊採納決策",
		EnUS: "Hunk Actions",
	},
	"intro.mergeFeat2Desc": {
		ZhCN: "一键采纳左侧、采纳右侧或保留修改，解决冲突",
		ZhTW: "一鍵採納左側、採納右側或保留修改，解決衝突",
		EnUS: "One-click adopt left, right, or custom edits per conflict",
	},
	"intro.mergeFeat3Title": {
		ZhCN: "合并输出预览",
		ZhTW: "合併輸出預覽",
		EnUS: "Output Preview",
	},
	"intro.mergeFeat3Desc": {
		ZhCN: "实时生成最终合并结果，可自由编辑并直接保存文件",
		ZhTW: "即時生成最終合併結果，可自由編輯並直接儲存檔案",
		EnUS: "Live preview of merge results with editable output buffer",
	},

	// Kind 3: Picture Compare
	"intro.pictureTag": {
		ZhCN: "像素级图像比对 · 视觉差异可视化",
		ZhTW: "像素級圖像比對 · 視覺差異視覺化",
		EnUS: "Pixel-Level Diff · Visual Inspection",
	},
	"intro.pictureTitle": {
		ZhCN: "图片比较",
		ZhTW: "圖片比較",
		EnUS: "Picture Compare",
	},
	"intro.pictureDesc": {
		ZhCN: "选择两张 PNG、JPEG 或 GIF 格式图片，以像素级精度对比视觉差异。",
		ZhTW: "選擇兩張 PNG、JPEG 或 GIF 格式圖片，以像素級精度比對視覺差異。",
		EnUS: "Select two PNG, JPEG, or GIF images to inspect pixel differences.",
	},
	"intro.pictureFeat1Title": {
		ZhCN: "多重视角比对",
		ZhTW: "多重視角比對",
		EnUS: "Multiple Views",
	},
	"intro.pictureFeat1Desc": {
		ZhCN: "支持并排排列、半透明覆盖叠加与高亮像素差值模式",
		ZhTW: "支援並排排列、半透明覆蓋疊加與高亮像素差值模式",
		EnUS: "Supports side-by-side, semi-transparent overlay, and pixel difference",
	},
	"intro.pictureFeat2Title": {
		ZhCN: "容差与色彩替换",
		ZhTW: "容差與色彩取代",
		EnUS: "Tolerance & Replace",
	},
	"intro.pictureFeat2Desc": {
		ZhCN: "0–255 容差滑块过滤压缩噪点并标红真实差异，可替换背景色",
		ZhTW: "0–255 容差滑桿過濾壓縮噪點並標紅真實差異，可取代背景色",
		EnUS: "0–255 tolerance slider filters noise, marks real differences in red, replaces background color",
	},
	"intro.pictureFeat3Title": {
		ZhCN: "缩放适应与翻转",
		ZhTW: "縮放適應與翻轉",
		EnUS: "Zoom & Transform",
	},
	"intro.pictureFeat3Desc": {
		ZhCN: "自动适应视口与 1:1、200% 缩放；翻转左图或右图后，所有模式按翻转结果比较",
		ZhTW: "自動適應視口與 1:1、200% 縮放；翻轉左圖或右圖後，所有模式按翻轉結果比較",
		EnUS: "Fit, 1:1 and 200% zoom; flip either image and every mode compares the flipped result",
	},

	// Kind 4: Media Compare
	"intro.mediaTag": {
		ZhCN: "音频 ID3v2 标签 · 元数据结构比对",
		ZhTW: "音訊 ID3v2 標籤 · 元數據結構比對",
		EnUS: "Audio ID3v2 Tags & Metadata Comparison",
	},
	"intro.mediaTitle": {
		ZhCN: "媒体标签",
		ZhTW: "媒體標籤",
		EnUS: "Media Tags",
	},
	"intro.mediaDesc": {
		ZhCN: "选择两个音频文件，直接比对 ID3v2 元数据标签与头部信息差异。",
		ZhTW: "選擇兩個音訊檔案，直接比對 ID3v2 元數據標籤與頭部資訊差異。",
		EnUS: "Select two audio files to compare ID3v2 metadata tags and header attributes.",
	},
	"intro.mediaFeat1Title": {
		ZhCN: "核心标签比对",
		ZhTW: "核心標籤比對",
		EnUS: "Core Metadata",
	},
	"intro.mediaFeat1Desc": {
		ZhCN: "解析对比歌曲标题、艺术家、专辑、曲目编号与年份",
		ZhTW: "解析對比歌曲標題、藝術家、專輯、曲目編號與年份",
		EnUS: "Extracts title, artist, album, track number, and release year",
	},
	"intro.mediaFeat2Title": {
		ZhCN: "头部极速分析",
		ZhTW: "頭部極速分析",
		EnUS: "Instant Header Parse",
	},
	"intro.mediaFeat2Desc": {
		ZhCN: "只读取音频头部标签，无需全量解码波形音频数据",
		ZhTW: "只讀取音訊頭部標籤，無需全量解碼波形音訊數據",
		EnUS: "Lightweight header reading without decoding full audio streams",
	},
	"intro.mediaFeat3Title": {
		ZhCN: "差异直观高亮",
		ZhTW: "差異直觀高亮",
		EnUS: "Visual Diff Highlighting",
	},
	"intro.mediaFeat3Desc": {
		ZhCN: "字段级差异高亮展示，快速找出元数据录入不一致",
		ZhTW: "欄位級差異高亮展示，快速找出元數據輸入不一致",
		EnUS: "Highlights mismatched fields clearly to identify metadata discrepancies",
	},

	// Kind 5: Version & Registry
	"intro.versionTag": {
		ZhCN: "PE 文件版本资源与注册表导出对比",
		ZhTW: "PE 檔案版本資源與登錄檔匯出比對",
		EnUS: "PE FileVersion & Registry Export Diff",
	},
	"intro.versionTitle": {
		ZhCN: "版本与注册表",
		ZhTW: "版本與登錄檔",
		EnUS: "Version & Registry",
	},
	"intro.versionDesc": {
		ZhCN: "选择两个 Windows 可执行文件 (EXE/DLL) 或两个 .reg 注册表导出文件进行结构比对。",
		ZhTW: "選擇兩個 Windows 執行檔 (EXE/DLL) 或兩個 .reg 登錄檔匯出檔案進行結構比對。",
		EnUS: "Select two Windows executables (EXE/DLL) or two .reg registry files to compare.",
	},
	"intro.versionFeat1Title": {
		ZhCN: "PE FileVersion 比对",
		ZhTW: "PE FileVersion 比對",
		EnUS: "FileVersion Inspection",
	},
	"intro.versionFeat1Desc": {
		ZhCN: "解析文件版本号、产品版本、公司名、版权信息与说明",
		ZhTW: "解析檔案版本號、產品版本、公司名、版權資訊與說明",
		EnUS: "Parses file version, product version, company name, and copyright",
	},
	"intro.versionFeat2Title": {
		ZhCN: ".reg 键值对比",
		ZhTW: ".reg 鍵值對比",
		EnUS: "Registry Key & Value Diff",
	},
	"intro.versionFeat2Desc": {
		ZhCN: "分析注册表路径、键名、数据类型与键值内容的变更",
		ZhTW: "分析登錄檔路徑、鍵名、資料類型與鍵值內容的變更",
		EnUS: "Tracks registry paths, value names, types, and data modifications",
	},
	"intro.versionFeat3Title": {
		ZhCN: "离线静态安全",
		ZhTW: "離線靜態安全",
		EnUS: "Offline & Safe",
	},
	"intro.versionFeat3Desc": {
		ZhCN: "纯离线静态文件解析，不读取或修改本机实时注册表",
		ZhTW: "純離線靜態檔案解析，不讀取或修改本機即時登錄檔",
		EnUS: "Offline static file parsing without querying or modifying live host registry",
	},

	"intro.tableTag": {
		ZhCN: "CSV / TSV / Excel / HTML",
		ZhTW: "CSV / TSV / Excel / HTML",
		EnUS: "CSV / TSV / Excel / HTML",
	},
	"intro.tableTitle": {
		ZhCN: "结构化数据表格对齐与单元格级比对",
		ZhTW: "結構化資料表格對齊與儲存格級比對",
		EnUS: "Structured Table Alignment & Cell-level Diff",
	},
	"intro.tableDesc": {
		ZhCN: "按行号对齐比较工作表，精准定位行列与单元格差异；命令行另支持主键对齐、列映射与数值容差。",
		ZhTW: "按列號對齊比較工作表，精準定位行列與儲存格差異；命令列另支援主鍵對齊、欄映射與數值容差。",
		EnUS: "Compare CSV, TSV, XLSX, and HTML tables row by row; the CLI adds key-based alignment, column mapping, and numeric tolerance.",
	},
	"intro.tableFeat1Title": {
		ZhCN: "多格式支持",
		ZhTW: "多格式支援",
		EnUS: "Multi-Format Support",
	},
	"intro.tableFeat1Desc": {
		ZhCN: "开箱即用解析 CSV、TSV、Excel (.xlsx) 工作表与 HTML <table> 节点。",
		ZhTW: "開箱即用解析 CSV、TSV、Excel (.xlsx) 工作表與 HTML <table> 節點。",
		EnUS: "Parse and compare CSV, TSV, Excel sheets, and HTML table elements out of the box.",
	},
	"intro.tableFeat2Title": {
		ZhCN: "复合主键对齐",
		ZhTW: "複合主鍵對齊",
		EnUS: "Key-Based Alignment",
	},
	"intro.tableFeat2Desc": {
		ZhCN: "命令行 --keys 可指定一列或多列作为主键，自动匹配对应记录，识别增删与重复。",
		ZhTW: "命令列 --keys 可指定一欄或多欄作為主鍵，自動匹配對應記錄，識別增刪與重複。",
		EnUS: "With the CLI --keys option, align records on one or more key columns to detect added, removed, and duplicate rows.",
	},
	"intro.tableFeat3Title": {
		ZhCN: "数值公差与列映射",
		ZhTW: "數值容差與欄映射",
		EnUS: "Tolerance & Mapping",
	},
	"intro.tableFeat3Desc": {
		ZhCN: "命令行 --tolerance 忽略数值微小扰动，--map 可映射排布不同的列。",
		ZhTW: "命令列 --tolerance 忽略數值微小擾動，--map 可映射排布不同的欄。",
		EnUS: "Use the CLI --tolerance option to ignore small numeric differences and --map to align differently ordered columns.",
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
	"settings.updateSection": {
		ZhCN: "软件更新",
		ZhTW: "軟體更新",
		EnUS: "Software Update",
	},
	"settings.currentVersion": {
		ZhCN: "当前版本：%s",
		ZhTW: "目前版本：%s",
		EnUS: "Current version: %s",
	},
	"settings.checkUpdate": {
		ZhCN: "检查更新",
		ZhTW: "檢查更新",
		EnUS: "Check for Updates",
	},
	"settings.checkingUpdate": {
		ZhCN: "正在检查更新...",
		ZhTW: "正在檢查更新...",
		EnUS: "Checking for updates...",
	},
	"settings.updateLatest": {
		ZhCN: "当前已是最新版本",
		ZhTW: "目前已是最新版本",
		EnUS: "You're up to date",
	},
	"settings.updateAvailable": {
		ZhCN: "发现新版本：%s",
		ZhTW: "發現新版本：%s",
		EnUS: "New version available: %s",
	},
	"settings.updateFailed": {
		ZhCN: "检查更新失败",
		ZhTW: "檢查更新失敗",
		EnUS: "Update check failed",
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
