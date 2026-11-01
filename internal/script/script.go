// Package script 执行文本脚本。每条命令都调用比较、合并或十六进制核心，而不是另一套实现。
package script

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"gocompare/internal/compare"
	"gocompare/internal/document"
	"gocompare/internal/hex"
	"gocompare/internal/merge"
)

// Execute 按行执行脚本。返回 0 表示全部相同，1 表示存在差异或冲突，错误时返回 2。
func Execute(ctx context.Context, text string, out io.Writer) (int, error) {
	code := 0
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		next, err := dispatch(ctx, fields, out)
		if err != nil {
			return 2, err
		}
		if next > code {
			code = next
		}
	}
	return code, nil
}

func dispatch(ctx context.Context, fields []string, out io.Writer) (int, error) {
	switch fields[0] {
	case "compare":
		if len(fields) != 3 {
			return 2, fmt.Errorf("compare needs two paths")
		}
		equal, err := compare.EqualFiles(ctx, fields[1], fields[2])
		if err != nil {
			return 2, err
		}
		if equal {
			fmt.Fprintln(out, "equal")
			return 0, nil
		}
		fmt.Fprintln(out, "different")
		return 1, nil
	case "hex":
		if len(fields) != 3 {
			return 2, fmt.Errorf("hex needs two paths")
		}
		left, err := readLimited(fields[1])
		if err != nil {
			return 2, err
		}
		right, err := readLimited(fields[2])
		if err != nil {
			return 2, err
		}
		chunks := hex.Diff(left, right, 16)
		fmt.Fprint(out, hex.FormatDiff(chunks))
		if len(chunks) > 0 {
			return 1, nil
		}
		return 0, nil
	case "merge":
		if len(fields) != 4 {
			return 2, fmt.Errorf("merge needs base, left and right")
		}
		base, err := document.ReadMergeable(ctx, fields[1])
		if err != nil {
			return 2, err
		}
		left, err := document.ReadMergeable(ctx, fields[2])
		if err != nil {
			return 2, err
		}
		right, err := document.ReadMergeable(ctx, fields[3])
		if err != nil {
			return 2, err
		}
		result, err := merge.MergeThree(base.Text, left.Text, right.Text)
		if err != nil {
			return 2, err
		}
		fmt.Fprint(out, result.Text())
		if result.ConflictCount() > 0 {
			return 1, nil
		}
		return 0, nil
	default:
		return 2, fmt.Errorf("unknown script command %s", fields[0])
	}
}

// readLimited 与 CLI 的 hex 命令使用同样的 16 MiB 上限，避免脚本把大文件整个读进内存。
func readLimited(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxHexBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxHexBytes {
		return nil, fmt.Errorf("%s exceeds 16 MiB", path)
	}
	return data, nil
}

const maxHexBytes = 16 << 20
