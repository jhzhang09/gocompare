package remote

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
)

// FTP 是显式 FTPS 可选的 FTP 客户端。TLS 为 nil 时使用明文；不为 nil 时先 AUTH TLS，数据连接使用 PROT P。
// 调用方必须提供校验服务端证书的 TLS 配置，客户端不会跳过校验。
type FTP struct {
	Addr string
	User string
	Pass string
	TLS  *tls.Config
}

func (f *FTP) Read(ctx context.Context, path string) ([]byte, error) {
	return f.transfer(ctx, "RETR", path, nil)
}

func (f *FTP) Write(ctx context.Context, path string, data []byte) error {
	_, err := f.transfer(ctx, "STOR", path, data)
	return err
}

func (f *FTP) Delete(ctx context.Context, path string) error {
	if err := checkFTPArg(path); err != nil {
		return err
	}
	return f.withCtrl(ctx, func(c *ftpCtrl) error {
		_, _, err := c.expect("DELE "+path, ftpPositive)
		return err
	})
}

func (f *FTP) transfer(ctx context.Context, verb, path string, upload []byte) ([]byte, error) {
	if err := checkFTPArg(path); err != nil {
		return nil, err
	}
	var data []byte
	err := f.withCtrl(ctx, func(c *ftpCtrl) error {
		_, text, err := c.expect("PASV", func(code int) bool { return code == 227 })
		if err != nil {
			return err
		}
		port, err := parsePASV(text)
		if err != nil {
			return err
		}
		// 只采用 PASV 返回的端口，主机沿用控制连接的对端地址：
		// 避免 NAT 后的服务端返回内网地址，也避免被引导连接任意主机（FTP bounce）。
		host, _, err := net.SplitHostPort(c.conn.RemoteAddr().String())
		if err != nil {
			return err
		}
		addr := net.JoinHostPort(host, port)
		dc, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
		if err != nil {
			return ctxErr(ctx, err)
		}
		defer dc.Close()
		stopData := make(chan struct{})
		defer close(stopData)
		go func() {
			select {
			case <-ctx.Done():
				dc.Close()
			case <-stopData:
			}
		}()
		if err = c.writeLine(verb + " " + path); err != nil {
			return err
		}
		raw := dc
		if f.TLS != nil {
			tc := tls.Client(dc, f.TLS)
			if err = tc.HandshakeContext(ctx); err != nil {
				return ctxErr(ctx, err)
			}
			raw = tc
		}
		code, reply, err := c.readReply()
		if err != nil {
			return err
		}
		if code >= 400 {
			return fmt.Errorf("ftp %s %s: %s", verb, path, strings.TrimSpace(reply))
		}
		if code < 100 || code >= 200 {
			return fmt.Errorf("ftp %s %s: %s", verb, path, strings.TrimSpace(reply))
		}
		if upload == nil {
			data, err = io.ReadAll(io.LimitReader(raw, maxResponseBytes+1))
			if err == nil && len(data) > maxResponseBytes {
				return fmt.Errorf("ftp %s %s: remote file exceeds 32 MiB", verb, path)
			}
		} else {
			_, err = raw.Write(upload)
			if err == nil {
				err = raw.Close()
			}
		}
		if err != nil {
			return ctxErr(ctx, err)
		}
		_, _, err = c.expect("", ftpPositive)
		return err
	})
	return data, err
}

