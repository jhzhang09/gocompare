// Package regfile 解析跨平台的 .reg 文本。它不读取实时注册表。
package regfile

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Value 是一条注册表值。
type Value struct {
	Key  string
	Name string
	Data string
}

// DiffEntry 是一条注册表差异及其分类（不包含敏感值内容）。
type DiffEntry struct {
	Path string
	Kind string // "changed", "left_only", "right_only"
}

// IsHeader 判断一行文本是否为合法的 .reg 文件头（支持 5.00 与 REGEDIT4）。
func IsHeader(line string) bool {
	return line == "Windows Registry Editor Version 5.00" || line == "REGEDIT4"
}

// Decode 把 .reg 文件字节转成文本。注册表编辑器默认导出带 BOM 的 UTF-16LE；
// 也接受 UTF-16BE、带或不带 BOM 的 UTF-8。其他字节返回错误，调用方据此判断不是注册表文本。
func Decode(data []byte) (string, error) {
	var units []uint16
	switch {
	case len(data) >= 2 && data[0] == 0xff && data[1] == 0xfe:
		for i := 2; i+1 < len(data); i += 2 {
			units = append(units, binary.LittleEndian.Uint16(data[i:]))
		}
	case len(data) >= 2 && data[0] == 0xfe && data[1] == 0xff:
		for i := 2; i+1 < len(data); i += 2 {
			units = append(units, binary.BigEndian.Uint16(data[i:]))
		}
	default:
		text := strings.TrimPrefix(string(data), "\ufeff")
		if !utf8.ValidString(text) {
			return "", errors.New("registry text must be UTF-8 or UTF-16 with BOM")
		}
		return text, nil
	}
	if len(data)%2 != 0 {
		return "", errors.New("truncated UTF-16 registry text")
	}
	return string(utf16.Decode(units)), nil
}

// Parse 读取 Windows Registry Editor Version 5.00 或 REGEDIT4 文本。行尾的反斜杠表示值在下一行继续，
// 注册表编辑器导出较长的 hex 值时会这样折行。
func Parse(text string) ([]Value, error) {
	s := bufio.NewScanner(strings.NewReader(text))
	s.Buffer(make([]byte, 64<<10), 16<<20)
	header := false
	key := ""
	keyHasValue := false
	var out []Value
	flushEmptyKey := func() {
		if key != "" && !keyHasValue {
			if strings.HasPrefix(key, "-") {
				out = append(out, Value{Key: strings.TrimPrefix(key, "-"), Name: "", Data: "<deleted-key>"})
			} else {
				out = append(out, Value{Key: key, Name: "", Data: "<empty-key>"})
			}
		}
	}
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		for strings.HasSuffix(line, "\\") && s.Scan() {
			line = strings.TrimSuffix(line, "\\") + strings.TrimSpace(s.Text())
		}
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}
		if !header {
			if !IsHeader(line) {
				return nil, fmt.Errorf("unsupported registry header: %s", line)
			}
			header = true
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			flushEmptyKey()
			key = line[1 : len(line)-1]
			keyHasValue = false
			continue
		}
		if key == "" {
			return nil, fmt.Errorf("value outside a key: %s", line)
		}
		name, data, ok := splitValue(line)
		if !ok {
			return nil, fmt.Errorf("bad registry value: %s", line)
		}
		keyHasValue = true
		out = append(out, Value{Key: strings.TrimPrefix(key, "-"), Name: name, Data: data})
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	if !header {
		return nil, fmt.Errorf("missing registry header")
	}
	flushEmptyKey()
	return out, nil
}

// Diff 返回两侧不一致的 "key\\name" 列表（键名与值名大小写不敏感匹配，保留首次出现的显示大小写）。
func Diff(left, right []Value) []string {
	details := DiffDetails(left, right)
	out := make([]string, len(details))
	for i, d := range details {
		out[i] = d.Path
	}
	return out
}

// DiffDetails 返回两侧不一致的路径及其差异分类（left_only / right_only / changed）。
func DiffDetails(left, right []Value) []DiffEntry {
	lData, lDisplay := indexCaseInsensitive(left)
	rData, rDisplay := indexCaseInsensitive(right)
	var out []DiffEntry
	seen := map[string]bool{}
	for normKey, lv := range lData {
		seen[normKey] = true
		rv, ok := rData[normKey]
		if !ok {
			out = append(out, DiffEntry{Path: lDisplay[normKey], Kind: "left_only"})
		} else if rv != lv {
			out = append(out, DiffEntry{Path: lDisplay[normKey], Kind: "changed"})
		}
	}
	for normKey := range rData {
		if !seen[normKey] {
			out = append(out, DiffEntry{Path: rDisplay[normKey], Kind: "right_only"})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Path < out[j].Path
	})
	return out
}

func indexCaseInsensitive(values []Value) (data map[string]string, display map[string]string) {
	data = map[string]string{}
	display = map[string]string{}
	for _, v := range values {
		rawPath := v.Key
		if v.Name != "" || (v.Data != "<empty-key>" && v.Data != "<deleted-key>") {
			rawPath = v.Key + `\` + v.Name
		}
		norm := strings.ToLower(rawPath)
		data[norm] = v.Data
		if _, exists := display[norm]; !exists {
			display[norm] = rawPath
		}
	}
	return data, display
}

// splitValue 拆出值名和数据。带引号的值名可能包含 = 和转义的 \" 或 \\，按引号边界解析。
func splitValue(line string) (string, string, bool) {
	if strings.HasPrefix(line, "@=") {
		return "", line[2:], true
	}
	if !strings.HasPrefix(line, `"`) {
		return "", "", false
	}
	var name strings.Builder
	for i := 1; i < len(line); i++ {
		switch c := line[i]; {
		case c == '\\' && i+1 < len(line):
			i++
			name.WriteByte(line[i])
		case c == '"':
			rest := line[i+1:]
			if !strings.HasPrefix(rest, "=") {
				return "", "", false
			}
			return name.String(), rest[1:], true
		default:
			name.WriteByte(c)
		}
	}
	return "", "", false
}
