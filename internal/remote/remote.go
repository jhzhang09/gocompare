// Package remote 实现可用可控夹具验收的远程来源客户端。
// 这里没有真实云账号；Dropbox 与 OneDrive 只实现各自声明的 HTTP 子集。
package remote

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// Store 是一个可读写、可删除的路径空间。
type Store interface {
	Read(ctx context.Context, path string) ([]byte, error)
	Write(ctx context.Context, path string, data []byte) error
	Delete(ctx context.Context, path string) error
}

func ctxErr(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if cerr := ctx.Err(); cerr != nil {
		return cerr
	}
	return err
}

func readLimited(ctx context.Context, client *http.Client, req *http.Request) ([]byte, int, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, ctxErr(ctx, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, resp.StatusCode, ctxErr(ctx, err)
	}
	return body, resp.StatusCode, nil
}

func statusError(kind, path string, code int, body []byte) error {
	if len(body) > 180 {
		body = body[:180]
	}
	return fmt.Errorf("%s %s: %d %s", kind, path, code, bytesTrim(body))
}

func bytesTrim(b []byte) string {
	return string(b)
}
