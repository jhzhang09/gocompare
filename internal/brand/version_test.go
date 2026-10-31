package brand

import "testing"

func TestAppVersionPrefersInjectedValue(t *testing.T) {
	old := Version
	defer func() { Version = old }()
	Version = "v1.2.3"
	if got := AppVersion(); got != "v1.2.3" {
		t.Fatalf("got %q", got)
	}
	Version = "dev"
	if got := AppVersion(); got == "" {
		t.Fatal("development builds must still report a version")
	}
}
