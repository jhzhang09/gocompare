package native

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"

	"github.com/egoist/mygo/ui"
)

func TestWorkspacePEVersions(t *testing.T) {
	dir := t.TempDir()
	leftPath := filepath.Join(dir, "left.exe")
	rightPath := filepath.Join(dir, "right.exe")
	samePath := filepath.Join(dir, "same.exe")
	if err := os.WriteFile(leftPath, workspacePE("1.2.3.4"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rightPath, workspacePE("9.8.7.6"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(samePath, workspacePE("1.2.3.4"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(leftPath)
	if err != nil {
		t.Fatal(err)
	}
	a, tt := openVersion(t, leftPath, rightPath)
	if a.err != "" || a.kind != 5 || !a.infoReady || a.infoKind != "pe" || a.panes[0] != nil {
		t.Fatalf("kind=%d ready=%v kindName=%s err=%s", a.kind, a.infoReady, a.infoKind, a.err)
	}
	if a.infoLeft != "1.2.3.4" || a.infoRight != "9.8.7.6" {
		t.Fatalf("versions %q %q", a.infoLeft, a.infoRight)
	}
	for _, text := range []string{"不读取实时注册表", "左侧版本 1.2.3.4", "右侧版本 9.8.7.6", "版本不同"} {
		if !tt.HasText(text) {
			t.Fatalf("missing %q in %v", text, tt.Texts())
		}
	}
	if tt.HasText("版本相同") || tt.HasText("注册表相同") {
		t.Fatalf("different versions look equal: %v", tt.Texts())
	}

	same, sameView := openVersion(t, leftPath, samePath)
	if same.err != "" || same.infoLeft != "1.2.3.4" || same.infoRight != "1.2.3.4" || !sameView.HasText("版本相同") || sameView.HasText("9.8.7.6") || sameView.HasText("版本不同") {
		t.Fatalf("same left=%s right=%s err=%s texts=%v", same.infoLeft, same.infoRight, same.err, sameView.Texts())
	}
	alignedPath := filepath.Join(dir, "aligned.exe")
	if err = os.WriteFile(alignedPath, workspacePEBlob(workspaceAlignedVersion("9.8.7.6")), 0o600); err != nil {
		t.Fatal(err)
	}
	aligned, alignedView := openVersion(t, leftPath, alignedPath)
	if aligned.err != "" || aligned.infoLeft != "1.2.3.4" || aligned.infoRight != "9.8.7.6" || !alignedView.HasText("右侧版本 9.8.7.6") || !alignedView.HasText("版本不同") {
		t.Fatalf("aligned left=%s right=%s err=%s texts=%v", aligned.infoLeft, aligned.infoRight, aligned.err, alignedView.Texts())
	}
	after, err := os.ReadFile(leftPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("pe bytes changed")
	}
	rightAfter, err := os.ReadFile(rightPath)
	if err != nil || !bytes.Equal(workspacePE("9.8.7.6"), rightAfter) {
		t.Fatal("right pe bytes changed")
	}
}

func TestWorkspaceRegistryDiff(t *testing.T) {
	dir := t.TempDir()
	leftPath := filepath.Join(dir, "left.reg")
	rightPath := filepath.Join(dir, "right.reg")
	leftText := "Windows Registry Editor Version 5.00\n\n[HKEY_CURRENT_USER\\Software\\GoCompare]\n\"Name\"=\"One\"\n\"Extra\"=\"Keep\"\n"
	rightText := "Windows Registry Editor Version 5.00\n\n[HKEY_CURRENT_USER\\Software\\GoCompare]\n\"Name\"=\"Two\"\n"
	if err := os.WriteFile(leftPath, []byte(leftText), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rightPath, []byte(rightText), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(leftPath)
	if err != nil {
		t.Fatal(err)
	}
	a, tt := openVersion(t, leftPath, rightPath)
	if a.err != "" || a.kind != 5 || !a.infoReady || a.infoKind != "reg" || a.panes[0] != nil || a.infoLeft != "" {
		t.Fatalf("kind=%d ready=%v name=%s left=%q err=%s", a.kind, a.infoReady, a.infoKind, a.infoLeft, a.err)
	}
	extra := `HKEY_CURRENT_USER\Software\GoCompare\Extra`
	name := `HKEY_CURRENT_USER\Software\GoCompare\Name`
	if len(a.infoDiffs) != 2 || a.infoDiffs[0] != extra || a.infoDiffs[1] != name {
		t.Fatalf("diffs %v", a.infoDiffs)
	}
	for _, text := range []string{"不读取实时注册表", "差异 " + extra, "差异 " + name} {
		if !tt.HasText(text) {
			t.Fatalf("missing %q in %v", text, tt.Texts())
		}
	}
	if tt.HasText("左侧版本") || tt.HasText("版本不同") || tt.HasText("注册表相同") || tt.HasText("One") {
		t.Fatalf("registry view leaked values: %v", tt.Texts())
	}
	same, sameView := openVersion(t, leftPath, leftPath)
	if same.err != "" || len(same.infoDiffs) != 0 || !sameView.HasText("注册表相同") || sameView.HasText("差异 "+name) {
		t.Fatalf("equal diffs=%v err=%s texts=%v", same.infoDiffs, same.err, sameView.Texts())
	}
	after, err := os.ReadFile(leftPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("reg bytes changed")
	}
}

func TestWorkspaceVersionRejectsText(t *testing.T) {
	dir := t.TempDir()
	leftPath := filepath.Join(dir, "note.txt")
	rightPath := filepath.Join(dir, "other.txt")
	if err := os.WriteFile(leftPath, []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rightPath, []byte("world\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(leftPath)
	if err != nil {
		t.Fatal(err)
	}
	a, tt := openVersion(t, leftPath, rightPath)
	if a.kind != 5 || a.infoReady || a.panes[0] != nil || a.err != "not a PE file" {
		t.Fatalf("kind=%d ready=%v err=%s", a.kind, a.infoReady, a.err)
	}
	if !tt.HasText("错误") || !tt.HasText("不读取实时注册表") || tt.HasText("左侧版本") || tt.HasText("差异 ") || tt.HasText("选择两个 PE") {
		t.Fatalf("texts %v err=%s", tt.Texts(), a.err)
	}
	regPath := filepath.Join(dir, "one.reg")
	pePath := filepath.Join(dir, "one.exe")
	regText := "Windows Registry Editor Version 5.00\n\n[HKEY_CURRENT_USER\\Software\\GoCompare]\n\"Name\"=\"One\"\n"
	if err = os.WriteFile(regPath, []byte(regText), 0o600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(pePath, workspacePE("1.2.3.4"), 0o600); err != nil {
		t.Fatal(err)
	}
	mixed, mixedView := openVersion(t, regPath, pePath)
	if mixed.kind != 5 || mixed.infoReady || mixed.err != "两侧须同为 .reg 或同为 PE" || mixed.infoLeft != "" || len(mixed.infoDiffs) != 0 {
		t.Fatalf("mixed ready=%v left=%q diffs=%v err=%s", mixed.infoReady, mixed.infoLeft, mixed.infoDiffs, mixed.err)
	}
	if mixedView.HasText("左侧版本") || mixedView.HasText("1.2.3.4") || mixedView.HasText(`差异 HKEY_CURRENT_USER\Software\GoCompare\Name`) {
		t.Fatalf("mixed view showed a file result: %v", mixedView.Texts())
	}
	after, err := os.ReadFile(leftPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("rejected file changed")
	}
	peAfter, err := os.ReadFile(pePath)
	if err != nil || !bytes.Equal(workspacePE("1.2.3.4"), peAfter) {
		t.Fatal("mixed pe changed")
	}
}

func openVersion(t *testing.T, left, right string) (*workspaceState, *ui.Tester) {
	t.Helper()
	a := &workspaceState{paths: [2]string{left, right}}
	tt := ui.NewTester(a.view, 1800, 880)
	tt.Frame()
	if err := tt.Click("版本与注册表"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("开始比较"); err != nil {
		t.Fatal(err)
	}
	return a, tt
}

func workspacePE(version string) []byte {
	return workspacePEBlob(append(workspaceUTF16Z("FileVersion"), workspaceUTF16Z(version)...))
}

func workspaceAlignedVersion(version string) []byte {
	key := workspaceUTF16Z("FileVersion")
	value := workspaceUTF16Z(version)
	body := make([]byte, 6+len(key))
	copy(body[6:], key)
	for len(body)%4 != 0 {
		body = append(body, 0)
	}
	body = append(body, value...)
	binary.LittleEndian.PutUint16(body[0:], uint16(len(body)))
	binary.LittleEndian.PutUint16(body[2:], uint16(len(value)/2))
	binary.LittleEndian.PutUint16(body[4:], 1)
	return body
}

func workspacePEBlob(blob []byte) []byte {
	const blobAt = 88
	sec := make([]byte, blobAt+len(blob))
	workspacePutDir(sec, 0, 16, 24, true)
	workspacePutDir(sec, 24, 1, 48, true)
	workspacePutDir(sec, 48, 0, 72, false)
	const sectionVA = 0x1000
	binary.LittleEndian.PutUint32(sec[72:], uint32(sectionVA+blobAt))
	binary.LittleEndian.PutUint32(sec[76:], uint32(len(blob)))
	copy(sec[blobAt:], blob)
	file := make([]byte, 0x80+len(sec))
	file[0], file[1] = 'M', 'Z'
	binary.LittleEndian.PutUint32(file[0x3c:], 0x40)
	copy(file[0x40:], []byte{'P', 'E', 0, 0})
	binary.LittleEndian.PutUint16(file[0x40+4+2:], 1)
	off := 0x40 + 4 + 20
	copy(file[off:off+8], []byte(".rsrc"))
	binary.LittleEndian.PutUint32(file[off+8:], uint32(len(sec)))
	binary.LittleEndian.PutUint32(file[off+12:], sectionVA)
	binary.LittleEndian.PutUint32(file[off+16:], uint32(len(sec)))
	binary.LittleEndian.PutUint32(file[off+20:], 0x80)
	copy(file[0x80:], sec)
	return file
}

func workspacePutDir(b []byte, off, id, target int, sub bool) {
	binary.LittleEndian.PutUint16(b[off+14:], 1)
	binary.LittleEndian.PutUint32(b[off+16:], uint32(id))
	rel := uint32(target)
	if sub {
		rel |= 0x80000000
	}
	binary.LittleEndian.PutUint32(b[off+20:], rel)
}

func workspaceUTF16Z(s string) []byte {
	u := utf16.Encode([]rune(s))
	out := make([]byte, (len(u)+1)*2)
	for i, v := range u {
		binary.LittleEndian.PutUint16(out[i*2:], v)
	}
	return out
}
