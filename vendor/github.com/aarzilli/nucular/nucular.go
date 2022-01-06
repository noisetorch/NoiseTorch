package nucular

import (
	"errors"
	"image"
	"image/color"
	"math"
	"strconv"
	"sync/atomic"

	"github.com/aarzilli/nucular/command"
	"github.com/aarzilli/nucular/font"
	"github.com/aarzilli/nucular/label"
	"github.com/aarzilli/nucular/rect"
	nstyle "github.com/aarzilli/nucular/style"

	"golang.org/x/mobile/event/key"
	"golang.org/x/mobile/event/mouse"
)

///////////////////////////////////////////////////////////////////////////////////
// CONTEXT & PANELS
///////////////////////////////////////////////////////////////////////////////////

type UpdateFn func(*Window)

type Window struct {
	LastWidgetBounds rect.Rect
	Data             interface{}
	title            string
	ctx              *context
	idx              int
	flags            WindowFlags
	Bounds           rect.Rect
	Scrollbar        image.Point
	cmds             command.Buffer
	widgets          widgetBuffer
	layout           *panel
	close, first     bool
	moving           bool
	scaling          bool
	// trigger rectangle of nonblocking windows
	header rect.Rect
	// root of the node tree
	rootNode *treeNode
	// current tree node see TreePush/TreePop
	curNode *treeNode
	// parent window of a popup
	parent *Window
	// helper windows to implement groups
	groupWnd map[string]*Window
	// update function
	updateFn      UpdateFn
	usingSub      bool
	began         bool
	rowCtor       rowConstructor
	menuItemWidth int
	lastLayoutCnt int
	adjust        map[int]map[int]*adjustCol
	undockedSz    image.Point
	editors       map[string]*TextEditor
}

type FittingWidthFn func(width int)

type adjustCol struct {
	id    int
	font  font.Face
	width int
	first bool
}

type treeNode struct {
	Open     bool
	Children map[string]*treeNode
	Parent   *treeNode
}

type panel struct {
	Cnt            int
	Flags          WindowFlags
	Bounds         rect.Rect
	Offset         *image.Point
	AtX            int
	AtY            int
	MaxX           int
	Width          int
	Height         int
	FooterH        int
	HeaderH        int
	Border         int
	Clip           rect.Rect
	Menu           menuState
	Row            rowLayout
	ReservedHeight int
}

type menuState struct {
	X      int
	Y      int
	W      int
	H      int
	Offset image.Point
}

type rowLayout struct {
	Type         int
	Index        int
	Index2       int
	CalcMaxWidth bool
	Height       int
	Columns      int
	Ratio        []float64
	WidthArr     []int
	ItemWidth    int
	ItemRatio    float64
	ItemHeight   int
	ItemOffset   int
	Filled       float64
	Item         rect.Rect
	TreeDepth    int

	DynamicFreeX, DynamicFreeY, DynamicFreeW, DynamicFreeH float64
}

type WindowFlags int

const (
	WindowBorder WindowFlags = 1 << iota
	WindowBorderHeader
	WindowMovable
	WindowScalable
	WindowClosable
	WindowDynamic
	WindowNoScrollbar
	WindowNoHScrollbar
	WindowTitle
	WindowContextualReplace
	WindowNonmodal

	windowSub
	windowGroup
	windowPopup
	windowNonblock
	windowContextual
	windowCombo
	windowMenu
	windowTooltip
	windowEnabled
	windowHDynamic
	windowDocked

	WindowDefaultFlags = WindowBorder | WindowMovable | WindowScalable | WindowClosable | WindowTitle
)

func createTreeNode(initialState bool, parent *treeNode) *treeNode {
	return &treeNode{initialState, map[string]*treeNode{}, parent}
}

func createWindow(ctx *context, title string) *Window {
	rootNode := createTreeNode(false, nil)
	r := &Window{ctx: ctx, title: title, rootNode: rootNode, curNode: rootNode, groupWnd: map[string]*Window{}, first: true}
	r.editors = make(map[string]*TextEditor)
	r.rowCtor.win = r
	r.widgets.cur = make(map[rect.Rect]frozenWidget)
	return r
}

type frozenWidget struct {
	ws         nstyle.WidgetStates
	frameCount int
}

type widgetBuffer struct {
	cur        map[rect.Rect]frozenWidget
	frameCount int
}

func (wbuf *widgetBuffer) PrevState(bounds rect.Rect) nstyle.WidgetStates {
	return wbuf.cur[bounds].ws
}

func (wbuf *widgetBuffer) Add(ws nstyle.WidgetStates, bounds rect.Rect) {
	wbuf.cur[bounds] = frozenWidget{ws, wbuf.frameCount}
}

func (wbuf *widgetBuffer) reset() {
	for k, v := range wbuf.cur {
		if v.frameCount != wbuf.frameCount {
			delete(wbuf.cur, k)
		}
	}
	wbuf.frameCount++
}

func (w *Window) Master() MasterWindow {
	return w.ctx.mw
}

func (win *Window) style() *nstyle.Window {
	switch {
	case win.flags&windowCombo != 0:
		return &win.ctx.Style.ComboWindow
	case win.flags&windowContextual != 0:
		return &win.ctx.Style.ContextualWindow
	case win.flags&windowMenu != 0:
		return &win.ctx.Style.MenuWindow
	case win.flags&windowGroup != 0:
		return &win.ctx.Style.GroupWindow
	case win.flags&windowTooltip != 0:
		return &win.ctx.Style.TooltipWindow
	default:
		return &win.ctx.Style.NormalWindow
	}
}

func (win *Window) WindowStyle() *nstyle.Window {
	return win.style()
}

func panelBegin(ctx *context, win *Window, title string) {
	in := &ctx.Input
	style := &ctx.Style
	font := style.Font
	wstyle := win.style()

	// window dragging
	if win.moving {
		if in == nil || !in.Mouse.Down(mouse.ButtonLeft) {
			if win.flags&windowDocked == 0 && in != nil {
				win.ctx.DockedWindows.Dock(win, in.Mouse.Pos, win.ctx.Windows[0].Bounds, win.ctx.Style.Scaling)
			}
			win.moving = false
		} else {
			win.move(in.Mouse.Delta, in.Mouse.Pos)
		}
	}

	win.usingSub = false
	in.Mouse.clip = nk_null_rect
	layout := win.layout

	window_padding := wstyle.Padding
	item_spacing := wstyle.Spacing
	scaler_size := wstyle.ScalerSize

	*layout = panel{}

	/* panel space with border */
	if win.flags&WindowBorder != 0 {
		layout.Bounds = shrinkRect(win.Bounds, wstyle.Border)
	} else {
		layout.Bounds = win.Bounds
	}

	/* setup panel */
	layout.Border = layout.Bounds.X - win.Bounds.X

	layout.AtX = layout.Bounds.X
	layout.AtY = layout.Bounds.Y
	layout.Width = layout.Bounds.W
	layout.Height = layout.Bounds.H
	layout.MaxX = 0
	layout.Row.Index = 0
	layout.Row.Index2 = 0
	layout.Row.CalcMaxWidth = false
	layout.Row.Columns = 0
	layout.Row.Height = 0
	layout.Row.Ratio = nil
	layout.Row.ItemWidth = 0
	layout.Row.ItemRatio = 0
	layout.Row.TreeDepth = 0
	layout.Flags = win.flags
	layout.ReservedHeight = 0
	win.lastLayoutCnt = 0

	for _, cols := range win.adjust {
		for _, col := range cols {
			col.first = false
		}
	}

	/* calculate window header */
	if win.flags&windowMenu != 0 || win.flags&windowContextual != 0 {
		layout.HeaderH = window_padding.Y
		layout.Row.Height = window_padding.Y
	} else {
		layout.HeaderH = window_padding.Y
		layout.Row.Height = window_padding.Y
	}

	/* calculate window footer height */
	if win.flags&windowNonblock == 0 && (((win.flags&WindowNoScrollbar == 0) && (win.flags&WindowNoHScrollbar == 0)) || (win.flags&WindowScalable != 0)) {
		layout.FooterH = scaler_size.Y + wstyle.FooterPadding.Y
	} else {
		layout.FooterH = 0
	}

	/* calculate the window size */
	if win.flags&WindowNoScrollbar == 0 {
		layout.Width = layout.Bounds.W - wstyle.ScrollbarSize.X
	}
	layout.Height = layout.Bounds.H - (layout.HeaderH + window_padding.Y)
	layout.Height -= layout.FooterH

	/* window header */

	var dwh drawableWindowHeader
	dwh.Dynamic = layout.Flags&WindowDynamic != 0
	dwh.Bounds = layout.Bounds
	dwh.HeaderActive = (win.idx != 0) && (win.flags&WindowTitle != 0)
	dwh.LayoutWidth = layout.Width
	dwh.Style = win.style()

	var closeButton rect.Rect

	if dwh.HeaderActive {
		/* calculate header bounds */
		dwh.Header.X = layout.Bounds.X
		dwh.Header.Y = layout.Bounds.Y
		dwh.Header.W = layout.Bounds.W

		/* calculate correct header height */
		layout.HeaderH = FontHeight(font) + 2.0*wstyle.Header.Padding.Y

		layout.HeaderH += 2.0 * wstyle.Header.LabelPadding.Y
		layout.Row.Height += layout.HeaderH
		dwh.Header.H = layout.HeaderH

		/* update window height */
		layout.Height = layout.Bounds.H - (dwh.Header.H + 2*item_spacing.Y)

		layout.Height -= layout.FooterH

		dwh.Hovered = ctx.Input.Mouse.HoveringRect(dwh.Header)
		dwh.Focused = win.toplevel()

		/* window header title */
		t := FontWidth(font, title)

		dwh.Label.X = dwh.Header.X + wstyle.Header.Padding.X
		dwh.Label.X += wstyle.Header.LabelPadding.X
		dwh.Label.Y = dwh.Header.Y + wstyle.Header.LabelPadding.Y
		dwh.Label.H = FontHeight(font) + 2*wstyle.Header.LabelPadding.Y
		dwh.Label.W = t + 2*wstyle.Header.Spacing.X
		dwh.LayoutHeaderH = layout.HeaderH
		dwh.RowHeight = layout.Row.Height
		dwh.Title = title

		win.widgets.Add(nstyle.WidgetStateInactive, layout.Bounds)
		dwh.Draw(&win.ctx.Style, &win.cmds)

		// window close button
		closeButton.Y = dwh.Header.Y + wstyle.Header.Padding.Y
		closeButton.H = layout.HeaderH - 2*wstyle.Header.Padding.Y
		closeButton.W = closeButton.H
		if win.flags&WindowClosable != 0 {
			if wstyle.Header.Align == nstyle.HeaderRight {
				closeButton.X = (dwh.Header.W + dwh.Header.X) - (closeButton.W + wstyle.Header.Padding.X)
				dwh.Header.W -= closeButton.W + wstyle.Header.Spacing.X + wstyle.Header.Padding.X
			} else {
				closeButton.X = dwh.Header.X + wstyle.Header.Padding.X
				dwh.Header.X += closeButton.W + wstyle.Header.Spacing.X + wstyle.Header.Padding.X
			}

			if doButton(win, label.S(wstyle.Header.CloseSymbol), closeButton, &wstyle.Header.CloseButton, in, false) {
				win.close = true
			}
		}
	} else {
		dwh.LayoutHeaderH = layout.HeaderH
		dwh.RowHeight = layout.Row.Height
		win.widgets.Add(nstyle.WidgetStateInactive, layout.Bounds)
		dwh.Draw(&win.ctx.Style, &win.cmds)
	}

	if (win.flags&WindowMovable != 0) && win.toplevel() {
		var move rect.Rect
		move.X = win.Bounds.X
		move.Y = win.Bounds.Y
		move.W = win.Bounds.W
		move.H = FontHeight(font) + 2.0*wstyle.Header.Padding.Y + 2.0*wstyle.Header.LabelPadding.Y

		if in.Mouse.IsClickDownInRect(mouse.ButtonLeft, move, true) && !in.Mouse.IsClickDownInRect(mouse.ButtonLeft, closeButton, true) {
			win.moving = true
		}
	}

	var dwb drawableWindowBody

	dwb.NoScrollbar = win.flags&WindowNoScrollbar != 0
	dwb.Style = win.style()

	/* calculate and set the window clipping rectangle*/
	if win.flags&WindowDynamic == 0 {
		layout.Clip.X = layout.Bounds.X + window_padding.X
		layout.Clip.W = layout.Width - 2*window_padding.X
	} else {
		layout.Clip.X = layout.Bounds.X
		layout.Clip.W = layout.Width
	}

	layout.Clip.H = layout.Bounds.H - (layout.FooterH + layout.HeaderH)
	// FooterH already includes the window padding
	layout.Clip.H -= window_padding.Y
	layout.Clip.Y = layout.Bounds.Y

	/* combo box and menu do not have header space */
	if win.flags&windowCombo == 0 && win.flags&windowMenu == 0 {
		layout.Clip.Y += layout.HeaderH
	}

	clip := unify(win.cmds.Clip, layout.Clip)
	layout.Clip = clip

	dwb.Bounds = layout.Bounds
	dwb.LayoutWidth = layout.Width
	dwb.Clip = layout.Clip
	win.cmds.Clip = dwb.Clip
	win.widgets.Add(nstyle.WidgetStateInactive, dwb.Bounds)
	dwb.Draw(&win.ctx.Style, &win.cmds)

	layout.Row.Type = layoutInvalid
}

