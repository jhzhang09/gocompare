package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestConfigLoadSave(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp) // Windows 上 os.UserHomeDir 读取它

	// Default when file does not exist
	s := Load()
	if s.Language != "zh-CN" || s.Theme != "system" {
		t.Fatalf("unexpected default config: %+v", s)
	}

	// Save modified
	newSettings := Settings{
		Language: "en",
		Theme:    "dark",
	}
	if err := Save(newSettings); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Verify file on disk
	cfgPath := filepath.Join(tmp, ".config", "gocompare", "settings.json")
	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf("expected config file at %s: %v", cfgPath, err)
	}

	// Load back
	loaded := Load()
	if loaded.Language != "en" || loaded.Theme != "dark" {
		t.Fatalf("unexpected loaded config: %+v", loaded)
	}
}

func TestRemoteBookmarks(t *testing.T) {
	p := filepath.Join(t.TempDir(), "remotes.json")
	if LoadRemotes(p) != nil {
		t.Fatal("missing file must give an empty list")
	}
	a := RemoteBookmark{Scheme: "sftp", Host: "web1", User: "alice", Dir: "/srv"}
	b := RemoteBookmark{Scheme: "smb", Host: "nas", Share: "docs"}
	AddRemote(p, a)
	AddRemote(p, b)
	a.Dir = "/etc"
	list, err := AddRemote(p, a)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Dir != "/etc" || list[1].Host != "nas" {
		t.Fatalf("list %+v", list)
	}
	for i := 0; i < 15; i++ {
		AddRemote(p, RemoteBookmark{Scheme: "ftp", Host: fmt.Sprintf("h%d", i)})
	}
	if got := LoadRemotes(p); len(got) != maxRemoteBookmarks {
		t.Fatalf("len %d", len(got))
	}
	if info, _ := os.Stat(p); runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", info.Mode())
	}
}
