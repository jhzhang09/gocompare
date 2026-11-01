// Package compare 提供与窗口无关的比较用例。
package compare

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
)

// EqualFiles 流式比较两个普通文件的原始字节；错误不会被视为相同。
func EqualFiles(ctx context.Context, left, right string) (bool, error) {
	for _, path := range []string{left, right} {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		info, err := os.Stat(path)
		if err != nil {
			return false, err
		}
		if !info.Mode().IsRegular() {
			return false, fmt.Errorf("not a regular file: %s", path)
		}
	}
	a, err := os.Open(left)
	if err != nil {
		return false, err
	}
	defer a.Close()
	b, err := os.Open(right)
	if err != nil {
		return false, err
	}
	defer b.Close()
	for _, f := range []*os.File{a, b} {
		st, err := f.Stat()
		if err != nil {
			return false, err
		}
		if !st.Mode().IsRegular() {
			return false, fmt.Errorf("not a regular file: %s", f.Name())
		}
	}
	return equalReaders(ctx, a, b)
}

// equalReaders 分块比较两个字节流，读到相同长度且内容一致才算相同；读取错误不会被视为相同。
func equalReaders(ctx context.Context, a, b io.Reader) (bool, error) {
	x, y := make([]byte, 64<<10), make([]byte, 64<<10)
	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		n, e := io.ReadFull(a, x)
		m, f := io.ReadFull(b, y)
		if e != nil && e != io.EOF && e != io.ErrUnexpectedEOF {
			return false, e
		}
		if f != nil && f != io.EOF && f != io.ErrUnexpectedEOF {
			return false, f
		}
		if !bytes.Equal(x[:n], y[:m]) {
			return false, nil
		}
		if e != nil || f != nil {
			return e == f, nil
		}
	}
}