func (win *Window) specialPanelBegin() {
	win.began = true
	w := win.Master()
	ctx := w.context()
	if win.flags&windowContextual != 0 {
		prevbody := win.Bounds
		prevbody.H = win.layout.Height
		// if the contextual menu ended up with its bottom right corner outside
		// the main window's bounds and it could be moved to be inside the main
		// window by popping it a different way do it.
		// Since the size of the contextual menu is only knowable after displaying
		// it once this must be done on the second frame.
		max := ctx.Windows[0].Bounds.Max()
		if (win.header.H <= 0 || win.header.W <= 0 || win.header.Contains(prevbody.Min())) && ((prevbody.Max().X > max.X) || (prevbody.Max().Y > max.Y)) && (win.Bounds.X-prevbody.W >= 0) && (win.Bounds.Y-prevbody.H >= 0) {
			win.Bounds.X = win.Bounds.X - prevbody.W
			win.Bounds.Y = win.Bounds.Y - prevbody.H
		}
	}

	if win.flags&windowHDynamic != 0 && !win.first {
		uw := win.menuItemWidth + 2*win.style().Padding.X + 2*win.style().Border
		if uw < win.Bounds.W {
			win.Bounds.W = uw
		}
	}

	if win.flags&windowCombo != 0 && win.flags&WindowDynamic != 0 {
		prevbody := win.Bounds
		prevbody.H = win.layout.Height
		// If the combo window ends up with the right corner below the
		// main winodw's lower bound make it non-dynamic and resize it to its
		// maximum possible size that will show the whole combo box.
		max := ctx.Windows[0].Bounds.Max()
		if prevbody.Y+prevbody.H > max.Y {
			prevbody.H = max.Y - prevbody.Y
			win.Bounds = prevbody
			win.flags &= ^windowCombo
		}
	}

	if win.flags&windowNonblock != 0 && !win.first {
		/* check if user clicked outside the popup and close if so */
		in_panel := ctx.Input.Mouse.IsClickInRect(mouse.ButtonLeft, win.ctx.Windows[0].layout.Bounds)
		prevbody := win.Bounds
		prevbody.H = win.layout.Height
		in_body := ctx.Input.Mouse.IsClickInRect(mouse.ButtonLeft, prevbody)
		in_header := ctx.Input.Mouse.IsClickInRect(mouse.ButtonLeft, win.header)
		if !in_body && in_panel || in_header {
			win.close = true
		}
	}

	if win.flags&windowPopup != 0 {
		win.cmds.PushScissor(nk_null_rect)

		panelBegin(ctx, win, win.title)
		win.layout.Offset = &win.Scrollbar
	}

	if win.first && (win.flags&windowContextual != 0 || win.flags&windowHDynamic != 0) {
		ctx.trashFrame = true
	}

	win.first = false
}

var nk_null_rect = rect.Rect{-8192.0, -8192.0, 16384.0, 16384.0}

func panelEnd(ctx *context, window *Window) {
	var footer = rect.Rect{0, 0, 0, 0}

	layout := window.layout
	style := &ctx.Style
	in := &Input{}
	if window.toplevel() {
		ctx.Input.Mouse.clip = nk_null_rect
		in = &ctx.Input
	}
	outclip := nk_null_rect
	if window.flags&windowGroup != 0 {
		outclip = window.parent.cmds.Clip
	}
	window.cmds.PushScissor(outclip)

	wstyle := window.style()

	/* cache configuration data */
	item_spacing := wstyle.Spacing
	window_padding := wstyle.Padding
	scrollbar_size := wstyle.ScrollbarSize
	scaler_size := wstyle.ScalerSize

	/* update the current cursor Y-position to point over the last added widget */
	layout.AtY += layout.Row.Height

	/* draw footer and fill empty spaces inside a dynamically growing panel */
	if layout.Flags&WindowDynamic != 0 {
		layout.Height = layout.AtY - layout.Bounds.Y
		layout.Height = min(layout.Height, layout.Bounds.H)

		// fill horizontal scrollbar space
		{
			var bounds rect.Rect
			bounds.X = window.Bounds.X
			bounds.Y = layout.AtY - item_spacing.Y
			bounds.W = window.Bounds.W
			bounds.H = window.Bounds.Y + layout.Height + item_spacing.Y + window.style().Padding.Y - bounds.Y

			window.cmds.FillRect(bounds, 0, wstyle.Background)
		}

		if (layout.Offset.X == 0) || (layout.Flags&WindowNoScrollbar != 0) {
			/* special case for dynamic windows without horizontal scrollbar
			 * or hidden scrollbars */
			footer.X = window.Bounds.X
			footer.Y = window.Bounds.Y + layout.Height + item_spacing.Y + window.style().Padding.Y
			footer.W = window.Bounds.W + scrollbar_size.X
			layout.FooterH = 0
			footer.H = 0

			if (layout.Offset.X == 0) && layout.Flags&WindowNoScrollbar == 0 {
				/* special case for windows like combobox, menu require draw call
				 * to fill the empty scrollbar background */
				var bounds rect.Rect
				bounds.X = layout.Bounds.X + layout.Width
				bounds.Y = layout.Clip.Y
				bounds.W = scrollbar_size.X
				bounds.H = layout.Height

				window.cmds.FillRect(bounds, 0, wstyle.Background)
			}
		} else {
			/* dynamic window with visible scrollbars and therefore bigger footer */
			footer.X = window.Bounds.X
			footer.W = window.Bounds.W + scrollbar_size.X
			footer.H = layout.FooterH
			if (layout.Flags&windowCombo != 0) || (layout.Flags&windowMenu != 0) || (layout.Flags&windowContextual != 0) {
				footer.Y = window.Bounds.Y + layout.Height
			} else {
				footer.Y = window.Bounds.Y + layout.Height + layout.FooterH
			}
			window.cmds.FillRect(footer, 0, wstyle.Background)

			if layout.Flags&windowCombo == 0 && layout.Flags&windowMenu == 0 {
				/* fill empty scrollbar space */
				var bounds rect.Rect
				bounds.X = layout.Bounds.X
				bounds.Y = window.Bounds.Y + layout.Height
				bounds.W = layout.Bounds.W
				bounds.H = layout.Row.Height
				window.cmds.FillRect(bounds, 0, wstyle.Background)
			}
		}
	}

	/* scrollbars */
	if layout.Flags&WindowNoScrollbar == 0 {
		var bounds rect.Rect
		var scroll_target float64
		var scroll_offset float64
		var scroll_step float64
		var scroll_inc float64
		{
			/* vertical scrollbar */
			bounds.X = layout.Bounds.X + layout.Width
			bounds.Y = layout.Clip.Y
			bounds.W = scrollbar_size.Y
			bounds.H = layout.Clip.H
			if layout.Flags&WindowBorder != 0 {
				bounds.H -= 1
			}

			scroll_offset = float64(layout.Offset.Y)
			scroll_step = float64(layout.Clip.H) * 0.10
			scroll_inc = float64(layout.Clip.H) * 0.01
			scroll_target = float64(layout.AtY - layout.Clip.Y)
			scroll_offset = doScrollbarv(window, bounds, layout.Bounds, scroll_offset, scroll_target, scroll_step, scroll_inc, &ctx.Style.Scrollv, in, style.Font)
			if layout.Offset.Y != int(scroll_offset) {
				ctx.trashFrame = true
			}
			layout.Offset.Y = int(scroll_offset)
		}
		if layout.Flags&WindowNoHScrollbar == 0 {
			/* horizontal scrollbar */
			bounds.X = layout.Bounds.X + window_padding.X
			if layout.Flags&windowSub != 0 {
				bounds.H = scrollbar_size.X
				bounds.Y = layout.Bounds.Y
				if layout.Flags&WindowBorder != 0 {
					bounds.Y++
				}
				bounds.Y += layout.HeaderH + layout.Menu.H + layout.Height
				bounds.W = layout.Clip.W
			} else if layout.Flags&WindowDynamic != 0 {
				bounds.H = min(scrollbar_size.X, layout.FooterH)
				bounds.W = layout.Bounds.W
				bounds.Y = footer.Y
			} else {
				bounds.H = min(scrollbar_size.X, layout.FooterH)
				bounds.Y = layout.Bounds.Y + window.Bounds.H
				bounds.Y -= max(layout.FooterH, scrollbar_size.X)
				bounds.W = layout.Width - 2*window_padding.X
			}

			scroll_offset = float64(layout.Offset.X)
			scroll_target = float64(layout.MaxX - bounds.X)
			scroll_step = float64(layout.MaxX) * 0.05
			scroll_inc = float64(layout.MaxX) * 0.005
			scroll_offset = doScrollbarh(window, bounds, scroll_offset, scroll_target, scroll_step, scroll_inc, &ctx.Style.Scrollh, in, style.Font)
			if layout.Offset.X != int(scroll_offset) {
				ctx.trashFrame = true
			}
			layout.Offset.X = int(scroll_offset)
		}
	}

	var dsab drawableScalerAndBorders
	dsab.Style = window.style()
	dsab.Bounds = window.Bounds
	dsab.Border = layout.Border
	dsab.HeaderH = layout.HeaderH

	/* scaler */
	if layout.Flags&WindowScalable != 0 {
		dsab.DrawScaler = true

		dsab.ScalerRect.W = max(0, scaler_size.X)
		dsab.ScalerRect.H = max(0, scaler_size.Y)
		dsab.ScalerRect.X = (layout.Bounds.X + layout.Bounds.W) - (window_padding.X + dsab.ScalerRect.W)
		/* calculate scaler bounds */
		if layout.Flags&WindowDynamic != 0 {
			dsab.ScalerRect.Y = footer.Y + layout.FooterH - scaler_size.Y
		} else {
			dsab.ScalerRect.Y = layout.Bounds.Y + layout.Bounds.H - (scaler_size.Y + window_padding.Y)
		}

		/* do window scaling logic */
		if window.toplevel() {
			if window.scaling {
				if in == nil || !in.Mouse.Down(mouse.ButtonLeft) {
					window.scaling = false
				} else {
					window.scale(in.Mouse.Delta)
				}
			} else if in != nil && in.Mouse.IsClickDownInRect(mouse.ButtonLeft, dsab.ScalerRect, true) {
				window.scaling = true
			}
		}
	}

	/* window border */
	if layout.Flags&WindowBorder != 0 {
		dsab.DrawBorders = true

		if layout.Flags&WindowDynamic != 0 {
			dsab.PaddingY = layout.FooterH + footer.Y
		} else {
			dsab.PaddingY = layout.Bounds.Y + layout.Bounds.H
		}
		/* select correct border color */
		dsab.BorderColor = wstyle.BorderColor

		/* draw border between header and window body */
		if window.flags&WindowBorderHeader != 0 {
			dsab.DrawHeaderBorder = true
		}
	}

	window.widgets.Add(nstyle.WidgetStateInactive, dsab.Bounds)
	dsab.Draw(&window.ctx.Style, &window.cmds)

	layout.Flags |= windowEnabled
	window.flags = layout.Flags

	/* helper to make sure you have a 'nk_tree_push'
	 * for every 'nk_tree_pop' */
	if layout.Row.TreeDepth != 0 {
		panic("Some TreePush not closed by TreePop")
	}
}

// MenubarBegin adds a menubar to the current window.
// A menubar is an area displayed at the top of the window that is unaffected by scrolling.
// Remember to call MenubarEnd when you are done adding elements to the menubar.
func (win *Window) MenubarBegin() {
	layout := win.layout

	layout.Menu.X = layout.AtX
	layout.Menu.Y = layout.Bounds.Y + layout.HeaderH
	layout.Menu.W = layout.Width
	layout.Menu.Offset = *layout.Offset
	layout.Offset.Y = 0
}

