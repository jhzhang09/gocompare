// Package snapshot 保存不含文件内容的目录快照。空目录与空文件的编码不同，快照也不是空文件。
package snapshot

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Magic 是快照文件头。没有这个头的字节，包括空文件，都不是快照。
var Magic = []byte("GOCMPSNP\n")

// Node 只记录路径、是否目录和声明的大小，不记录内容。
type Node struct {
	Path string
	Dir  bool
	Size int64
}

// Encode 写出快照。没有任何节点时仍然带有 Magic，因此不同于空文件。
func Encode(nodes []Node) ([]byte, error) {
	var buf bytes.Buffer
	buf.Write(Magic)
	if err := binary.Write(&buf, binary.LittleEndian, uint32(len(nodes))); err != nil {
		return nil, err
	}
	for _, n := range nodes {
		if n.Size < 0 {
			return nil, fmt.Errorf("negative size for %s", n.Path)
		}
		flag := byte(0)
		if n.Dir {
			flag = 1
		}
		buf.WriteByte(flag)
		if err := binary.Write(&buf, binary.LittleEndian, uint64(n.Size)); err != nil {
			return nil, err
		}
		if err := binary.Write(&buf, binary.LittleEndian, uint32(len(n.Path))); err != nil {
			return nil, err
		}
		buf.WriteString(n.Path)
	}
	return buf.Bytes(), nil
}

// Decode 解析快照。输入缺少 Magic 时返回错误。
func Decode(data []byte) ([]Node, error) {
	if len(data) < len(Magic) || !bytes.Equal(data[:len(Magic)], Magic) {
		return nil, errors.New("not a directory snapshot")
	}
	r := bytes.NewReader(data[len(Magic):])
	var count uint32
	if err := binary.Read(r, binary.LittleEndian, &count); err != nil {
		return nil, err
	}
	out := make([]Node, 0, count)
	for i := uint32(0); i < count; i++ {
		flag, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		var size uint64
		var n uint32
		if err = binary.Read(r, binary.LittleEndian, &size); err != nil {
			return nil, err
		}
		if err = binary.Read(r, binary.LittleEndian, &n); err != nil {
			return nil, err
		}
		if n > 1<<20 {
			return nil, errors.New("snapshot path too long")
		}
		path := make([]byte, n)
		if _, err = io.ReadFull(r, path); err != nil {
			return nil, err
		}
		out = append(out, Node{Path: string(path), Dir: flag == 1, Size: int64(size)})
	}
	return out, nil
}
