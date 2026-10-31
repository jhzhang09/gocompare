// Package table 比较 CSV、TSV、Excel 和 HTML 表格。
package table

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// Sheet 是一张已解析的表。
type Sheet struct {
	Name string
	Rows [][]string
}

// Options 控制键、列映射和数值容差。
// Map[leftColumn] 是对应的右侧列；省略时按相同下标比较。
// Keys 是左侧列下标，组成复合键。重复键会单独报告。
type Options struct {
	Keys      []int
	Map       []int
	Tolerance float64
}

// Diff 是一个单元格或整行差异。
type Diff struct {
	Sheet  string
	Key    string
	Column int
	Left   string
	Right  string
	Kind   string
}

// ParseCSV 解析逗号分隔的文本。
func ParseCSV(data []byte) (Sheet, error) {
	return parseDelim(data, ',')
}

// ParseTSV 解析制表符分隔的文本。
func ParseTSV(data []byte) (Sheet, error) {
	return parseDelim(data, '\t')
}

func parseDelim(data []byte, comma rune) (Sheet, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = comma
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		return Sheet{}, err
	}
	return Sheet{Name: "sheet", Rows: rows}, nil
}

// ParseXLSX 读取 xlsx 中的全部工作表，支持 shared strings 和 inlineStr。
func ParseXLSX(data []byte) ([]Sheet, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[f.Name] = f
	}
	shared, err := readShared(files["xl/sharedStrings.xml"])
	if err != nil {
		return nil, err
	}
	names, targets, err := workbookSheets(files)
	if err != nil {
		return nil, err
	}
	var sheets []Sheet
	for i, name := range names {
		target := targets[i]
		if !strings.HasPrefix(target, "xl/") {
			target = "xl/" + strings.TrimPrefix(target, "/")
		}
		body, err := readZip(files[target])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		rows, err := worksheetRows(body, shared)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		sheets = append(sheets, Sheet{Name: name, Rows: rows})
	}
	return sheets, nil
}

// ParseHTML 读取文档中的每张 table。单元格取标签之间的文本。
func ParseHTML(s string) []Sheet {
	lower := strings.ToLower(s)
	var sheets []Sheet
	from := 0
	n := 0
	for {
		start := strings.Index(lower[from:], "<table")
		if start < 0 {
			break
		}
		start += from
		endRel := strings.Index(lower[start:], "</table>")
		if endRel < 0 {
			break
		}
		end := start + endRel
		n++
		sheets = append(sheets, Sheet{Name: fmt.Sprintf("table%d", n), Rows: htmlRows(s[start:end])})
		from = end + len("</table>")
	}
	return sheets
}

// Compare 按键或行号对齐两张表。数值差不超过 Tolerance 时视为相同。
func Compare(left, right Sheet, opt Options) []Diff {
	var diffs []Diff
	if len(opt.Keys) == 0 {
		n := len(left.Rows)
		if len(right.Rows) > n {
			n = len(right.Rows)
		}
		for i := 0; i < n; i++ {
			var l, r []string
			if i < len(left.Rows) {
				l = left.Rows[i]
			}
			if i < len(right.Rows) {
				r = right.Rows[i]
			}
			diffs = append(diffs, compareRow(left.Name, strconv.Itoa(i), l, r, opt)...)
		}
		return diffs
	}
	// 重复键只报告一次，单元格比较使用第一次出现的行。
	rightAt := map[string][]string{}
	seenRight := map[string]int{}
	for i, row := range right.Rows {
		key := mappedKey(row, opt)
		if prev, ok := seenRight[key]; ok {
			diffs = append(diffs, Diff{Sheet: right.Name, Key: key, Kind: "duplicate", Left: strconv.Itoa(prev), Right: strconv.Itoa(i)})
			continue
		}
		seenRight[key] = i
		rightAt[key] = row
	}
	used := map[string]bool{}
	seenLeft := map[string]int{}
	for i, row := range left.Rows {
		key := rowKey(row, opt.Keys)
		if prev, ok := seenLeft[key]; ok {
			diffs = append(diffs, Diff{Sheet: left.Name, Key: key, Kind: "duplicate", Left: strconv.Itoa(prev), Right: strconv.Itoa(i)})
			continue
		}
		seenLeft[key] = i
		other, ok := rightAt[key]
		if !ok {
			diffs = append(diffs, Diff{Sheet: left.Name, Key: key, Kind: "left_only", Left: strings.Join(row, ",")})
			continue
		}
		used[key] = true
		diffs = append(diffs, compareRow(left.Name, key, row, other, opt)...)
	}
	reported := map[string]bool{}
	for _, row := range right.Rows {
		key := mappedKey(row, opt)
		if used[key] || reported[key] {
			continue
		}
		reported[key] = true
		diffs = append(diffs, Diff{Sheet: right.Name, Key: key, Kind: "right_only", Right: strings.Join(row, ",")})
	}
	return diffs
}