func (f *FTP) withCtrl(ctx context.Context, fn func(*ftpCtrl) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := checkFTPArg(f.User); err != nil {
		return err
	}
	if err := checkFTPArg(f.Pass); err != nil {
		return err
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", f.Addr)
	if err != nil {
		return ctxErr(ctx, err)
	}
	defer conn.Close()
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-stop:
		}
	}()
	br := bufio.NewReader(conn)
	c := &ftpCtrl{conn: conn, r: br, w: conn}
	if _, _, err = c.readReply(); err != nil {
		return ctxErr(ctx, err)
	}
	if f.TLS != nil {
		if _, _, err = c.expect("AUTH TLS", func(code int) bool { return code == 234 || ftpPositive(code) }); err != nil {
			return err
		}
		plain := conn
		if br.Buffered() > 0 {
			plain = prefixConn{Reader: io.MultiReader(io.LimitReader(br, int64(br.Buffered())), conn), Conn: conn}
		}
		tc := tls.Client(plain, f.TLS)
		if err = tc.HandshakeContext(ctx); err != nil {
			return ctxErr(ctx, err)
		}
		c = &ftpCtrl{conn: conn, r: bufio.NewReader(tc), w: tc}
		if _, _, err = c.expect("PBSZ 0", ftpPositive); err != nil {
			return err
		}
		if _, _, err = c.expect("PROT P", ftpPositive); err != nil {
			return err
		}
	}
	code, _, err := c.expect("USER "+f.User, func(code int) bool { return ftpPositive(code) || (code >= 300 && code < 400) })
	if err != nil {
		return err
	}
	if code >= 300 && code < 400 {
		if _, _, err = c.expect("PASS "+f.Pass, ftpPositive); err != nil {
			return err
		}
	}
	if _, _, err = c.expect("TYPE I", ftpPositive); err != nil {
		return err
	}
	err = fn(c)
	return ctxErr(ctx, err)
}

type ftpCtrl struct {
	conn net.Conn
	r    *bufio.Reader
	w    io.Writer
}

func (c *ftpCtrl) writeLine(line string) error {
	if line == "" {
		return nil
	}
	_, err := io.WriteString(c.w, line+"\r\n")
	return err
}

func (c *ftpCtrl) expect(line string, ok func(int) bool) (int, string, error) {
	if line != "" {
		if err := c.writeLine(line); err != nil {
			return 0, "", err
		}
	}
	code, text, err := c.readReply()
	if err != nil {
		return code, text, err
	}
	if ok != nil && !ok(code) {
		return code, text, fmt.Errorf("ftp reply %s", strings.TrimSpace(text))
	}
	return code, text, nil
}

func (c *ftpCtrl) readReply() (int, string, error) {
	line, err := c.r.ReadString('\n')
	if err != nil {
		return 0, line, err
	}
	if len(line) < 4 {
		return 0, line, fmt.Errorf("short ftp reply %q", line)
	}
	code, err := strconv.Atoi(line[:3])
	if err != nil {
		return 0, line, fmt.Errorf("ftp reply %q", line)
	}
	if line[3] == '-' {
		for {
			next, err2 := c.r.ReadString('\n')
			line += next
			if err2 != nil {
				return code, line, err2
			}
			if len(next) >= 4 && next[:3] == line[:3] && next[3] == ' ' {
				break
			}
		}
	}
	return code, line, nil
}

func ftpPositive(code int) bool { return code >= 200 && code < 300 }

// parsePASV 返回 227 应答中的数据端口。
func parsePASV(text string) (string, error) {
	i := strings.LastIndex(text, "(")
	j := strings.LastIndex(text, ")")
	if i < 0 || j <= i {
		return "", fmt.Errorf("pasv reply %q", text)
	}
	parts := strings.Split(text[i+1:j], ",")
	if len(parts) != 6 {
		return "", fmt.Errorf("pasv reply %q", text)
	}
	nums := make([]int, 6)
	for i, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || n < 0 || n > 255 {
			return "", fmt.Errorf("pasv reply %q", text)
		}
		nums[i] = n
	}
	return strconv.Itoa(nums[4]*256 + nums[5]), nil
}

// checkFTPArg 拒绝 CR/LF：URL 中的 %0D%0A 解码后会拼进控制连接，形成额外的 FTP 命令。
func checkFTPArg(s string) error {
	if strings.ContainsAny(s, "\r\n") {
		return fmt.Errorf("ftp argument must not contain CR or LF")
	}
	return nil
}

type prefixConn struct {
	io.Reader
	net.Conn
}

func (p prefixConn) Read(b []byte) (int, error) { return p.Reader.Read(b) }
