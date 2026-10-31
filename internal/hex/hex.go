// Package hex 提供分块比较和可撤销的字节编辑。
package hex

import "bytes"

// Chunk 是两侧在 Offset 处不相同的一段字节。
type Chunk struct {
	Offset int
	Left   []byte
	Right  []byte
}

// Diff 按固定块长比较。只有内容不同的块会被返回；末尾长度差单独成块。
func Diff(left, right []byte, size int) []Chunk {
	if size <= 0 {
		size = 16
	}
	n := len(left)
	if len(right) > n {
		n = len(right)
	}
	var out []Chunk
	for off := 0; off < n; off += size {
		lend := off + size
		if lend > len(left) {
			lend = len(left)
		}
		rend := off + size
		if rend > len(right) {
			rend = len(right)
		}
		var l, r []byte
		if off < len(left) {
			l = left[off:lend]
		}
		if off < len(right) {
			r = right[off:rend]
		}
		if !bytes.Equal(l, r) {
			out = append(out, Chunk{Offset: off, Left: append([]byte(nil), l...), Right: append([]byte(nil), r...)})
		}
	}
	return out
}

// Buffer 是一份可搜索、可插删、可撤销的字节缓冲。
type Buffer struct {
	data []byte
	undo [][]byte
}

// New 复制输入，后续编辑不影响调用方的切片。
func New(data []byte) *Buffer {
	return &Buffer{data: append([]byte(nil), data...)}
}

// Bytes 返回当前内容的副本。
func (b *Buffer) Bytes() []byte {
	return append([]byte(nil), b.data...)
}

// Search 返回 needle 第一次出现的位置；找不到时返回 -1。空 needle 返回 -1。
func (b *Buffer) Search(needle []byte) int {
	if len(needle) == 0 {
		return -1
	}
	return bytes.Index(b.data, needle)
}

// Insert 在 off 处插入字节，并记下可撤销的原内容。
func (b *Buffer) Insert(off int, p []byte) {
	if off < 0 || off > len(b.data) {
		return
	}
	b.push()
	next := make([]byte, 0, len(b.data)+len(p))
	next = append(next, b.data[:off]...)
	next = append(next, p...)
	next = append(next, b.data[off:]...)
	b.data = next
}

// Delete 删除 off 起的 n 个字节。越界时不修改。
func (b *Buffer) Delete(off, n int) {
	if off < 0 || n < 0 || off+n > len(b.data) {
		return
	}
	b.push()
	next := make([]byte, 0, len(b.data)-n)
	next = append(next, b.data[:off]...)
	next = append(next, b.data[off+n:]...)
	b.data = next
}

// Undo 恢复上一次插删之前的内容。没有历史时返回 false。
func (b *Buffer) Undo() bool {
	if len(b.undo) == 0 {
		return false
	}
	b.data = b.undo[len(b.undo)-1]
	b.undo = b.undo[:len(b.undo)-1]
	return true
}

func (b *Buffer) push() {
	b.undo = append(b.undo, append([]byte(nil), b.data...))
}
