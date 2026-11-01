// Package snapshot 保存不含文件内容的目录快照。空目录与空文件的编码不同，快照也不是空文件。
package snapshot

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	// 每条节点至少包含类型、大小和路径长度，共 13 字节。
	if uint64(count) > uint64(r.Len()/13) {
		return nil, errors.New("invalid snapshot node count")
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

// NodeDiff 表示快照节点之间的差异。
type NodeDiff struct {
	Path   string
	Status string // "same", "different", "left_only", "right_only"
	Left   int64
	Right  int64
}

// Create 扫描本地目录并构建无内容快照节点列表。
func Create(root string) ([]Node, error) {
	var nodes []Node
	cleanRoot := filepath.Clean(root)
	err := filepath.Walk(cleanRoot, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(cleanRoot, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		size := int64(0)
		if !info.IsDir() {
			size = info.Size()
		}
		nodes = append(nodes, Node{Path: rel, Dir: info.IsDir(), Size: size})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return nodes, nil
}

// Compare 对比两个快照。
func Compare(left, right []Node) []NodeDiff {
	rightMap := make(map[string]Node, len(right))
	for _, n := range right {
		rightMap[n.Path] = n
	}
	var diffs []NodeDiff
	used := make(map[string]bool, len(right))
	for _, l := range left {
		r, ok := rightMap[l.Path]
		if !ok {
			diffs = append(diffs, NodeDiff{Path: l.Path, Status: "left_only", Left: l.Size})
			continue
		}
		used[l.Path] = true
		if l.Dir != r.Dir || l.Size != r.Size {
			diffs = append(diffs, NodeDiff{Path: l.Path, Status: "different", Left: l.Size, Right: r.Size})
		} else {
			diffs = append(diffs, NodeDiff{Path: l.Path, Status: "same", Left: l.Size, Right: r.Size})
		}
	}
	for _, r := range right {
		if !used[r.Path] {
			diffs = append(diffs, NodeDiff{Path: r.Path, Status: "right_only", Right: r.Size})
		}
	}
	return diffs
}
