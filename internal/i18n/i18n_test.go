package i18n

import (
	"regexp"
	"strings"
	"testing"
)

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

// 每个键都要有三种语言，且各语言的格式化动词顺序一致，否则 TArgs 会在某种语言下错位。
func TestAllMessagesComplete(t *testing.T) {
	verbs := regexp.MustCompile(`%[-+#0-9.]*[a-zA-Z%]`)
	for key, langs := range messages {
		want := verbs.FindAllString(langs[ZhCN], -1)
		for _, lang := range []Language{ZhCN, ZhTW, EnUS} {
			text := langs[lang]
			if text == "" {
				t.Errorf("%s missing %s", key, lang)
				continue
			}
			if got := verbs.FindAllString(text, -1); strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("%s %s verbs %v, want %v", key, lang, got, want)
			}
		}
	}
}

func TestMatchesAnyLanguage(t *testing.T) {
	if !Matches("sync.mirrorRight", "镜像到右侧") || !Matches("sync.mirrorRight", "Mirror to right") || Matches("sync.mirrorRight", "向右更新") {
		t.Fatal("Matches must accept every language of the same key only")
	}
}
