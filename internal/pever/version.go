// Package pever 从 PE 资源目录读取 RT_VERSION 中的 FileVersion。
// 当合法 PE 不含 RT_VERSION（如未嵌入版本资源的安装壳或命令行程序）时，
// 回退至应用清单版本或基于 PE 架构与内容摘要的确定性标识。
package pever

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"unicode/utf16"
)

// Info 是从 PE 文件提取的版本与构建元数据。
type Info struct {
	FileVersion      string
	ProductVersion   string
	ProductName      string
	FileDescription  string
	CompanyName      string
	LegalCopyright   string
	OriginalFilename string
	Architecture     string
	SHA256           string
	Size             int
}

// Inspect 解析 PE 文件的版本信息、字符串表元数据、目标架构与摘要。
func Inspect(data []byte) (Info, error) {
	fv, err := FileVersion(data)
	if err != nil {
		return Info{}, err
	}
	lfanew := int(binary.LittleEndian.Uint32(data[0x3c:]))
	coff := lfanew + 4
	machine := binary.LittleEndian.Uint16(data[coff:])
	sum := sha256.Sum256(data)
	info := Info{
		FileVersion:  fv,
		Architecture: peArch(machine),
		SHA256:       fmt.Sprintf("%x", sum[:8]),
		Size:         len(data),
	}
	sections := int(binary.LittleEndian.Uint16(data[coff+2:]))
	optSize := int(binary.LittleEndian.Uint16(data[coff+16:]))
	table := coff + 20 + optSize
	for i := 0; i < sections; i++ {
		off := table + i*40
		if off+40 > len(data) {
			break
		}
		if string(bytes.TrimRight(data[off:off+8], "\x00")) != ".rsrc" {
			continue
		}
		va := int(binary.LittleEndian.Uint32(data[off+12:]))
		size := int(binary.LittleEndian.Uint32(data[off+16:]))
		raw := int(binary.LittleEndian.Uint32(data[off+20:]))
		if raw >= 0 && size >= 0 && raw+size <= len(data) {
			if blob, bErr := resourceBlob(data[raw:raw+size], va, 16); bErr == nil {
				info.ProductVersion, _ = utf16Value(blob, "ProductVersion")
				info.ProductName, _ = utf16Value(blob, "ProductName")
				info.FileDescription, _ = utf16Value(blob, "FileDescription")
				info.CompanyName, _ = utf16Value(blob, "CompanyName")
				info.LegalCopyright, _ = utf16Value(blob, "LegalCopyright")
				info.OriginalFilename, _ = utf16Value(blob, "OriginalFilename")
			}
		}
		break
	}
	return info, nil
}

// FileVersion 解析最小 PE 的资源树，优先返回 RT_VERSION（类型 16）里的 FileVersion。
func FileVersion(data []byte) (string, error) {
	if len(data) < 0x40 || data[0] != 'M' || data[1] != 'Z' {
		return "", errors.New("not a PE file")
	}
	lfanew := int(binary.LittleEndian.Uint32(data[0x3c:]))
	if lfanew < 0 || lfanew+24 > len(data) || string(data[lfanew:lfanew+4]) != "PE\x00\x00" {
		return "", errors.New("missing PE signature")
	}
	coff := lfanew + 4
	machine := binary.LittleEndian.Uint16(data[coff:])
	sections := int(binary.LittleEndian.Uint16(data[coff+2:]))
	optSize := int(binary.LittleEndian.Uint16(data[coff+16:]))
	table := coff + 20 + optSize
	if table < 0 || table > len(data) {
		return "", errors.New("truncated section table")
	}
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
		return peFallbackVersion(data, machine), nil
	}
	if raw < 0 || size < 0 || raw+size > len(data) {
		return "", errors.New("resource section out of range")
	}
	sec := data[raw : raw+size]
	if _, dir, ok := findID(sec, 0, 16); !ok || !dir {
		if v, ok := manifestVersion(sec, va); ok {
			return v, nil
		}
		return peFallbackVersion(data, machine), nil
	}
	blob, err := resourceBlob(sec, va, 16)
	if err != nil {
		return "", err
	}
	v, err := utf16Value(blob, "FileVersion")
	if err == nil {
		return v, nil
	}
	if err.Error() == "empty FileVersion" {
		return "", err
	}
	if pv, pvErr := utf16Value(blob, "ProductVersion"); pvErr == nil {
		return pv, nil
	}
	if fv, ok := fixedFileVersion(blob); ok {
		return fv, nil
	}
	return "", err
}

func versionBlob(sec []byte, sectionVA int) ([]byte, error) {
	return resourceBlob(sec, sectionVA, 16)
}

func resourceBlob(sec []byte, sectionVA, resType int) ([]byte, error) {
	typeOff, dir, ok := findID(sec, 0, resType)
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

func fixedFileVersion(blob []byte) (string, bool) {
	sig := []byte{0xbd, 0x04, 0xef, 0xfe}
	idx := bytes.Index(blob, sig)
	if idx < 0 || idx+16 > len(blob) {
		return "", false
	}
	ms := binary.LittleEndian.Uint32(blob[idx+8:])
	ls := binary.LittleEndian.Uint32(blob[idx+12:])
	return fmt.Sprintf("%d.%d.%d.%d", ms>>16, ms&0xffff, ls>>16, ls&0xffff), true
}

func manifestVersion(sec []byte, sectionVA int) (string, bool) {
	blob, err := resourceBlob(sec, sectionVA, 24)
	if err != nil {
		return "", false
	}
	text := string(blob)
	if strings.Contains(text, "Nullsoft.NSIS") {
		return "", false
	}
	idx := strings.Index(text, "<assemblyIdentity")
	if idx < 0 {
		return "", false
	}
	sub := text[idx:]
	if end := strings.Index(sub, ">"); end >= 0 {
		sub = sub[:end]
	}
	if strings.Contains(sub, "Microsoft.Windows.Common-Controls") {
		return "", false
	}
	const key = `version="`
	vAt := strings.Index(sub, key)
	if vAt < 0 {
		return "", false
	}
	rest := sub[vAt+len(key):]
	vEnd := strings.IndexByte(rest, '"')
	if vEnd <= 0 {
		return "", false
	}
	return rest[:vEnd], true
}

func peFallbackVersion(data []byte, machine uint16) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("PE/%s (%x)", peArch(machine), sum[:6])
}

func peArch(machine uint16) string {
	switch machine {
	case 0x8664:
		return "amd64"
	case 0xaa64:
		return "arm64"
	case 0x014c:
		return "x86"
	default:
		return fmt.Sprintf("0x%04x", machine)
	}
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