func compareRow(sheet, key string, left, right []string, opt Options) []Diff {
	width := len(left)
	if len(opt.Map) > width {
		width = len(opt.Map)
	}
	if len(opt.Map) == 0 && len(right) > width {
		width = len(right)
	}
	var diffs []Diff
	for col := 0; col < width; col++ {
		otherCol := col
		if len(opt.Map) > 0 {
			if col >= len(opt.Map) {
				break
			}
			otherCol = opt.Map[col]
			if otherCol < 0 {
				continue
			}
		}
		lv, rv := cell(left, col), cell(right, otherCol)
		if lv == rv || within(lv, rv, opt.Tolerance) {
			continue
		}
		diffs = append(diffs, Diff{Sheet: sheet, Key: key, Column: col, Left: lv, Right: rv, Kind: "changed"})
	}
	return diffs
}

func within(left, right string, tol float64) bool {
	if tol < 0 {
		return false
	}
	a, errA := strconv.ParseFloat(strings.TrimSpace(left), 64)
	b, errB := strconv.ParseFloat(strings.TrimSpace(right), 64)
	if errA != nil || errB != nil {
		return false
	}
	return math.Abs(a-b) <= tol
}

func cell(row []string, i int) string {
	if i < 0 || i >= len(row) {
		return ""
	}
	return row[i]
}

func rowKey(row []string, keys []int) string {
	parts := make([]string, len(keys))
	for i, col := range keys {
		parts[i] = cell(row, col)
	}
	return strings.Join(parts, "\x1f")
}

func mappedKey(row []string, opt Options) string {
	if len(opt.Map) == 0 {
		return rowKey(row, opt.Keys)
	}
	parts := make([]string, len(opt.Keys))
	for i, col := range opt.Keys {
		src := col
		if col < len(opt.Map) {
			src = opt.Map[col]
		}
		parts[i] = cell(row, src)
	}
	return strings.Join(parts, "\x1f")
}

func htmlRows(fragment string) [][]string {
	lower := strings.ToLower(fragment)
	var rows [][]string
	from := 0
	for {
		rel := strings.Index(lower[from:], "<tr")
		if rel < 0 {
			break
		}
		start := from + rel
		next := strings.Index(lower[start+3:], "<tr")
		end := len(fragment)
		if next >= 0 {
			end = start + 3 + next
		}
		rows = append(rows, htmlCells(fragment[start:end]))
		from = end
	}
	return rows
}

func htmlCells(row string) []string {
	lower := strings.ToLower(row)
	var cells []string
	from := 0
	for {
		td := strings.Index(lower[from:], "<td")
		th := strings.Index(lower[from:], "<th")
		if td < 0 && th < 0 {
			break
		}
		rel := td
		if rel < 0 || (th >= 0 && th < rel) {
			rel = th
		}
		start := from + rel
		gt := strings.Index(row[start:], ">")
		if gt < 0 {
			break
		}
		contentStart := start + gt + 1
		lt := strings.Index(row[contentStart:], "<")
		text := ""
		if lt >= 0 {
			text = row[contentStart : contentStart+lt]
		}
		cells = append(cells, strings.TrimSpace(text))
		from = contentStart
	}
	return cells
}

