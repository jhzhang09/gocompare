package remote

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
)

// SFTP 是用于受控原始 TCP 夹具的 SSH_FXP 消息层，不实现 SSH 握手和认证。
// 不能直接连接真实 SFTP 服务；ParseURL 会拒绝 sftp URL。
type SFTP struct {
	Addr string
}

const (
	fxpInit    = 1
	fxpVersion = 2
	fxpOpen    = 3
	fxpClose   = 4
	fxpRead    = 5
	fxpWrite   = 6
	fxpRemove  = 13
	fxpStatus  = 101
	fxpHandle  = 102
	fxpData    = 103

	fxOK         = 0
	fxEOF        = 1
	fxNoSuchFile = 2
	fxfRead      = 0x1
	fxfWrite     = 0x2
	fxfCreat     = 0x8
	fxfTrunc     = 0x10
)

func (s *SFTP) Read(ctx context.Context, path string) ([]byte, error) {
	c, err := s.dial(ctx)
	if err != nil {
		return nil, err
	}
	defer c.close()
	h, err := c.open(ctx, path, fxfRead)
	if err != nil {
		return nil, err
	}
	var all []byte
	var off uint64
	for {
		var body bytes.Buffer
		putSSHString(&body, h)
		binary.Write(&body, binary.BigEndian, off)
		binary.Write(&body, binary.BigEndian, uint32(32768))
		pkt, err := c.call(ctx, fxpRead, body.Bytes())
		if err != nil {
			return nil, err
		}
		switch pkt[0] {
		case fxpData:
			chunk, err := sshString(pkt[5:])
			if err != nil {
				return nil, err
			}
			all = append(all, chunk...)
			off += uint64(len(chunk))
		case fxpStatus:
			code, msg, err := sshStatus(pkt)
			if err != nil {
				return nil, err
			}
			if code == fxEOF || code == fxOK {
				_ = c.closeHandle(ctx, h)
				return all, nil
			}
			return nil, fmt.Errorf("sftp read %s: %d %s", path, code, msg)
		default:
			return nil, fmt.Errorf("sftp read type %d", pkt[0])
		}
	}
}

func (s *SFTP) Write(ctx context.Context, path string, data []byte) error {
	c, err := s.dial(ctx)
	if err != nil {
		return err
	}
	defer c.close()
	h, err := c.open(ctx, path, fxfWrite|fxfCreat|fxfTrunc)
	if err != nil {
		return err
	}
	var body bytes.Buffer
	putSSHString(&body, h)
	binary.Write(&body, binary.BigEndian, uint64(0))
	putSSHBytes(&body, data)
	pkt, err := c.call(ctx, fxpWrite, body.Bytes())
	if err != nil {
		return err
	}
	if err = mustOK(pkt, path); err != nil {
		return err
	}
	return c.closeHandle(ctx, h)
}

func (s *SFTP) Delete(ctx context.Context, path string) error {
	c, err := s.dial(ctx)
	if err != nil {
		return err
	}
	defer c.close()
	var body bytes.Buffer
	putSSHString(&body, path)
	pkt, err := c.call(ctx, fxpRemove, body.Bytes())
	if err != nil {
		return err
	}
	return mustOK(pkt, path)
}

type sftpConn struct {
	conn net.Conn
	id   uint32
	stop chan struct{}
}

func (s *SFTP) dial(ctx context.Context) (*sftpConn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", s.Addr)
	if err != nil {
		return nil, ctxErr(ctx, err)
	}
	c := &sftpConn{conn: conn}
	stop := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-stop:
		}
	}()
	c.stop = stop
	var ver [4]byte
	binary.BigEndian.PutUint32(ver[:], 3)
	if err = writeFXP(conn, append([]byte{fxpInit}, ver[:]...)); err != nil {
		c.close()
		return nil, ctxErr(ctx, err)
	}
	pkt, err := readFXP(conn)
	if err != nil || len(pkt) < 1 || pkt[0] != fxpVersion {
		c.close()
		if err == nil {
			err = errors.New("sftp version missing")
		}
		return nil, ctxErr(ctx, err)
	}
	return c, nil
}

