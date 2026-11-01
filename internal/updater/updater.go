// Package updater 提供 GoCompare 的版本检查与更新发现功能。
package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// DefaultReleaseURL 是 GitHub 最新发布的默认查询地址。
const DefaultReleaseURL = "https://api.github.com/repos/jhzhang09/gocompare/releases/latest"

// MaxResponseBytes 限制单次响应读取上限为 1 MiB。
const MaxResponseBytes = 1 << 20

// Release 代表远程发布的元数据。
type Release struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []Asset   `json:"assets"`
}

// Asset 代表发布中附带的二进制或压缩包产物。
type Asset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// CheckResult 包含更新检查的完整结果。
type CheckResult struct {
	CurrentVersion string   `json:"current_version"`
	LatestVersion  string   `json:"latest_version"`
	HasUpdate      bool     `json:"has_update"`
	DownloadURL    string   `json:"download_url"`
	ReleaseNotes   string   `json:"release_notes"`
	Release        *Release `json:"release,omitempty"`
}

// Product 区分发布中的产物类别：同一个 Release 同时包含桌面应用安装包和 CLI 压缩包。
type Product int

const (
	// ProductCLI 匹配名称含 "cli" 的命令行压缩包。
	ProductCLI Product = iota
	// ProductApp 匹配桌面应用安装包（dmg、安装程序、deb、免安装包等）。
	ProductApp
)

// Check 查询远程发布地址，并与当前版本进行对比；下载地址按 product 选择对应产物。
// 若 apiURL 为空，则默认使用 DefaultReleaseURL。
func Check(ctx context.Context, apiURL string, currentVersion string, product Product) (*CheckResult, error) {
	if apiURL == "" {
		apiURL = DefaultReleaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create update request: %w", err)
	}
	req.Header.Set("User-Agent", "GoCompare/"+currentVersion)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch update info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from update server", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("read update response: %w", err)
	}

	var rel Release
	if err := json.Unmarshal(data, &rel); err != nil {
		return nil, fmt.Errorf("parse release JSON: %w", err)
	}

	hasUpdate := IsNewer(rel.TagName, currentVersion)
	res := &CheckResult{
		CurrentVersion: currentVersion,
		LatestVersion:  rel.TagName,
		HasUpdate:      hasUpdate,
		DownloadURL:    httpsOnly(rel.HTMLURL),
		ReleaseNotes:   rel.Body,
		Release:        &rel,
	}

	// 尝试寻找与当前平台匹配的产物；下载地址只接受 https，避免把服务端返回的任意 scheme 展示给用户。
	if asset := FindAsset(rel.Assets, runtime.GOOS, runtime.GOARCH, product); asset != nil && httpsOnly(asset.BrowserDownloadURL) != "" {
		res.DownloadURL = asset.BrowserDownloadURL
	}

	return res, nil
}

func httpsOnly(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return ""
	}
	return raw
}

// IsNewer 比较远程版本是否严格高于当前版本。
// 支持带有前缀 "v" 的语义化版本（例如 v1.2.3 或 1.2.3-rc.1）；数字部分相同时，
// 正式版高于预发布版，预发布标识按 SemVer 规则逐段比较，构建元数据（+xxx）不参与比较。
// 当前版本不是合法版本（如 "dev"）时，只要远程是合法版本就认为有更新。
func IsNewer(remoteVer, currentVer string) bool {
	rNums, rPre, rOK := parseSemver(remoteVer)
	cNums, cPre, cOK := parseSemver(currentVer)
	if !rOK {
		return false
	}
	if !cOK {
		return true
	}
	for i := 0; i < max(len(rNums), len(cNums)); i++ {
		rVal, cVal := 0, 0
		if i < len(rNums) {
			rVal = rNums[i]
		}
		if i < len(cNums) {
			cVal = cNums[i]
		}
		if rVal != cVal {
			return rVal > cVal
		}
	}
	return comparePrerelease(rPre, cPre) > 0
}

