// Package pever 从 PE 资源目录读取 RT_VERSION 中的 FileVersion。
package pever

import (
	"bytes"
	"encoding/binary"
	"errors"
	"unicode/utf16"
)

// FileVersion 解析最小 PE 的资源树，返回 RT_VERSION（类型 16）里的 FileVersion。
func FileVersion(data []byte) (string, error) {
	if len(data) < 0x40 || data[0] != 'M' || data[1] != 'Z' {
		return "", errors.New("not a PE file")
	}
	lfanew := int(binary.LittleEndian.Uint32(data[0x3c:]))
	if lfanew < 0 || lfanew+24 > len(data) || string(data[lfanew:lfanew+4]) != "PE\x00\x00" {
		return "", errors.New("missing PE signature")
	}
	coff := lfanew + 4
	sections := int(binary.LittleEndian.Uint16(data[coff+2:]))
	optSize := int(binary.LittleEndian.Uint16(data[coff+16:]))
	table := coff + 20 + optSize
	found := false
	var va, raw, size int
	for i := 0; i < sections; i++ {
		off := table + i*40
		if off+40 > len(data) {
			return "", errors.New("truncated section table")
		}
		name := string(bytes.TrimRight(data[off:off+8], "\x00"))
		if name != ".rsrc" {
			continue
		}
		va = int(binary.LittleEndian.Uint32(data[off+12:]))
		size = int(binary.LittleEndian.Uint32(data[off+16:]))
		raw = int(binary.LittleEndian.Uint32(data[off+20:]))
		found = true
	}
	if !found {
		return "", errors.New("resource section not found")
	}
	if raw < 0 || size < 0 || raw+size > len(data) {
		return "", errors.New("resource section out of range")
	}
	blob, err := versionBlob(data[raw:raw+size], va)
	if err != nil {
		return "", err
	}
	return utf16Value(blob, "FileVersion")
}

func versionBlob(sec []byte, sectionVA int) ([]byte, error) {
	typeOff, dir, ok := findID(sec, 0, 16)
	if !ok || !dir {
		return nil, errors.New("RT_VERSION not found")
	}
	nameOff, dir, ok := firstEntry(sec, typeOff)
	if !ok || !dir {
		return nil, errors.New("version name entry missing")
	}
	dataOff, dir, ok := firstEntry(sec, nameOff)
	if !ok || dir {
		return nil, errors.New("version data entry missing")
	}
	if dataOff+16 > len(sec) {
		return nil, errors.New("truncated version data entry")
	}
	rva := int(binary.LittleEndian.Uint32(sec[dataOff:]))
	n := int(binary.LittleEndian.Uint32(sec[dataOff+4:]))
	local := rva - sectionVA
	if local < 0 || n < 0 || local+n > len(sec) {
		return nil, errors.New("version bytes out of range")
	}
	return sec[local : local+n], nil
}

func findID(sec []byte, dirOff, id int) (int, bool, bool) {
	named, count, ok := dirCounts(sec, dirOff)
	if !ok {
		return 0, false, false
	}
	base := dirOff + 16 + named*8
	for i := 0; i < count; i++ {
		e := base + i*8
		if e+8 > len(sec) {
			return 0, false, false
		}
		if int(binary.LittleEndian.Uint32(sec[e:])) != id {
			continue
		}
		rel := binary.LittleEndian.Uint32(sec[e+4:])
		return int(rel & 0x7fffffff), rel&0x80000000 != 0, true
	}
	return 0, false, false
}

func firstEntry(sec []byte, dirOff int) (int, bool, bool) {
	named, count, ok := dirCounts(sec, dirOff)
	if !ok || named+count == 0 {
		return 0, false, false
	}
	e := dirOff + 16
	if e+8 > len(sec) {
		return 0, false, false
	}
	rel := binary.LittleEndian.Uint32(sec[e+4:])
	return int(rel & 0x7fffffff), rel&0x80000000 != 0, true
}

func dirCounts(sec []byte, dirOff int) (int, int, bool) {
	if dirOff < 0 || dirOff+16 > len(sec) {
		return 0, 0, false
	}
	return int(binary.LittleEndian.Uint16(sec[dirOff+12:])), int(binary.LittleEndian.Uint16(sec[dirOff+14:])), true
}

// utf16Value 读取 FileVersion。标准 String 在键前有 6 字节头（wLength、wValueLength、wType），
// 键的 NUL 之后要先对齐到 32 位，填充的 0 不是值的结束符。wValueLength 的单位是字。
// 没有这个头时，值紧接在键后面，供最小夹具使用。
func utf16Value(blob []byte, key string) (string, error) {
	needle := append(encodeUTF16(key), 0, 0)
	first := -1
	for from := 0; from+len(needle) <= len(blob); {
		rel := bytes.Index(blob[from:], needle)
		if rel < 0 {
			break
		}
		at := from + rel
		if first < 0 {
			first = at
		}
		if s, ok := dwordAlignedValue(blob, at, len(needle)); ok {
			if s == "" {
				return "", errors.New("empty FileVersion")
			}
			return s, nil
		}
		from = at + 2
	}
	if first < 0 {
		return "", errors.New("FileVersion not found")
	}
	return nulTerminatedValue(blob[first+len(needle):])
}

// dwordAlignedValue 在键前识别 String 头。值从 32 位边界开始，长度由 wValueLength 决定。
func dwordAlignedValue(blob []byte, keyAt, keyLen int) (string, bool) {
	header := keyAt - 6
	if header < 0 || keyAt+keyLen > len(blob) {
		return "", false
	}
	wLength := int(binary.LittleEndian.Uint16(blob[header:]))
	wValueLength := int(binary.LittleEndian.Uint16(blob[header+2:]))
	wType := binary.LittleEndian.Uint16(blob[header+4:])
	if wType != 1 || wValueLength == 0 || wLength < 6+keyLen || header+wLength > len(blob) {
		return "", false
	}
	valueAt := (keyAt + keyLen + 3) &^ 3
	end := valueAt + wValueLength*2
	if valueAt < keyAt+keyLen || end > len(blob) || end > header+wLength {
		return "", false
	}
	units := make([]uint16, wValueLength)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(blob[valueAt+i*2:])
	}
	for len(units) > 0 && units[len(units)-1] == 0 {
		units = units[:len(units)-1]
	}
	return string(utf16.Decode(units)), true
}

func nulTerminatedValue(rest []byte) (string, error) {
	var units []uint16
	for len(rest) >= 2 {
		u := binary.LittleEndian.Uint16(rest)
		rest = rest[2:]
		if u == 0 {
			break
		}
		units = append(units, u)
	}
	if len(units) == 0 {
		return "", errors.New("empty FileVersion")
	}
	return string(utf16.Decode(units)), nil
}

func encodeUTF16(s string) []byte {
	u := utf16.Encode([]rune(s))
	out := make([]byte, len(u)*2)
	for i, v := range u {
		binary.LittleEndian.PutUint16(out[i*2:], v)
	}
	return out
}
