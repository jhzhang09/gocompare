package paginate

import (
	"strings"
	"testing"
)

func TestPagesKeepText(t *testing.T) {
	text := "a\nb\nc\nd\ne"
	pages := Pages(text, 10, 2)
	if len(pages) != 3 || pages[0] != "a\nb" || pages[1] != "c\nd" || pages[2] != "e" {
		t.Fatalf("%q", pages)
	}
	if strings.Join(pages, "\n") != text {
		t.Fatalf("joined %q", strings.Join(pages, "\n"))
	}
	wide := Pages("你好世界", 2, 1)
	if len(wide) != 2 || wide[0] != "你好" || wide[1] != "世界" {
		t.Fatalf("%q", wide)
	}
	if Pages("abc", 0, 1) != nil {
		t.Fatal("invalid width still paginated")
	}
}
