package updater

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIsNewer(t *testing.T) {
	tests := []struct {
		remote  string
		current string
		want    bool
	}{
		{"v1.2.4", "v1.2.3", true},
		{"v1.3.0", "v1.2.9", true},
		{"v2.0.0", "v1.9.9", true},
		{"v1.2.3", "v1.2.3", false},
		{"v1.2.2", "v1.2.3", false},
		{"1.2.4", "1.2.3", true},
		{"v1.2.4-rc1", "v1.2.3", true},
		{"v1.0.0", "dev", true},
		{"invalid", "v1.0.0", false},
		// 正式版高于同号预发布版，预发布按 SemVer 逐段比较。
		{"v1.0.0", "v1.0.0-rc1", true},
		{"v1.0.0-rc1", "v1.0.0", false},
		{"v1.0.0-rc.10", "v1.0.0-rc.2", true},
		{"v1.0.0-beta", "v1.0.0-alpha.1", true},
		{"v1.0.0-alpha.1", "v1.0.0-alpha", true},
		{"v1.0.0+build2", "v1.0.0+build1", false},
	}

	for _, tt := range tests {
		got := IsNewer(tt.remote, tt.current)
		if got != tt.want {
			t.Errorf("IsNewer(%q, %q) = %v; want %v", tt.remote, tt.current, got, tt.want)
		}
	}
}

func TestFindAsset(t *testing.T) {
	assets := []Asset{
		{Name: "gocompare-cli_1.0.0_darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/darwin_arm64"},
		{Name: "gocompare-cli_1.0.0_linux_amd64.tar.gz", BrowserDownloadURL: "https://example.com/linux_amd64"},
		{Name: "gocompare-cli_1.0.0_windows_amd64.zip", BrowserDownloadURL: "https://example.com/windows_amd64"},
	}

	a := FindAsset(assets, "darwin", "arm64", ProductCLI)
	if a == nil || a.BrowserDownloadURL != "https://example.com/darwin_arm64" {
		t.Fatalf("unexpected darwin arm64 asset: %v", a)
	}

	a = FindAsset(assets, "windows", "amd64", ProductCLI)
	if a == nil || a.BrowserDownloadURL != "https://example.com/windows_amd64" {
		t.Fatalf("unexpected windows amd64 asset: %v", a)
	}

	a = FindAsset(assets, "freebsd", "386", ProductCLI)
	if a != nil {
		t.Fatalf("expected nil for unsupported platform, got: %v", a)
	}
}

func TestCheckUpdate(t *testing.T) {
	mockRelease := Release{
		TagName:     "v1.5.0",
		Name:        "Release 1.5.0",
		Body:        "Bug fixes and improvements",
		HTMLURL:     "https://github.com/jhzhang09/gocompare/releases/tag/v1.5.0",
		PublishedAt: time.Now(),
		Assets: []Asset{
			{Name: "gocompare-cli_1.5.0_darwin_arm64.tar.gz", BrowserDownloadURL: "https://dl.example.com/darwin_arm64"},
			{Name: "gocompare-cli_1.5.0_darwin_amd64.tar.gz", BrowserDownloadURL: "https://dl.example.com/darwin_amd64"},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/vnd.github.v3+json" {
			t.Errorf("missing or wrong Accept header: %s", r.Header.Get("Accept"))
		}
		_ = json.NewEncoder(w).Encode(mockRelease)
	}))
	defer ts.Close()

	ctx := context.Background()

	// 1. Current version older -> has update
	res, err := Check(ctx, ts.URL, "v1.4.0", ProductCLI)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if !res.HasUpdate {
		t.Errorf("expected HasUpdate = true")
	}
	if res.LatestVersion != "v1.5.0" {
		t.Errorf("expected LatestVersion v1.5.0, got %s", res.LatestVersion)
	}
	if res.ReleaseNotes != "Bug fixes and improvements" {
		t.Errorf("unexpected release notes: %s", res.ReleaseNotes)
	}

	// 2. Current version equal -> no update
	res, err = Check(ctx, ts.URL, "v1.5.0", ProductCLI)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if res.HasUpdate {
		t.Errorf("expected HasUpdate = false for identical version")
	}
}