func (win *Window) move(delta image.Point, pos image.Point) {
	if win.flags&windowDocked != 0 {
		if delta.X != 0 && delta.Y != 0 {
			win.ctx.DockedWindows.Undock(win)
		}
		return
	}
	if canDock, bounds := win.ctx.DockedWindows.Dock(nil, pos, win.ctx.Windows[0].Bounds, win.ctx.Style.Scaling); canDock {
		win.ctx.finalCmds.FillRect(bounds, 0, color.RGBA{0x0, 0x0, 0x50, 0x50})
	}
	win.Bounds.X = win.Bounds.X + delta.X
	win.Bounds.X = clampInt(0, win.Bounds.X, win.ctx.Windows[0].Bounds.X+win.ctx.Windows[0].Bounds.W-FontHeight(win.ctx.Style.Font))
	win.Bounds.Y = win.Bounds.Y + delta.Y
	win.Bounds.Y = clampInt(0, win.Bounds.Y, win.ctx.Windows[0].Bounds.Y+win.ctx.Windows[0].Bounds.H-FontHeight(win.ctx.Style.Font))
}

func (win *Window) scale(delta image.Point) {
	if win.flags&windowDocked != 0 {
		win.ctx.DockedWindows.Scale(win, delta, win.ctx.Style.Scaling)
		return
	}
	window_size := win.style().MinSize
	win.Bounds.W = max(window_size.X, win.Bounds.W+delta.X)

	/* dragging in y-direction is only possible if static window */
	if win.layout.Flags&WindowDynamic == 0 {
		win.Bounds.H = max(window_size.Y, win.Bounds.H+delta.Y)
	}
}

// MenubarEnd signals that all widgets have been added to the menubar.
func (win *Window) MenubarEnd() {
	layout := win.layout

	layout.Menu.H = layout.AtY - layout.Menu.Y
	layout.Clip.Y = layout.Bounds.Y + layout.HeaderH + layout.Menu.H + layout.Row.Height
	layout.Height -= layout.Menu.H
	*layout.Offset = layout.Menu.Offset
	layout.Clip.H -= layout.Menu.H + layout.Row.Height
	layout.AtY = layout.Menu.Y + layout.Menu.H
	win.cmds.PushScissor(layout.Clip)
}

func (win *Window) widget() (valid bool, bounds rect.Rect, calcFittingWidth FittingWidthFn) {
	/* allocate space  and check if the widget needs to be updated and drawn */
	calcFittingWidth = panelAllocSpace(&bounds, win)

	if !win.layout.Clip.Intersect(&bounds) {
		return false, bounds, calcFittingWidth
	}

	return (bounds.W > 0 && bounds.H > 0), bounds, calcFittingWidth
}

func (win *Window) widgetFitting(item_padding image.Point) (valid bool, bounds rect.Rect) {
	/* update the bounds to stand without padding  */
	style := win.style()
	layout := win.layout
	valid, bounds, _ = win.widget()
	if layout.Row.Index == 1 {
		bounds.W += style.Padding.X
		bounds.X -= style.Padding.X
	} else {
		bounds.X -= item_padding.X
	}

	if layout.Row.Columns > 0 && layout.Row.Index == layout.Row.Columns {
		bounds.W += style.Padding.X
	} else {
		bounds.W += item_padding.X
	}
	return valid, bounds
}

func panelAllocSpace(bounds *rect.Rect, win *Window) FittingWidthFn {
	if win.usingSub {
		panic(UsingSubErr)
	}
	/* check if the end of the row has been hit and begin new row if so */
	layout := win.layout
	if layout.Row.Columns > 0 && layout.Row.Index >= layout.Row.Columns {
		panelAllocRow(win)
	}

	/* calculate widget position and size */
	layoutWidgetSpace(bounds, win.ctx, win, true)

	win.LastWidgetBounds = *bounds

	layout.Row.Index++

	if win.layout.Row.CalcMaxWidth {
		col := win.adjust[win.layout.Cnt][win.layout.Row.Index2-1]
		return func(width int) {
			if width > col.width {
				col.width = width
			}
		}
	}
	return nil
}

func panelAllocRow(win *Window) {
	layout := win.layout
	spacing := win.style().Spacing
	row_height := layout.Row.Height - spacing.Y
	panelLayout(win.ctx, win, row_height, layout.Row.Columns, 0)
}

func panelLayout(ctx *context, win *Window, height int, cols int, cnt int) {
	/* prefetch some configuration data */
	layout := win.layout

	style := win.style()
	item_spacing := style.Spacing

	if height == 0 {
		height = layout.Height - (layout.AtY - layout.Bounds.Y) - 1
		if layout.Row.Index != 0 && (win.flags&windowPopup == 0) {
			height -= layout.Row.Height
		} else {
			height -= item_spacing.Y
		}
		if layout.ReservedHeight > 0 {
			height -= layout.ReservedHeight
		}
	}

	/* update the current row and set the current row layout */
	layout.Cnt = cnt
	layout.Row.Index = 0
	layout.Row.Index2 = 0
	layout.Row.CalcMaxWidth = false

	layout.AtY += layout.Row.Height
	layout.Row.Columns = cols
	layout.Row.Height = height + item_spacing.Y
	layout.Row.ItemOffset = 0
	if layout.Flags&WindowDynamic != 0 {
		win.cmds.FillRect(rect.Rect{layout.Bounds.X, layout.AtY, layout.Bounds.W, height + item_spacing.Y}, 0, style.Background)
	}
}

const (
	layoutDynamicFixed = iota
	layoutDynamicFree
	layoutDynamic
	layoutStaticFree
	layoutStatic
	layoutInvalid
)

var InvalidLayoutErr = errors.New("invalid layout")
var UsingSubErr = errors.New("parent window used while populating a sub window")

func layoutWidgetSpace(bounds *rect.Rect, ctx *context, win *Window, modify bool) {
	layout := win.layout

	/* cache some configuration data */
	style := win.style()
	spacing := style.Spacing
	padding := style.Padding

	/* calculate the usable panel space */
	panel_padding := 2 * padding.X

	panel_spacing := int(float64(layout.Row.Columns-1) * float64(spacing.X))
	panel_space := layout.Width - panel_padding - panel_spacing

	/* calculate the width of one item inside the current layout space */
	item_offset := 0
	item_width := 0
	item_spacing := 0

	switch layout.Row.Type {
	case layoutInvalid:
		panic(InvalidLayoutErr)
	case layoutDynamicFixed:
		/* scaling fixed size widgets item width */
		item_width = int(float64(panel_space) / float64(layout.Row.Columns))

		item_offset = layout.Row.Index * item_width
		item_spacing = layout.Row.Index * spacing.X
	case layoutDynamicFree:
		/* panel width depended free widget placing */
		bounds.X = layout.AtX + int(float64(layout.Width)*layout.Row.DynamicFreeX)
		bounds.X -= layout.Offset.X
		bounds.Y = layout.AtY + int(float64(layout.Row.Height)*layout.Row.DynamicFreeY)
		bounds.Y -= layout.Offset.Y
		bounds.W = int(float64(layout.Width) * layout.Row.DynamicFreeW)
		bounds.H = int(float64(layout.Row.Height) * layout.Row.DynamicFreeH)
		return
	case layoutDynamic:
		/* scaling arrays of panel width ratios for every widget */
		var ratio float64
		if layout.Row.Ratio[layout.Row.Index] < 0 {
			ratio = layout.Row.ItemRatio
		} else {
			ratio = layout.Row.Ratio[layout.Row.Index]
		}

		item_spacing = layout.Row.Index * spacing.X
		item_width = int(ratio * float64(panel_space))
		item_offset = layout.Row.ItemOffset
		if modify {
			layout.Row.ItemOffset += item_width
			layout.Row.Filled += ratio
		}
	case layoutStaticFree:
		/* free widget placing */
		atx, aty := layout.AtX, layout.AtY
		if atx < layout.Clip.X {
			atx = layout.Clip.X
		}
		if aty < layout.Clip.Y {
			aty = layout.Clip.Y
		}

		bounds.X = atx + layout.Row.Item.X

		bounds.W = layout.Row.Item.W
		if ((bounds.X + bounds.W) > layout.MaxX) && modify {
			layout.MaxX = (bounds.X + bounds.W)
		}
		bounds.X -= layout.Offset.X
		bounds.Y = aty + layout.Row.Item.Y
		bounds.Y -= layout.Offset.Y
		bounds.H = layout.Row.Item.H
		return
	case layoutStatic:
		/* non-scaling array of panel pixel width for every widget */
		item_spacing = layout.Row.Index * spacing.X

		if len(layout.Row.WidthArr) > 0 {
			item_width = layout.Row.WidthArr[layout.Row.Index]
		} else {
			item_width = layout.Row.ItemWidth
		}
		item_offset = layout.Row.ItemOffset
		if modify {
			layout.Row.ItemOffset += item_width
		}

	default:
		panic("internal error unknown layout")
	}

	/* set the bounds of the newly allocated widget */
	bounds.W = item_width

	bounds.H = layout.Row.Height - spacing.Y
	bounds.Y = layout.AtY - layout.Offset.Y
	bounds.X = layout.AtX + item_offset + item_spacing + padding.X
	if ((bounds.X + bounds.W) > layout.MaxX) && modify {
		layout.MaxX = bounds.X + bounds.W
	}
	bounds.X -= layout.Offset.X
}

func (ctx *context) scale(x int) int {
	return int(float64(x) * ctx.Style.Scaling)
}

func rowLayoutCtr(win *Window, height int, cols int, width int) {
	/* update the current row and set the current row layout */
	panelLayout(win.ctx, win, height, cols, 0)
	win.layout.Row.Type = layoutDynamicFixed

	win.layout.Row.ItemWidth = width
	win.layout.Row.ItemRatio = 0.0
	win.layout.Row.Ratio = nil
	win.layout.Row.ItemOffset = 0
	win.layout.Row.Filled = 0
}

// Reserves space for num rows of the specified height at the bottom
// of the panel.
// If a row of height == 0  is inserted it will take reserved space
// into account.
func (win *Window) LayoutReserveRow(height int, num int) {
	win.LayoutReserveRowScaled(win.ctx.scale(height), num)
}

// Like LayoutReserveRow but with a scaled height.
func (win *Window) LayoutReserveRowScaled(height int, num int) {
	win.layout.ReservedHeight += height*num + win.style().Spacing.Y*num
}

// Changes row layout and starts a new row.
// Use the returned value to configure the new row layout:
//  win.Row(10).Static(100, 120, 100)
// If height == 0 all the row is stretched to fill all the remaining space.
func (win *Window) Row(height int) *rowConstructor {
	win.rowCtor.height = win.ctx.scale(height)
	return &win.rowCtor
}

// Same as Row but with scaled units.
func (win *Window) RowScaled(height int) *rowConstructor {
	win.rowCtor.height = height
	return &win.rowCtor
}

type rowConstructor struct {
	win    *Window
	height int
}

// Starts new row that has cols columns of equal width that automatically
// resize to fill the available space.
func (ctr *rowConstructor) Dynamic(cols int) {
	rowLayoutCtr(ctr.win, ctr.height, cols, 0)
}

// Starts new row with a fixed number of columns of width proportional
// to the size of the window.
func (ctr *rowConstructor) Ratio(ratio ...float64) {
	layout := ctr.win.layout
	panelLayout(ctr.win.ctx, ctr.win, ctr.height, len(ratio), 0)

	/* calculate width of undefined widget ratios */
	r := 0.0
	n_undef := 0
	layout.Row.Ratio = ratio
	for i := range ratio {
		if ratio[i] < 0.0 {
			n_undef++
		} else {
			r += ratio[i]
		}
	}

	r = saturateFloat(1.0 - r)
	layout.Row.Type = layoutDynamic
	layout.Row.ItemWidth = 0
	layout.Row.ItemRatio = 0.0
	if r > 0 && n_undef > 0 {
		layout.Row.ItemRatio = (r / float64(n_undef))
	}

	layout.Row.ItemOffset = 0
	layout.Row.Filled = 0

}

// Starts new row with a fixed number of columns with the specfieid widths.
// If no widths are specified the row will never autowrap
// and the width of the next widget can be specified using
// LayoutSetWidth/LayoutSetWidthScaled/LayoutFitWidth.
func (ctr *rowConstructor) Static(width ...int) {
	for i := range width {
		width[i] = ctr.win.ctx.scale(width[i])
	}

	ctr.StaticScaled(width...)
}

func (win *Window) staticZeros(width []int) {
	layout := win.layout

	nzero := 0
	used := 0
	for i := range width {
		if width[i] == 0 {
			nzero++
		}
		used += width[i]
	}

	if nzero > 0 {
		style := win.style()
		spacing := style.Spacing
		padding := style.Padding
		panel_padding := 2 * padding.X
		panel_spacing := int(float64(len(width)-1) * float64(spacing.X))
		panel_space := layout.Width - panel_padding - panel_spacing

		unused := panel_space - used

		zerowidth := unused / nzero

		for i := range width {
			if width[i] == 0 {
				width[i] = zerowidth
			}
		}
	}
}

