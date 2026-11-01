package remote

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"unicode/utf16"
)

// SMB 实现 SMB2 的 NEGOTIATE、SESSION_SETUP、TREE_CONNECT、CREATE、READ、WRITE 和 CLOSE。
// 删除通过 CREATE 的 FILE_DELETE_ON_CLOSE 再 CLOSE 完成。这不是完整的 SMB 栈：
// SESSION_SETUP 不携带安全令牌，只能连接受控夹具，ParseURL 会拒绝 smb URL。
type SMB struct {
	Addr  string
	Share string
}

const (
	smbNegotiate = 0
	smbSession   = 1
	smbTree      = 3
	smbCreate    = 5
	smbCloseCmd  = 6
	smbReadCmd   = 8
	smbWriteCmd  = 9

	fileOpen        = 1
	fileOverwriteIf = 5 // 存在则截断，不存在则创建；FILE_OPEN_IF 不截断，短写会残留旧尾部
	fileDelete      = 0x00001000

	// 直连 TCP 的帧长度字段只有 24 位。
	smbMaxFrame = 1<<24 - 1
)

func (s *SMB) Read(ctx context.Context, path string) ([]byte, error) {
	return s.exchange(ctx, path, fileOpen, 0x00000001, 0, nil)
}

func (s *SMB) Write(ctx context.Context, path string, data []byte) error {
	// 单个 WRITE 携带全部数据，帧需容纳 64 字节头和 48 字节 WRITE 体。
	if len(data) > smbMaxFrame-64-48 {
		return fmt.Errorf("smb write exceeds %d bytes", smbMaxFrame-64-48)
	}
	_, err := s.exchange(ctx, path, fileOverwriteIf, 0x00000002, 0, data)
	return err
}

func (s *SMB) Delete(ctx context.Context, path string) error {
	_, err := s.exchange(ctx, path, fileOpen, 0x00010000, fileDelete, nil)
	return err
}

