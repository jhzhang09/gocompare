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
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/net/html"
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
		target := sheetPath(targets[i])
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
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		return nil
	}
	var sheets []Sheet
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "table" {
			sheets = append(sheets, Sheet{Name: fmt.Sprintf("table%d", len(sheets)+1), Rows: htmlTableRows(n)})
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return sheets
}

// ParseFile 根据文件路径或内容特征解析工作表。
func ParseFile(path string, data []byte) ([]Sheet, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".csv":
		s, err := ParseCSV(data)
		if err != nil {
			return nil, err
		}
		s.Name = filepath.Base(path)
		return []Sheet{s}, nil
	case ".tsv":
		s, err := ParseTSV(data)
		if err != nil {
			return nil, err
		}
		s.Name = filepath.Base(path)
		return []Sheet{s}, nil
	case ".xlsx":
		return ParseXLSX(data)
	case ".html", ".htm":
		sheets := ParseHTML(string(data))
		if len(sheets) == 0 {
			return nil, fmt.Errorf("no table found in html: %s", path)
		}
		return sheets, nil
	default:
		if bytes.HasPrefix(data, []byte("PK\x03\x04")) {
			return ParseXLSX(data)
		}
		if bytes.Contains(bytes.ToLower(data), []byte("<table")) {
			return ParseHTML(string(data)), nil
		}
		s, err := ParseCSV(data)
		if err == nil {
			s.Name = filepath.Base(path)
			return []Sheet{s}, nil
		}
		return nil, fmt.Errorf("unsupported table format: %s", path)
	}
}

// CompareSheets 比较两组工作表。
func CompareSheets(left, right []Sheet, opt Options) []Diff {
	var diffs []Diff
	n := len(left)
	if len(right) > n {
		n = len(right)
	}
	for i := 0; i < n; i++ {
		if i >= len(left) {
			diffs = append(diffs, Diff{Sheet: right[i].Name, Kind: "sheet_right_only"})
			continue
		}
		if i >= len(right) {
			diffs = append(diffs, Diff{Sheet: left[i].Name, Kind: "sheet_left_only"})
			continue
		}
		diffs = append(diffs, Compare(left[i], right[i], opt)...)
	}
	return diffs
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

// htmlTableRows 只收集当前表的行，单元格保留后代文本并由 HTML 解析器解码实体。
func htmlTableRows(table *html.Node) [][]string {
	var rows [][]string
	var text func(*html.Node, *strings.Builder)
	text = func(n *html.Node, b *strings.Builder) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style") {
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			text(c, b)
		}
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n != table && n.Type == html.ElementNode && n.Data == "table" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "tr" {
			var row []string
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") {
					var b strings.Builder
					text(c, &b)
					row = append(row, strings.TrimSpace(b.String()))
				}
			}
			rows = append(rows, row)
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(table)
	return rows
}

// sheetPath 把关系文件中的 Target 转成包内路径：以 "/" 开头的是包根绝对路径，
// 否则相对于 xl/ 目录。
func sheetPath(target string) string {
	if strings.HasPrefix(target, "/") {
		return path.Clean(strings.TrimPrefix(target, "/"))
	}
	return path.Clean("xl/" + target)
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
	data, err := io.ReadAll(io.LimitReader(r, maxPartBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxPartBytes {
		return nil, fmt.Errorf("xlsx part %s exceeds 32 MiB", f.Name)
	}
	return data, nil
}

// maxPartBytes 限制 xlsx 单个部件解压后的大小，超过时报错而不是截断解析。
const maxPartBytes = 32 << 20

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
	rowNumber, cellColumn, slots := 0, 0, 0
	var inRow, inCell, inV, inT bool
	cellType := ""
	var value strings.Builder
	flushCell := func() error {
		if slots+cellColumn+1-len(row) > 2<<20 {
			return fmt.Errorf("worksheet exceeds cell limit")
		}
		for len(row) < cellColumn {
			row = append(row, "")
			slots++
		}
		text := value.String()
		if cellType == "s" {
			n, err := strconv.Atoi(strings.TrimSpace(text))
			if err == nil && n >= 0 && n < len(shared) {
				text = shared[n]
			}
		}
		row = append(row, text)
		slots++
		value.Reset()
		cellType = ""
		return nil
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
				rowNumber = len(rows) + 1
				for _, attr := range el.Attr {
					if attr.Name.Local == "r" {
						n, err := strconv.Atoi(attr.Value)
						if err != nil || n <= len(rows) || n > 1048576 {
							return nil, fmt.Errorf("invalid worksheet row: %s", attr.Value)
						}
						rowNumber = n
					}
				}
				if rowNumber > 1048576 {
					return nil, fmt.Errorf("worksheet row limit")
				}
				for len(rows) < rowNumber-1 {
					rows = append(rows, nil)
				}
				inRow = true
				row = nil
			case "c":
				if inRow {
					inCell = true
					cellColumn = len(row)
					for _, attr := range el.Attr {
						if attr.Name.Local == "t" {
							cellType = attr.Value
						}
						if attr.Name.Local == "r" {
							col, r, err := cellReference(attr.Value)
							if err != nil || r != rowNumber || col < len(row) {
								return nil, fmt.Errorf("invalid cell reference: %s", attr.Value)
							}
							cellColumn = col
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
					if err := flushCell(); err != nil {
						return nil, err
					}
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

// cellReference 解析 Excel 的 A1 坐标并限制到工作表支持的范围。
func cellReference(ref string) (int, int, error) {
	col, i := 0, 0
	for i < len(ref) && ref[i] >= 'A' && ref[i] <= 'Z' {
		col = col*26 + int(ref[i]-'A') + 1
		if col > 16384 {
			return 0, 0, fmt.Errorf("column out of range")
		}
		i++
	}
	row, err := strconv.Atoi(ref[i:])
	if err != nil || col == 0 || row < 1 || row > 1048576 {
		return 0, 0, fmt.Errorf("invalid cell reference")
	}
	return col - 1, row, nil
}
