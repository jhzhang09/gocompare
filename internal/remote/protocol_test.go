package remote

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestProtocols(t *testing.T) {
	cases := []struct {
		name string
		run  func(t *testing.T)
	}{
		{"ftp", testFTP},
		{"ftps", testFTPS},
		{"webdav", testWebDAV},
		{"s3", testS3},
		{"dropbox", testDropbox},
		{"onedrive", testOneDrive},
		{"svn", testSVN},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			done := make(chan struct{})
			go func() {
				defer close(done)
				tc.run(t)
			}()
			select {
			case <-done:
			case <-time.After(8 * time.Second):
				t.Fatal("timed out")
			}
		})
	}
}

func exercise(t *testing.T, store Store, slow func(context.Context) error, ready <-chan struct{}) {
	t.Helper()
	ctx := context.Background()
	if err := store.Write(ctx, "ok", []byte("alpha")); err != nil {
		t.Fatal(err)
	}
	got, err := store.Read(ctx, "ok")
	if err != nil || string(got) != "alpha" {
		t.Fatalf("read back %q %v", got, err)
	}
	if err = store.Write(ctx, "other", []byte("beta")); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Read(ctx, "fail"); err == nil {
		t.Fatal("fail path succeeded")
	}
	got, err = store.Read(ctx, "other")
	if err != nil || string(got) != "beta" {
		t.Fatalf("partial %q %v", got, err)
	}
	if err = store.Delete(ctx, "ok"); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Read(ctx, "ok"); err == nil {
		t.Fatal("deleted path still readable")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- slow(ctx) }()
	select {
	case <-ready:
	case err = <-done:
		t.Fatalf("slow returned before cancel: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("slow request did not start")
	}
	cancel()
	select {
	case err = <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancel: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancel hung")
	}
}

type memFS struct {
	mu    sync.Mutex
	files map[string][]byte
	once  sync.Once
	slow  chan struct{}
}

func newMem() *memFS {
	return &memFS{files: map[string][]byte{}, slow: make(chan struct{})}
}

func (m *memFS) entered() { m.once.Do(func() { close(m.slow) }) }

func (m *memFS) get(path string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.files[path]
	return append([]byte(nil), b...), ok
}

func (m *memFS) put(path string, b []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[path] = append([]byte(nil), b...)
}

func (m *memFS) del(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.files, path)
}

func serveTCP(t *testing.T, handle func(net.Conn)) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go handle(c)
		}
	}()
	return ln.Addr().String()
}

func testFTP(t *testing.T) {
	files := newMem()
	addr := serveTCP(t, func(conn net.Conn) { ftpSession(conn, nil, files) })
	client := &FTP{Addr: addr, User: "u", Pass: "p"}
	exercise(t, client, func(ctx context.Context) error {
		_, err := client.Read(ctx, "slow")
		return err
	}, files.slow)
}

func testFTPS(t *testing.T) {
	cert, pool := testCert(t)
	files := newMem()
	addr := serveTCP(t, func(conn net.Conn) { ftpSession(conn, &cert, files) })
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	store, _, err := ParseURL("ftps://u:p@localhost:" + port + "/file")
	if err != nil {
		t.Fatal(err)
	}
	client := store.(*FTP)
	client.TLS.RootCAs = pool
	exercise(t, client, func(ctx context.Context) error {
		_, err := client.Read(ctx, "slow")
		return err
	}, files.slow)
}

func ftpSession(conn net.Conn, cert *tls.Certificate, files *memFS) {
	defer conn.Close()
	br := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)
	write := func(s string) {
		fmt.Fprintf(w, "%s\r\n", s)
		w.Flush()
	}
	write("220 gocompare")
	prot := false
	var dataLn net.Listener
	defer func() {
		if dataLn != nil {
			dataLn.Close()
		}
	}()
	var srv *tls.Config
	if cert != nil {
		srv = &tls.Config{Certificates: []tls.Certificate{*cert}}
	}
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)
		cmd, arg, _ := strings.Cut(line, " ")
		switch strings.ToUpper(cmd) {
		case "AUTH":
			if srv == nil {
				write("502 no tls")
				continue
			}
			write("234 go")
			raw := net.Conn(conn)
			if br.Buffered() > 0 {
				raw = prefixConn{Reader: io.MultiReader(io.LimitReader(br, int64(br.Buffered())), conn), Conn: conn}
			}
			tc := tls.Server(raw, srv)
			if err = tc.Handshake(); err != nil {
				return
			}
			conn = tc
			br = bufio.NewReader(tc)
			w = bufio.NewWriter(tc)
		case "PBSZ":
			write("200 ok")
		case "PROT":
			prot = true
			write("200 ok")
		case "USER":
			write("331 password")
		case "PASS", "TYPE":
			write("200 ok")
		case "PASV":
			if dataLn != nil {
				dataLn.Close()
			}
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				return
			}
			dataLn = ln
			port := ln.Addr().(*net.TCPAddr).Port
			write(fmt.Sprintf("227 Entering Passive Mode (127,0,0,1,%d,%d)", port/256, port%256))
		case "RETR":
			if arg == "slow" {
				files.entered()
				io.Copy(io.Discard, br)
				return
			}
			dc, err := acceptData(dataLn, prot, srv)
			if err != nil {
				return
			}
			if arg == "fail" {
				dc.Close()
				write("550 missing")
				continue
			}
			payload, ok := files.get(arg)
			if !ok {
				dc.Close()
				write("550 missing")
				continue
			}
			write("150 ok")
			dc.Write(payload)
			dc.Close()
			write("226 done")
		case "STOR":
			dc, err := acceptData(dataLn, prot, srv)
			if err != nil {
				return
			}
			write("150 ok")
			payload, err := io.ReadAll(io.LimitReader(dc, 32<<20))
			dc.Close()
			if err != nil {
				return
			}
			files.put(arg, payload)
			write("226 done")
		case "DELE":
			files.del(arg)
			write("250 deleted")
		case "QUIT":
			write("221 bye")
			return
		default:
			write("502 " + cmd)
		}
	}
}

