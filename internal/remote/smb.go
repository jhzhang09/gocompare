package remote

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/hirochachacha/go-smb2"
)

// SMB 通过 SMB2/3 访问共享目录，使用 NTLMv2 认证；协议版本、签名和加密由服务端协商决定。
// User 为空时以匿名身份连接，是否允许由服务端决定。
type SMB struct {
	Addr   string // host:port
	Share  string // 共享名，不含主机
	User   string
	Pass   string
	Domain string
}

func (s *SMB) Read(ctx context.Context, path string) ([]byte, error) {
	var data []byte
	err := s.withShare(ctx, func(fs *smb2.Share) error {
		f, err := fs.Open(smbPath(path))
		if err != nil {
			return fmt.Errorf("smb read %s: %w", path, err)
		}
		defer f.Close()
		data, err = io.ReadAll(io.LimitReader(f, maxResponseBytes+1))
		if err != nil {
			return fmt.Errorf("smb read %s: %w", path, err)
		}
		if len(data) > maxResponseBytes {
			return fmt.Errorf("smb read %s: remote file exceeds 32 MiB", path)
		}
		return nil
	})
	return data, err
}

// Write 以 O_TRUNC 打开，已有文件会被截断后重写。
func (s *SMB) Write(ctx context.Context, path string, data []byte) error {
	return s.withShare(ctx, func(fs *smb2.Share) error {
		if err := fs.WriteFile(smbPath(path), data, 0o644); err != nil {
			return fmt.Errorf("smb write %s: %w", path, err)
		}
		return nil
	})
}

func (s *SMB) Delete(ctx context.Context, path string) error {
	return s.withShare(ctx, func(fs *smb2.Share) error {
		if err := fs.Remove(smbPath(path)); err != nil {
			return fmt.Errorf("smb delete %s: %w", path, err)
		}
		return nil
	})
}

func (s *SMB) withShare(ctx context.Context, fn func(*smb2.Share) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.Share == "" {
		return fmt.Errorf("smb share name is required")
	}
	host, _, err := net.SplitHostPort(s.Addr)
	if err != nil {
		return fmt.Errorf("smb address %q: %w", s.Addr, err)
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", s.Addr)
	if err != nil {
		return ctxErr(ctx, err)
	}
	defer conn.Close()
	// 库在等待响应时会检查 context；这里再兜底关闭连接，保证取消时阻塞的读写也能返回。
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-stop:
		}
	}()
	d := &smb2.Dialer{Initiator: &smb2.NTLMInitiator{User: s.User, Password: s.Pass, Domain: s.Domain}}
	session, err := d.DialContext(ctx, conn)
	if err != nil {
		return ctxErr(ctx, fmt.Errorf("smb %s: %w", s.Addr, err))
	}
	defer session.Logoff()
	// 挂载路径只用主机名：库会把带端口的地址拼进 \\server\share，导致路径非法。
	share, err := session.WithContext(ctx).Mount(`\\` + host + `\` + s.Share)
	if err != nil {
		return ctxErr(ctx, fmt.Errorf("smb mount %s: %w", s.Share, err))
	}
	defer share.Umount()
	return ctxErr(ctx, fn(share.WithContext(ctx)))
}

// smbPath 把 URL 风格路径转成共享内的相对路径；库要求不带前导分隔符。
func smbPath(path string) string {
	return strings.ReplaceAll(strings.TrimLeft(path, `/\`), "/", `\`)
}
