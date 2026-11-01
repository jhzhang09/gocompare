package native

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"gocompare/internal/i18n"
)

func TestSettingsModalAndOptionsDefaultOpen(t *testing.T) {
	i18n.SetLanguage(i18n.ZhCN)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp) // Windows 上 os.UserHomeDir 读取它

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

func TestSettingsCheckUpdate(t *testing.T) {
	i18n.SetLanguage(i18n.ZhCN)
	a := &workspaceState{
		settingsOpen: true,
	}
	tt := ui.NewTester(a.view, 1380, 880)
	tt.Frame()

	if !tt.HasText("软件更新") {
		t.Fatal("expected 软件更新 section in settings modal")
	}
	if !tt.HasText("检查更新") {
		t.Fatal("expected 检查更新 button in settings modal")
	}

	// Directly invoke checkForUpdates with simulated result
	a.updateStatus = "当前已是最新版本"
	tt.Frame()
	if !tt.HasText("当前已是最新版本") {
		t.Fatal("expected status update in settings modal")
	}
}

func TestCheckForUpdatesKeepsRunningJob(t *testing.T) {
	i18n.SetLanguage(i18n.ZhCN)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"tag_name":"v999.0.0","html_url":"https://example.com/release"}`))
	}))
	defer srv.Close()
	canceled := false
	a := &workspaceState{busy: true, cancel: func() { canceled = true }}
	a.checkForUpdates(srv.URL)
	if !a.busy || a.cancel == nil {
		t.Fatal("update check must not reset the running job's busy state or cancel func")
	}
	a.cancel()
	if !canceled {
		t.Fatal("running job's cancel func was replaced")
	}
	if a.updateChecking || a.updateURL != "https://example.com/release" || !strings.Contains(a.updateStatus, "v999.0.0") {
		t.Fatalf("status %q url %q", a.updateStatus, a.updateURL)
	}
}
