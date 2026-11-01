package remote

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// ErrAuth 表示服务端拒绝了凭据或缺少凭据；界面据此询问密码后重试。
var ErrAuth = errors.New("remote authentication failed")

// ErrHostKeyChanged 表示 SFTP 服务端的主机密钥与 known_hosts 中登记的不一致，连接被拒绝。
var ErrHostKeyChanged = errors.New("SSH host key does not match known_hosts")

// ErrKeyPassphrase 表示私钥受口令保护但没有提供口令，且其他方式也没能登录。
var ErrKeyPassphrase = errors.New("private key is protected by a passphrase")

// ErrKeyPassphraseWrong 表示提供的私钥口令不正确。
var ErrKeyPassphraseWrong = errors.New("private key passphrase is incorrect")

// HostKeyError 表示 SFTP 服务端的主机密钥尚未登记。界面展示指纹，用户确认后调用 Trust 登记再重试。
// 密钥与已登记的不一致时不会返回此错误，而是直接拒绝连接。
type HostKeyError struct {
	Host        string // known_hosts 中的主机写法
	Fingerprint string // SHA256 指纹
	KeyType     string
	KnownHosts  string // 将要写入的 known_hosts 文件
	key         ssh.PublicKey
}

func (e *HostKeyError) Error() string {
	return fmt.Sprintf("unknown SSH host key for %s (%s %s); verify the fingerprint with the server owner and add it to %s", e.Host, e.KeyType, e.Fingerprint, e.KnownHosts)
}

// Trust 把主机密钥追加到 known_hosts，文件或 ~/.ssh 不存在时以仅本人可读写的权限创建。
func (e *HostKeyError) Trust() error {
	if err := os.MkdirAll(filepath.Dir(e.KnownHosts), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(e.KnownHosts, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	_, err = f.WriteString(knownhosts.Line([]string{e.Host}, e.key) + "\n")
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

// File 把存储与其中的路径绑定为单个远程文件，供文档层读写。
// Timeout 大于 0 时限制每次读写的总时长，避免界面在无响应的服务端上一直忙碌。
type File struct {
	Store   Store
	Path    string
	Timeout time.Duration
}

func (f File) Read(ctx context.Context) ([]byte, error) {
	ctx, cancel := f.limit(ctx)
	defer cancel()
	return f.Store.Read(ctx, f.Path)
}

func (f File) Write(ctx context.Context, data []byte) error {
	ctx, cancel := f.limit(ctx)
	defer cancel()
	return f.Store.Write(ctx, f.Path, data)
}

func (f File) limit(ctx context.Context) (context.Context, context.CancelFunc) {
	if f.Timeout > 0 {
		return context.WithTimeout(ctx, f.Timeout)
	}
	return context.WithCancel(ctx)
}

// IsURL 判断输入是否为本包支持的远程地址；Windows 盘符路径（C:\...）不算。
func IsURL(s string) bool {
	scheme, _, ok := strings.Cut(strings.TrimSpace(s), "://")
	if !ok {
		return false
	}
	switch strings.ToLower(scheme) {
	case "ftp", "ftps", "sftp", "smb", "webdav", "webdav+https", "s3", "s3+https", "dropbox", "onedrive", "svn", "svn+http", "svn+https":
		return true
	}
	return false
}

// Redact 去掉 URL 中的密码，用于界面显示、标题和会话保存。
func Redact(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.User == nil {
		return strings.TrimSpace(raw)
	}
	if _, ok := u.User.Password(); ok {
		u.User = url.User(u.User.Username())
	}
	return u.String()
}

// CredentialKey 标识一组可复用的凭据：协议、用户与主机端口，不含路径和密码。
func CredentialKey(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	name := ""
	if u.User != nil {
		name = u.User.Username()
	}
	return strings.ToLower(u.Scheme) + "://" + name + "@" + strings.ToLower(u.Host)
}

// PasswordOf 返回 URL 中携带的密码，没有时返回空串。
func PasswordOf(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.User == nil {
		return ""
	}
	pass, _ := u.User.Password()
	return pass
}

// AcceptsPassword 报告该协议是否用密码认证；S3、Dropbox、OneDrive 的凭据只能写在 URL 里。
func AcceptsPassword(s Store) bool {
	switch s.(type) {
	case *FTP, *SFTP, *SMB, *WebDAV, *SVN:
		return true
	}
	return false
}

// SetPassword 为存储设置密码；返回 false 表示该协议不用密码认证。
func SetPassword(s Store, pass string) bool {
	switch v := s.(type) {
	case *FTP:
		v.Pass = pass
	case *SFTP:
		v.Pass = pass
	case *SMB:
		v.Pass = pass
	case *WebDAV:
		v.Pass = pass
	case *SVN:
		v.Pass = pass
	default:
		return false
	}
	return true
}

// HasPassword 报告存储是否已带密码。
func HasPassword(s Store) bool {
	switch v := s.(type) {
	case *FTP:
		return v.Pass != ""
	case *SFTP:
		return v.Pass != ""
	case *SMB:
		return v.Pass != ""
	case *WebDAV:
		return v.Pass != ""
	case *SVN:
		return v.Pass != ""
	}
	return false
}

// authError 把认证失败统一包装成 ErrAuth，保留原始信息。
func authError(err error) error {
	return fmt.Errorf("%w: %v", ErrAuth, err)
}
