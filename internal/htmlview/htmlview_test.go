package htmlview

import (
	"bytes"
	"strings"
	"testing"
)

// 块按原文顺序输出、实体只转义一次，<pre> 不被当成 <p>。
func TestBlocksOrderEntitiesAndTagBoundary(t *testing.T) {
	blocks := Blocks(`<p class="x">first &amp; second</p><h1>Title</h1><pre>code</pre><P>last</P>`)
	want := []Block{{"p", "first & second"}, {"h1", "Title"}, {"p", "last"}}
	if len(blocks) != len(want) {
		t.Fatalf("blocks %+v", blocks)
	}
	for i := range want {
		if blocks[i] != want[i] {
			t.Fatalf("block %d = %+v, want %+v", i, blocks[i], want[i])
		}
	}
	var buf bytes.Buffer
	if err := WriteReport(&buf, "t", blocks); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "<p>first &amp; second</p>") || strings.Contains(buf.String(), "&amp;amp;") {
		t.Fatalf("report %s", buf.String())
	}
}
