package native

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"gocompare/internal/i18n"
)

func TestSettingsModalAndOptionsDefaultOpen(t *testing.T) {
	i18n.SetLanguage(i18n.ZhCN)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	a := &workspaceState{
		optionsOpen: true,
	}
	a.selectKind(6)

	tt := ui.NewTester(a.view, 1380, 880)
	tt.Frame()

	// Options drawer should be open by default
	if !a.optionsOpen {
		t.Fatal("options drawer should be open by default")
	}

	// Click settings in sidebar
	if err := tt.Click("设置"); err != nil {
		t.Fatalf("failed to click 设置: %v", err)
	}
	tt.Frame()

	if !a.settingsOpen {
		t.Fatal("settings dialog should be open")
	}

	// Switch language to English
	a.applySettings(2, 1) // en, dark
	tt.Frame()

	if i18n.CurrentLanguage() != i18n.EnUS {
		t.Fatalf("expected EnUS, got %s", i18n.CurrentLanguage())
	}

	// Close settings modal
	if err := tt.Click("Close"); err != nil {
		t.Fatalf("failed to click Close button in settings modal: %v", err)
	}
	tt.Frame()

	if a.settingsOpen {
		t.Fatal("settings dialog should be closed")
	}

	// Restore language to ZhCN
	i18n.SetLanguage(i18n.ZhCN)
}
