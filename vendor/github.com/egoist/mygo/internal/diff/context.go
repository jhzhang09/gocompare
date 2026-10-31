package diff

import "context"

// cancelled 只用于内部递归算法的取消展开，不拦截其他 panic。
type cancelled struct{ err error }

func (d *differ) checkpoint() {
	if d.ctx == nil {
		return
	}
	d.steps++
	if d.steps&1023 == 0 {
		if err := d.ctx.Err(); err != nil {
			panic(cancelled{err})
		}
	}
}

// LinesContext 与 Lines 使用同一算法，在分词、匹配及压缩阶段检查取消。
func LinesContext(ctx context.Context, a, b []string) (hunks []Hunk, err error) {
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	defer func() {
		if p := recover(); p != nil {
			if c, ok := p.(cancelled); ok {
				hunks = nil
				err = c.err
			} else {
				panic(p)
			}
		}
	}()
	d := differ{ctx: ctx}
	ids := make(map[string]int32, len(a)+len(b))
	intern := func(lines []string) []int32 {
		out := make([]int32, len(lines))
		for i, line := range lines {
			d.checkpoint()
			id, ok := ids[line]
			if !ok {
				id = int32(len(ids))
				ids[line] = id
			}
			out[i] = id
		}
		return out
	}
	d.a, d.b = intern(a), intern(b)
	d.histogram(0, len(a), 0, len(b), 0)
	if d.da != nil {
		compact(d.da, d.db, d.a, a, d.checkpoint)
		compact(d.db, d.da, d.b, b, d.checkpoint)
		hunks = d.hunks()
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return hunks, nil
}
