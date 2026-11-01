package remote

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// SFTP 通过 SSH 连接读写远程文件。
// 主机密钥必须已在 known_hosts 中登记：不做首次连接自动信任，未知或不匹配的密钥都拒绝连接。
// 认证顺序：Pass 非空时只用密码（含 keyboard-interactive）；否则依次尝试 ssh-agent 和私钥文件。
type SFTP struct {
	Addr string // host:port
	User string // 为空时使用当前系统用户
	Pass string
	// IdentityFiles 是私钥文件；为空时尝试 ~/.ssh/id_ed25519、id_ecdsa、id_rsa。
	// 带口令的私钥需先加入 ssh-agent。
	IdentityFiles []string
	// KnownHosts 是 known_hosts 文件；为空时使用 ~/.ssh/known_hosts。
	KnownHosts []string
	// HostKeyCallback 非空时代替 KnownHosts，供嵌入方自行校验主机密钥。
	HostKeyCallback ssh.HostKeyCallback
}

func (s *SFTP) Read(ctx context.Context, path string) ([]byte, error) {
	var data []byte
	err := s.withClient(ctx, func(c *sftp.Client) error {
		f, err := c.Open(path)
		if err != nil {
			return fmt.Errorf("sftp read %s: %w", path, err)
		}
		defer f.Close()
		data, err = io.ReadAll(io.LimitReader(f, maxResponseBytes+1))
		if err != nil {
			return fmt.Errorf("sftp read %s: %w", path, err)
		}
		if len(data) > maxResponseBytes {
			return fmt.Errorf("sftp read %s: remote file exceeds 32 MiB", path)
		}
		return nil
	})
	return data, err
}

func (s *SFTP) Write(ctx context.Context, path string, data []byte) error {
	return s.withClient(ctx, func(c *sftp.Client) error {
		f, err := c.Create(path)
		if err != nil {
			return fmt.Errorf("sftp write %s: %w", path, err)
		}
		_, err = f.Write(data)
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			return fmt.Errorf("sftp write %s: %w", path, err)
		}
		return nil
	})
}

func (s *SFTP) Delete(ctx context.Context, path string) error {
	return s.withClient(ctx, func(c *sftp.Client) error {
		if err := c.Remove(path); err != nil {
			return fmt.Errorf("sftp delete %s: %w", path, err)
		}
		return nil
	})
}

func (s *SFTP) withClient(ctx context.Context, fn func(*sftp.Client) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	config, release, err := s.clientConfig()
	if err != nil {
		return err
	}
	defer release()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", s.Addr)
	if err != nil {
		return ctxErr(ctx, err)
	}
	defer conn.Close()
	// SSH 握手和 SFTP 请求都不接受 context；取消时关闭底层连接让阻塞调用返回。
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-stop:
		}
	}()
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, s.Addr, config)
	if err != nil {
		return ctxErr(ctx, fmt.Errorf("ssh %s: %w", s.Addr, err))
	}
	sshClient := ssh.NewClient(sshConn, chans, reqs)
	defer sshClient.Close()
	client, err := sftp.NewClient(sshClient)
	if err != nil {
		return ctxErr(ctx, fmt.Errorf("sftp %s: %w", s.Addr, err))
	}
	defer client.Close()
	return ctxErr(ctx, fn(client))
}

// clientConfig 返回 SSH 配置和释放 ssh-agent 连接的函数。
func (s *SFTP) clientConfig() (*ssh.ClientConfig, func(), error) {
	name := s.User
	if name == "" {
		u, err := user.Current()
		if err != nil {
			return nil, nil, fmt.Errorf("sftp user is required: %w", err)
		}
		// Windows 返回 DOMAIN\user，SSH 登录名只取用户部分。
		name = u.Username[strings.LastIndex(u.Username, `\`)+1:]
	}
	hostKey := s.HostKeyCallback
	if hostKey == nil {
		var err error
		if hostKey, err = s.knownHostsCallback(); err != nil {
			return nil, nil, err
		}
	}
	auth, release, err := s.authMethods()
	if err != nil {
		return nil, nil, err
	}
	return &ssh.ClientConfig{User: name, Auth: auth, HostKeyCallback: hostKey}, release, nil
}

// knownHostsCallback 包装 knownhosts 的错误，给出可执行的处理方法。
func (s *SFTP) knownHostsCallback() (ssh.HostKeyCallback, error) {
	files := s.KnownHosts
	if len(files) == 0 {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("locate known_hosts: %w", err)
		}
		files = []string{filepath.Join(home, ".ssh", "known_hosts")}
	}
	cb, err := knownhosts.New(files...)
	if err != nil {
		return nil, fmt.Errorf("load known_hosts %v: %w; add the server key first, e.g. ssh-keyscan -p <port> <host> >> ~/.ssh/known_hosts", files, err)
	}
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := cb(hostname, remote, key)
		var keyErr *knownhosts.KeyError
		if errors.As(err, &keyErr) {
			if len(keyErr.Want) == 0 {
				return fmt.Errorf("unknown SSH host key for %s (%s %s); verify the fingerprint with the server owner and add it to %v", hostname, key.Type(), ssh.FingerprintSHA256(key), files)
			}
			return fmt.Errorf("SSH host key for %s does not match known_hosts (got %s %s); the server key changed or the connection is being intercepted", hostname, key.Type(), ssh.FingerprintSHA256(key))
		}
		return err
	}, nil
}

func (s *SFTP) authMethods() ([]ssh.AuthMethod, func(), error) {
	release := func() {}
	if s.Pass != "" {
		pass := s.Pass
		return []ssh.AuthMethod{
			ssh.Password(pass),
			// 部分服务端只开放 keyboard-interactive，提示一律用同一个密码回答。
			ssh.KeyboardInteractive(func(_, _ string, questions []string, _ []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range answers {
					answers[i] = pass
				}
				return answers, nil
			}),
		}, release, nil
	}
	var methods []ssh.AuthMethod
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		if conn, err := net.Dial("unix", sock); err == nil {
			release = func() { conn.Close() }
			methods = append(methods, ssh.PublicKeysCallback(agent.NewClient(conn).Signers))
		}
	}
	fail := func(err error) ([]ssh.AuthMethod, func(), error) {
		release()
		return nil, nil, err
	}
	files := s.IdentityFiles
	explicit := len(files) > 0
	if !explicit {
		if home, err := os.UserHomeDir(); err == nil {
			for _, name := range []string{"id_ed25519", "id_ecdsa", "id_rsa"} {
				files = append(files, filepath.Join(home, ".ssh", name))
			}
		}
	}
	var signers []ssh.Signer
	for _, file := range files {
		pem, err := os.ReadFile(file)
		if err != nil {
			if explicit {
				return fail(fmt.Errorf("read identity %s: %w", file, err))
			}
			continue
		}
		signer, err := ssh.ParsePrivateKey(pem)
		if err != nil {
			var missing *ssh.PassphraseMissingError
			if explicit && errors.As(err, &missing) {
				return fail(fmt.Errorf("identity %s is passphrase-protected; add it to ssh-agent instead", file))
			}
			if explicit {
				return fail(fmt.Errorf("parse identity %s: %w", file, err))
			}
			continue
		}
		signers = append(signers, signer)
	}
	if len(signers) > 0 {
		methods = append(methods, ssh.PublicKeys(signers...))
	}
	if len(methods) == 0 {
		return fail(errors.New("no SSH credentials: put a password in the URL, start ssh-agent, or pass an identity file"))
	}
	return methods, release, nil
}
