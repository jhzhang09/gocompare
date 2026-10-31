package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAutosaveRecoversMultipleSessionsWithoutBuffers(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "left")
	right := filepath.Join(dir, "right")
	base := filepath.Join(dir, "base")
	if err := os.WriteFile(left, []byte("secret-buffer"), 0o600); err != nil {
		t.Fatal(err)
	}
	text := Session{Version: 1, Kind: "text", Left: left, Right: right}
	merged := Session{Version: 1, Kind: "merge", Left: left, Right: right, Base: base}
	path := filepath.Join(dir, "workspace.json")
	if err := Autosave(path, []Session{text, merged}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "secret-buffer") {
		t.Fatal("workspace stored file contents")
	}
	got, err := Recover(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 1 || len(got.Sessions) != 2 || got.Sessions[0].Kind != "text" || got.Sessions[1].Kind != "merge" || got.Sessions[1].Base != base {
		t.Fatalf("%+v", got)
	}
	rel := filepath.Join(dir, "relative.json")
	if err = os.WriteFile(rel, []byte("{\"version\":1,\"sessions\":[{\"version\":1,\"kind\":\"directory\",\"left\":\"L\",\"right\":\"R\"}]}"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = Recover(rel)
	if err != nil || got.Sessions[0].Left != filepath.Join(dir, "L") || got.Sessions[0].Kind != "directory" {
		t.Fatalf("%+v %v", got, err)
	}
	bad := filepath.Join(dir, "bad.json")
	if err = os.WriteFile(bad, []byte(`{"version":2,"sessions":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = Recover(bad); err == nil {
		t.Fatal("accepted workspace version 2")
	}
}