// Like Static but with scaled sizes.
func (ctr *rowConstructor) StaticScaled(width ...int) {
	layout := ctr.win.layout

	cnt := 0

	if len(width) == 0 {
		if len(layout.Row.WidthArr) == 0 {
			cnt = layout.Cnt
		} else {
			cnt = ctr.win.lastLayoutCnt + 1
			ctr.win.lastLayoutCnt = cnt
		}
	}

	panelLayout(ctr.win.ctx, ctr.win, ctr.height, len(width), cnt)

	ctr.win.staticZeros(width)

	layout.Row.WidthArr = width
	layout.Row.Type = layoutStatic
	layout.Row.ItemWidth = 0
	layout.Row.ItemRatio = 0.0
	layout.Row.ItemOffset = 0
	layout.Row.Filled = 0
}

// Reset static row
func (win *Window) LayoutResetStatic(width ...int) {
	for i := range width {
		width[i] = win.ctx.scale(width[i])
	}
	win.LayoutResetStaticScaled(width...)
}

func (win *Window) LayoutResetStaticScaled(width ...int) {
	layout := win.layout
	if layout.Row.Type != layoutStatic {
		panic(WrongLayoutErr)
	}
	win.staticZeros(width)
	layout.Row.Index = 0
	layout.Row.Index2 = 0
	layout.Row.CalcMaxWidth = false
	layout.Row.Columns = len(width)
	layout.Row.WidthArr = width
	layout.Row.ItemWidth = 0
	layout.Row.ItemRatio = 0.0
	layout.Row.ItemOffset = 0
	layout.Row.Filled = 0
}

// Starts new row that will contain widget_count widgets.
// The size and position of widgets inside this row will be specified
// by callling LayoutSpacePush/LayoutSpacePushScaled.
func (ctr *rowConstructor) SpaceBegin(widget_count int) (bounds rect.Rect) {
	layout := ctr.win.layout
	panelLayout(ctr.win.ctx, ctr.win, ctr.height, widget_count, 0)
	layout.Row.Type = layoutStaticFree

	layout.Row.Ratio = nil
	layout.Row.ItemWidth = 0
	layout.Row.ItemRatio = 0.0
	layout.Row.ItemOffset = 0
	layout.Row.Filled = 0

	style := ctr.win.style()
	spacing := style.Spacing
	padding := style.Padding

	bounds.W = layout.Width - 2*padding.X
	bounds.H = layout.Row.Height - spacing.Y

	return bounds
}

// Starts new row that will contain widget_count widgets.
// The size and position of widgets inside this row will be specified
// by callling LayoutSpacePushRatio.
func (ctr *rowConstructor) SpaceBeginRatio(widget_count int) {
	layout := ctr.win.layout
	panelLayout(ctr.win.ctx, ctr.win, ctr.height, widget_count, 0)
	layout.Row.Type = layoutDynamicFree

	layout.Row.Ratio = nil
	layout.Row.ItemWidth = 0
	layout.Row.ItemRatio = 0.0
	layout.Row.ItemOffset = 0
	layout.Row.Filled = 0
}

// LayoutSetWidth adds a new column with the specified width to a static
// layout.
func (win *Window) LayoutSetWidth(width int) {
	layout := win.layout
	if layout.Row.Type != layoutStatic || len(layout.Row.WidthArr) > 0 {
		panic(WrongLayoutErr)
	}
	layout.Row.Index2++
	layout.Row.CalcMaxWidth = false
	layout.Row.ItemWidth = win.ctx.scale(width)
}

// LayoutSetWidthScaled adds a new column width the specified scaled width
// to a static layout.
func (win *Window) LayoutSetWidthScaled(width int) {
	layout := win.layout
	if layout.Row.Type != layoutStatic || len(layout.Row.WidthArr) > 0 {
		panic(WrongLayoutErr)
	}
	layout.Row.Index2++
	layout.Row.CalcMaxWidth = false
	layout.Row.ItemWidth = width
}

// LayoutFitWidth adds a new column to a static layout.
// The width of the column will be large enough to fit the largest widget
// exactly. The largest widget will only be calculated once per id, if the
// dataset changes the id should change.
func (win *Window) LayoutFitWidth(id int, minwidth int) {
	layout := win.layout
	if layout.Row.Type != layoutStatic || len(layout.Row.WidthArr) > 0 {
		panic(WrongLayoutErr)
	}
	if win.adjust == nil {
		win.adjust = make(map[int]map[int]*adjustCol)
	}
	adjust, ok := win.adjust[layout.Cnt]
	if !ok {
		adjust = make(map[int]*adjustCol)
		win.adjust[layout.Cnt] = adjust
	}
	col, ok := adjust[layout.Row.Index2]
	if !ok || col.id != id || col.font != win.ctx.Style.Font {
		if !ok {
			col = &adjustCol{id: id, width: minwidth}
			win.adjust[layout.Cnt][layout.Row.Index2] = col
		}
		col.id = id
		col.font = win.ctx.Style.Font
		col.width = minwidth
		col.first = true
		win.ctx.trashFrame = true
		win.LayoutSetWidth(minwidth)
		layout.Row.CalcMaxWidth = true
		return
	}
	win.LayoutSetWidthScaled(col.width)
	layout.Row.CalcMaxWidth = col.first
}

var WrongLayoutErr = errors.New("Command not available with current layout")

// Sets position and size of the next widgets in a Space row layout
func (win *Window) LayoutSpacePush(rect rect.Rect) {
	if win.layout.Row.Type != layoutStaticFree {
		panic(WrongLayoutErr)
	}
	rect.X = win.ctx.scale(rect.X)
	rect.Y = win.ctx.scale(rect.Y)
	rect.W = win.ctx.scale(rect.W)
	rect.H = win.ctx.scale(rect.H)
	win.layout.Row.Item = rect
}

// Like LayoutSpacePush but with scaled units
func (win *Window) LayoutSpacePushScaled(rect rect.Rect) {
	if win.layout.Row.Type != layoutStaticFree {
		panic(WrongLayoutErr)
	}
	win.layout.Row.Item = rect
}

// Sets position and size of the next widgets in a Space row layout
func (win *Window) LayoutSpacePushRatio(x, y, w, h float64) {
	if win.layout.Row.Type != layoutDynamicFree {
		panic(WrongLayoutErr)
	}
	win.layout.Row.DynamicFreeX, win.layout.Row.DynamicFreeY, win.layout.Row.DynamicFreeH, win.layout.Row.DynamicFreeW = x, y, w, h
}

func (win *Window) layoutPeek(bounds *rect.Rect) {
	layout := win.layout
	y := layout.AtY
	off := layout.Row.ItemOffset
	index := layout.Row.Index
	if layout.Row.Columns > 0 && layout.Row.Index >= layout.Row.Columns {
		layout.AtY += layout.Row.Height
		layout.Row.ItemOffset = 0
		layout.Row.Index = 0
		layout.Row.Index2 = 0
	}

	layoutWidgetSpace(bounds, win.ctx, win, false)
	layout.AtY = y
	layout.Row.ItemOffset = off
	layout.Row.Index = index
}

// Returns the position and size of the next widget that will be
// added to the current row.
// Note that the return value is in scaled units.
func (win *Window) WidgetBounds() rect.Rect {
	var bounds rect.Rect
	win.layoutPeek(&bounds)
	return bounds
}

// Returns remaining available height of win in scaled units.
func (win *Window) LayoutAvailableHeight() int {
	return win.layout.Clip.H - (win.layout.AtY - win.layout.Bounds.Y) - win.style().Spacing.Y - win.layout.Row.Height
}

func (win *Window) LayoutAvailableWidth() int {
	switch win.layout.Row.Type {
	case layoutDynamicFree, layoutStaticFree:
		return win.layout.Clip.W
	default:
		style := win.style()
		panel_spacing := int(float64(win.layout.Row.Columns-1) * float64(style.Spacing.X))
		return win.layout.Width - style.Padding.X*2 - panel_spacing - win.layout.AtX
	}
}

// Will return (false, false) if the last widget is visible, (true,
// false) if it is above the visible area, (false, true) if it is
// below the visible area
func (win *Window) Invisible(slop int) (above, below bool) {
	return (win.LastWidgetBounds.Y - slop) < win.layout.Clip.Y, (win.LastWidgetBounds.Y + win.LastWidgetBounds.H + slop) > (win.layout.Clip.Y + win.layout.Clip.H)
}

func (win *Window) At() image.Point {
	return image.Point{win.layout.AtX - win.layout.Clip.X, win.layout.AtY - win.layout.Clip.Y}
}

///////////////////////////////////////////////////////////////////////////////////
// TREE
///////////////////////////////////////////////////////////////////////////////////

type TreeType int

const (
	TreeNode TreeType = iota
	TreeTab
)

func (win *Window) TreePush(type_ TreeType, title string, initialOpen bool) bool {
	return win.TreePushNamed(type_, title, title, initialOpen)
}

// Creates a new collapsable section inside win. Returns true
// when the section is open. Widgets that are inside this collapsable
// section should be added to win only when this function returns true.
// Once you are done adding elements to the collapsable section
// call TreePop.
// Initial_open will determine whether this collapsable section
// will be initially open.
// Type_ will determine the style of this collapsable section.
func (win *Window) TreePushNamed(type_ TreeType, name, title string, initial_open bool) bool {
	labelBounds, _, ok := win.TreePushCustom(type_, name, initial_open)

	style := win.style()
	z := &win.ctx.Style
	out := &win.cmds

	var text textWidget
	if type_ == TreeTab {
		var background *nstyle.Item = &z.Tab.Background
		if background.Type == nstyle.ItemImage {
			text.Background = color.RGBA{0, 0, 0, 0}
		} else {
			text.Background = background.Data.Color
		}
	} else {
		text.Background = style.Background
	}

	text.Text = z.Tab.Text
	widgetText(out, labelBounds, title, &text, "LC", z.Font)

	return ok
}

func (win *Window) TreePushCustom(type_ TreeType, name string, initial_open bool) (bounds rect.Rect, out *command.Buffer, ok bool) {
	/* cache some data */
	layout := win.layout
	style := &win.ctx.Style
	panel_padding := win.style().Padding

	if type_ == TreeTab {
		/* calculate header bounds and draw background */
		panelLayout(win.ctx, win, FontHeight(style.Font)+2*style.Tab.Padding.Y, 1, 0)
		win.layout.Row.Type = layoutDynamicFixed
		win.layout.Row.ItemWidth = 0
		win.layout.Row.ItemRatio = 0.0
		win.layout.Row.Ratio = nil
		win.layout.Row.ItemOffset = 0
		win.layout.Row.Filled = 0
	}

	widget_state, header, _ := win.widget()

	/* find or create tab persistent state (open/closed) */

	node := win.curNode.Children[name]
	if node == nil {
		node = createTreeNode(initial_open, win.curNode)
		win.curNode.Children[name] = node
	}

	/* update node state */
	in := &Input{}
	if win.toplevel() {
		if widget_state {
			in = &win.ctx.Input
			in.Mouse.clip = win.cmds.Clip
		}
	}

	ws := win.widgets.PrevState(header)
	if buttonBehaviorDo(&ws, header, in, false) {
		node.Open = !node.Open
	}

	/* calculate the triangle bounds */
	var sym rect.Rect
	sym.H = FontHeight(style.Font)
	sym.W = sym.H
	sym.Y = header.Y + style.Tab.Padding.Y
	sym.X = header.X + panel_padding.X + style.Tab.Padding.X

	win.widgets.Add(ws, header)
	labelBounds := drawTreeNode(win, win.style(), type_, header, sym)

	/* calculate the triangle points and draw triangle */
	symbolType := style.Tab.SymMaximize
	if node.Open {
		symbolType = style.Tab.SymMinimize
	}
	styleButton := &style.Tab.NodeButton
	if type_ == TreeTab {
		styleButton = &style.Tab.TabButton
	}
	doButton(win, label.S(symbolType), sym, styleButton, in, false)

	out = &win.cmds
	if !widget_state {
		out = nil
	}

	/* increase x-axis cursor widget position pointer */
	if node.Open {
		layout.AtX = header.X + layout.Offset.X + style.Tab.Indent
		layout.Width = max(layout.Width, 2*panel_padding.X)
		layout.Width -= (style.Tab.Indent + panel_padding.X)
		layout.Row.TreeDepth++
		win.curNode = node
		return labelBounds, out, true
	} else {
		return labelBounds, out, false
	}
}

