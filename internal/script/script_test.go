package script

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScriptUsesCompareHexAndMerge(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "left")
	right := filepath.Join(dir, "right")
	base := filepath.Join(dir, "base")
	if err := os.WriteFile(left, []byte("aaaa"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(right, []byte("aaab"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(base, []byte("a\nb\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(left+"m", []byte("a\nleft\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(right+"m", []byte("a\nright\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	src := "hex " + left + " " + right + "\nmerge " + base + " " + left + "m " + right + "m\n"
	var out bytes.Buffer
	code, err := Execute(context.Background(), src, &out)
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 || !strings.Contains(out.String(), "offset 0") || !strings.Contains(out.String(), "<<<<<<< left") {
		t.Fatalf("%d %s", code, out.String())
	}
}

func TestHexRejectsOversizedFiles(t *testing.T) {
	dir := t.TempDir()
	big := filepath.Join(dir, "big")
	if err := os.WriteFile(big, make([]byte, maxHexBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, err := Execute(context.Background(), "hex "+big+" "+big, io.Discard); code != 2 || err == nil {
		t.Fatalf("oversized hex input accepted: %d %v", code, err)
	}
}
