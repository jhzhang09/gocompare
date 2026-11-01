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
	"encoding/binary"
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
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf16"
)

func TestProtocols(t *testing.T) {
	cases := []struct {
		name string
		run  func(t *testing.T)
	}{
		{"ftp", testFTP},
		{"ftps", testFTPS},
		{"sftp", testSFTP},
		{"smb", testSMB},
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

func testSFTP(t *testing.T) {
	files := newMem()
	addr := serveTCP(t, func(conn net.Conn) { sftpSession(conn, files) })
	client := &SFTP{Addr: addr}
	exercise(t, client, func(ctx context.Context) error {
		_, err := client.Read(ctx, "slow")
		return err
	}, files.slow)
}

func sftpSession(conn net.Conn, files *memFS) {
	defer conn.Close()
	handles := map[string]string{}
	next := 0
	for {
		pkt, err := readFXP(conn)
		if err != nil || len(pkt) == 0 {
			return
		}
		switch pkt[0] {
		case fxpInit:
			var b bytes.Buffer
			b.WriteByte(fxpVersion)
			binary.Write(&b, binary.BigEndian, uint32(3))
			writeFXP(conn, b.Bytes())
		case fxpOpen:
			id := binary.BigEndian.Uint32(pkt[1:5])
			name, err := sshString(pkt[5:])
			if err != nil {
				return
			}
			path := string(name)
			if path == "slow" {
				files.entered()
				io.Copy(io.Discard, conn)
				return
			}
			rest := pkt[5+4+len(name):]
			if len(rest) < 4 {
				return
			}
			flags := binary.BigEndian.Uint32(rest[:4])
			_, ok := files.get(path)
			if path == "fail" || (flags&fxfCreat == 0 && !ok) {
				writeFXP(conn, fxpStatusPacket(id, fxNoSuchFile, "missing"))
				continue
			}
			if !ok {
				files.put(path, nil)
			}
			next++
			h := strconv.Itoa(next)
			handles[h] = path
			var b bytes.Buffer
			b.WriteByte(fxpHandle)
			binary.Write(&b, binary.BigEndian, id)
			putSSHString(&b, h)
			writeFXP(conn, b.Bytes())
		case fxpRead:
			id := binary.BigEndian.Uint32(pkt[1:5])
			h, err := sshString(pkt[5:])
			if err != nil {
				return
			}
			path := handles[string(h)]
			data, _ := files.get(path)
			offBytes := pkt[5+4+len(h):]
			if len(offBytes) < 8 {
				return
			}
			off := binary.BigEndian.Uint64(offBytes[:8])
			if off >= uint64(len(data)) {
				writeFXP(conn, fxpStatusPacket(id, fxEOF, "eof"))
				continue
			}
			var b bytes.Buffer
			b.WriteByte(fxpData)
			binary.Write(&b, binary.BigEndian, id)
			putSSHBytes(&b, data[off:])
			writeFXP(conn, b.Bytes())
		case fxpWrite:
			id := binary.BigEndian.Uint32(pkt[1:5])
			h, err := sshString(pkt[5:])
			if err != nil {
				return
			}
			rest := pkt[5+4+len(h):]
			if len(rest) < 8 {
				return
			}
			data, err := sshString(rest[8:])
			if err != nil {
				return
			}
			files.put(handles[string(h)], data)
			writeFXP(conn, fxpStatusPacket(id, fxOK, "ok"))
		case fxpRemove:
			id := binary.BigEndian.Uint32(pkt[1:5])
			name, err := sshString(pkt[5:])
			if err != nil {
				return
			}
			files.del(string(name))
			writeFXP(conn, fxpStatusPacket(id, fxOK, "ok"))
		case fxpClose:
			id := binary.BigEndian.Uint32(pkt[1:5])
			writeFXP(conn, fxpStatusPacket(id, fxOK, "ok"))
		default:
			return
		}
	}
}

func fxpStatusPacket(id, code uint32, msg string) []byte {
	var b bytes.Buffer
	b.WriteByte(fxpStatus)
	binary.Write(&b, binary.BigEndian, id)
	binary.Write(&b, binary.BigEndian, code)
	putSSHString(&b, msg)
	putSSHString(&b, "en")
	return b.Bytes()
}

func testSMB(t *testing.T) {
	files := newMem()
	addr := serveTCP(t, func(conn net.Conn) { serveSMB(conn, files) })
	client := &SMB{Addr: addr}
	exercise(t, client, func(ctx context.Context) error {
		_, err := client.Read(ctx, "slow")
		return err
	}, files.slow)
}

func serveSMB(conn net.Conn, files *memFS) {
	defer conn.Close()
	type handle struct {
		path   string
		delete bool
	}
	handles := map[string]handle{}
	var next uint64
	const session uint64 = 0x11
	const tree uint32 = 0x22
	for {
		pkt, err := readSMB(conn)
		if err != nil || len(pkt) < 64 || pkt[0] != 0xFE || string(pkt[1:4]) != "SMB" || binary.LittleEndian.Uint16(pkt[4:6]) != 64 {
			return
		}
		cmd := binary.LittleEndian.Uint16(pkt[12:14])
		if cmd == smbCreate || cmd == smbReadCmd || cmd == smbWriteCmd || cmd == smbCloseCmd {
			if binary.LittleEndian.Uint64(pkt[40:48]) != session || binary.LittleEndian.Uint32(pkt[36:40]) != tree {
				writeSMB(conn, smbTestReply(pkt, 0xC0000022, cmd, session, tree, nil))
				continue
			}
		}
		switch cmd {
		case smbNegotiate:
			body := make([]byte, 8)
			binary.LittleEndian.PutUint16(body[0:], 65)
			binary.LittleEndian.PutUint16(body[4:], 0x0202)
			writeSMB(conn, smbTestReply(pkt, 0, cmd, 0, 0, body))
		case smbSession:
			writeSMB(conn, smbTestReply(pkt, 0, cmd, session, 0, []byte{9, 0}))
		case smbTree:
			writeSMB(conn, smbTestReply(pkt, 0, cmd, session, tree, []byte{16, 0}))
		case smbCreate:
			body := pkt[64:]
			if len(body) < 56 {
				return
			}
			nameOff := int(binary.LittleEndian.Uint16(body[44:46]))
			nameLen := int(binary.LittleEndian.Uint16(body[46:48]))
			if nameOff < 0 || nameLen < 0 || nameOff+nameLen > len(pkt) {
				return
			}
			name := decodeUTF16(pkt[nameOff : nameOff+nameLen])
			if name == "slow" {
				files.entered()
				io.Copy(io.Discard, conn)
				return
			}
			if name == "fail" {
				writeSMB(conn, smbTestReply(pkt, 0xC0000022, cmd, session, tree, nil))
				continue
			}
			disp := binary.LittleEndian.Uint32(body[36:40])
			_, ok := files.get(name)
			// 按 MS-SMB2 区分：FILE_OPEN_IF(3) 保留已有内容，FILE_OVERWRITE_IF(5) 截断。
			if !ok && disp != 3 && disp != fileOverwriteIf {
				writeSMB(conn, smbTestReply(pkt, 0xC0000034, cmd, session, tree, nil))
				continue
			}
			if !ok || disp == fileOverwriteIf {
				files.put(name, nil)
			}
			next++
			var id [16]byte
			binary.LittleEndian.PutUint64(id[:], next)
			handles[string(id[:])] = handle{path: name, delete: binary.LittleEndian.Uint32(body[40:44])&fileDelete != 0}
			resp := make([]byte, 88)
			binary.LittleEndian.PutUint16(resp[0:], 89)
			copy(resp[64:], id[:])
			writeSMB(conn, smbTestReply(pkt, 0, cmd, session, tree, resp))
		case smbReadCmd:
			body := pkt[64:]
			if len(body) < 32 {
				return
			}
			h := handles[string(body[16:32])]
			data, _ := files.get(h.path)
			offset := binary.LittleEndian.Uint64(body[8:16])
			length := binary.LittleEndian.Uint32(body[4:8])
			if offset >= uint64(len(data)) {
				writeSMB(conn, smbTestReply(pkt, 0xC0000011, cmd, session, tree, nil))
				continue
			}
			end := min(uint64(len(data)), offset+uint64(length))
			data = data[offset:end]
			resp := make([]byte, 16+len(data))
			binary.LittleEndian.PutUint16(resp[0:], 17)
			resp[2] = 80
			binary.LittleEndian.PutUint32(resp[4:], uint32(len(data)))
			copy(resp[16:], data)
			writeSMB(conn, smbTestReply(pkt, 0, cmd, session, tree, resp))
		case smbWriteCmd:
			body := pkt[64:]
			if len(body) < 48 {
				return
			}
			h := handles[string(body[16:32])]
			off := int(binary.LittleEndian.Uint16(body[2:4]))
			n := int(binary.LittleEndian.Uint32(body[4:8]))
			if off < 0 || n < 0 || off+n > len(pkt) {
				return
			}
			// 按 WRITE 偏移覆盖写入，不隐式截断，与真实服务端一致。
			cur, _ := files.get(h.path)
			woff := int(binary.LittleEndian.Uint64(body[8:16]))
			buf := append([]byte(nil), cur...)
			if len(buf) < woff+n {
				buf = append(buf, make([]byte, woff+n-len(buf))...)
			}
			copy(buf[woff:], pkt[off:off+n])
			files.put(h.path, buf)
			resp := make([]byte, 16)
			binary.LittleEndian.PutUint16(resp[0:], 17)
			binary.LittleEndian.PutUint32(resp[4:], uint32(n))
			writeSMB(conn, smbTestReply(pkt, 0, cmd, session, tree, resp))
		case smbCloseCmd:
			body := pkt[64:]
			if len(body) < 24 {
				return
			}
			key := string(body[8:24])
			if h, ok := handles[key]; ok && h.delete {
				files.del(h.path)
			}
			delete(handles, key)
			writeSMB(conn, smbTestReply(pkt, 0, cmd, session, tree, []byte{24, 0}))
		default:
			return
		}
	}
}

func smbTestReply(req []byte, status uint32, cmd uint16, session uint64, tree uint32, body []byte) []byte {
	h := make([]byte, 64)
	copy(h, req[:64])
	binary.LittleEndian.PutUint32(h[8:], status)
	binary.LittleEndian.PutUint16(h[12:], cmd)
	binary.LittleEndian.PutUint32(h[16:], 1)
	binary.LittleEndian.PutUint32(h[36:], tree)
	binary.LittleEndian.PutUint64(h[40:], session)
	return append(h, body...)
}

func decodeUTF16(b []byte) string {
	if len(b)%2 == 1 {
		b = b[:len(b)-1]
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(b[i*2:])
	}
	return string(utf16.Decode(u))
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

func TestSMBReadMultipleChunks(t *testing.T) {
	files := newMem()
	data := bytes.Repeat([]byte("payload"), 400000)
	files.put("large", data)
	addr := serveTCP(t, func(conn net.Conn) { serveSMB(conn, files) })
	client := &SMB{Addr: addr}
	got, err := client.Read(context.Background(), "large")
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("incomplete read: got %d want %d: %v", len(got), len(data), err)
	}
}

func TestSMBWriteTruncatesExistingFile(t *testing.T) {
	files := newMem()
	files.put("doc", []byte("a much longer original body"))
	addr := serveTCP(t, func(conn net.Conn) { serveSMB(conn, files) })
	client := &SMB{Addr: addr}
	if err := client.Write(context.Background(), "doc", []byte("short")); err != nil {
		t.Fatal(err)
	}
	got, err := client.Read(context.Background(), "doc")
	if err != nil || string(got) != "short" {
		t.Fatalf("stale tail after overwrite: %q %v", got, err)
	}
}

func TestSMBWriteRejectsOversizedFrame(t *testing.T) {
	err := (&SMB{Addr: "127.0.0.1:1"}).Write(context.Background(), "big", make([]byte, smbMaxFrame))
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized write must fail before dialing: %v", err)
	}
}
