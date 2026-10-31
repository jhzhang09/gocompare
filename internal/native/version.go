package native

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/egoist/mygo/ui"
	"gocompare/internal/pever"
	"gocompare/internal/regfile"
)

// versionResult 是一次 PE 版本或 .reg 文本比较的结果。kind 为 pe 或 reg。
type versionResult struct {
	kind        string
	left, right string
	diffs       []string
}

// loadVersion 在后台读取两侧文件。两侧都以注册表 5.00 头开始时比较 .reg 文本，否则读取 PE 的 FileVersion。
// 不访问实时注册表。失败时清掉上一次的版本和差异，不交给文本比较。
func (a *workspaceState) loadVersion() {
	paths := a.paths
	a.infoGen++
	gen := a.infoGen
	a.job(func(ctx context.Context) {
		left, err := readVersionFile(paths[0])
		var right []byte
		if err == nil {
			right, err = readVersionFile(paths[1])
		}
		var result versionResult
		if err == nil && ctx.Err() != nil {
			err = ctx.Err()
		}
		if err == nil {
			result, err = compareVersionBytes(left, right)
		}
		a.publish(func() {
			if gen != a.infoGen {
				return
			}
			a.busy = false
			if err != nil {
				a.err = err.Error()
				a.clearVersion()
				return
			}
			a.infoKind = result.kind
			a.infoLeft = result.left
			a.infoRight = result.right
			a.infoDiffs = result.diffs
			a.infoReady = true
			a.err = ""
			a.message = "已读取版本或注册表"
		})
	})
}

func (a *workspaceState) clearVersion() {
	a.infoKind = ""
	a.infoLeft = ""
	a.infoRight = ""
	a.infoDiffs = nil
	a.infoReady = false
}

// versionView 只展示已经读到的 FileVersion 或 .reg 差异。实时注册表不在这里读取。
func (a *workspaceState) versionView(c *ui.Context) {
	ui.Text(c, "不读取实时注册表").Padding(8, 20)
	if !a.infoReady {
		if a.err == "" {
			ui.Text(c, "选择两个 PE 或两个 .reg 文件").Padding(8, 20)
		}
		return
	}
	if a.infoKind == "pe" {
		ui.Text(c, "左侧版本 "+a.infoLeft).Padding(4, 20)
		ui.Text(c, "右侧版本 "+a.infoRight).Padding(4, 20)
		note := "版本不同"
		if a.infoLeft == a.infoRight {
			note = "版本相同"
		}
		ui.Text(c, note).Padding(4, 20)
		return
	}
	if len(a.infoDiffs) == 0 {
		ui.Text(c, "注册表相同").Padding(4, 20)
		return
	}
	for _, line := range a.infoDiffs {
		ui.Text(c, "差异 "+line).Padding(4, 20)
	}
}

func compareVersionBytes(left, right []byte) (versionResult, error) {
	leftReg, rightReg := looksLikeRegistry(left), looksLikeRegistry(right)
	if leftReg || rightReg {
		if !leftReg || !rightReg {
			return versionResult{}, errors.New("两侧须同为 .reg 或同为 PE")
		}
		lv, err := regfile.Parse(string(left))
		if err != nil {
			return versionResult{}, err
		}
		rv, err := regfile.Parse(string(right))
		if err != nil {
			return versionResult{}, err
		}
		return versionResult{kind: "reg", diffs: regfile.Diff(lv, rv)}, nil
	}
	lv, err := pever.FileVersion(left)
	if err != nil {
		return versionResult{}, err
	}
	rv, err := pever.FileVersion(right)
	if err != nil {
		return versionResult{}, err
	}
	return versionResult{kind: "pe", left: lv, right: rv}, nil
}

// looksLikeRegistry 只看第一条实质内容是否为 5.00 头，用来选择比较器。它不读取注册表。
func looksLikeRegistry(data []byte) bool {
	s := bufio.NewScanner(bytes.NewReader(data))
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}
		return line == "Windows Registry Editor Version 5.00"
	}
	return false
}

func readVersionFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	const max = 16 << 20
	data, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, err
	}
	if len(data) > max {
		return nil, fmt.Errorf("%s exceeds 16 MiB", path)
	}
	return data, nil
}
