package editor

import (
	"context"
	"errors"
	"github.com/egoist/mygo/internal/diff"
)

// ComparisonRequest 固定输入快照与缓冲区身份，可在后台计算而不读取控件状态。
type ComparisonRequest struct {
	old, new      *Buffer
	before, after Snapshot
}

// Comparison 是只读计算结果；只能应用到请求对应的缓冲区版本。
type Comparison struct {
	request ComparisonRequest
	inline  DiffInline
	words   map[wordKey]*hunkWords
	hunks   []diff.Hunk
}

// NewComparisonRequest 在调用线程捕获不可变快照。
func NewComparisonRequest(old, new *Buffer) ComparisonRequest {
	if old == nil || new == nil {
		return ComparisonRequest{}
	}
	return ComparisonRequest{old: old, new: new, before: old.Snapshot(), after: new.Snapshot()}
}

// Compute 执行可取消的行级比较，适合由应用的后台任务调用。
func (r ComparisonRequest) Compute(ctx context.Context, inline ...DiffInline) (Comparison, error) {
	if err := ctx.Err(); err != nil {
		return Comparison{}, err
	}
	if r.old == nil || r.new == nil {
		return Comparison{}, errors.New("invalid comparison request")
	}
	a, b := r.before.t, r.after.t
	hunks, err := diff.LinesContext(ctx, ropeLines(a, 0, a.LineCount()), ropeLines(b, 0, b.LineCount()))
	if err != nil {
		return Comparison{}, err
	}
	mode := DiffInlineWordAlt
	if len(inline) > 1 {
		return Comparison{}, errors.New("at most one inline mode is allowed")
	}
	if len(inline) == 1 {
		mode = inline[0]
	}
	if mode > DiffInlineNone {
		return Comparison{}, errors.New("invalid inline mode")
	}
	model := diffModel{oldText: a, newText: b, hunks: hunks}
	for i := range hunks {
		if _, err := model.wordsOfContext(ctx, i, mode); err != nil {
			return Comparison{}, err
		}
	}
	return Comparison{request: r, hunks: hunks, inline: mode, words: model.words}, nil
}

// Hunks 返回独立的差异块副本，不能修改结果内部切片。
func (r Comparison) Hunks() []DiffHunk {
	out := make([]DiffHunk, len(r.hunks))
	for i, h := range r.hunks {
		out[i] = DiffHunk{h.A0, h.A1, h.B0, h.B1}
	}
	return out
}

// ComparisonReady 区分“正在计算”和“无差异”。
func (d *DiffState) ComparisonReady() bool { d.ensure(); return !d.ExternalComparison || d.model.ready }

// ApplyComparison 在 UI 线程应用结果，过期版本或其他缓冲区的结果返回 false。
func (d *DiffState) ApplyComparison(result Comparison) bool {
	d.ensure()
	r := result.request
	if !d.ExternalComparison || result.inline != d.Inline || r.old != d.Old || r.new != d.New || r.old == nil || r.new == nil {
		return false
	}
	if r.before.Version() != d.Old.Version() || r.after.Version() != d.New.Version() {
		return false
	}
	m := &d.model
	m.hunks = append([]diff.Hunk(nil), result.hunks...)
	m.words = make(map[wordKey]*hunkWords, len(result.words))
	for key, words := range result.words {
		m.words[key] = words
	}
	m.ready = true
	m.version++
	return true
}

