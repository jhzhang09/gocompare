package remote

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
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
// 认证顺序见 authMethods。
type SFTP struct {
	Addr string // host:port
	User string // 为空时使用当前系统用户
	Pass string
	// IdentityFiles 是私钥文件；为空时尝试 ~/.ssh/id_ed25519、id_ecdsa、id_rsa。
	IdentityFiles []string
	// Passphrase 解密受口令保护的私钥；为空时这类私钥只能经 ssh-agent 使用。
	Passphrase string
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
	client, done, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer done()
	return ctxErr(ctx, fn(client))
}

// connect 建立 SSH 连接并打开 SFTP 子系统；done 按相反顺序释放全部资源。
// 连接在 ctx 取消时被关闭，因此 ctx 必须覆盖连接的整个使用期。
func (s *SFTP) connect(ctx context.Context) (*sftp.Client, func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	config, known, release, pending, err := s.clientConfig()
	if err != nil {
		return nil, nil, err
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", s.Addr)
	if err != nil {
		release()
		return nil, nil, ctxErr(ctx, err)
	}
	// SSH 握手和 SFTP 请求都不接受 context；取消时关闭底层连接让阻塞调用返回。
	stop := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-stop:
		}
	}()
	var closers []func()
	done := func() {
		for i := len(closers) - 1; i >= 0; i-- {
			closers[i]()
		}
	}
	closers = append(closers, release, func() { conn.Close() }, func() { close(stop) })
	if known != nil {
		config.HostKeyAlgorithms = knownAlgorithms(known, s.Addr, conn.RemoteAddr())
	}
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, s.Addr, config)
	if err != nil {
		done()
		if strings.Contains(err.Error(), "unable to authenticate") {
			if pending != nil {
				return nil, nil, ctxErr(ctx, pending)
			}
			return nil, nil, ctxErr(ctx, authError(fmt.Errorf("ssh %s: %v", s.Addr, err)))
		}
		return nil, nil, ctxErr(ctx, fmt.Errorf("ssh %s: %w", s.Addr, err))
	}
	sshClient := ssh.NewClient(sshConn, chans, reqs)
	closers = append(closers, func() { sshClient.Close() })
	client, err := sftp.NewClient(sshClient)
	if err != nil {
		done()
		return nil, nil, ctxErr(ctx, fmt.Errorf("sftp %s: %w", s.Addr, err))
	}
	closers = append(closers, func() { client.Close() })
	return client, done, nil
}