func TestCheckUpdateErrors(t *testing.T) {
	// 1. Server returns 500
	tsErr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer tsErr.Close()

	_, err := Check(context.Background(), tsErr.URL, "v1.0.0", ProductCLI)
	if err == nil {
		t.Fatal("expected error on HTTP 500")
	}

	// 2. Server returns invalid JSON
	tsBadJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer tsBadJSON.Close()

	_, err = Check(context.Background(), tsBadJSON.URL, "v1.0.0", ProductCLI)
	if err == nil {
		t.Fatal("expected error on invalid JSON")
	}
}

func TestFindAssetWholeWords(t *testing.T) {
	assets := []Asset{
		{Name: "gocompare-cli_1.0.0_linux_arm64.tar.gz.sha256", BrowserDownloadURL: "https://example.com/sum"},
		{Name: "gocompare-cli_1.0.0_linux_arm64.tar.gz", BrowserDownloadURL: "https://example.com/arm64"},
		{Name: "gocompare-cli_1.0.0_linux_arm.tar.gz", BrowserDownloadURL: "https://example.com/arm"},
	}
	if a := FindAsset(assets, "linux", "arm", ProductCLI); a == nil || a.BrowserDownloadURL != "https://example.com/arm" {
		t.Fatalf("arm matched %+v", a)
	}
	if a := FindAsset(assets, "linux", "arm64", ProductCLI); a == nil || a.BrowserDownloadURL != "https://example.com/arm64" {
		t.Fatalf("checksum or wrong asset matched %+v", a)
	}
}

func TestFindAssetByProduct(t *testing.T) {
	// 与 release.yml / package-cli.sh 的实际命名一致。
	assets := []Asset{
		{Name: "gocompare-cli_0.2.0_darwin_arm64.tar.gz"},
		{Name: "gocompare-cli_0.2.0_linux_amd64.tar.gz"},
		{Name: "gocompare-cli_0.2.0_windows_amd64.zip"},
		{Name: "gocompare-0.2.0.dmg"},
		{Name: "gocompare-setup-0.2.0-amd64.exe"},
		{Name: "gocompare-0.2.0-windows-arm64-portable.zip"},
		{Name: "gocompare_0.2.0_amd64.deb"},
		{Name: "gocompare-0.2.0-linux-arm64.tar.gz"},
		{Name: "SHA256SUMS.txt"},
	}
	cases := []struct {
		goos, goarch string
		product      Product
		want         string
	}{
		{"darwin", "arm64", ProductApp, "gocompare-0.2.0.dmg"},
		{"darwin", "amd64", ProductApp, "gocompare-0.2.0.dmg"},
		{"darwin", "arm64", ProductCLI, "gocompare-cli_0.2.0_darwin_arm64.tar.gz"},
		{"windows", "amd64", ProductApp, "gocompare-setup-0.2.0-amd64.exe"},
		{"windows", "arm64", ProductApp, "gocompare-0.2.0-windows-arm64-portable.zip"},
		{"windows", "amd64", ProductCLI, "gocompare-cli_0.2.0_windows_amd64.zip"},
		{"linux", "amd64", ProductApp, "gocompare_0.2.0_amd64.deb"},
		{"linux", "arm64", ProductApp, "gocompare-0.2.0-linux-arm64.tar.gz"},
		{"linux", "amd64", ProductCLI, "gocompare-cli_0.2.0_linux_amd64.tar.gz"},
	}
	for _, tc := range cases {
		a := FindAsset(assets, tc.goos, tc.goarch, tc.product)
		if a == nil || a.Name != tc.want {
			t.Errorf("%s/%s product %d = %+v, want %s", tc.goos, tc.goarch, tc.product, a, tc.want)
		}
	}
}
