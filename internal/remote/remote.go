// Package remote 实现远程来源客户端：FTP/FTPS、SFTP（SSH）、SMB2/3、WebDAV、S3、Dropbox、OneDrive 与 SVN HTTP 子集。
// Dropbox 与 OneDrive 只实现各自声明的 HTTP 子集。
package remote

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// Store 是一个可读写、可删除的路径空间。
type Store interface {
	Read(ctx context.Context, path string) ([]byte, error)
	Write(ctx context.Context, path string, data []byte) error
	Delete(ctx context.Context, path string) error
}

// ParseURL 解析 URL 并返回对应的 Store 及路径。
// webdav、s3、svn 默认走 http；使用 webdav+https、s3+https、svn+https 走 https。
func ParseURL(rawURL string) (Store, string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, "", err
	}
	user := ""
	pass := ""
	if u.User != nil {
		user = u.User.Username()
		pass, _ = u.User.Password()
	}
	path := strings.TrimPrefix(u.Path, "/")
	switch u.Scheme {
	case "ftp":
		return &FTP{Addr: u.Host, User: user, Pass: pass}, path, nil
	case "ftps":
		return &FTP{Addr: u.Host, User: user, Pass: pass, TLS: &tls.Config{ServerName: u.Hostname()}}, path, nil
	case "sftp":
		// 路径按 curl 约定：/abs 为绝对路径，/~/rel 相对登录用户的主目录。
		p := u.Path
		if strings.HasPrefix(p, "/~/") {
			p = p[3:]
		}
		if p == "" || p == "/" {
			return nil, "", fmt.Errorf("sftp URL needs a file path")
		}
		return &SFTP{Addr: hostPort(u, "22"), User: user, Pass: pass}, p, nil
	case "smb":
		// smb://[domain;]user:pass@host[:port]/share/path，域也可写成 domain\user（需编码为 %5C）。
		share, rel, _ := strings.Cut(path, "/")
		if share == "" || rel == "" {
			return nil, "", fmt.Errorf("smb URL must be smb://host/share/path")
		}
		domain, name := "", user
		if i := strings.IndexAny(user, ";\\"); i >= 0 {
			domain, name = user[:i], user[i+1:]
		}
		return &SMB{Addr: hostPort(u, "445"), Share: share, User: name, Pass: pass, Domain: domain}, rel, nil
	case "webdav", "webdav+https":
		return &WebDAV{Base: httpBase(u.Scheme, u.Host), User: user, Pass: pass}, path, nil
	case "s3", "s3+https":
		return &S3{Base: httpBase(u.Scheme, u.Host), AccessKey: user, SecretKey: pass}, path, nil
	case "dropbox":
		token := user
		if token == "" {
			token = u.Host
		}
		return &Dropbox{Base: "https://api.dropboxapi.com", ContentBase: "https://content.dropboxapi.com", Token: token}, path, nil
	case "onedrive":
		token := user
		if token == "" {
			token = u.Host
		}
		return &OneDrive{Base: "https://graph.microsoft.com", Token: token}, path, nil
	case "svn", "svn+http", "svn+https":
		return &SVN{Base: httpBase(u.Scheme, u.Host), User: user, Pass: pass}, path, nil
	default:
		return nil, "", fmt.Errorf("unsupported remote scheme: %s", u.Scheme)
	}
}

// hostPort 返回带端口的地址，URL 未写端口时使用协议默认端口。
func hostPort(u *url.URL, defaultPort string) string {
	port := u.Port()
	if port == "" {
		port = defaultPort
	}
	return net.JoinHostPort(u.Hostname(), port)
}

// httpBase 按 scheme 的 "+https" 后缀选择传输协议；未声明时保持既有的 http 行为。
func httpBase(scheme, host string) string {
	if strings.HasSuffix(scheme, "+https") {
		return "https://" + host
	}
	return "http://" + host
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

// maxResponseBytes 是所有协议单次读取的上限；超过时报错，不返回截断内容。
const maxResponseBytes = 32 << 20

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
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, resp.StatusCode, ctxErr(ctx, err)
	}
	if len(body) > maxResponseBytes {
		return nil, resp.StatusCode, fmt.Errorf("remote response exceeds 32 MiB")
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
