package remote

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"

	"github.com/hirochachacha/go-smb2"
	"github.com/pkg/sftp"
)

// Conn 在一次连接内完成多次列目录和读文件，目录比较据此避免每个目录、每个文件都重新握手。
// 连接绑定到 Connect 的 ctx：ctx 取消时连接关闭；用完必须 Close。
type Conn interface {
	Lister
	Read(ctx context.Context, path string) ([]byte, error)
	Readlink(ctx context.Context, path string) (string, error)
	Close() error
}

// Connect 为存储建立可复用的连接。SFTP、SMB 保持同一条连接；其他协议本身是无状态请求，逐次调用。
func Connect(ctx context.Context, s Store) (Conn, error) {
	switch v := s.(type) {
	case *SFTP:
		client, done, err := v.connect(ctx)
		if err != nil {
			return nil, err
		}
		return &sftpConn{c: client, done: done}, nil
	case *SMB:
		share, done, err := v.connectShare(ctx)
		if err != nil {
			return nil, err
		}
		return &smbConn{fs: share, done: done}, nil
	}
	l, ok := s.(Lister)
	if !ok {
		return nil, ErrListUnsupported
	}
	return statelessConn{Store: s, Lister: l}, nil
}

type sftpConn struct {
	c    *sftp.Client
	done func()
}

func (c *sftpConn) List(ctx context.Context, dir string) (string, []Entry, error) {
	return sftpList(ctx, c.c, dir)
}

func (c *sftpConn) Read(ctx context.Context, p string) ([]byte, error) {
	f, err := c.c.Open(p)
	if err != nil {
		return nil, fmt.Errorf("sftp read %s: %w", p, err)
	}
	defer f.Close()
	return readCapped(f, "sftp read "+p)
}

func (c *sftpConn) Readlink(ctx context.Context, p string) (string, error) {
	return c.c.ReadLink(p)
}

func (c *sftpConn) Close() error { c.done(); return nil }

type smbConn struct {
	fs   *smb2.Share
	done func()
}

func (c *smbConn) List(ctx context.Context, dir string) (string, []Entry, error) {
	dir = strings.Trim(dir, "/")
	infos, err := c.fs.ReadDir(smbPath(dir))
	if err != nil {
		return "", nil, fmt.Errorf("smb list %s: %w", dir, err)
	}
	return dir, entriesOf(infos), nil
}

func (c *smbConn) Read(ctx context.Context, p string) ([]byte, error) {
	f, err := c.fs.Open(smbPath(p))
	if err != nil {
		return nil, fmt.Errorf("smb read %s: %w", p, err)
	}
	defer f.Close()
	return readCapped(f, "smb read "+p)
}

func (c *smbConn) Readlink(ctx context.Context, p string) (string, error) {
	return c.fs.Readlink(smbPath(p))
}

func (c *smbConn) Close() error { c.done(); return nil }

// statelessConn 用于 FTP、WebDAV 等：每次调用各自完成请求，没有需要保持的连接。
type statelessConn struct {
	Store
	Lister
}

func (statelessConn) Readlink(context.Context, string) (string, error) {
	return "", fmt.Errorf("symbolic links are not supported by this protocol")
}

func (statelessConn) Close() error { return nil }

func readCapped(r io.Reader, what string) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", what, err)
	}
	if len(data) > maxResponseBytes {
		return nil, fmt.Errorf("%s: remote file exceeds 32 MiB", what)
	}
	return data, nil
}

// ParseDirURL 解析指向远程目录的地址，返回存储与目录路径（写法同 Lister）。
// SFTP 的空路径表示登录目录，"/" 表示根目录；SMB 必须带共享名。
func ParseDirURL(rawURL string) (Store, string, error) {
	s, p, err := parse(rawURL, false)
	if err != nil {
		return nil, "", err
	}
	if v, ok := s.(*SMB); ok && v.Share == "" {
		return nil, "", fmt.Errorf("smb URL must name a share: smb://host/share/dir")
	}
	return s, p, nil
}

// JoinPath 在远程地址的路径后追加相对路径 rel，保留协议、用户与主机。
func JoinPath(raw, rel string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return strings.TrimRight(raw, "/") + "/" + rel
	}
	base := u.Path
	if base == "" {
		base = "/"
	}
	u.Path = path.Join(base, rel)
	u.RawPath = ""
	return u.String()
}
