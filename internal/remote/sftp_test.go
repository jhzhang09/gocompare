package remote

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// sshServer 是进程内的真实 SSH + SFTP 服务端，校验密码 u/p 或指定的客户端公钥。
type sshServer struct {
	addr    string
	hostKey ssh.Signer
	root    string
}

func newSigner(t *testing.T) (ssh.Signer, ed25519.PrivateKey) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return signer, priv
}

func startSSHServer(t *testing.T, clientKey ssh.PublicKey, extraHostKeys ...ssh.Signer) *sshServer {
	t.Helper()
	hostKey, _ := newSigner(t)
	config := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if c.User() == "u" && string(pass) == "p" {
				return nil, nil
			}
			return nil, errors.New("denied")
		},
		PublicKeyCallback: func(c ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if clientKey != nil && bytes.Equal(key.Marshal(), clientKey.Marshal()) {
				return nil, nil
			}
			return nil, errors.New("denied")
		},
	}
	config.AddHostKey(hostKey)
	for _, k := range extraHostKeys {
		config.AddHostKey(k)
	}
	root := t.TempDir()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serveSSH(conn, config, root)
		}
	}()
	return &sshServer{addr: ln.Addr().String(), hostKey: hostKey, root: root}
}

func serveSSH(conn net.Conn, config *ssh.ServerConfig, root string) {
	defer conn.Close()
	_, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return
	}
	go ssh.DiscardRequests(reqs)
	for nc := range chans {
		if nc.ChannelType() != "session" {
			nc.Reject(ssh.UnknownChannelType, "session only")
			continue
		}
		ch, requests, err := nc.Accept()
		if err != nil {
			return
		}
		go func() {
			for req := range requests {
				ok := req.Type == "subsystem" && len(req.Payload) > 4 && string(req.Payload[4:]) == "sftp"
				req.Reply(ok, nil)
				if ok {
					server, err := sftp.NewServer(ch, sftp.WithServerWorkingDirectory(root))
					if err == nil {
						server.Serve()
					}
					ch.Close()
				}
			}
		}()
	}
}