func acceptData(ln net.Listener, prot bool, cfg *tls.Config) (net.Conn, error) {
	if ln == nil {
		return nil, errors.New("no pasv")
	}
	dc, err := ln.Accept()
	if err != nil {
		return nil, err
	}
	if prot {
		tc := tls.Server(dc, cfg)
		if err = tc.Handshake(); err != nil {
			dc.Close()
			return nil, err
		}
		return tc, nil
	}
	return dc, nil
}

func testCert(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	return cert, pool
}

func testWebDAV(t *testing.T) {
	files := newMem()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpObject(files, w, r, strings.TrimPrefix(r.URL.Path, "/"))
	}))
	t.Cleanup(srv.Close)
	client := &WebDAV{Base: srv.URL}
	exercise(t, client, func(ctx context.Context) error {
		_, err := client.Read(ctx, "slow")
		return err
	}, files.slow)
}

func testS3(t *testing.T) {
	files := newMem()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 Credential=AKIA/") {
			http.Error(w, "auth", http.StatusUnauthorized)
			return
		}
		body, _ := io.ReadAll(r.Body)
		sum := sha256.Sum256(body)
		if r.Header.Get("x-amz-content-sha256") != hex.EncodeToString(sum[:]) {
			http.Error(w, "hash", http.StatusUnauthorized)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		httpObject(files, w, r, strings.TrimPrefix(r.URL.Path, "/bucket/"))
	}))
	t.Cleanup(srv.Close)
	client := &S3{Base: srv.URL + "/bucket", Region: "us-east-1", AccessKey: "AKIA", SecretKey: "secret"}
	exercise(t, client, func(ctx context.Context) error {
		_, err := client.Read(ctx, "slow")
		return err
	}, files.slow)
}

func testDropbox(t *testing.T) {
	files := newMem()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" {
			http.Error(w, "auth", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/2/files/download", "/2/files/upload":
			var arg struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal([]byte(r.Header.Get("Dropbox-API-Arg")), &arg); err != nil || arg.Path == "" {
				http.Error(w, "arg", http.StatusBadRequest)
				return
			}
			path := strings.TrimPrefix(arg.Path, "/")
			if r.URL.Path == "/2/files/upload" {
				body, _ := io.ReadAll(r.Body)
				if path == "fail" {
					http.Error(w, "partial", http.StatusInternalServerError)
					return
				}
				files.put(path, body)
				w.WriteHeader(http.StatusOK)
				return
			}
			httpObject(files, w, r, path)
		case "/2/files/delete_v2":
			var arg struct {
				Path string `json:"path"`
			}
			if err := json.NewDecoder(r.Body).Decode(&arg); err != nil {
				http.Error(w, "arg", http.StatusBadRequest)
				return
			}
			files.del(strings.TrimPrefix(arg.Path, "/"))
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	client := &Dropbox{Base: srv.URL, Token: "token"}
	exercise(t, client, func(ctx context.Context) error {
		_, err := client.Read(ctx, "slow")
		return err
	}, files.slow)
}

func testOneDrive(t *testing.T) {
	files := newMem()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" {
			http.Error(w, "auth", http.StatusUnauthorized)
			return
		}
		const prefix = "/v1.0/me/drive/root:/"
		if !strings.HasPrefix(r.URL.Path, prefix) {
			http.NotFound(w, r)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, prefix)
		path = strings.TrimSuffix(path, ":/content")
		httpObject(files, w, r, path)
	}))
	t.Cleanup(srv.Close)
	client := &OneDrive{Base: srv.URL, Token: "token"}
	exercise(t, client, func(ctx context.Context) error {
		_, err := client.Read(ctx, "slow")
		return err
	}, files.slow)
}

func testSVN(t *testing.T) {
	files := newMem()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-SVN-Request") != "subset" {
			http.Error(w, "svn", http.StatusBadRequest)
			return
		}
		httpObject(files, w, r, strings.TrimPrefix(r.URL.Path, "/"))
	}))
	t.Cleanup(srv.Close)
	client := &SVN{Base: srv.URL}
	exercise(t, client, func(ctx context.Context) error {
		_, err := client.Read(ctx, "slow")
		return err
	}, files.slow)
}

func httpObject(files *memFS, w http.ResponseWriter, r *http.Request, path string) {
	if path == "slow" {
		files.entered()
		<-r.Context().Done()
		return
	}
	if path == "fail" {
		http.Error(w, "partial", http.StatusInternalServerError)
		return
	}
	switch r.Method {
	case http.MethodGet, http.MethodPost:
		data, ok := files.get(path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(data)
	case http.MethodPut:
		data, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		files.put(path, data)
		w.WriteHeader(http.StatusCreated)
	case http.MethodDelete:
		files.del(path)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method", http.StatusMethodNotAllowed)
	}
}