func (c *sftpConn) close() {
	if c.stop != nil {
		close(c.stop)
		c.stop = nil
	}
	c.conn.Close()
}

func (c *sftpConn) open(ctx context.Context, path string, flags uint32) (string, error) {
	var body bytes.Buffer
	putSSHString(&body, path)
	binary.Write(&body, binary.BigEndian, flags)
	binary.Write(&body, binary.BigEndian, uint32(0))
	pkt, err := c.call(ctx, fxpOpen, body.Bytes())
	if err != nil {
		return "", err
	}
	if pkt[0] == fxpStatus {
		code, msg, err := sshStatus(pkt)
		if err != nil {
			return "", err
		}
		return "", fmt.Errorf("sftp open %s: %d %s", path, code, msg)
	}
	if pkt[0] != fxpHandle {
		return "", fmt.Errorf("sftp open type %d", pkt[0])
	}
	h, err := sshString(pkt[5:])
	return string(h), err
}

func (c *sftpConn) closeHandle(ctx context.Context, h string) error {
	var body bytes.Buffer
	putSSHString(&body, h)
	pkt, err := c.call(ctx, fxpClose, body.Bytes())
	if err != nil {
		return err
	}
	return mustOK(pkt, h)
}

func (c *sftpConn) call(ctx context.Context, typ byte, payload []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.id++
	buf := make([]byte, 1+4+len(payload))
	buf[0] = typ
	binary.BigEndian.PutUint32(buf[1:5], c.id)
	copy(buf[5:], payload)
	if err := writeFXP(c.conn, buf); err != nil {
		return nil, ctxErr(ctx, err)
	}
	pkt, err := readFXP(c.conn)
	if err != nil {
		return nil, ctxErr(ctx, err)
	}
	if len(pkt) < 5 {
		return nil, errors.New("short sftp packet")
	}
	return pkt, nil
}

func writeFXP(w io.Writer, payload []byte) error {
	var n [4]byte
	binary.BigEndian.PutUint32(n[:], uint32(len(payload)))
	if _, err := w.Write(n[:]); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

func readFXP(r io.Reader) ([]byte, error) {
	var n [4]byte
	if _, err := io.ReadFull(r, n[:]); err != nil {
		return nil, err
	}
	sz := binary.BigEndian.Uint32(n[:])
	if sz == 0 || sz > 32<<20 {
		return nil, errors.New("sftp packet size")
	}
	buf := make([]byte, sz)
	_, err := io.ReadFull(r, buf)
	return buf, err
}

func putSSHString(b *bytes.Buffer, s string) { putSSHBytes(b, []byte(s)) }

func putSSHBytes(b *bytes.Buffer, p []byte) {
	var n [4]byte
	binary.BigEndian.PutUint32(n[:], uint32(len(p)))
	b.Write(n[:])
	b.Write(p)
}

func sshString(p []byte) ([]byte, error) {
	if len(p) < 4 {
		return nil, errors.New("short ssh string")
	}
	n := binary.BigEndian.Uint32(p[:4])
	if uint32(len(p)) < 4+n {
		return nil, errors.New("short ssh string")
	}
	return p[4 : 4+n], nil
}

func sshStatus(pkt []byte) (uint32, string, error) {
	if len(pkt) < 9 {
		return 0, "", errors.New("short sftp status")
	}
	code := binary.BigEndian.Uint32(pkt[5:9])
	msg, err := sshString(pkt[9:])
	return code, string(msg), err
}

func mustOK(pkt []byte, path string) error {
	if len(pkt) == 0 || pkt[0] != fxpStatus {
		return fmt.Errorf("sftp status missing for %s", path)
	}
	code, msg, err := sshStatus(pkt)
	if err != nil {
		return err
	}
	if code != fxOK {
		return fmt.Errorf("sftp %s: %d %s", path, code, msg)
	}
	return nil
}
