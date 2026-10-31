package textcompare

import "fmt"

func diffLines(a, b []string, algorithm Algorithm) ([]Hunk, error) {
	switch algorithm {
	case "", Myers:
		return myers(a, b), nil
	case Patience:
		return patience(a, b), nil
	case Histogram:
		return histogram(a, b), nil
	default:
		return nil, fmt.Errorf("unknown diff algorithm: %s", algorithm)
	}
}

func myers(a, b []string) []Hunk {
	n, m := len(a), len(b)
	if n == 0 && m == 0 {
		return nil
	}
	prefix := 0
	for prefix < n && prefix < m && a[prefix] == b[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < n-prefix && suffix < m-prefix && a[n-1-suffix] == b[m-1-suffix] {
		suffix++
	}
	a = a[prefix : n-suffix]
	b = b[prefix : m-suffix]
	n, m = len(a), len(b)
	if n == 0 && m == 0 {
		return nil
	}
	if n == 0 {
		return []Hunk{{prefix, prefix, prefix, prefix + m}}
	}
	if m == 0 {
		return []Hunk{{prefix, prefix + n, prefix, prefix}}
	}
	max := n + m
	offset := max
	v := make([]int, 2*max+1)
	trace := make([][]int, 0, max+1)
	var found int
	done := false
	for d := 0; d <= max && !done; d++ {
		snap := append([]int(nil), v...)
		trace = append(trace, snap)
		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[offset+k-1] < v[offset+k+1]) {
				x = v[offset+k+1]
			} else {
				x = v[offset+k-1] + 1
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			v[offset+k] = x
			if x >= n && y >= m {
				found = d
				done = true
				break
			}
		}
	}
	if !done {
		return []Hunk{{prefix, prefix + n, prefix, prefix + m}}
	}
	// trace[d] 是第 d 轮开始前的前沿。回溯必须用它，才能复现当时读到的 k-1 与 k+1。
	hunks := backtrack(trace, found, offset, n, m)
	for i := range hunks {
		hunks[i].LeftStart += prefix
		hunks[i].LeftEnd += prefix
		hunks[i].RightStart += prefix
		hunks[i].RightEnd += prefix
	}
	return coalesce(hunks)
}

func backtrack(trace [][]int, d, offset, n, m int) []Hunk {
	x, y := n, m
	type edit struct {
		del  bool
		x, y int
	}
	var edits []edit
	for d > 0 {
		v := trace[d]
		k := x - y
		var prevK int
		if k == -d || (k != d && v[offset+k-1] < v[offset+k+1]) {
			prevK = k + 1
		} else {
			prevK = k - 1
		}
		prevX := v[offset+prevK]
		prevY := prevX - prevK
		for x > prevX && y > prevY {
			x--
			y--
		}
		// 蛇走完后只剩一步水平删除或垂直插入。
		if x == prevX {
			y--
			edits = append(edits, edit{del: false, x: x, y: y})
		} else {
			x--
			edits = append(edits, edit{del: true, x: x, y: y})
		}
		d--
	}
	var hunks []Hunk
	var current *Hunk
	flush := func() {
		if current != nil {
			hunks = append(hunks, *current)
			current = nil
		}
	}
	for i := len(edits) - 1; i >= 0; i-- {
		e := edits[i]
		extends := current != nil && e.x == current.LeftEnd && e.y == current.RightEnd
		if !extends {
			flush()
			if e.del {
				current = &Hunk{LeftStart: e.x, LeftEnd: e.x + 1, RightStart: e.y, RightEnd: e.y}
			} else {
				current = &Hunk{LeftStart: e.x, LeftEnd: e.x, RightStart: e.y, RightEnd: e.y + 1}
			}
			continue
		}
		if e.del {
			current.LeftEnd = e.x + 1
		} else {
			current.RightEnd = e.y + 1
		}
	}
	flush()
	return hunks
}

func coalesce(hunks []Hunk) []Hunk {
	if len(hunks) == 0 {
		return nil
	}
	out := []Hunk{hunks[0]}
	for _, h := range hunks[1:] {
		last := &out[len(out)-1]
		if h.LeftStart <= last.LeftEnd && h.RightStart <= last.RightEnd {
			if h.LeftEnd > last.LeftEnd {
				last.LeftEnd = h.LeftEnd
			}
			if h.RightEnd > last.RightEnd {
				last.RightEnd = h.RightEnd
			}
			continue
		}
		out = append(out, h)
	}
	return out
}

func patience(a, b []string) []Hunk {
	countA, countB := map[string]int{}, map[string]int{}
	indexB := map[string]int{}
	for _, line := range a {
		countA[line]++
	}
	for j, line := range b {
		countB[line]++
		if countB[line] == 1 {
			indexB[line] = j
		}
	}
	var pairs [][2]int
	for i, line := range a {
		if countA[line] == 1 && countB[line] == 1 {
			pairs = append(pairs, [2]int{i, indexB[line]})
		}
	}
	matched := lis(pairs)
	points := append([][2]int{{-1, -1}}, matched...)
	points = append(points, [2]int{len(a), len(b)})
	var hunks []Hunk
	for i := 1; i < len(points); i++ {
		a0, b0 := points[i-1][0]+1, points[i-1][1]+1
		a1, b1 := points[i][0], points[i][1]
		part := myers(a[a0:a1], b[b0:b1])
		for _, h := range part {
			hunks = append(hunks, Hunk{h.LeftStart + a0, h.LeftEnd + a0, h.RightStart + b0, h.RightEnd + b0})
		}
	}
	return coalesce(hunks)
}

func lis(pairs [][2]int) [][2]int {
	if len(pairs) == 0 {
		return nil
	}
	prev := make([]int, len(pairs))
	size := make([]int, len(pairs))
	best := 0
	for i := range pairs {
		prev[i] = -1
		size[i] = 1
		for j := 0; j < i; j++ {
			if pairs[j][1] < pairs[i][1] && size[j]+1 > size[i] {
				size[i] = size[j] + 1
				prev[i] = j
			}
		}
		if size[i] > size[best] {
			best = i
		}
	}
	var out [][2]int
	for i := best; i >= 0; i = prev[i] {
		out = append(out, pairs[i])
		if prev[i] < 0 {
			break
		}
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func histogram(a, b []string) []Hunk {
	return hist(a, b, 0, len(a), 0, len(b))
}

func hist(a, b []string, a0, a1, b0, b1 int) []Hunk {
	for a0 < a1 && b0 < b1 && a[a0] == b[b0] {
		a0++
		b0++
	}
	for a0 < a1 && b0 < b1 && a[a1-1] == b[b1-1] {
		a1--
		b1--
	}
	if a0 == a1 && b0 == b1 {
		return nil
	}
	if a0 == a1 || b0 == b1 {
		return []Hunk{{a0, a1, b0, b1}}
	}
	countA, countB := map[string]int{}, map[string]int{}
	for i := a0; i < a1; i++ {
		countA[a[i]]++
	}
	for i := b0; i < b1; i++ {
		countB[b[i]]++
	}
	best, score := -1, 0
	for i := a0; i < a1; i++ {
		cb := countB[a[i]]
		if cb == 0 {
			continue
		}
		value := countA[a[i]] + cb
		if best < 0 || value < score {
			best, score = i, value
		}
	}
	if best < 0 {
		return []Hunk{{a0, a1, b0, b1}}
	}
	line := a[best]
	other := b0
	for other < b1 && b[other] != line {
		other++
	}
	left := hist(a, b, a0, best, b0, other)
	right := hist(a, b, best+1, a1, other+1, b1)
	return coalesce(append(left, right...))
}