func (win *Window) treeOpenClose(open bool, path []string) {
	node := win.curNode
	for i := range path {
		var ok bool
		node, ok = node.Children[path[i]]
		if !ok {
			return
		}
	}
	if node != nil {
		node.Open = open
	}
}

// Opens the collapsable section specified by path
func (win *Window) TreeOpen(path ...string) {
	win.treeOpenClose(true, path)
}

// Closes the collapsable section specified by path
func (win *Window) TreeClose(path ...string) {
	win.treeOpenClose(false, path)
}

// Returns true if the specified node is open
func (win *Window) TreeIsOpen(name string) bool {
	node := win.curNode.Children[name]
	if node != nil {
		return node.Open
	}
	return false
}

// TreePop signals that the program is done adding elements to the
// current collapsable section.
func (win *Window) TreePop() {
	layout := win.layout
	panel_padding := win.style().Padding
	layout.AtX -= panel_padding.X + win.ctx.Style.Tab.Indent
	layout.Width += panel_padding.X + win.ctx.Style.Tab.Indent
	if layout.Row.TreeDepth == 0 {
		panic("TreePop called without opened tree nodes")
	}
	win.curNode = win.curNode.Parent
	layout.Row.TreeDepth--
}

///////////////////////////////////////////////////////////////////////////////////
// NON-INTERACTIVE WIDGETS
///////////////////////////////////////////////////////////////////////////////////

// LabelColored draws a text label with the specified background color.
func (win *Window) LabelColored(str string, alignment label.Align, color color.RGBA) {
	var bounds rect.Rect
	var text textWidget

	style := &win.ctx.Style
	fitting := panelAllocSpace(&bounds, win)
	item_padding := style.Text.Padding
	if fitting != nil {
		fitting(2*item_padding.X + FontWidth(win.ctx.Style.Font, str))
	}

	text.Padding.X = item_padding.X
	text.Padding.Y = item_padding.Y
	text.Background = win.style().Background
	text.Text = color
	win.widgets.Add(nstyle.WidgetStateInactive, bounds)
	widgetText(&win.cmds, bounds, str, &text, alignment, win.ctx.Style.Font)

}

// LabelWrapColored draws a text label with the specified background
// color autowrappping the text.
func (win *Window) LabelWrapColored(str string, color color.RGBA) {
	var bounds rect.Rect
	var text textWidget

	style := &win.ctx.Style
	panelAllocSpace(&bounds, win)
	item_padding := style.Text.Padding

	text.Padding.X = item_padding.X
	text.Padding.Y = item_padding.Y
	text.Background = win.style().Background
	text.Text = color
	win.widgets.Add(nstyle.WidgetStateInactive, bounds)
	widgetTextWrap(&win.cmds, bounds, []rune(str), &text, win.ctx.Style.Font)
}

// Label draws a text label.
func (win *Window) Label(str string, alignment label.Align) {
	win.LabelColored(str, alignment, win.ctx.Style.Text.Color)
}

// LabelWrap draws a text label, autowrapping its contents.
func (win *Window) LabelWrap(str string) {
	win.LabelWrapColored(str, win.ctx.Style.Text.Color)
}

// Image draws an image.
func (win *Window) Image(img *image.RGBA) {
	s, bounds, fitting := win.widget()
	if fitting != nil {
		fitting(img.Bounds().Dx())
	}
	if !s {
		return
	}
	win.widgets.Add(nstyle.WidgetStateInactive, bounds)
	win.cmds.DrawImage(bounds, img)
}

// Spacing adds empty space
func (win *Window) Spacing(cols int) {
	for i := 0; i < cols; i++ {
		win.widget()
	}
}

// CustomState returns the widget state of a custom widget.
func (win *Window) CustomState() nstyle.WidgetStates {
	bounds := win.WidgetBounds()
	s := true
	if !win.layout.Clip.Intersect(&bounds) {
		s = false
	}

	ws := win.widgets.PrevState(bounds)
	basicWidgetStateControl(&ws, win.inputMaybe(s), bounds)
	return ws
}

// Custom adds a custom widget.
func (win *Window) Custom(state nstyle.WidgetStates) (bounds rect.Rect, out *command.Buffer) {
	var s bool

	if s, bounds, _ = win.widget(); !s {
		return
	}
	prevstate := win.widgets.PrevState(bounds)
	exitstate := basicWidgetStateControl(&prevstate, win.inputMaybe(s), bounds)
	if state != nstyle.WidgetStateActive {
		state = exitstate
	}
	win.widgets.Add(state, bounds)
	return bounds, &win.cmds
}

func (win *Window) Commands() *command.Buffer {
	return &win.cmds
}

///////////////////////////////////////////////////////////////////////////////////
// BUTTON
///////////////////////////////////////////////////////////////////////////////////

func buttonBehaviorDo(state *nstyle.WidgetStates, r rect.Rect, i *Input, repeat bool) (ret bool) {
	exitstate := basicWidgetStateControl(state, i, r)

	if *state == nstyle.WidgetStateActive {
		if exitstate == nstyle.WidgetStateHovered {
			if repeat {
				ret = i.Mouse.Down(mouse.ButtonLeft)
			} else {
				ret = i.Mouse.Released(mouse.ButtonLeft)
			}
		}
		if !i.Mouse.Down(mouse.ButtonLeft) {
			*state = exitstate
		}
	}

	return ret
}

func symbolWidth(sym label.SymbolType, font font.Face) int {
	switch sym {
	case label.SymbolX:
		return FontWidth(font, "x")
	case label.SymbolUnderscore:
		return FontWidth(font, "_")
	case label.SymbolPlus:
		return FontWidth(font, "+")
	case label.SymbolMinus:
		return FontWidth(font, "-")
	default:
		return FontWidth(font, "M")
	}
}

func buttonWidth(lbl label.Label, style *nstyle.Button, font font.Face) int {
	w := 2*style.Padding.X + 2*style.TouchPadding.X + 2*style.Border
	switch lbl.Kind {
	case label.TextLabel:
		w += FontWidth(font, lbl.Text)
	case label.SymbolLabel:
		w += symbolWidth(lbl.Symbol, font)
	case label.ImageLabel:
		w += lbl.Img.Bounds().Dx() + 2*style.ImagePadding.X
	case label.SymbolTextLabel:
		w += FontWidth(font, lbl.Text) + symbolWidth(lbl.Symbol, font) + 2*style.Padding.X
	case label.ImageTextLabel:
	}
	return w
}

func doButton(win *Window, lbl label.Label, r rect.Rect, style *nstyle.Button, in *Input, repeat bool) bool {
	out := win.widgets
	if lbl.Kind == label.ColorLabel {
		button := *style
		button.Normal = nstyle.MakeItemColor(lbl.Color)
		button.Hover = nstyle.MakeItemColor(lbl.Color)
		button.Active = nstyle.MakeItemColor(lbl.Color)
		button.Padding = image.Point{0, 0}
		style = &button
	}

	/* calculate button content space */
	var content rect.Rect
	content.X = r.X + style.Padding.X + style.Border
	content.Y = r.Y + style.Padding.Y + style.Border
	content.W = r.W - 2*style.Padding.X + style.Border
	content.H = r.H - 2*style.Padding.Y + style.Border

	/* execute button behavior */
	var bounds rect.Rect
	bounds.X = r.X - style.TouchPadding.X
	bounds.Y = r.Y - style.TouchPadding.Y
	bounds.W = r.W + 2*style.TouchPadding.X
	bounds.H = r.H + 2*style.TouchPadding.Y
	state := out.PrevState(bounds)
	ok := buttonBehaviorDo(&state, bounds, in, repeat)

	switch lbl.Kind {
	case label.TextLabel:
		if lbl.Align == "" {
			lbl.Align = "CC"
		}
		out.Add(state, bounds)
		drawTextButton(win, bounds, content, state, style, lbl.Text, lbl.Align)

	case label.SymbolLabel:
		out.Add(state, bounds)
		drawSymbolButton(win, bounds, content, state, style, lbl.Symbol)

	case label.ImageLabel:
		content.X += style.ImagePadding.X
		content.Y += style.ImagePadding.Y
		content.W -= 2 * style.ImagePadding.X
		content.H -= 2 * style.ImagePadding.Y

		out.Add(state, bounds)
		drawImageButton(win, bounds, content, state, style, lbl.Img)

	case label.SymbolTextLabel:
		if lbl.Align == "" {
			lbl.Align = "CC"
		}
		font := win.ctx.Style.Font
		var tri rect.Rect
		tri.Y = content.Y + (content.H / 2) - FontHeight(font)/2
		tri.W = FontHeight(font)
		tri.H = FontHeight(font)
		if lbl.Align[0] == 'L' {
			tri.X = (content.X + content.W) - (2*style.Padding.X + tri.W)
			tri.X = max(tri.X, 0)
		} else {
			tri.X = content.X + 2*style.Padding.X
		}

		out.Add(state, bounds)
		drawTextSymbolButton(win, bounds, content, tri, state, style, lbl.Text, lbl.Symbol)

	case label.ImageTextLabel:
		if lbl.Align == "" {
			lbl.Align = "CC"
		}
		var icon rect.Rect
		icon.Y = bounds.Y + style.Padding.Y
		icon.H = bounds.H - 2*style.Padding.Y
		icon.W = icon.H
		if lbl.Align[0] == 'L' {
			icon.X = (bounds.X + bounds.W) - (2*style.Padding.X + icon.W)
			icon.X = max(icon.X, 0)
		} else {
			icon.X = bounds.X + 2*style.Padding.X
		}

		icon.X += style.ImagePadding.X
		icon.Y += style.ImagePadding.Y
		icon.W -= 2 * style.ImagePadding.X
		icon.H -= 2 * style.ImagePadding.Y

		out.Add(state, bounds)
		drawTextImageButton(win, bounds, content, icon, state, style, lbl.Text, lbl.Img)

	case label.ColorLabel:
		out.Add(state, bounds)
		drawSymbolButton(win, bounds, bounds, state, style, label.SymbolNone)

	}

	return ok
}

// Button adds a button
func (win *Window) Button(lbl label.Label, repeat bool) bool {
	style := &win.ctx.Style
	state, bounds, fitting := win.widget()
	if fitting != nil {
		buttonWidth(lbl, &style.Button, style.Font)
	}
	if !state {
		return false
	}
	in := win.inputMaybe(state)
	return doButton(win, lbl, bounds, &style.Button, in, repeat)
}

func (win *Window) ButtonText(text string) bool {
	return win.Button(label.T(text), false)
}

///////////////////////////////////////////////////////////////////////////////////
// SELECTABLE
///////////////////////////////////////////////////////////////////////////////////

func selectableWidth(str string, style *nstyle.Selectable, font font.Face) int {
	return 2*style.Padding.X + 2*style.TouchPadding.X + FontWidth(font, str)
}

func doSelectable(win *Window, bounds rect.Rect, str string, align label.Align, value *bool, style *nstyle.Selectable, in *Input) bool {
	if str == "" {
		return false
	}
	old_value := *value

	/* remove padding */
	var touch rect.Rect
	touch.X = bounds.X - style.TouchPadding.X
	touch.Y = bounds.Y - style.TouchPadding.Y
	touch.W = bounds.W + style.TouchPadding.X*2
	touch.H = bounds.H + style.TouchPadding.Y*2

	/* update button */
	state := win.widgets.PrevState(bounds)
	if buttonBehaviorDo(&state, touch, in, false) {
		*value = !*value
	}

	win.widgets.Add(state, bounds)
	drawSelectable(win, state, style, *value, bounds, str, align)
	return old_value != *value
}

// SelectableLabel adds a selectable label. Value is a pointer
// to a flag that will be changed to reflect the selected state of
// this label.
// Returns true when the label is clicked.
func (win *Window) SelectableLabel(str string, align label.Align, value *bool) bool {
	style := &win.ctx.Style
	state, bounds, fitting := win.widget()
	if fitting != nil {
		fitting(selectableWidth(str, &style.Selectable, style.Font))
	}
	if !state {
		return false
	}
	in := win.inputMaybe(state)
	return doSelectable(win, bounds, str, align, value, &style.Selectable, in)
}

///////////////////////////////////////////////////////////////////////////////////
// SCROLLBARS
///////////////////////////////////////////////////////////////////////////////////

type orientation int

const (
	vertical orientation = iota
	horizontal
)

