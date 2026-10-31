// Package extend 按版本 1 的规格启动外部进程，做比较、转换或压缩。
// 参数按 argv 逐项替换，不经过 shell。规格决定可执行文件，调用方只能提交可信规格。
// 这不是 Beyond Compare 的插件包格式，也不自带新的压缩算法。
package extend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gocompare/internal/proc"
)

const (
	maxSpec   = 1 << 20
	maxOutput = 16 << 20
)

// Spec 是一条外部进程规则。Kind 取 compare、convert 或 compress。
// Args[0] 必须是绝对路径。占位符 %left、%right、%in、%out 必须独占一个参数。
type Spec struct {
	Version int      `json:"version"`
	Kind    string   `json:"kind"`
	Args    []string `json:"args"`
}

// Result 是进程成功结束后的产物。Code 只在 compare 时为 0 或 1。
// Payload 在 compare 和 convert 时是标准输出，在 compress 时是 %out 文件的字节。
type Result struct {
	Payload []byte
	Code    int
}

// Load 读取规格文件。超过 1 MiB、版本不是 1 或种类未知时返回错误。
func Load(path string) (Spec, error) {
	f, err := os.Open(path)
	if err != nil {
		return Spec{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxSpec+1))
	if err != nil {
		return Spec{}, err
	}
	if len(data) > maxSpec {
		return Spec{}, errors.New("extension spec exceeds 1 MiB")
	}
	return Parse(data)
}

// Parse 解析规格 JSON。未知字段会被拒绝。
func Parse(data []byte) (Spec, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var spec Spec
	if err := dec.Decode(&spec); err != nil {
		return Spec{}, err
	}
	if dec.More() {
		return Spec{}, errors.New("extension spec has extra data")
	}
	if spec.Version != 1 {
		return Spec{}, errors.New("extension version must be 1")
	}
	switch spec.Kind {
	case "compare", "convert", "compress":
	default:
		return Spec{}, errors.New("unknown extension kind")
	}
	if len(spec.Args) == 0 {
		return Spec{}, errors.New("extension command is empty")
	}
	return spec, nil
}

// Paths 是占位符要换成的真实路径。比较用 Left 与 Right，转换用 In，压缩用 In 与 Out。
type Paths struct {
	Left  string
	Right string
	In    string
	Out   string
}

// Execute 启动规格中的进程。compare 以退出码 0 为相同、1 为不同。
// 其他退出码、无法启动、输出超过 16 MiB，或压缩结果文件缺失，都是错误。
// 转换失败时不把标准输出当作转换结果。压缩失败时不返回已写出的部分字节。
// 取消 ctx 会结束该进程组。
func Execute(ctx context.Context, spec Spec, paths Paths) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	argv, err := bind(spec, paths)
	if err != nil {
		return Result{}, err
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	proc.HideConsole(cmd)
	proc.KillGroup(cmd)
	stdout := &capture{limit: maxOutput}
	stderr := &capture{limit: maxOutput}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	runErr := cmd.Run()
	if stdout.exceeded || stderr.exceeded {
		return Result{}, errors.New("extension output exceeds 16 MiB")
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return Result{}, ctxErr
	}
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			code := exitErr.ExitCode()
			if spec.Kind == "compare" && code == 1 {
				return Result{Payload: stdout.buf.Bytes(), Code: 1}, nil
			}
			msg := strings.TrimSpace(stderr.buf.String())
			if msg == "" {
				return Result{}, fmt.Errorf("extension exit %d", code)
			}
			return Result{}, fmt.Errorf("extension exit %d: %s", code, msg)
		}
		return Result{}, fmt.Errorf("extension start: %w", runErr)
	}
	if spec.Kind == "compress" {
		payload, readErr := readOutput(paths.Out)
		if readErr != nil {
			return Result{}, readErr
		}
		return Result{Payload: payload}, nil
	}
	return Result{Payload: stdout.buf.Bytes()}, nil
}

func bind(spec Spec, paths Paths) ([]string, error) {
	if spec.Version != 1 {
		return nil, errors.New("extension version must be 1")
	}
	switch spec.Kind {
	case "compare", "convert", "compress":
	default:
		return nil, errors.New("unknown extension kind")
	}
	if len(spec.Args) == 0 || !filepath.IsAbs(spec.Args[0]) {
		return nil, errors.New("extension command must be a path")
	}
	argv := make([]string, len(spec.Args))
	var sawLeft, sawRight, sawIn, sawOut bool
	for i, arg := range spec.Args {
		switch arg {
		case "%left":
			if strings.TrimSpace(paths.Left) == "" {
				return nil, errors.New("%left path is empty")
			}
			argv[i] = paths.Left
			sawLeft = true
		case "%right":
			if strings.TrimSpace(paths.Right) == "" {
				return nil, errors.New("%right path is empty")
			}
			argv[i] = paths.Right
			sawRight = true
		case "%in":
			if strings.TrimSpace(paths.In) == "" {
				return nil, errors.New("%in path is empty")
			}
			argv[i] = paths.In
			sawIn = true
		case "%out":
			if strings.TrimSpace(paths.Out) == "" {
				return nil, errors.New("%out path is empty")
			}
			argv[i] = paths.Out
			sawOut = true
		default:
			if strings.Contains(arg, "%left") || strings.Contains(arg, "%right") || strings.Contains(arg, "%in") || strings.Contains(arg, "%out") {
				return nil, errors.New("placeholder must be a whole argument")
			}
			argv[i] = arg
		}
	}
	switch spec.Kind {
	case "compare":
		if !sawLeft || !sawRight {
			return nil, errors.New("compare requires %left and %right")
		}
	case "convert":
		if !sawIn {
			return nil, errors.New("convert requires %in")
		}
		if sawOut {
			return nil, errors.New("convert uses stdout, not %out")
		}
	case "compress":
		if !sawIn || !sawOut {
			return nil, errors.New("compress requires %in and %out")
		}
	}
	return argv, nil
}

func readOutput(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("extension output missing")
		}
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxOutput+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxOutput {
		return nil, errors.New("extension output exceeds 16 MiB")
	}
	return data, nil
}

type capture struct {
	buf      bytes.Buffer
	limit    int
	exceeded bool
}

func (c *capture) Write(p []byte) (int, error) {
	if c.exceeded || c.buf.Len() >= c.limit {
		c.exceeded = true
		return 0, errors.New("extension output exceeds 16 MiB")
	}
	remain := c.limit - c.buf.Len()
	if len(p) > remain {
		_, _ = c.buf.Write(p[:remain])
		c.exceeded = true
		return remain, errors.New("extension output exceeds 16 MiB")
	}
	return c.buf.Write(p)
}