// ApplyHunks 用调用方给出的半开行区间替换外部比较结果。
// 区间必须落在当前缓冲区内且按左右起点非递减；否则返回 false，不改已有结果。
func (d *DiffState) ApplyHunks(hunks []DiffHunk) bool {
	d.ensure()
	if !d.ExternalComparison || d.Old == nil || d.New == nil {
		return false
	}
	oldN, newN := d.Old.LineCount(), d.New.LineCount()
	out := make([]diff.Hunk, len(hunks))
	prevOld, prevNew := 0, 0
	for i, h := range hunks {
		if h.OldStart < prevOld || h.NewStart < prevNew || h.OldEnd < h.OldStart || h.NewEnd < h.NewStart || h.OldEnd > oldN || h.NewEnd > newN {
			return false
		}
		out[i] = diff.Hunk{A0: h.OldStart, A1: h.OldEnd, B0: h.NewStart, B1: h.NewEnd}
		prevOld, prevNew = h.OldEnd, h.NewEnd
	}
	oldText, oldVer := d.Old.snapshot()
	newText, newVer := d.New.snapshot()
	m := &d.model
	m.oldText, m.newText, m.oldVer, m.newVer = oldText, newText, oldVer, newVer
	m.hunks = out
	m.words = nil
	m.ready = true
	m.version++
	return true
}

// DiffHunkBand 描述分栏视图中一个差异块在当前视口内的纵向像素范围（DIP）。
type DiffHunkBand struct {
	Index       int
	Hunk        DiffHunk
	Top, Height float32
}

// HunkBands 返回分栏模式下各差异块在视口中的纵向位置，供中间差异操作列对齐绘制。
func (d *DiffState) HunkBands() []DiffHunkBand {
	d.ensure()
	if d.Layout != DiffSplit || len(d.model.hunks) == 0 {
		return nil
	}
	out := make([]DiffHunkBand, 0, len(d.model.hunks))
	scrollY := d.eds[1].scrollY
	for i, h := range d.model.hunks {
		nA, nB := h.A1-h.A0, h.B1-h.B0
		if nA <= 0 && nB <= 0 {
			continue
		}
		side, start, end := 1, h.B0, h.B1
		if nA >= nB {
			side, start, end = 0, h.A0, h.A1
		}
		vh := &d.eds[side].view.heights
		if vh.n <= 0 || vh.lh <= 0 {
			continue
		}
		start = max(0, min(start, vh.n-1))
		last := max(start, min(end-1, vh.n-1))
		top := vh.geometry(start).rows
		gLast := vh.geometry(last)
		bottom := gLast.below
		_, below := vh.blocksOf(last)
		for _, b := range below {
			if b.kind == blockSpacer || b.kind == blockAnnotation {
				bottom += float64(b.height)
			}
		}
		if bottom < top {
			bottom = top
		}
		out = append(out, DiffHunkBand{
			Index:  i,
			Hunk:   DiffHunk{h.A0, h.A1, h.B0, h.B1},
			Top:    float32(top - scrollY),
			Height: float32(bottom - top),
		})
	}
	return out
}

// DiffLineBand 描述分栏视图中某一侧的一段行范围在当前视口内的纵向像素范围（DIP）。
type DiffLineBand struct {
	Range       DiffLineRange
	Top, Height float32
}

// LineBands 返回分栏模式下指定行区间在当前视口中的纵向位置，供中间列绘制已应用标记。
func (d *DiffState) LineBands(ranges []DiffLineRange) []DiffLineBand {
	d.ensure()
	if d.Layout != DiffSplit || len(ranges) == 0 {
		return nil
	}
	out := make([]DiffLineBand, 0, len(ranges))
	scrollY := d.eds[1].scrollY
	for _, r := range ranges {
		side := 0
		if r.Side == DiffNew {
			side = 1
		}
		vh := &d.eds[side].view.heights
		if vh.n <= 0 || vh.lh <= 0 {
			continue
		}
		start := max(0, min(r.Start, vh.n-1))
		last := max(start, min(r.End, vh.n-1))
		top := vh.geometry(start).rows
		gLast := vh.geometry(last)
		bottom := gLast.below
		if bottom < top {
			bottom = top
		}
		out = append(out, DiffLineBand{
			Range:  r,
			Top:    float32(top - scrollY),
			Height: float32(bottom - top),
		})
	}
	return out
}