func scrollbarBehavior(state *nstyle.WidgetStates, in *Input, scroll, cursor, empty0, empty1 rect.Rect, scroll_offset float64, target float64, scroll_step float64, o orientation) float64 {
	exitstate := basicWidgetStateControl(state, in, cursor)

	if *state == nstyle.WidgetStateActive {
		if !in.Mouse.Down(mouse.ButtonLeft) {
			*state = exitstate
		} else {
			switch o {
			case vertical:
				pixel := in.Mouse.Delta.Y
				delta := (float64(pixel) / float64(scroll.H)) * target
				scroll_offset = clampFloat(0, scroll_offset+delta, target-float64(scroll.H))
			case horizontal:
				pixel := in.Mouse.Delta.X
				delta := (float64(pixel) / float64(scroll.W)) * target
				scroll_offset = clampFloat(0, scroll_offset+delta, target-float64(scroll.W))
			}
		}
	} else if in.Mouse.IsClickInRect(mouse.ButtonLeft, empty0) {
		switch o {
		case vertical:
			scroll_offset -= float64(scroll.H)
		case horizontal:
			scroll_offset -= float64(scroll.W)
		}

		if scroll_offset < 0 {
			scroll_offset = 0
		}
	} else if in.Mouse.IsClickInRect(mouse.ButtonLeft, empty1) {
		var max float64
		switch o {
		case vertical:
			scroll_offset += float64(scroll.H)
			max = target - float64(scroll.H)
		case horizontal:
			scroll_offset += float64(scroll.W)
			max = target - float64(scroll.W)
		}

		if scroll_offset > max {
			scroll_offset = max
		}
	}

	return scroll_offset
}

func scrollwheelBehavior(win *Window, scroll, scrollwheel_bounds rect.Rect, scroll_offset, target, scroll_step float64) float64 {
	in := win.scrollwheelInput()

	if ((in.Mouse.ScrollDelta < 0) || (in.Mouse.ScrollDelta > 0)) && in.Mouse.HoveringRect(scrollwheel_bounds) {
		/* update cursor by mouse scrolling */
		old_scroll_offset := scroll_offset
		scroll_offset = scroll_offset + scroll_step*float64(-in.Mouse.ScrollDelta)
		scroll_offset = clampFloat(0, scroll_offset, target-float64(scroll.H))
		used_delta := (scroll_offset - old_scroll_offset) / scroll_step
		residual := float64(in.Mouse.ScrollDelta) + used_delta
		if residual < 0 {
			in.Mouse.ScrollDelta = int(math.Ceil(residual))
		} else {
			in.Mouse.ScrollDelta = int(math.Floor(residual))
		}
	}
	return scroll_offset
}

func doScrollbarv(win *Window, scroll, scrollwheel_bounds rect.Rect, offset float64, target float64, step float64, button_pixel_inc float64, style *nstyle.Scrollbar, in *Input, font font.Face) float64 {
	var cursor rect.Rect
	var scroll_step float64
	var scroll_offset float64
	var scroll_off float64
	var scroll_ratio float64

	if scroll.W < 1 {
		scroll.W = 1
	}

	if scroll.H < 2*scroll.W {
		scroll.H = 2 * scroll.W
	}

	if target <= float64(scroll.H) {
		return 0
	}

	/* optional scrollbar buttons */
	if style.ShowButtons {
		var button rect.Rect
		button.X = scroll.X
		button.W = scroll.W
		button.H = scroll.W

		scroll_h := float64(scroll.H - 2*button.H)
		scroll_step = minFloat(step, button_pixel_inc)

		/* decrement button */
		button.Y = scroll.Y

		if doButton(win, label.S(style.DecSymbol), button, &style.DecButton, in, true) {
			offset = offset - scroll_step
		}

		/* increment button */
		button.Y = scroll.Y + scroll.H - button.H

		if doButton(win, label.S(style.IncSymbol), button, &style.IncButton, in, true) {
			offset = offset + scroll_step
		}

		scroll.Y = scroll.Y + button.H
		scroll.H = int(scroll_h)
	}

	/* calculate scrollbar constants */
	scroll_step = minFloat(step, float64(scroll.H))

	scroll_offset = clampFloat(0, offset, target-float64(scroll.H))
	scroll_ratio = float64(scroll.H) / target
	scroll_off = scroll_offset / target

	originalScroll := scroll

	/* calculate scrollbar cursor bounds */
	cursor.H = int(scroll_ratio*float64(scroll.H) - 2)
	if minh := FontHeight(font); cursor.H < minh {
		cursor.H = minh
		scroll.H -= minh
	}
	cursor.Y = scroll.Y + int(scroll_off*float64(scroll.H)) + 1
	cursor.W = scroll.W - 2
	cursor.X = scroll.X + 1

	emptyNorth := scroll
	emptyNorth.H = cursor.Y - scroll.Y

	emptySouth := scroll
	emptySouth.Y = cursor.Y + cursor.H
	emptySouth.H = (scroll.Y + scroll.H) - emptySouth.Y

	/* update scrollbar */
	out := &win.widgets
	state := out.PrevState(scroll)
	scroll_offset = scrollbarBehavior(&state, in, scroll, cursor, emptyNorth, emptySouth, scroll_offset, target, scroll_step, vertical)
	scroll_offset = scrollwheelBehavior(win, originalScroll, scrollwheel_bounds, scroll_offset, target, scroll_step)

	scroll_off = scroll_offset / target
	cursor.Y = scroll.Y + int(scroll_off*float64(scroll.H))

	out.Add(state, scroll)
	drawScrollbar(win, state, style, originalScroll, cursor)

	return scroll_offset
}

func doScrollbarh(win *Window, scroll rect.Rect, offset float64, target float64, step float64, button_pixel_inc float64, style *nstyle.Scrollbar, in *Input, font font.Face) float64 {
	var cursor rect.Rect
	var scroll_step float64
	var scroll_offset float64
	var scroll_off float64
	var scroll_ratio float64

	/* scrollbar background */
	if scroll.H < 1 {
		scroll.H = 1
	}

	if scroll.W < 2*scroll.H {
		scroll.Y = 2 * scroll.H
	}

	if target <= float64(scroll.W) {
		return 0
	}

	/* optional scrollbar buttons */
	if style.ShowButtons {
		var scroll_w float64
		var button rect.Rect
		button.Y = scroll.Y
		button.W = scroll.H
		button.H = scroll.H

		scroll_w = float64(scroll.W - 2*button.W)
		scroll_step = minFloat(step, button_pixel_inc)

		/* decrement button */
		button.X = scroll.X

		if doButton(win, label.S(style.DecSymbol), button, &style.DecButton, in, true) {
			offset = offset - scroll_step
		}

		/* increment button */
		button.X = scroll.X + scroll.W - button.W

		if doButton(win, label.S(style.IncSymbol), button, &style.IncButton, in, true) {
			offset = offset + scroll_step
		}

		scroll.X = scroll.X + button.W
		scroll.W = int(scroll_w)
	}

	/* calculate scrollbar constants */
	scroll_step = minFloat(step, float64(scroll.W))

	scroll_offset = clampFloat(0, offset, target-float64(scroll.W))
	scroll_ratio = float64(scroll.W) / target
	scroll_off = scroll_offset / target

	/* calculate cursor bounds */
	cursor.W = int(scroll_ratio*float64(scroll.W) - 2)
	cursor.X = scroll.X + int(scroll_off*float64(scroll.W)) + 1
	cursor.H = scroll.H - 2
	cursor.Y = scroll.Y + 1

	emptyWest := scroll
	emptyWest.W = cursor.X - scroll.X

	emptyEast := scroll
	emptyEast.X = cursor.X + cursor.W
	emptyEast.W = (scroll.X + scroll.W) - emptyEast.X

	/* update scrollbar */
	out := &win.widgets
	state := out.PrevState(scroll)
	scroll_offset = scrollbarBehavior(&state, in, scroll, cursor, emptyWest, emptyEast, scroll_offset, target, scroll_step, horizontal)

	scroll_off = scroll_offset / target
	cursor.X = scroll.X + int(scroll_off*float64(scroll.W))

	out.Add(state, scroll)
	drawScrollbar(win, state, style, scroll, cursor)

	return scroll_offset
}

///////////////////////////////////////////////////////////////////////////////////
// TOGGLE BOXES
///////////////////////////////////////////////////////////////////////////////////

type toggleType int

const (
	toggleCheck = toggleType(iota)
	toggleOption
)

func toggleBehavior(in *Input, b rect.Rect, state *nstyle.WidgetStates, active bool) bool {
	//TODO: rewrite using basicWidgetStateControl
	if in.Mouse.HoveringRect(b) {
		*state = nstyle.WidgetStateHovered
	} else {
		*state = nstyle.WidgetStateInactive
	}
	if *state == nstyle.WidgetStateHovered && in.Mouse.Clicked(mouse.ButtonLeft, b) {
		*state = nstyle.WidgetStateActive
		active = !active
	}

	return active
}

func toggleWidth(str string, type_ toggleType, style *nstyle.Toggle, font font.Face) int {
	w := 2*style.Padding.X + 2*style.TouchPadding.X + FontWidth(font, str)
	sw := FontHeight(font) + style.Padding.X
	w += sw
	if type_ == toggleOption {
		w += sw / 4
	} else {
		w += sw / 6
	}
	return w
}

func doToggle(win *Window, r rect.Rect, active bool, str string, type_ toggleType, style *nstyle.Toggle, in *Input, font font.Face) bool {
	var bounds rect.Rect
	var select_ rect.Rect
	var cursor rect.Rect
	var label rect.Rect
	var cursor_pad int

	r.W = max(r.W, FontHeight(font)+2*style.Padding.X)
	r.H = max(r.H, FontHeight(font)+2*style.Padding.Y)

	/* add additional touch padding for touch screen devices */
	bounds.X = r.X - style.TouchPadding.X
	bounds.Y = r.Y - style.TouchPadding.Y
	bounds.W = r.W + 2*style.TouchPadding.X
	bounds.H = r.H + 2*style.TouchPadding.Y

	/* calculate the selector space */
	select_.W = min(r.H, FontHeight(font)+style.Padding.Y)

	select_.H = select_.W
	select_.X = r.X + style.Padding.X
	select_.Y = r.Y + (r.H/2 - select_.H/2)
	if type_ == toggleOption {
		cursor_pad = select_.W / 4
	} else {
		cursor_pad = select_.H / 6
	}

	/* calculate the bounds of the cursor inside the selector */
	select_.H = max(select_.W, cursor_pad*2)

	cursor.H = select_.H - cursor_pad*2
	cursor.W = cursor.H
	cursor.X = select_.X + cursor_pad
	cursor.Y = select_.Y + cursor_pad

	/* label behind the selector */
	label.X = r.X + select_.W + style.Padding.X*2
	label.Y = select_.Y
	label.W = max(r.X+r.W, label.X+style.Padding.X)
	label.W -= (label.X + style.Padding.X)
	label.H = select_.W

	/* update selector */
	state := win.widgets.PrevState(bounds)
	active = toggleBehavior(in, bounds, &state, active)

	win.widgets.Add(state, r)
	drawTogglebox(win, type_, state, style, active, label, select_, cursor, str)

	return active
}

// OptionText adds a radio button to win. If is_active is true the
// radio button will be drawn selected. Returns true when the button
// is clicked once.
// You are responsible for ensuring that only one radio button is selected at once.
func (win *Window) OptionText(text string, is_active bool) bool {
	style := &win.ctx.Style
	state, bounds, fitting := win.widget()
	if fitting != nil {
		fitting(toggleWidth(text, toggleOption, &style.Option, style.Font))
	}
	if !state {
		return false
	}
	in := win.inputMaybe(state)
	is_active = doToggle(win, bounds, is_active, text, toggleOption, &style.Option, in, style.Font)
	return is_active
}

// CheckboxText adds a checkbox button to win. Active will contain
// the checkbox value.
// Returns true when value changes.
func (win *Window) CheckboxText(text string, active *bool) bool {
	state, bounds, fitting := win.widget()
	if fitting != nil {
		fitting(toggleWidth(text, toggleCheck, &win.ctx.Style.Checkbox, win.ctx.Style.Font))
	}
	if !state {
		return false
	}
	in := win.inputMaybe(state)
	old_active := *active
	*active = doToggle(win, bounds, *active, text, toggleCheck, &win.ctx.Style.Checkbox, in, win.ctx.Style.Font)
	return *active != old_active
}

///////////////////////////////////////////////////////////////////////////////////
// SLIDER
///////////////////////////////////////////////////////////////////////////////////

