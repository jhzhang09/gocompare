// Package regfile 解析跨平台的 .reg 文本。它不读取实时注册表。
package regfile

import (
	"bufio"
	"fmt"
	"sort"
	"strings"
)

// Value 是一条注册表值。
type Value struct {
	Key  string
	Name string
	Data string
}

// Parse 读取 Windows Registry Editor Version 5.00 文本。
func Parse(text string) ([]Value, error) {
	s := bufio.NewScanner(strings.NewReader(text))
	header := false
	key := ""
	var out []Value
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}
		if !header {
			if line != "Windows Registry Editor Version 5.00" {
				return nil, fmt.Errorf("unsupported registry header: %s", line)
			}
			header = true
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			key = line[1 : len(line)-1]
			continue
		}
		if key == "" {
			return nil, fmt.Errorf("value outside a key: %s", line)
		}
		name, data, ok := splitValue(line)
		if !ok {
			return nil, fmt.Errorf("bad registry value: %s", line)
		}
		out = append(out, Value{Key: key, Name: name, Data: data})
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	if !header {
		return nil, fmt.Errorf("missing registry header")
	}
	return out, nil
}

// Diff 返回两侧不一致的 "key\\name" 列表。只在一侧出现也算差异。
func Diff(left, right []Value) []string {
	l := index(left)
	r := index(right)
	var out []string
	seen := map[string]bool{}
	for k, lv := range l {
		seen[k] = true
		if rv, ok := r[k]; !ok || rv != lv {
			out = append(out, k)
		}
	}
	for k := range r {
		if !seen[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func index(values []Value) map[string]string {
	out := map[string]string{}
	for _, v := range values {
		out[v.Key+`\`+v.Name] = v.Data
	}
	return out
}

func splitValue(line string) (string, string, bool) {
	eq := strings.Index(line, "=")
	if eq < 0 {
		return "", "", false
	}
	name := line[:eq]
	data := line[eq+1:]
	if name == "@" {
		name = ""
	} else if len(name) >= 2 && name[0] == '"' && name[len(name)-1] == '"' {
		name = name[1 : len(name)-1]
	} else {
		return "", "", false
	}
	return name, data, true
}
