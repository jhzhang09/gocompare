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

// RemoteBookmark 是最近使用的远程连接，用于连接对话框一键填写。绝不保存密码。
type RemoteBookmark struct {
	Scheme       string `json:"scheme"`
	Host         string `json:"host"`
	Port         string `json:"port,omitempty"`
	User         string `json:"user,omitempty"`
	Domain       string `json:"domain,omitempty"`
	Share        string `json:"share,omitempty"`
	Dir          string `json:"dir,omitempty"`
	IdentityFile string `json:"identityFile,omitempty"`
}

// maxRemoteBookmarks 限制最近连接的条数。
const maxRemoteBookmarks = 10

// LoadRemotes 读取最近连接；文件不存在或损坏时返回空列表。
func LoadRemotes(path string) []RemoteBookmark {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var list []RemoteBookmark
	if json.Unmarshal(data, &list) != nil {
		return nil
	}
	return list
}

// AddRemote 把连接放到最前面（同一协议、用户、主机、端口、共享只保留一条）并写回文件。
// 文件只允许本人读写，因为其中包含主机与用户名。
func AddRemote(path string, b RemoteBookmark) ([]RemoteBookmark, error) {
	list := []RemoteBookmark{b}
	for _, old := range LoadRemotes(path) {
		if old.Scheme == b.Scheme && old.Host == b.Host && old.Port == b.Port && old.User == b.User && old.Domain == b.Domain && old.Share == b.Share {
			continue
		}
		list = append(list, old)
	}
	if len(list) > maxRemoteBookmarks {
		list = list[:maxRemoteBookmarks]
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	return list, os.WriteFile(path, data, 0o600)
}