func (s *SMB) exchange(ctx context.Context, path string, disposition, access uint32, options uint32, upload []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", s.Addr)
	if err != nil {
		return nil, ctxErr(ctx, err)
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
	c := &smbConn{conn: conn}
	if _, err = c.round(ctx, smbNegotiate, negotiateBody()); err != nil {
		return nil, err
	}
	if _, err = c.round(ctx, smbSession, sessionBody()); err != nil {
		return nil, err
	}
	share := s.Share
	if share == "" {
		share = `\\gocompare\share`
	}
	if _, err = c.round(ctx, smbTree, treeBody(share)); err != nil {
		return nil, err
	}
	created, err := c.round(ctx, smbCreate, createBody(path, disposition, access, options))
	if err != nil {
		return nil, err
	}
	if len(created) < 80 {
		return nil, fmt.Errorf("smb create response short")
	}
	var fileID [16]byte
	copy(fileID[:], created[64:80])
	var data []byte
	if upload != nil {
		if _, err = c.round(ctx, smbWriteCmd, writeBody(fileID, upload)); err != nil {
			return nil, err
		}
	} else if options&fileDelete == 0 {
		for {
			body, err := c.round(ctx, smbReadCmd, readBody(fileID, uint64(len(data))))
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			chunk, err := readData(body)
			if err != nil {
				return nil, err
			}
			if len(chunk) == 0 {
				break
			}
			if len(data)+len(chunk) > maxResponseBytes {
				return nil, fmt.Errorf("smb file exceeds 32 MiB")
			}
			data = append(data, chunk...)
		}
	}
	if _, err = c.round(ctx, smbCloseCmd, closeBody(fileID)); err != nil {
		return nil, err
	}
	return data, nil
}

type smbConn struct {
	conn    net.Conn
	next    uint64
	session uint64
	tree    uint32
}

func (c *smbConn) round(ctx context.Context, cmd uint16, body []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.next++
	pkt := append(smbHeader(cmd, c.next, c.session, c.tree), body...)
	if err := writeSMB(c.conn, pkt); err != nil {
		return nil, ctxErr(ctx, err)
	}
	resp, err := readSMB(c.conn)
	if err != nil {
		return nil, ctxErr(ctx, err)
	}
	if len(resp) < 64 {
		return nil, fmt.Errorf("short smb response")
	}
	if resp[0] != 0xFE || string(resp[1:4]) != "SMB" {
		return nil, fmt.Errorf("smb protocol id")
	}
	status := binary.LittleEndian.Uint32(resp[8:12])
	if cmd == smbReadCmd && status == 0xC0000011 {
		return nil, io.EOF
	}
	if status != 0 {
		return nil, fmt.Errorf("smb status %08x", status)
	}
	if binary.LittleEndian.Uint16(resp[12:14]) != cmd {
		return nil, fmt.Errorf("smb command %d", binary.LittleEndian.Uint16(resp[12:14]))
	}
	if cmd == smbNegotiate {
		body := resp[64:]
		if len(body) < 6 || binary.LittleEndian.Uint16(body[4:6]) != 0x0202 {
			return nil, fmt.Errorf("smb dialect")
		}
	}
	if cmd == smbSession {
		c.session = binary.LittleEndian.Uint64(resp[40:48])
	}
	if cmd == smbTree {
		c.tree = binary.LittleEndian.Uint32(resp[36:40])
	}
	return resp[64:], nil
}

func smbHeader(cmd uint16, msgID, session uint64, tree uint32) []byte {
	h := make([]byte, 64)
	h[0] = 0xFE
	h[1], h[2], h[3] = 'S', 'M', 'B'
	binary.LittleEndian.PutUint16(h[4:], 64)
	binary.LittleEndian.PutUint16(h[12:], cmd)
	binary.LittleEndian.PutUint16(h[14:], 1)
	binary.LittleEndian.PutUint64(h[24:], msgID)
	binary.LittleEndian.PutUint32(h[36:], tree)
	binary.LittleEndian.PutUint64(h[40:], session)
	return h
}

func negotiateBody() []byte {
	body := make([]byte, 38)
	binary.LittleEndian.PutUint16(body[0:], 36)
	binary.LittleEndian.PutUint16(body[2:], 1)
	binary.LittleEndian.PutUint16(body[4:], 1)
	binary.LittleEndian.PutUint16(body[36:], 0x0202)
	return body
}

func sessionBody() []byte {
	body := make([]byte, 24)
	binary.LittleEndian.PutUint16(body[0:], 25)
	return body
}

func treeBody(share string) []byte {
	u := encodeUTF16(share)
	body := make([]byte, 8+len(u))
	binary.LittleEndian.PutUint16(body[0:], 9)
	binary.LittleEndian.PutUint16(body[4:], 72)
	binary.LittleEndian.PutUint16(body[6:], uint16(len(u)))
	copy(body[8:], u)
	return body
}

func createBody(path string, disposition, access, options uint32) []byte {
	name := encodeUTF16(path)
	body := make([]byte, 56+len(name))
	binary.LittleEndian.PutUint16(body[0:], 57)
	binary.LittleEndian.PutUint32(body[4:], 2)
	binary.LittleEndian.PutUint32(body[24:], access)
	binary.LittleEndian.PutUint32(body[32:], 0x3)
	binary.LittleEndian.PutUint32(body[36:], disposition)
	binary.LittleEndian.PutUint32(body[40:], options)
	binary.LittleEndian.PutUint16(body[44:], 120)
	binary.LittleEndian.PutUint16(body[46:], uint16(len(name)))
	copy(body[56:], name)
	return body
}

func readBody(id [16]byte, offset uint64) []byte {
	body := make([]byte, 49)
	binary.LittleEndian.PutUint16(body[0:], 49)
	binary.LittleEndian.PutUint32(body[4:], 1<<20)
	binary.LittleEndian.PutUint64(body[8:], offset)
	copy(body[16:], id[:])
	return body
}

func writeBody(id [16]byte, data []byte) []byte {
	body := make([]byte, 48+len(data))
	binary.LittleEndian.PutUint16(body[0:], 49)
	binary.LittleEndian.PutUint16(body[2:], 112)
	binary.LittleEndian.PutUint32(body[4:], uint32(len(data)))
	copy(body[16:], id[:])
	copy(body[48:], data)
	return body
}

func closeBody(id [16]byte) []byte {
	body := make([]byte, 24)
	binary.LittleEndian.PutUint16(body[0:], 24)
	copy(body[8:], id[:])
	return body
}

func readData(body []byte) ([]byte, error) {
	if len(body) < 16 {
		return nil, fmt.Errorf("short smb read")
	}
	off := int(body[2])
	n := int(binary.LittleEndian.Uint32(body[4:8]))
	// DataOffset 从 SMB2 头开始计算，body 已经去掉了 64 字节头。
	rel := off - 64
	if rel < 0 || n < 0 || rel+n > len(body) {
		return nil, fmt.Errorf("smb read data out of range")
	}
	return append([]byte(nil), body[rel:rel+n]...), nil
}

func writeSMB(w io.Writer, pkt []byte) error {
	if len(pkt) > smbMaxFrame {
		return fmt.Errorf("smb frame %d exceeds %d bytes", len(pkt), smbMaxFrame)
	}
	hdr := []byte{0, byte(len(pkt) >> 16), byte(len(pkt) >> 8), byte(len(pkt))}
	if _, err := w.Write(hdr); err != nil {
		return err
	}
	_, err := w.Write(pkt)
	return err
}

func readSMB(r io.Reader) ([]byte, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	n := int(hdr[1])<<16 | int(hdr[2])<<8 | int(hdr[3])
	if n < 64 {
		return nil, fmt.Errorf("smb frame %d", n)
	}
	buf := make([]byte, n)
	_, err := io.ReadFull(r, buf)
	return buf, err
}

func encodeUTF16(s string) []byte {
	u := utf16.Encode([]rune(s))
	out := make([]byte, len(u)*2)
	for i, v := range u {
		binary.LittleEndian.PutUint16(out[i*2:], v)
	}
	return out
}