// clientConfig 返回 SSH 配置、原始 known_hosts 校验函数（自定义 HostKeyCallback 时为 nil）、
// 释放 ssh-agent 连接的函数，以及认证失败时应报告的口令问题。
func (s *SFTP) clientConfig() (*ssh.ClientConfig, ssh.HostKeyCallback, func(), error, error) {
	name := s.User
	if name == "" {
		u, err := user.Current()
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("sftp user is required: %w", err)
		}
		// Windows 返回 DOMAIN\user，SSH 登录名只取用户部分。
		name = u.Username[strings.LastIndex(u.Username, `\`)+1:]
	}
	hostKey := s.HostKeyCallback
	var known ssh.HostKeyCallback
	if hostKey == nil {
		var err error
		if hostKey, known, err = s.knownHostsCallback(); err != nil {
			return nil, nil, nil, nil, err
		}
	}
	auth, release, pending, err := s.authMethods()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return &ssh.ClientConfig{User: name, Auth: auth, HostKeyCallback: hostKey}, known, release, pending, nil
}

// knownHostsCallback 包装 knownhosts 的错误，给出可执行的处理方法。
// knownHostsCallback 返回包装后的校验函数，以及未包装的原始校验函数（供 knownAlgorithms 探测已登记的密钥类型）。
func (s *SFTP) knownHostsCallback() (ssh.HostKeyCallback, ssh.HostKeyCallback, error) {
	files := s.KnownHosts
	if len(files) == 0 {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, nil, fmt.Errorf("locate known_hosts: %w", err)
		}
		files = []string{filepath.Join(home, ".ssh", "known_hosts")}
	}
	// 不存在的文件视为空登记表：新机器常常还没有 known_hosts，此时所有主机都是未知主机，
	// 由调用方确认指纹后经 HostKeyError.Trust 创建文件。
	var existing []string
	for _, f := range files {
		if _, err := os.Stat(f); err == nil {
			existing = append(existing, f)
		}
	}
	cb := func(string, net.Addr, ssh.PublicKey) error { return &knownhosts.KeyError{} }
	if len(existing) > 0 {
		var err error
		if cb, err = knownhosts.New(existing...); err != nil {
			return nil, nil, fmt.Errorf("load known_hosts %v: %w", existing, err)
		}
	}
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := cb(hostname, remote, key)
		var keyErr *knownhosts.KeyError
		if errors.As(err, &keyErr) {
			if len(keyErr.Want) == 0 {
				// 登记到第一个 known_hosts 文件；Error() 文案保持原样，供 CLI 直接输出。
				return &HostKeyError{Host: knownhosts.Normalize(hostname), Fingerprint: ssh.FingerprintSHA256(key), KeyType: key.Type(), KnownHosts: files[0], key: key}
			}
			return fmt.Errorf("%w: %s (got %s %s); the server key changed or the connection is being intercepted", ErrHostKeyChanged, hostname, key.Type(), ssh.FingerprintSHA256(key))
		}
		return err
	}, cb, nil
}

// knownAlgorithms 返回 known_hosts 中该主机已登记密钥对应的主机密钥算法。
// 与 OpenSSH 一致：优先协商已登记的类型，否则服务端同时有 ed25519 与 ECDSA 时，
// 库默认选 ECDSA，而只登记了 ed25519 的主机会被误判为密钥不一致。没有登记时返回 nil，使用默认顺序。
func knownAlgorithms(cb ssh.HostKeyCallback, hostname string, remote net.Addr) []string {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil
	}
	probe, err := ssh.NewPublicKey(priv.Public())
	if err != nil {
		return nil
	}
	var keyErr *knownhosts.KeyError
	if !errors.As(cb(hostname, remote, probe), &keyErr) {
		return nil
	}
	var algos []string
	seen := map[string]bool{}
	for _, k := range keyErr.Want {
		types := []string{k.Key.Type()}
		if types[0] == ssh.KeyAlgoRSA {
			// RSA 密钥可以用三种签名算法验证，新服务端多半已禁用 SHA-1 的 ssh-rsa。
			types = []string{ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSA}
		}
		for _, t := range types {
			if !seen[t] {
				seen[t] = true
				algos = append(algos, t)
			}
		}
	}
	return algos
}

// authMethods 按"用户明确指定的私钥 → 密码 → ssh-agent → 默认私钥"的顺序组装认证方式。
// 密码排在 agent 之前：agent 里密钥较多时会先耗尽服务端的尝试次数（MaxAuthTries）。
// pending 是延后报告的口令问题：受口令保护的私钥没给口令时仍会尝试 agent（可能已 ssh-add），
// 只有最终认证失败才把它作为原因返回。
func (s *SFTP) authMethods() (methods []ssh.AuthMethod, release func(), pending error, err error) {
	release = func() {}
	fail := func(e error) ([]ssh.AuthMethod, func(), error, error) {
		release()
		return nil, nil, nil, e
	}
	parse := func(file string, explicit bool) (ssh.Signer, error) {
		pem, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read identity %s: %w", file, err)
		}
		signer, err := ssh.ParsePrivateKey(pem)
		var missing *ssh.PassphraseMissingError
		if errors.As(err, &missing) {
			if s.Passphrase == "" {
				if explicit {
					pending = fmt.Errorf("%w: %s", ErrKeyPassphrase, file)
				}
				return nil, nil
			}
			signer, err = ssh.ParsePrivateKeyWithPassphrase(pem, []byte(s.Passphrase))
			if errors.Is(err, x509.IncorrectPasswordError) {
				if explicit {
					return nil, fmt.Errorf("%w: %s", ErrKeyPassphraseWrong, file)
				}
				return nil, nil
			}
		}
		if err != nil {
			return nil, fmt.Errorf("parse identity %s: %w", file, err)
		}
		return signer, nil
	}

	var signers []ssh.Signer
	for _, file := range s.IdentityFiles {
		signer, err := parse(file, true)
		if err != nil {
			return fail(err)
		}
		if signer != nil {
			signers = append(signers, signer)
		}
	}
	if len(signers) > 0 {
		methods = append(methods, ssh.PublicKeys(signers...))
	}
	if s.Pass != "" {
		pass := s.Pass
		methods = append(methods,
			ssh.Password(pass),
			// 部分服务端只开放 keyboard-interactive，提示一律用同一个密码回答。
			ssh.KeyboardInteractive(func(_, _ string, questions []string, _ []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range answers {
					answers[i] = pass
				}
				return answers, nil
			}))
	}
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		if conn, err := net.Dial("unix", sock); err == nil {
			release = func() { conn.Close() }
			methods = append(methods, ssh.PublicKeysCallback(agent.NewClient(conn).Signers))
		}
	}
	if len(s.IdentityFiles) == 0 {
		var defaults []ssh.Signer
		if home, err := os.UserHomeDir(); err == nil {
			for _, name := range []string{"id_ed25519", "id_ecdsa", "id_rsa"} {
				// 默认私钥只是尝试：不存在、读不了或口令不对都跳过。
				if signer, err := parse(filepath.Join(home, ".ssh", name), false); err == nil && signer != nil {
					defaults = append(defaults, signer)
				}
			}
		}
		if len(defaults) > 0 {
			methods = append(methods, ssh.PublicKeys(defaults...))
		}
	}
	if len(methods) == 0 {
		if pending != nil {
			return fail(pending)
		}
		return fail(authError(errors.New("no SSH credentials: enter a password, start ssh-agent, or choose a private key")))
	}
	return methods, release, pending, nil
}
