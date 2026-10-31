package i18n

import "testing"

func TestI18nTranslations(t *testing.T) {
	// Default ZhCN
	SetLanguage(ZhCN)
	if got := T("mode.scratch"); got != "文本比较" {
		t.Fatalf("expected 文本比较, got %s", got)
	}
	if got := T("settings.appearance"); got != "显示设置" {
		t.Fatalf("expected 显示设置, got %s", got)
	}

	// Switch to ZhTW
	SetLanguage(ZhTW)
	if got := T("mode.scratch"); got != "文本比較" {
		t.Fatalf("expected 文本比較, got %s", got)
	}
	if got := T("settings.appearance"); got != "顯示設定" {
		t.Fatalf("expected 顯示設定, got %s", got)
	}

	// Switch to EnUS
	SetLanguage(EnUS)
	if got := T("mode.scratch"); got != "Text Compare" {
		t.Fatalf("expected Text Compare, got %s", got)
	}
	if got := T("settings.appearance"); got != "Appearance" {
		t.Fatalf("expected Appearance, got %s", got)
	}

	// Fallback to key if unknown
	if got := T("non.existent.key"); got != "non.existent.key" {
		t.Fatalf("expected key fallback, got %s", got)
	}

	// Restore to ZhCN
	SetLanguage(ZhCN)
}