func sliderBehavior(state *nstyle.WidgetStates, cursor *rect.Rect, in *Input, style *nstyle.Slider, bounds rect.Rect, slider_min float64, slider_value float64, slider_max float64, slider_step float64, slider_steps int) float64 {
	exitstate := basicWidgetStateControl(state, in, bounds)

	if *state == nstyle.WidgetStateActive {
		if !in.Mouse.Down(mouse.ButtonLeft) {
			*state = exitstate
		} else {
			d := in.Mouse.Pos.X - (cursor.X + cursor.W/2.0)
			var pxstep float64 = float64(bounds.W-(2*style.Padding.X)) / float64(slider_steps)

			if math.Abs(float64(d)) >= pxstep {
				steps := float64(int(math.Abs(float64(d)) / pxstep))
				if d > 0 {
					slider_value += slider_step * steps
				} else {
					slider_value -= slider_step * steps
				}
				slider_value = clampFloat(slider_min, slider_value, slider_max)
			}
		}
	}

	return slider_value
}

func doSlider(win *Window, bounds rect.Rect, minval float64, val float64, maxval float64, step float64, style *nstyle.Slider, in *Input) float64 {
	var slider_range float64
	var cursor_offset float64
	var cursor rect.Rect

	/* remove padding from slider bounds */
	bounds.X = bounds.X + style.Padding.X

	bounds.Y = bounds.Y + style.Padding.Y
	bounds.H = max(bounds.H, 2*style.Padding.Y)
	bounds.W = max(bounds.W, 1+bounds.H+2*style.Padding.X)
	bounds.H -= 2 * style.Padding.Y
	bounds.W -= 2 * style.Padding.Y

	/* optional buttons */
	if style.ShowButtons {
		var button rect.Rect
		button.Y = bounds.Y
		button.W = bounds.H
		button.H = bounds.H

		/* decrement button */
		button.X = bounds.X

		if doButton(win, label.S(style.DecSymbol), button, &style.DecButton, in, false) {
			val -= step
		}

		/* increment button */
		button.X = (bounds.X + bounds.W) - button.W

		if doButton(win, label.S(style.IncSymbol), button, &style.IncButton, in, false) {
			val += step
		}

		bounds.X = bounds.X + button.W + style.Spacing.X
		bounds.W = bounds.W - (2*button.W + 2*style.Spacing.X)
	}

	/* make sure the provided values are correct */
	slider_value := clampFloat(minval, val, maxval)
	slider_range = maxval - minval
	slider_steps := int(slider_range / step)

	/* calculate slider virtual cursor bounds */
	cursor_offset = (slider_value - minval) / step

	cursor.H = bounds.H
	tempW := float64(bounds.W) / float64(slider_steps+1)
	cursor.W = int(tempW)
	cursor.X = bounds.X + int((tempW * cursor_offset))
	cursor.Y = bounds.Y

	out := &win.widgets
	state := out.PrevState(bounds)
	slider_value = sliderBehavior(&state, &cursor, in, style, bounds, minval, slider_value, maxval, step, slider_steps)
	out.Add(state, bounds)
	drawSlider(win, state, style, bounds, cursor, minval, slider_value, maxval)
	return slider_value
}

// Adds a slider with a floating point value to win.
// Returns true when the slider's value is changed.
func (win *Window) SliderFloat(min_value float64, value *float64, max_value float64, value_step float64) bool {
	style := &win.ctx.Style
	state, bounds, _ := win.widget()
	if !state {
		return false
	}
	in := win.inputMaybe(state)

	old_value := *value
	*value = doSlider(win, bounds, min_value, old_value, max_value, value_step, &style.Slider, in)
	return old_value > *value || old_value < *value
}

// Adds a slider with an integer value to win.
// Returns true when the slider's value changes.
func (win *Window) SliderInt(min int, val *int, max int, step int) bool {
	value := float64(*val)
	ret := win.SliderFloat(float64(min), &value, float64(max), float64(step))
	*val = int(value)
	return ret
}

///////////////////////////////////////////////////////////////////////////////////
// PROGRESS BAR
///////////////////////////////////////////////////////////////////////////////////

func progressBehavior(state *nstyle.WidgetStates, in *Input, r rect.Rect, maxval int, value int, modifiable bool) int {
	if !modifiable {
		*state = nstyle.WidgetStateInactive
		return value
	}

	exitstate := basicWidgetStateControl(state, in, r)

	if *state == nstyle.WidgetStateActive {
		if !in.Mouse.Down(mouse.ButtonLeft) {
			*state = exitstate
		} else {
			ratio := maxFloat(0, float64(in.Mouse.Pos.X-r.X)) / float64(r.W)
			value = int(float64(maxval) * ratio)
			if value < 0 {
				value = 0
			}
		}
	}

	if maxval > 0 && value > maxval {
		value = maxval
	}
	return value
}

func doProgress(win *Window, bounds rect.Rect, value int, maxval int, modifiable bool, style *nstyle.Progress, in *Input) int {
	var prog_scale float64
	var cursor rect.Rect

	/* calculate progressbar cursor */
	cursor = padRect(bounds, style.Padding)
	prog_scale = float64(value) / float64(maxval)
	cursor.W = int(float64(cursor.W) * prog_scale)

	/* update progressbar */
	if value > maxval {
		value = maxval
	}

	state := win.widgets.PrevState(bounds)
	value = progressBehavior(&state, in, bounds, maxval, value, modifiable)
	win.widgets.Add(state, bounds)
	drawProgress(win, state, style, bounds, cursor, value, maxval)

	return value
}

// Adds a progress bar to win. if is_modifiable is true the progress
// bar will be user modifiable through click-and-drag.
// Returns true when the progress bar values is modified.
func (win *Window) Progress(cur *int, maxval int, is_modifiable bool) bool {
	style := &win.ctx.Style
	state, bounds, _ := win.widget()
	if !state {
		return false
	}

	in := win.inputMaybe(state)
	old_value := *cur
	*cur = doProgress(win, bounds, *cur, maxval, is_modifiable, &style.Progress, in)
	return *cur != old_value
}

///////////////////////////////////////////////////////////////////////////////////
// PROPERTY
///////////////////////////////////////////////////////////////////////////////////

type FilterFunc func(c rune) bool

func FilterDefault(c rune) bool {
	return true
}

func FilterDecimal(c rune) bool {
	return !((c < '0' || c > '9') && c != '-')
}

func FilterFloat(c rune) bool {
	return !((c < '0' || c > '9') && c != '.' && c != '-')
}

func (ed *TextEditor) propertyBehavior(ws *nstyle.WidgetStates, in *Input, property rect.Rect, label rect.Rect, edit rect.Rect, empty rect.Rect) (drag bool, delta int) {
	if ed.propertyStatus == propertyDefault {
		if buttonBehaviorDo(ws, edit, in, false) {
			ed.propertyStatus = propertyEdit
		} else if in.Mouse.IsClickDownInRect(mouse.ButtonLeft, label, true) {
			ed.propertyStatus = propertyDrag
		} else if in.Mouse.IsClickDownInRect(mouse.ButtonLeft, empty, true) {
			ed.propertyStatus = propertyDrag
		}
	}

	if ed.propertyStatus == propertyDrag {
		if in.Mouse.Released(mouse.ButtonLeft) {
			ed.propertyStatus = propertyDefault
		} else {
			delta = in.Mouse.Delta.X
			drag = true
		}
	}

	if ed.propertyStatus == propertyDefault {
		if in.Mouse.HoveringRect(property) {
			*ws = nstyle.WidgetStateHovered
		} else {
			*ws = nstyle.WidgetStateInactive
		}
	} else {
		*ws = nstyle.WidgetStateActive
	}

	return
}

type doPropertyRet int

const (
	doPropertyStay = doPropertyRet(iota)
	doPropertyInc
	doPropertyDec
	doPropertyDrag
	doPropertySet
)

func digits(n int) int {
	if n <= 0 {
		n = 1
	}
	return int(math.Log2(float64(n)) + 1)
}

func propertyWidth(max int, style *nstyle.Property, font font.Face) int {
	return 2*FontHeight(font)/2 + digits(max)*FontWidth(font, "0") + 4*style.Padding.X + 2*style.Border
}

func (win *Window) doProperty(property rect.Rect, name string, text string, filter FilterFunc, in *Input) (ret doPropertyRet, delta int, ed *TextEditor) {
	ret = doPropertyStay
	style := &win.ctx.Style.Property
	font := win.ctx.Style.Font

	// left decrement button
	var left rect.Rect
	left.H = FontHeight(font) / 2
	left.W = left.H
	left.X = property.X + style.Border + style.Padding.X
	left.Y = property.Y + style.Border + property.H/2.0 - left.H/2

	// text label
	size := FontWidth(font, name)
	var lblrect rect.Rect
	lblrect.X = left.X + left.W + style.Padding.X
	lblrect.W = size + 2*style.Padding.X
	lblrect.Y = property.Y + style.Border
	lblrect.H = property.H - 2*style.Border

	/* right increment button */
	var right rect.Rect
	right.Y = left.Y
	right.W = left.W
	right.H = left.H
	right.X = property.X + property.W - (right.W + style.Padding.X)

	ws := win.widgets.PrevState(property)
	oldws := ws
	if ws == nstyle.WidgetStateActive && win.editors[name] != nil {
		ed = win.editors[name]
	} else {
		ed = &TextEditor{}
		ed.init(win)
		ed.Buffer = []rune(text)
	}

	size = FontWidth(font, string(ed.Buffer)) + FontWidth(font, "i")

	/* edit */
	var edit rect.Rect
	edit.W = size + 2*style.Padding.X
	edit.X = right.X - (edit.W + style.Padding.X)
	edit.Y = property.Y + style.Border + 1
	edit.H = property.H - (2*style.Border + 2)

	/* empty left space activator */
	var empty rect.Rect
	empty.W = edit.X - (lblrect.X + lblrect.W)
	empty.X = lblrect.X + lblrect.W
	empty.Y = property.Y
	empty.H = property.H

	/* update property */
	old := ed.propertyStatus == propertyEdit

	drag, delta := ed.propertyBehavior(&ws, in, property, lblrect, edit, empty)
	if drag {
		ret = doPropertyDrag
	}
	if ws == nstyle.WidgetStateActive {
		ed.Active = true
		win.editors[name] = ed
	} else if oldws == nstyle.WidgetStateActive {
		delete(win.editors, name)
	}
	ed.win.widgets.Add(ws, property)
	drawProperty(ed.win, style, property, lblrect, ws, name)

	/* execute right and left button  */
	if doButton(ed.win, label.S(style.SymLeft), left, &style.DecButton, in, false) {
		ret = doPropertyDec
	}
	if doButton(ed.win, label.S(style.SymRight), right, &style.IncButton, in, false) {
		ret = doPropertyInc
	}

	active := ed.propertyStatus == propertyEdit
	if !old && active {
		/* property has been activated so setup buffer */
		ed.Cursor = len(ed.Buffer)
	}
	ed.Flags = EditAlwaysInsertMode | EditNoHorizontalScroll
	ed.doEdit(edit, &style.Edit, in, false, false, false)
	active = ed.Active

	if active && in.Keyboard.Pressed(key.CodeReturnEnter) {
		active = !active
	}

	if old && !active {
		/* property is now not active so convert edit text to value*/
		ed.propertyStatus = propertyDefault
		ed.Active = false
		ret = doPropertySet
	}

	return
}

// Adds a property widget to win for floating point properties.
// A property widget will display a text label, a small text editor
// for the property value and one up and one down button.
// The value can be modified by editing the text value, by clicking
// the up/down buttons (which will increase/decrease the value by step)
// or by clicking and dragging over the label.
// Returns true when the property's value is changed
func (win *Window) PropertyFloat(name string, min float64, val *float64, max, step, inc_per_pixel float64, prec int) (changed bool) {
	s, bounds, fitting := win.widget()
	if fitting != nil {
		fitting(propertyWidth(int(max+1), &win.ctx.Style.Property, win.ctx.Style.Font))
	}
	if !s {
		return
	}
	in := win.inputMaybe(s)
	text := strconv.FormatFloat(*val, 'G', prec, 32)
	ret, delta, ed := win.doProperty(bounds, name, text, FilterFloat, in)
	switch ret {
	case doPropertyDec:
		*val -= step
	case doPropertyInc:
		*val += step
	case doPropertyDrag:
		*val += float64(delta) * inc_per_pixel
	case doPropertySet:
		*val, _ = strconv.ParseFloat(string(ed.Buffer), 64)
	}
	changed = ret != doPropertyStay
	if changed {
		*val = clampFloat(min, *val, max)
	}
	return
}

