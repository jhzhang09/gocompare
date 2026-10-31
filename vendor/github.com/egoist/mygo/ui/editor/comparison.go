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