func readZip(f *zip.File) ([]byte, error) {
	if f == nil {
		return nil, fmt.Errorf("missing zip entry")
	}
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(io.LimitReader(r, 32<<20))
}

func readShared(f *zip.File) ([]string, error) {
	if f == nil {
		return nil, nil
	}
	body, err := readZip(f)
	if err != nil {
		return nil, err
	}
	var out []string
	dec := xml.NewDecoder(bytes.NewReader(body))
	var b strings.Builder
	in := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch el := tok.(type) {
		case xml.StartElement:
			if el.Name.Local == "si" {
				b.Reset()
				in = true
			}
		case xml.EndElement:
			if el.Name.Local == "si" && in {
				out = append(out, b.String())
				in = false
			}
		case xml.CharData:
			if in {
				b.Write(el)
			}
		}
	}
	return out, nil
}

func workbookSheets(files map[string]*zip.File) ([]string, []string, error) {
	body, err := readZip(files["xl/workbook.xml"])
	if err != nil {
		return nil, nil, err
	}
	var names, ids []string
	dec := xml.NewDecoder(bytes.NewReader(body))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		el, ok := tok.(xml.StartElement)
		if !ok || el.Name.Local != "sheet" {
			continue
		}
		var name, id string
		for _, attr := range el.Attr {
			if attr.Name.Local == "name" {
				name = attr.Value
			}
			if attr.Name.Local == "id" {
				id = attr.Value
			}
		}
		names = append(names, name)
		ids = append(ids, id)
	}
	relsBody, err := readZip(files["xl/_rels/workbook.xml.rels"])
	if err != nil {
		return nil, nil, err
	}
	targets := map[string]string{}
	dec = xml.NewDecoder(bytes.NewReader(relsBody))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		el, ok := tok.(xml.StartElement)
		if !ok || el.Name.Local != "Relationship" {
			continue
		}
		var id, target string
		for _, attr := range el.Attr {
			if attr.Name.Local == "Id" {
				id = attr.Value
			}
			if attr.Name.Local == "Target" {
				target = attr.Value
			}
		}
		targets[id] = target
	}
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = targets[id]
	}
	return names, out, nil
}

func worksheetRows(body []byte, shared []string) ([][]string, error) {
	dec := xml.NewDecoder(bytes.NewReader(body))
	var rows [][]string
	var row []string
	var inRow, inCell, inV, inT bool
	cellType := ""
	var value strings.Builder
	flushCell := func() {
		text := value.String()
		if cellType == "s" {
			n, err := strconv.Atoi(strings.TrimSpace(text))
			if err == nil && n >= 0 && n < len(shared) {
				text = shared[n]
			}
		}
		row = append(row, text)
		value.Reset()
		cellType = ""
	}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch el := tok.(type) {
		case xml.StartElement:
			switch el.Name.Local {
			case "row":
				inRow = true
				row = nil
			case "c":
				if inRow {
					inCell = true
					for _, attr := range el.Attr {
						if attr.Name.Local == "t" {
							cellType = attr.Value
						}
					}
				}
			case "v":
				inV = true
			case "t":
				if inCell {
					inT = true
				}
			}
		case xml.EndElement:
			switch el.Name.Local {
			case "c":
				if inCell {
					flushCell()
					inCell = false
				}
			case "row":
				if inRow {
					rows = append(rows, row)
					inRow = false
				}
			case "v":
				inV = false
			case "t":
				inT = false
			}
		case xml.CharData:
			if inV || inT {
				value.Write(el)
			}
		}
	}
	return rows, nil
}