// Same as PropertyFloat but with integer values.
func (win *Window) PropertyInt(name string, min int, val *int, max, step, inc_per_pixel int) (changed bool) {
	s, bounds, fitting := win.widget()
	if fitting != nil {
		fitting(propertyWidth(max, &win.ctx.Style.Property, win.ctx.Style.Font))
	}
	if !s {
		return
	}
	in := win.inputMaybe(s)
	text := strconv.Itoa(*val)
	ret, delta, ed := win.doProperty(bounds, name, text, FilterDecimal, in)
	switch ret {
	case doPropertyDec:
		*val -= step
	case doPropertyInc:
		*val += step
	case doPropertyDrag:
		*val += delta * inc_per_pixel
	case doPropertySet:
		*val, _ = strconv.Atoi(string(ed.Buffer))
	}
	changed = ret != doPropertyStay
	if changed {
		*val = clampInt(min, *val, max)
	}
	return
}

///////////////////////////////////////////////////////////////////////////////////
// POPUP
///////////////////////////////////////////////////////////////////////////////////

func (ctx *context) nonblockOpen(flags WindowFlags, body rect.Rect, header rect.Rect, updateFn UpdateFn) *Window {
	popup := createWindow(ctx, "")
	popup.idx = len(ctx.Windows)
	popup.updateFn = updateFn
	ctx.Windows = append(ctx.Windows, popup)

	popup.Bounds = body
	popup.layout = &panel{}
	popup.flags = flags
	popup.flags |= WindowBorder | windowPopup
	popup.flags |= WindowDynamic | windowSub
	popup.flags |= windowNonblock

	popup.header = header

	if updateFn == nil {
		popup.specialPanelBegin()
	}
	return popup
}

func (ctx *context) popupOpen(title string, flags WindowFlags, rect rect.Rect, scale bool, updateFn UpdateFn) {
	popup := createWindow(ctx, title)
	popup.idx = len(ctx.Windows)
	popup.updateFn = updateFn
	if updateFn == nil {
		panic("nil update function")
	}
	ctx.Windows = append(ctx.Windows, popup)
	ctx.dockedWindowFocus = 0

	if scale {
		rect.X = ctx.scale(rect.X)
		rect.Y = ctx.scale(rect.Y)
		rect.W = ctx.scale(rect.W)
		rect.H = ctx.scale(rect.H)
	}

	if ((rect.X+rect.W <= 0) && (rect.Y+rect.H <= 0)) || ((rect.X >= ctx.Windows[0].Bounds.W) && (rect.Y >= ctx.Windows[0].Bounds.H)) {
		// out of bounds
		rect.X = 0
		rect.Y = 0
	}

	if rect.X == 0 && rect.Y == 0 && flags&WindowNonmodal != 0 {
		rect.X, rect.Y = ctx.autoPosition()
	}

	popup.Bounds = rect
	popup.layout = &panel{}
	popup.flags = flags | WindowBorder | windowSub | windowPopup
}

func (ctx *context) autoPosition() (int, int) {
	x, y := ctx.autopos.X, ctx.autopos.Y

	z := FontHeight(ctx.Style.Font) + 2.0*ctx.Style.NormalWindow.Header.Padding.Y

	ctx.autopos.X += ctx.scale(z)
	ctx.autopos.Y += ctx.scale(z)

	if ctx.Windows[0].Bounds.W != 0 && ctx.Windows[0].Bounds.H != 0 {
		if ctx.autopos.X >= ctx.Windows[0].Bounds.W || ctx.autopos.Y >= ctx.Windows[0].Bounds.H {
			ctx.autopos.X = 0
			ctx.autopos.Y = 0
		}
	}

	return x, y
}

// Programmatically closes this window
func (win *Window) Close() {
	if win.idx != 0 {
		win.close = true
	}
}

///////////////////////////////////////////////////////////////////////////////////
// CONTEXTUAL
///////////////////////////////////////////////////////////////////////////////////

// Opens a contextual menu with maximum size equal to 'size'.
// Specify size == image.Point{} if you want a menu big enough to fit its larges MenuItem
func (win *Window) ContextualOpen(flags WindowFlags, size image.Point, trigger_bounds rect.Rect, updateFn UpdateFn) *Window {
	if popup := win.ctx.Windows[len(win.ctx.Windows)-1]; popup.header == trigger_bounds {
		popup.specialPanelBegin()
		return popup
	}
	if size == (image.Point{}) {
		size.X = nk_null_rect.W
		size.Y = nk_null_rect.H
		flags = flags | windowHDynamic
	}
	size.X = win.ctx.scale(size.X)
	size.Y = win.ctx.scale(size.Y)
	if trigger_bounds.W > 0 && trigger_bounds.H > 0 {
		if !win.Input().Mouse.Clicked(mouse.ButtonRight, trigger_bounds) {
			return nil
		}
	}

	var body rect.Rect
	body.X = win.ctx.Input.Mouse.Pos.X
	body.Y = win.ctx.Input.Mouse.Pos.Y
	body.W = size.X
	body.H = size.Y

	if flags&WindowContextualReplace != 0 {
		if popup := win.ctx.Windows[len(win.ctx.Windows)-1]; popup.flags&windowContextual != 0 {
			body.X = popup.Bounds.X
			body.Y = popup.Bounds.Y
		}
	}

	atomic.AddInt32(&win.ctx.changed, 1)
	return win.ctx.nonblockOpen(flags|windowContextual|WindowNoScrollbar, body, trigger_bounds, updateFn)
}

// MenuItem adds a menu item
func (win *Window) MenuItem(lbl label.Label) bool {
	style := &win.ctx.Style
	state, bounds := win.widgetFitting(style.ContextualButton.Padding)
	if !state {
		return false
	}

	if win.flags&windowHDynamic != 0 {
		w := FontWidth(style.Font, lbl.Text) + 2*style.ContextualButton.Padding.X
		if w > win.menuItemWidth {
			win.menuItemWidth = w
		}
	}

	in := win.inputMaybe(state)
	if doButton(win, lbl, bounds, &style.ContextualButton, in, false) {
		win.Close()
		return true
	}

	return false
}

///////////////////////////////////////////////////////////////////////////////////
// TOOLTIP
///////////////////////////////////////////////////////////////////////////////////

const tooltipWindowTitle = "__##Tooltip##__"

// Displays a tooltip window.
func (win *Window) TooltipOpen(width int, scale bool, updateFn UpdateFn) {
	in := &win.ctx.Input

	if scale {
		width = win.ctx.scale(width)
	}

	var bounds rect.Rect
	bounds.W = width
	bounds.H = nk_null_rect.H
	bounds.X = (in.Mouse.Pos.X + 1)
	bounds.Y = (in.Mouse.Pos.Y + 1)

	win.ctx.popupOpen(tooltipWindowTitle, WindowDynamic|WindowNoScrollbar|windowTooltip, bounds, false, updateFn)
}

// Shows a tooltip window containing the specified text.
func (win *Window) Tooltip(text string) {
	if text == "" {
		return
	}

	/* fetch configuration data */
	padding := win.ctx.Style.TooltipWindow.Padding
	item_spacing := win.ctx.Style.TooltipWindow.Spacing

	/* calculate size of the text and tooltip */
	text_width := FontWidth(win.ctx.Style.Font, text) + win.ctx.scale(4*padding.X) + win.ctx.scale(2*item_spacing.X)
	text_height := FontHeight(win.ctx.Style.Font)

	win.TooltipOpen(text_width, false, func(tw *Window) {
		tw.RowScaled(text_height).Dynamic(1)
		tw.Label(text, "LC")
	})
}

///////////////////////////////////////////////////////////////////////////////////
// COMBO-BOX
///////////////////////////////////////////////////////////////////////////////////

// Adds a drop-down list to win.
func (win *Window) Combo(lbl label.Label, height int, updateFn UpdateFn) *Window {
	s, header, _ := win.widget()
	if !s {
		return nil
	}

	in := win.inputMaybe(s)
	state := win.widgets.PrevState(header)
	is_clicked := buttonBehaviorDo(&state, header, in, false)

	switch lbl.Kind {
	case label.ColorLabel:
		win.widgets.Add(state, header)
		drawComboColor(win, state, header, is_clicked, lbl.Color)
	case label.ImageLabel:
		win.widgets.Add(state, header)
		drawComboImage(win, state, header, is_clicked, lbl.Img)
	case label.ImageTextLabel:
		win.widgets.Add(state, header)
		drawComboImageText(win, state, header, is_clicked, lbl.Text, lbl.Img)
	case label.SymbolLabel:
		win.widgets.Add(state, header)
		drawComboSymbol(win, state, header, is_clicked, lbl.Symbol)
	case label.SymbolTextLabel:
		win.widgets.Add(state, header)
		drawComboSymbolText(win, state, header, is_clicked, lbl.Symbol, lbl.Text)
	case label.TextLabel:
		win.widgets.Add(state, header)
		drawComboText(win, state, header, is_clicked, lbl.Text)
	}

	if popup := win.ctx.Windows[len(win.ctx.Windows)-1]; updateFn == nil && popup.header == header {
		popup.specialPanelBegin()
		return popup
	}

	if !is_clicked {
		return nil
	}

	height = win.ctx.scale(height)

	var body rect.Rect
	body.X = header.X
	body.W = header.W
	body.Y = header.Y + header.H - 1
	body.H = height

	return win.ctx.nonblockOpen(windowCombo, body, header, updateFn)
}

// Adds a drop-down list to win. The contents are specified by items,
// with selected being the index of the selected item.
func (win *Window) ComboSimple(items []string, selected int, item_height int) int {
	if len(items) == 0 {
		return selected
	}

	item_height = win.ctx.scale(item_height)
	item_padding := win.ctx.Style.Combo.ButtonPadding.Y
	window_padding := win.style().Padding.Y
	max_height := (len(items)+1)*item_height + item_padding*3 + window_padding*2
	if w := win.Combo(label.T(items[selected]), max_height, nil); w != nil {
		w.RowScaled(item_height).Dynamic(1)
		for i := range items {
			if w.MenuItem(label.TA(items[i], "LC")) {
				selected = i
			}
		}
	}
	return selected
}

///////////////////////////////////////////////////////////////////////////////////
// MENU
///////////////////////////////////////////////////////////////////////////////////

// Adds a menu to win with a text label.
// If width == 0 the width will be automatically adjusted to fit the largest MenuItem
func (win *Window) Menu(lbl label.Label, width int, updateFn UpdateFn) *Window {
	state, header, fitting := win.widget()
	if fitting != nil {
		fitting(buttonWidth(lbl, &win.ctx.Style.MenuButton, win.ctx.Style.Font))
	}
	if !state {
		return nil
	}

	in := &Input{}
	if win.toplevel() {
		in = &win.ctx.Input
		in.Mouse.clip = win.cmds.Clip
	}
	is_clicked := doButton(win, lbl, header, &win.ctx.Style.MenuButton, in, false)

	if popup := win.ctx.Windows[len(win.ctx.Windows)-1]; updateFn == nil && popup.header == header {
		popup.specialPanelBegin()
		return popup
	}

	if !is_clicked {
		return nil
	}

	flags := windowMenu | WindowNoScrollbar

	width = win.ctx.scale(width)

	if width == 0 {
		width = nk_null_rect.W
		flags = flags | windowHDynamic
	}

	var body rect.Rect
	body.X = header.X
	body.W = width
	body.Y = header.Y + header.H
	body.H = (win.layout.Bounds.Y + win.layout.Bounds.H) - body.Y

	return win.ctx.nonblockOpen(flags, body, header, updateFn)
}

///////////////////////////////////////////////////////////////////////////////////
// GROUPS
///////////////////////////////////////////////////////////////////////////////////

// Creates a group of widgets.
// Group are useful for creating lists as well as splitting a main
// window into tiled subwindows.
// Items that you want to add to the group should be added to the
// returned window.
func (win *Window) GroupBegin(title string, flags WindowFlags) *Window {
	sw := win.groupWnd[title]
	if sw == nil {
		sw = createWindow(win.ctx, title)
		sw.parent = win
		win.groupWnd[title] = sw
		sw.Scrollbar.X = 0
		sw.Scrollbar.Y = 0
		sw.layout = &panel{}
	}

	sw.curNode = sw.rootNode
	sw.widgets.reset()
	sw.cmds.Reset()
	sw.idx = win.idx

	sw.cmds.Clip = win.cmds.Clip

	state, bounds, _ := win.widget()
	if !state {
		return nil
	}

	flags |= windowSub | windowGroup

	if win.flags&windowEnabled != 0 {
		flags |= windowEnabled
	}

	sw.Bounds = bounds
	sw.flags = flags

	panelBegin(win.ctx, sw, title)

	sw.layout.Offset = &sw.Scrollbar

	win.usingSub = true

	return sw
}

// Signals that you are done adding widgets to a group.
func (win *Window) GroupEnd() {
	panelEnd(win.ctx, win)
	win.parent.usingSub = false

	// immediate drawing
	win.parent.cmds.Commands = append(win.parent.cmds.Commands, win.cmds.Commands...)
}