// knownHostsFile 写出只登记 key 的 known_hosts。
func knownHostsFile(t *testing.T, addr string, key ssh.PublicKey) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(p, []byte(knownhosts.Line([]string{knownhosts.Normalize(addr)}, key)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSFTPPasswordRoundTrip(t *testing.T) {
	srv := startSSHServer(t, nil)
	client := &SFTP{Addr: srv.addr, User: "u", Pass: "p", KnownHosts: []string{knownHostsFile(t, srv.addr, srv.hostKey.PublicKey())}}
	ctx := context.Background()
	// 相对路径落在服务端工作目录（srv.root）下，避免各平台绝对路径写法差异。
	target := "doc.txt"
	if err := client.Write(ctx, target, []byte("a much longer original body")); err != nil {
		t.Fatal(err)
	}
	// 覆盖写入必须截断旧内容。
	if err := client.Write(ctx, target, []byte("short")); err != nil {
		t.Fatal(err)
	}
	got, err := client.Read(ctx, target)
	if err != nil || string(got) != "short" {
		t.Fatalf("read %q %v", got, err)
	}
	if onDisk, _ := os.ReadFile(filepath.Join(srv.root, "doc.txt")); string(onDisk) != "short" {
		t.Fatalf("server file %q", onDisk)
	}
	if err = client.Delete(ctx, target); err != nil {
		t.Fatal(err)
	}
	if _, err = client.Read(ctx, target); err == nil {
		t.Fatal("deleted file still readable")
	}
}

func TestSFTPIdentityFile(t *testing.T) {
	signer, priv := newSigner(t)
	srv := startSSHServer(t, signer.PublicKey())
	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatal(err)
	}
	keyFile := filepath.Join(t.TempDir(), "id_ed25519")
	if err = os.WriteFile(keyFile, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SSH_AUTH_SOCK", "")
	client := &SFTP{Addr: srv.addr, User: "u", IdentityFiles: []string{keyFile}, KnownHosts: []string{knownHostsFile(t, srv.addr, srv.hostKey.PublicKey())}}
	target := "k.txt"
	if err = client.Write(context.Background(), target, []byte("key auth")); err != nil {
		t.Fatal(err)
	}
	if onDisk, _ := os.ReadFile(filepath.Join(srv.root, "k.txt")); string(onDisk) != "key auth" {
		t.Fatalf("server file %q", onDisk)
	}
}

func TestSFTPRejectsUnknownAndChangedHostKeys(t *testing.T) {
	srv := startSSHServer(t, nil)
	target := "x"
	empty := filepath.Join(t.TempDir(), "known_hosts")
	os.WriteFile(empty, nil, 0o600)
	unknown := &SFTP{Addr: srv.addr, User: "u", Pass: "p", KnownHosts: []string{empty}}
	if _, err := unknown.Read(context.Background(), target); err == nil || !strings.Contains(err.Error(), "unknown SSH host key") || !strings.Contains(err.Error(), "SHA256:") {
		t.Fatalf("unknown host must be rejected with fingerprint: %v", err)
	}
	other, _ := newSigner(t)
	changed := &SFTP{Addr: srv.addr, User: "u", Pass: "p", KnownHosts: []string{knownHostsFile(t, srv.addr, other.PublicKey())}}
	if _, err := changed.Read(context.Background(), target); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("changed host key must be rejected: %v", err)
	}
	if _, err := os.Stat(filepath.Join(srv.root, "x")); !os.IsNotExist(err) {
		t.Fatal("nothing may be created before host verification")
	}
}

func TestSFTPWrongPassword(t *testing.T) {
	srv := startSSHServer(t, nil)
	client := &SFTP{Addr: srv.addr, User: "u", Pass: "wrong", KnownHosts: []string{knownHostsFile(t, srv.addr, srv.hostKey.PublicKey())}}
	if _, err := client.Read(context.Background(), "/x"); err == nil || !strings.Contains(err.Error(), "unable to authenticate") {
		t.Fatalf("wrong password: %v", err)
	}
}

func TestSFTPNoCredentials(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	client := &SFTP{Addr: "127.0.0.1:1", User: "u", HostKeyCallback: ssh.InsecureIgnoreHostKey()}
	if _, err := client.Read(context.Background(), "/x"); err == nil || !strings.Contains(err.Error(), "no SSH credentials") {
		t.Fatalf("missing credentials: %v", err)
	}
}

func TestSFTPCancelDuringHandshake(t *testing.T) {
	// 服务端接受连接后不回应 SSH 版本行，客户端只能靠取消返回。
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			defer c.Close()
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	client := &SFTP{Addr: ln.Addr().String(), User: "u", Pass: "p", HostKeyCallback: ssh.InsecureIgnoreHostKey()}
	done := make(chan error, 1)
	go func() { _, err := client.Read(ctx, "/x"); done <- err }()
	select {
	case err = <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("want deadline exceeded, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("handshake ignored context cancellation")
	}
}

func TestSFTPTrustUnknownHostKey(t *testing.T) {
	srv := startSSHServer(t, nil)
	knownHosts := filepath.Join(t.TempDir(), "ssh", "known_hosts")
	client := &SFTP{Addr: srv.addr, User: "u", Pass: "p", KnownHosts: []string{knownHosts}}
	// known_hosts 及其目录都不存在：按未知主机处理，信任时创建。
	err := client.Write(context.Background(), "t.txt", []byte("x"))
	var hostKey *HostKeyError
	if !errors.As(err, &hostKey) || hostKey.Fingerprint != ssh.FingerprintSHA256(srv.hostKey.PublicKey()) {
		t.Fatalf("want HostKeyError with server fingerprint, got %v", err)
	}
	if err = hostKey.Trust(); err != nil {
		t.Fatal(err)
	}
	if err = client.Write(context.Background(), "t.txt", []byte("x")); err != nil {
		t.Fatalf("trusted host still rejected: %v", err)
	}
}

func TestSFTPAuthErrorsAreClassified(t *testing.T) {
	srv := startSSHServer(t, nil)
	client := &SFTP{Addr: srv.addr, User: "u", Pass: "wrong", KnownHosts: []string{knownHostsFile(t, srv.addr, srv.hostKey.PublicKey())}}
	if _, err := client.Read(context.Background(), "x"); !errors.Is(err, ErrAuth) {
		t.Fatalf("wrong password must wrap ErrAuth: %v", err)
	}
}

func TestSFTPEncryptedIdentity(t *testing.T) {
	signer, priv := newSigner(t)
	srv := startSSHServer(t, signer.PublicKey())
	block, err := ssh.MarshalPrivateKeyWithPassphrase(priv, "", []byte("open sesame"))
	if err != nil {
		t.Fatal(err)
	}
	keyFile := filepath.Join(t.TempDir(), "id_ed25519")
	os.WriteFile(keyFile, pem.EncodeToMemory(block), 0o600)
	t.Setenv("SSH_AUTH_SOCK", "")
	kh := []string{knownHostsFile(t, srv.addr, srv.hostKey.PublicKey())}
	ctx := context.Background()

	missing := &SFTP{Addr: srv.addr, User: "u", IdentityFiles: []string{keyFile}, KnownHosts: kh}
	if _, err = missing.Read(ctx, "x"); !errors.Is(err, ErrKeyPassphrase) {
		t.Fatalf("missing passphrase: %v", err)
	}
	wrong := &SFTP{Addr: srv.addr, User: "u", IdentityFiles: []string{keyFile}, Passphrase: "nope", KnownHosts: kh}
	if _, err = wrong.Read(ctx, "x"); !errors.Is(err, ErrKeyPassphraseWrong) {
		t.Fatalf("wrong passphrase: %v", err)
	}
	ok := &SFTP{Addr: srv.addr, User: "u", IdentityFiles: []string{keyFile}, Passphrase: "open sesame", KnownHosts: kh}
	if err = ok.Write(ctx, "enc.txt", []byte("ok")); err != nil {
		t.Fatalf("correct passphrase: %v", err)
	}
	// 私钥被拒时仍可用密码登录：密码排在私钥之后作为后备。
	_, otherPriv := newSigner(t)
	block2, _ := ssh.MarshalPrivateKey(otherPriv, "")
	otherFile := filepath.Join(t.TempDir(), "other")
	os.WriteFile(otherFile, pem.EncodeToMemory(block2), 0o600)
	fallback := &SFTP{Addr: srv.addr, User: "u", Pass: "p", IdentityFiles: []string{otherFile}, KnownHosts: kh}
	if err = fallback.Write(ctx, "fb.txt", []byte("ok")); err != nil {
		t.Fatalf("password fallback: %v", err)
	}
}

func TestSFTPPrefersRegisteredHostKeyType(t *testing.T) {
	// 服务端同时有 ed25519 与 ECDSA 主机密钥，known_hosts 只登记 ed25519：
	// 必须协商 ed25519 并连接成功，而不是拿到 ECDSA 后误报密钥不一致。
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ecSigner, err := ssh.NewSignerFromKey(ecKey)
	if err != nil {
		t.Fatal(err)
	}
	srv := startSSHServer(t, nil, ecSigner)
	client := &SFTP{Addr: srv.addr, User: "u", Pass: "p", KnownHosts: []string{knownHostsFile(t, srv.addr, srv.hostKey.PublicKey())}}
	if err = client.Write(context.Background(), "k.txt", []byte("ok")); err != nil {
		t.Fatalf("registered ed25519 key not preferred: %v", err)
	}
	// 只登记了 ECDSA 时同样只协商 ECDSA。
	onlyEC := &SFTP{Addr: srv.addr, User: "u", Pass: "p", KnownHosts: []string{knownHostsFile(t, srv.addr, ecSigner.PublicKey())}}
	if err = onlyEC.Write(context.Background(), "k.txt", []byte("ok")); err != nil {
		t.Fatalf("registered ECDSA key not used: %v", err)
	}
}
