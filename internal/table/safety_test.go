package table

import (
	"testing"
)

func TestSparseCellsKeepCoordinates(t *testing.T) {
	a, e := worksheetRows([]byte(`<worksheet><sheetData><row r="1"><c r="A1"><v>1</v></c><c r="C1"><v>2</v></c></row></sheetData></worksheet>`), nil)
	if e != nil {
		t.Fatal(e)
	}
	b, e := worksheetRows([]byte(`<worksheet><sheetData><row r="1"><c r="A1"><v>1</v></c><c r="B1"><v>2</v></c></row></sheetData></worksheet>`), nil)
	if e != nil {
		t.Fatal(e)
	}
	if len(Compare(Sheet{Rows: a}, Sheet{Rows: b}, Options{})) == 0 {
		t.Fatalf("different cell positions compare equal: %v / %v", a, b)
	}
}
func TestHTMLNestedTextDifferences(t *testing.T) {
	a := ParseHTML(`<table><tr><td><b>left</b></td></tr></table>`)
	b := ParseHTML(`<table><tr><td><b>right</b></td></tr></table>`)
	if len(CompareSheets(a, b, Options{})) == 0 {
		t.Fatal("different nested text compares equal")
	}
}

func TestWorksheetRowCoordinates(t *testing.T) {
	rows, err := worksheetRows([]byte(`<worksheet><sheetData><row r="3"><c r="C3" t="inlineStr"><is><t>value</t></is></c></row></sheetData></worksheet>`), nil)
	if err != nil || len(rows) != 3 || len(rows[2]) != 3 || rows[2][2] != "value" {
		t.Fatalf("lost coordinates: %v %v", rows, err)
	}
	for _, ref := range []string{"XFE1", "A1048577", "A0", "AAAAAA1"} {
		if _, _, err := cellReference(ref); err == nil {
			t.Fatalf("accepted %s", ref)
		}
	}
}
func TestHTMLTextAndEntities(t *testing.T) {
	sheets := ParseHTML(`<table><tr><td>before <b>A&amp;B</b> after</td></tr></table>`)
	if len(sheets) != 1 || sheets[0].Rows[0][0] != "before A&B after" {
		t.Fatalf("lost text: %v", sheets)
	}
}
