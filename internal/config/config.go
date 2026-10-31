package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Settings struct {
	Language string `json:"language"` // "zh-CN", "zh-TW", "en"
	Theme    string `json:"theme"`    // "system", "light", "dark"
}

var (
	currentSettings = Settings{
		Language: "zh-CN",
		Theme:    "system",
	}
	configMu sync.Mutex
)

func configFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "gocompare")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.json"), nil
}

func Load() Settings {
	configMu.Lock()
	defer configMu.Unlock()
	p, err := configFilePath()
	if err != nil {
		return currentSettings
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return currentSettings
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err == nil {
		if s.Language != "" {
			currentSettings.Language = s.Language
		}
		if s.Theme != "" {
			currentSettings.Theme = s.Theme
		}
	}
	return currentSettings
}

func Save(s Settings) error {
	configMu.Lock()
	defer configMu.Unlock()
	currentSettings = s
	p, err := configFilePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}

func Current() Settings {
	configMu.Lock()
	defer configMu.Unlock()
	return currentSettings
}
