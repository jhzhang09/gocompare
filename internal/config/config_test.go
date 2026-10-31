package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigLoadSave(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

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
