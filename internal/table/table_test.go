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

func TestParseFileAndCompareSheets(t *testing.T) {
	csvSheets, err := ParseFile("test.csv", []byte("a,b\n1,2\n"))
	if err != nil || len(csvSheets) != 1 || csvSheets[0].Rows[0][0] != "a" {
		t.Fatalf("csv: %+v %v", csvSheets, err)
	}
	tsvSheets, err := ParseFile("test.tsv", []byte("a\tb\n1\t2\n"))
	if err != nil || len(tsvSheets) != 1 || tsvSheets[0].Rows[0][1] != "b" {
		t.Fatalf("tsv: %+v %v", tsvSheets, err)
	}
	diffs := CompareSheets(csvSheets, tsvSheets, Options{})
	if len(diffs) != 0 {
		t.Fatalf("expected identical sheets, got %+v", diffs)
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

func TestXLSXAbsoluteRelationshipTarget(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range map[string]string{
		"xl/workbook.xml":            `<workbook><sheets><sheet name="Abs" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships><Relationship Id="rId1" Target="/xl/worksheets/sheet1.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml":   `<worksheet><sheetData><row><c t="inlineStr"><is><t>abs</t></is></c></row></sheetData></worksheet>`,
	} {
		w, _ := zw.Create(name)
		w.Write([]byte(body))
	}
	zw.Close()
	sheets, err := ParseXLSX(buf.Bytes())
	if err != nil || len(sheets) != 1 || sheets[0].Rows[0][0] != "abs" {
		t.Fatalf("absolute target: %+v %v", sheets, err)
	}
}

func TestColLetter(t *testing.T) {
	cases := []struct {
		col  int
		want string
	}{
		{-1, ""},
		{0, "A"},
		{1, "B"},
		{25, "Z"},
		{26, "AA"},
		{27, "AB"},
		{51, "AZ"},
		{52, "BA"},
		{701, "ZZ"},
		{702, "AAA"},
	}
	for _, tc := range cases {
		if got := ColLetter(tc.col); got != tc.want {
			t.Errorf("ColLetter(%d) = %q, want %q", tc.col, got, tc.want)
		}
	}
}

func TestCompareSheetsMultiSheetByName(t *testing.T) {
	left := []Sheet{
		{Name: "Summary", Rows: [][]string{{"a", "1"}}},
		{Name: "LeftOnly", Rows: [][]string{{"x"}}},
	}
	right := []Sheet{
		{Name: "RightOnly", Rows: [][]string{{"y"}}},
		{Name: "Summary", Rows: [][]string{{"a", "2"}}},
	}

	diffs := CompareSheets(left, right, Options{})
	kinds := map[string]int{}
	for _, d := range diffs {
		kinds[d.Kind]++
	}

	if kinds["changed"] != 1 {
		t.Errorf("expected 1 changed diff in Summary, got %d (diffs: %+v)", kinds["changed"], diffs)
	}
	if kinds["sheet_left_only"] != 1 || kinds["sheet_right_only"] != 1 {
		t.Errorf("expected 1 sheet_left_only and 1 sheet_right_only, got %+v", kinds)
	}
}
