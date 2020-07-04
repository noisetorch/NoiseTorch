package nucular

type GroupList struct {
	w   *Window
	num int

	idx               int
	scrollbary        int
	done              bool
	skippedLineHeight int
}

// GroupListStart starts a scrollable list of <num> rows of <height> height
func GroupListStart(w *Window, num int, name string, flags WindowFlags) (GroupList, *Window) {
	var gl GroupList
	gl.w = w.GroupBegin(name, flags)
	gl.num = num
	gl.idx = -1
	if gl.w != nil {
		gl.scrollbary = gl.w.Scrollbar.Y
	}

	return gl, gl.w
}

func (gl *GroupList) Next() bool {
	if gl.w == nil {
		return false
	}
	if gl.skippedLineHeight > 0 && gl.idx >= 0 {
		if _, below := gl.w.Invisible(0); below {
			n := gl.num - gl.idx
			gl.idx = gl.num
			gl.empty(n)
		}
	}
	gl.idx++
	if gl.idx >= gl.num {
		if !gl.done {
			gl.done = true
			if gl.scrollbary != gl.w.Scrollbar.Y {
				gl.w.Scrollbar.Y = gl.scrollbary
				gl.w.Master().Changed()
			}
			gl.w.GroupEnd()
		}
		return false
	}
	return true
}

func (gl *GroupList) SkipToVisible(lineheight int) {
	if gl.w == nil {
		return
	}
	gl.SkipToVisibleScaled(gl.w.ctx.scale(lineheight))
}

func (gl *GroupList) SkipToVisibleScaled(lineheight int) {
	if gl.w == nil {
		return
	}
	skip := gl.w.Scrollbar.Y/(lineheight+gl.w.style().Spacing.Y) - 2
	if maxskip := gl.num - 3; skip > maxskip {
		skip = maxskip
	}
	if skip < 0 {
		skip = 0
	}
	gl.skippedLineHeight = lineheight
	gl.empty(skip)
	gl.idx = skip - 1
}

func (gl *GroupList) empty(n int) {
	if n <= 0 {
		return
	}
	gl.w.RowScaled(n*gl.skippedLineHeight + (n-1)*gl.w.style().Spacing.Y).Dynamic(1)
	gl.w.Label("More...", "LC")
}

func (gl *GroupList) Index() int {
	return gl.idx
}

func (gl *GroupList) Center() {
	if above, below := gl.w.Invisible(gl.w.LastWidgetBounds.H * 2); above || below {
		gl.scrollbary = gl.w.At().Y - gl.w.Bounds.H/2
		if gl.scrollbary < 0 {
			gl.scrollbary = 0
		}
	}
}
