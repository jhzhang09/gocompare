package table

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestCSVKeysToleranceAndMapping(t *testing.T) {
	left, err := ParseCSV([]byte("id,name,qty\n1,a,1.00\n1,a,9\n2,b,3\n"))
	if err != nil {
		t.Fatal(err)
	}
	right, err := ParseCSV([]byte("name,id,qty\na,1,1.005\nb,2,4\n"))
	if err != nil {
		t.Fatal(err)
	}
	diffs := Compare(left, right, Options{Keys: []int{0, 1}, Map: []int{1, 0, 2}, Tolerance: 0.01})
	kinds := map[string]int{}
	for _, d := range diffs {
		kinds[d.Kind]++
		if d.Kind == "changed" && !(d.Key == "2\x1fb" && d.Left == "3" && d.Right == "4") {
			t.Fatalf("unexpected change: %+v", d)
		}
	}
	if kinds["duplicate"] != 1 || kinds["changed"] != 1 {
		t.Fatalf("diffs: %+v", diffs)
	}
	tsv, err := ParseTSV([]byte("a\tb\n"))
	if err != nil || len(tsv.Rows) != 1 || tsv.Rows[0][1] != "b" {
		t.Fatalf("tsv: %+v %v", tsv, err)
	}
}

func TestXLSXAndHTML(t *testing.T) {
	raw := buildXLSX(t)
	sheets, err := ParseXLSX(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(sheets) != 2 || sheets[0].Name != "One" || sheets[0].Rows[0][0] != "shared" || sheets[1].Rows[0][0] != "inline" {
		t.Fatalf("xlsx: %+v", sheets)
	}
	htmlSheets := ParseHTML(`<table><tr><td>a</td><td>b</td></tr></table><table><tr><th>c</th></tr></table>`)
	if len(htmlSheets) != 2 || htmlSheets[0].Rows[0][1] != "b" || htmlSheets[1].Rows[0][0] != "c" {
		t.Fatalf("html: %+v", htmlSheets)
	}
}

func buildXLSX(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := map[string]string{
		"xl/workbook.xml":            `<?xml version="1.0"?><workbook xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="One" sheetId="1" r:id="rId1"/><sheet name="Two" sheetId="2" r:id="rId2"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="worksheet" Target="worksheets/sheet2.xml"/></Relationships>`,
		"xl/sharedStrings.xml":       `<?xml version="1.0"?><sst><si><t>shared</t></si></sst>`,
		"xl/worksheets/sheet1.xml":   `<?xml version="1.0"?><worksheet><sheetData><row><c t="s"><v>0</v></c></row></sheetData></worksheet>`,
		"xl/worksheets/sheet2.xml":   `<?xml version="1.0"?><worksheet><sheetData><row><c t="inlineStr"><is><t>inline</t></is></c></row></sheetData></worksheet>`,
	}
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