// parseSemver 拆出数字段和预发布标识段。
func parseSemver(v string) ([]int, []string, bool) {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	if idx := strings.IndexByte(v, '+'); idx >= 0 {
		v = v[:idx]
	}
	var pre []string
	if idx := strings.IndexByte(v, '-'); idx >= 0 {
		if v[idx+1:] == "" {
			return nil, nil, false
		}
		pre = strings.Split(v[idx+1:], ".")
		v = v[:idx]
	}
	var nums []int
	for _, p := range strings.Split(v, ".") {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return nil, nil, false
		}
		nums = append(nums, n)
	}
	return nums, pre, len(nums) > 0
}

// comparePrerelease 按 SemVer 第 11 条比较：无预发布标识的版本更高；
// 数字标识按数值、其他按 ASCII 比较，数字标识低于非数字标识，前缀相同时段数多者更高。
func comparePrerelease(a, b []string) int {
	switch {
	case len(a) == 0 && len(b) == 0:
		return 0
	case len(a) == 0:
		return 1
	case len(b) == 0:
		return -1
	}
	for i := 0; i < min(len(a), len(b)); i++ {
		an, aErr := strconv.Atoi(a[i])
		bn, bErr := strconv.Atoi(b[i])
		switch {
		case aErr == nil && bErr == nil:
			if an != bn {
				if an > bn {
					return 1
				}
				return -1
			}
		case aErr == nil:
			return -1
		case bErr == nil:
			return 1
		default:
			if c := strings.Compare(a[i], b[i]); c != 0 {
				return c
			}
		}
	}
	switch {
	case len(a) > len(b):
		return 1
	case len(a) < len(b):
		return -1
	}
	return 0
}

// archAliases 列出常见发布命名中与 GOARCH 等价的写法。
var archAliases = map[string][]string{
	"amd64": {"amd64", "x86_64", "x64"},
	"arm64": {"arm64", "aarch64"},
	"386":   {"386", "i386", "x86"},
}

// osAliases 列出常见发布命名中与 GOOS 等价的写法。
var osAliases = map[string][]string{
	"darwin":  {"darwin", "macos", "mac"},
	"windows": {"windows", "win"},
	"linux":   {"linux"},
}

// skipAssetSuffixes 是校验和、签名、SBOM 等附属文件，不能作为下载包。
var skipAssetSuffixes = []string{".sha256", ".sha512", ".sig", ".asc", ".pem", ".sbom", ".spdx", ".json", ".txt", ".intoto.jsonl"}

// extOS 是安装包格式本身隐含的平台：dmg、deb 的文件名里不一定带系统名。
var extOS = map[string]string{".dmg": "darwin", ".pkg": "darwin", ".deb": "linux", ".rpm": "linux", ".exe": "windows", ".msi": "windows"}

// FindAsset 按照产物类别、OS 和架构在产物列表中查找对应的下载包。
// 名称按非字母数字切分成词后整词匹配，避免 "arm" 误中 "arm64"；
// 名称里没有任何架构词的 macOS 包（universal dmg）视为适用所有架构。
func FindAsset(assets []Asset, goos, goarch string, product Product) *Asset {
	goos = strings.ToLower(goos)
	wantOS := aliases(osAliases, goos)
	wantArch := aliases(archAliases, strings.ToLower(goarch))
	for i := range assets {
		name := strings.ToLower(assets[i].Name)
		skip := false
		for _, suffix := range skipAssetSuffixes {
			if strings.HasSuffix(name, suffix) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		words := map[string]bool{}
		for _, w := range strings.FieldsFunc(name, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
		}) {
			words[w] = true
		}
		if words["cli"] != (product == ProductCLI) {
			continue
		}
		osOK := anyWord(words, wantOS) || extOS[path.Ext(name)] == goos
		archOK := anyWord(words, wantArch) || (goos == "darwin" && !hasArch(words))
		if osOK && archOK {
			return &assets[i]
		}
	}
	return nil
}

func aliases(table map[string][]string, key string) []string {
	if list, ok := table[key]; ok {
		return list
	}
	return []string{key}
}

func hasArch(words map[string]bool) bool {
	for _, list := range archAliases {
		if anyWord(words, list) {
			return true
		}
	}
	return words["arm"]
}

func anyWord(words map[string]bool, candidates []string) bool {
	for _, c := range candidates {
		if words[c] {
			return true
		}
	}
	return false
}
