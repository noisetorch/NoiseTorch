package nucular

import (
	"image"
	"image/color"

	"github.com/aarzilli/nucular/command"
	"github.com/aarzilli/nucular/font"
	"github.com/aarzilli/nucular/label"
	"github.com/aarzilli/nucular/rect"
	nstyle "github.com/aarzilli/nucular/style"
)

func drawSymbol(out *command.Buffer, type_ label.SymbolType, content rect.Rect, background color.RGBA, foreground color.RGBA, border_width int, font font.Face) {
	triangleSymbol := func(heading Heading) {
		points := triangleFromDirection(content, border_width, border_width, heading)
		out.FillTriangle(points[0], points[1], points[2], foreground)
	}
	switch type_ {
	case label.SymbolX,
		label.SymbolUnderscore,
		label.SymbolPlus,
		label.SymbolMinus:
		var X string
		switch type_ {
		case label.SymbolX:
			X = "x"
		case label.SymbolUnderscore:
			X = "_"
		case label.SymbolPlus:
			X = "+"
		case label.SymbolMinus:
			X = "-"
		}

		var text textWidget
		text.Padding = image.Point{0, 0}
		text.Background = background
		text.Text = foreground
		widgetText(out, content, X, &text, "CC", font)
	case label.SymbolRect, label.SymbolRectFilled:
		out.FillRect(content, 0, foreground)
		if type_ == label.SymbolRectFilled {
			out.FillRect(shrinkRect(content, border_width), 0, background)
		}
	case label.SymbolCircle, label.SymbolCircleFilled:
		out.FillCircle(content, foreground)
		if type_ == label.SymbolCircleFilled {
			out.FillCircle(shrinkRect(content, 1), background)
		}
	case label.SymbolTriangleUp:
		triangleSymbol(Up)
	case label.SymbolTriangleDown:
		triangleSymbol(Down)
	case label.SymbolTriangleLeft:
		triangleSymbol(Left)
	case label.SymbolTriangleRight:
		triangleSymbol(Right)
	default:
		fallthrough
	case label.SymbolNone:
		break
	}
}

///////////////////////////////////////////////////////////////////////////////////
// WINOWS
///////////////////////////////////////////////////////////////////////////////////

type drawableWindowHeader struct {
	Header  rect.Rect
	Label   rect.Rect
	Hovered bool // titlebar is hovered
	Focused bool // window has focus
	Title   string

	Dynamic       bool
	HeaderActive  bool // should display titlebar
	Bounds        rect.Rect
	RowHeight     int
	LayoutWidth   int
	LayoutHeaderH int
	Style         *nstyle.Window
}

func (dwh *drawableWindowHeader) Draw(z *nstyle.Style, out *command.Buffer) {
	style := dwh.Style
	if !dwh.Dynamic {
		/* draw fixed window body */
		body := dwh.Bounds
		if dwh.HeaderActive {
			body.Y += dwh.LayoutHeaderH - 1
			body.H -= dwh.LayoutHeaderH
		}

		if style.FixedBackground.Type == nstyle.ItemImage {
			out.DrawImage(body, style.FixedBackground.Data.Image)
		} else {
			out.FillRect(body, 0, style.FixedBackground.Data.Color)
		}
	} else {
		/* draw dynamic window body */
		out.FillRect(rect.Rect{dwh.Bounds.X, dwh.Bounds.Y, dwh.Bounds.W, dwh.RowHeight + style.Padding.Y}, 0, style.Background)
	}

	if dwh.HeaderActive {
		var background *nstyle.Item
		var text textWidget

		/* select correct header background and text color */
		switch {
		case dwh.Focused:
			background = &style.Header.Active
			text.Text = style.Header.LabelActive
		case dwh.Hovered:
			background = &style.Header.Hover
			text.Text = style.Header.LabelHover
		default:
			background = &style.Header.Normal
			text.Text = style.Header.LabelNormal
		}

		/* draw header background */
		if background.Type == nstyle.ItemImage {
			text.Background = color.RGBA{0, 0, 0, 0}
			out.DrawImage(dwh.Header, background.Data.Image)
		} else {
			text.Background = background.Data.Color
			out.FillRect(dwh.Header, 0, background.Data.Color)
		}

		text.Padding = image.Point{0, 0}
		widgetText(out, dwh.Label, dwh.Title, &text, "LC", z.Font)
	}
}

type drawableWindowBody struct {
	NoScrollbar bool
	Bounds      rect.Rect
	LayoutWidth int
	Clip        rect.Rect
	Style       *nstyle.Window
}

func (dwb *drawableWindowBody) Draw(z *nstyle.Style, out *command.Buffer) {

	out.PushScissor(dwb.Clip)
	out.Clip.X = dwb.Bounds.X
	out.Clip.W = dwb.LayoutWidth
	if !dwb.NoScrollbar {
		out.Clip.W += dwb.Style.ScrollbarSize.X
	}
}

type drawableScalerAndBorders struct {
	DrawScaler bool
	ScalerRect rect.Rect

	DrawHeaderBorder bool
	DrawBorders      bool
	Bounds           rect.Rect
	Border           int
	HeaderH          int
	BorderColor      color.RGBA
	PaddingY         int
	Style            *nstyle.Window
}

func (d *drawableScalerAndBorders) Draw(z *nstyle.Style, out *command.Buffer) {
	style := d.Style
	if d.DrawScaler {
		/* draw scaler */
		if style.Scaler.Type == nstyle.ItemImage {
			out.DrawImage(d.ScalerRect, style.Scaler.Data.Image)
		} else {
			out.FillTriangle(image.Point{d.ScalerRect.X + d.ScalerRect.W, d.ScalerRect.Y}, d.ScalerRect.Max(), image.Point{d.ScalerRect.X, d.ScalerRect.Y + d.ScalerRect.H}, style.Scaler.Data.Color)
		}
	}

	if d.DrawHeaderBorder {
		out.StrokeLine(image.Point{d.Bounds.X + d.Border/2.0, d.Bounds.Y + d.HeaderH - d.Border}, image.Point{d.Bounds.X + d.Bounds.W - d.Border, d.Bounds.Y + d.HeaderH - d.Border}, d.Border, d.BorderColor)
	}

	if d.DrawBorders {
		/* draw border top */
		out.StrokeLine(image.Point{d.Bounds.X + d.Border/2.0, d.Bounds.Y + d.Border/2.0}, image.Point{d.Bounds.X + d.Bounds.W - d.Border, d.Bounds.Y + d.Border/2.0}, style.Border, d.BorderColor)

		/* draw bottom border */
		out.StrokeLine(image.Point{d.Bounds.X + d.Border/2.0, d.PaddingY - d.Border}, image.Point{d.Bounds.X + d.Bounds.W - d.Border, d.PaddingY - d.Border}, d.Border, d.BorderColor)

		/* draw left border */
		out.StrokeLine(image.Point{d.Bounds.X + d.Border/2.0, d.Bounds.Y + d.Border/2.0}, image.Point{d.Bounds.X + d.Border/2.0, d.PaddingY - d.Border}, d.Border, d.BorderColor)

		/* draw right border */
		out.StrokeLine(image.Point{d.Bounds.X + d.Bounds.W - d.Border, d.Bounds.Y + d.Border/2.0}, image.Point{d.Bounds.X + d.Bounds.W - d.Border, d.PaddingY - d.Border}, d.Border, d.BorderColor)
	}

}

///////////////////////////////////////////////////////////////////////////////////
// TREE
///////////////////////////////////////////////////////////////////////////////////

func drawTreeNode(win *Window, style *nstyle.Window, type_ TreeType, header rect.Rect, sym rect.Rect) rect.Rect {
	z := &win.ctx.Style
	out := &win.cmds

	item_spacing := style.Spacing
	panel_padding := style.Padding

	if type_ == TreeTab {
		var background *nstyle.Item = &z.Tab.Background
		if background.Type == nstyle.ItemImage {
			win.cmds.DrawImage(header, background.Data.Image)
		} else {
			out.FillRect(header, 0, z.Tab.BorderColor)
			out.FillRect(shrinkRect(header, z.Tab.Border), z.Tab.Rounding, background.Data.Color)
		}
	}

	/* draw node label */
	var lblrect rect.Rect
	header.W = max(header.W, sym.W+item_spacing.Y+panel_padding.X)
	lblrect.X = sym.X + sym.W + item_spacing.X + 2*z.Tab.Spacing.X
	lblrect.Y = sym.Y
	lblrect.W = header.W - (sym.W + 2*z.Tab.Spacing.X + item_spacing.Y + panel_padding.X)
	lblrect.H = FontHeight(z.Font)

	return lblrect
}

///////////////////////////////////////////////////////////////////////////////////
// BUTTON
///////////////////////////////////////////////////////////////////////////////////

func drawButton(out *command.Buffer, bounds rect.Rect, state nstyle.WidgetStates, style *nstyle.Button) *nstyle.Item {
	var background *nstyle.Item
	if state&nstyle.WidgetStateHovered != 0 {
		background = &style.Hover
	} else if state&nstyle.WidgetStateActive != 0 {
		background = &style.Active
	} else {
		background = &style.Normal
	}

	if background.Type == nstyle.ItemImage {
		out.DrawImage(bounds, background.Data.Image)
	} else {
		out.FillRect(bounds, style.Rounding, style.BorderColor)
		out.FillRect(shrinkRect(bounds, style.Border), style.Rounding, background.Data.Color)
	}

	return background
}

func drawTextButton(win *Window, bounds rect.Rect, content rect.Rect, state nstyle.WidgetStates, style *nstyle.Button, txt string, align label.Align) {
	out := &win.cmds
	font := win.ctx.Style.Font

	if style.DrawBegin != nil {
		style.DrawBegin(out)
	}
	if style.DrawEnd != nil {
		defer style.DrawEnd(out)
	}

	if style.Draw.ButtonText != nil {
		style.Draw.ButtonText(out, bounds.Rectangle(), content.Rectangle(), state, style, txt, align, font)
		return
	}

	background := drawButton(out, bounds, state, style)

	/* select correct colors/images */
	var text textWidget
	if background.Type == nstyle.ItemColor {
		text.Background = background.Data.Color
	} else {
		text.Background = style.TextBackground
	}
	if state&nstyle.WidgetStateHovered != 0 {
		text.Text = style.TextHover
	} else if state&nstyle.WidgetStateActive != 0 {
		text.Text = style.TextActive
	} else {
		text.Text = style.TextNormal
	}

	widgetText(out, content, txt, &text, align, font)

}

func drawSymbolButton(win *Window, bounds rect.Rect, content rect.Rect, state nstyle.WidgetStates, style *nstyle.Button, symbol label.SymbolType) {
	out := &win.cmds
	font := win.ctx.Style.Font

	if style.DrawBegin != nil {
		style.DrawBegin(out)
	}
	if style.DrawEnd != nil {
		defer style.DrawEnd(out)
	}
	if style.Draw.ButtonSymbol != nil {
		style.Draw.ButtonSymbol(out, bounds.Rectangle(), content.Rectangle(), state, style, symbol, font)
		return
	}

	background := drawButton(out, bounds, state, style)
	var bg color.RGBA
	if background.Type == nstyle.ItemColor {
		bg = background.Data.Color
	} else {
		bg = style.TextBackground
	}

	var sym color.RGBA
	if state&nstyle.WidgetStateHovered != 0 {
		sym = style.TextHover
	} else if state&nstyle.WidgetStateActive != 0 {
		sym = style.TextActive
	} else {
		sym = style.TextNormal
	}
	drawSymbol(out, symbol, content, bg, sym, style.SymbolBorderWidth, font)
}

func drawImageButton(win *Window, bounds rect.Rect, content rect.Rect, state nstyle.WidgetStates, style *nstyle.Button, img *image.RGBA) {
	out := &win.cmds
	if style.DrawBegin != nil {
		style.DrawBegin(out)
	}
	if style.DrawEnd != nil {
		defer style.DrawEnd(out)
	}
	if style.Draw.ButtonImage != nil {
		style.Draw.ButtonImage(out, bounds.Rectangle(), content.Rectangle(), state, style, img)
		return
	}
	drawButton(out, bounds, state, style)
	out.DrawImage(content, img)
}

func drawTextSymbolButton(win *Window, bounds, labelrect, symbolrect rect.Rect, state nstyle.WidgetStates, style *nstyle.Button, str string, symbol label.SymbolType) {
	out := &win.cmds
	font := win.ctx.Style.Font

	if style.DrawBegin != nil {
		style.DrawBegin(out)
	}
	if style.DrawEnd != nil {
		defer style.DrawEnd(out)
	}
	if style.Draw.ButtonTextSymbol != nil {
		style.Draw.ButtonTextSymbol(out, bounds.Rectangle(), labelrect.Rectangle(), symbolrect.Rectangle(), state, style, str, symbol, font)
		return
	}

	/* select correct background colors/images */
	background := drawButton(out, bounds, state, style)

	var text textWidget
	if background.Type == nstyle.ItemColor {
		text.Background = background.Data.Color
	} else {
		text.Background = style.TextBackground
	}

	/* select correct text colors */
	var sym color.RGBA
	if state&nstyle.WidgetStateHovered != 0 {
		sym = style.TextHover
		text.Text = style.TextHover
	} else if state&nstyle.WidgetStateActive != 0 {
		sym = style.TextActive
		text.Text = style.TextActive
	} else {
		sym = style.TextNormal
		text.Text = style.TextNormal
	}

	text.Padding = image.Point{0, 0}
	drawSymbol(out, symbol, symbolrect, style.TextBackground, sym, 0, font)
	widgetText(out, labelrect, str, &text, "CC", font)
}

func drawTextImageButton(win *Window, bounds, labelrect, imgrect rect.Rect, state nstyle.WidgetStates, style *nstyle.Button, str string, img *image.RGBA) {
	out := &win.cmds
	font := win.ctx.Style.Font

	if style.DrawBegin != nil {
		style.DrawBegin(out)
	}
	if style.DrawEnd != nil {
		defer style.DrawEnd(out)
	}
	if style.Draw.ButtonTextImage != nil {
		style.Draw.ButtonTextImage(out, bounds.Rectangle(), labelrect.Rectangle(), imgrect.Rectangle(), state, style, str, font, img)
		return
	}

	background := drawButton(out, bounds, state, style)

	/* select correct colors */
	var text textWidget
	if background.Type == nstyle.ItemColor {
		text.Background = background.Data.Color
	} else {
		text.Background = style.TextBackground
	}
	if state&nstyle.WidgetStateHovered != 0 {
		text.Text = style.TextHover
	} else if state&nstyle.WidgetStateActive != 0 {
		text.Text = style.TextActive
	} else {
		text.Text = style.TextNormal
	}

	text.Padding = image.Point{0, 0}
	widgetText(out, labelrect, str, &text, "CC", font)
	out.DrawImage(imgrect, img)
}

///////////////////////////////////////////////////////////////////////////////////
// SELECTABLE
///////////////////////////////////////////////////////////////////////////////////

func drawSelectable(win *Window, state nstyle.WidgetStates, style *nstyle.Selectable, active bool, bounds rect.Rect, str string, align label.Align) {
	out := &win.cmds
	font := win.ctx.Style.Font

	if style.DrawBegin != nil {
		style.DrawBegin(out)
	}
	if style.DrawEnd != nil {
		defer style.DrawEnd(out)
	}
	if style.Draw != nil {
		style.Draw(out, state, style, active, bounds.Rectangle(), str, align, font)
		return
	}

	var background *nstyle.Item
	var text textWidget
	text.Padding = style.Padding

	/* select correct colors/images */
	if !active {
		if state&nstyle.WidgetStateActive != 0 {
			background = &style.Pressed
			text.Text = style.TextPressed
		} else if state&nstyle.WidgetStateHovered != 0 {
			background = &style.Hover
			text.Text = style.TextHover
		} else {
			background = &style.Normal
			text.Text = style.TextNormal
		}
	} else {
		if state&nstyle.WidgetStateActive != 0 {
			background = &style.PressedActive
			text.Text = style.TextPressedActive
		} else if state&nstyle.WidgetStateHovered != 0 {
			background = &style.HoverActive
			text.Text = style.TextHoverActive
		} else {
			background = &style.NormalActive
			text.Text = style.TextNormalActive
		}
	}

	/* draw selectable background and text */
	if background.Type == nstyle.ItemImage {
		out.DrawImage(bounds, background.Data.Image)
		text.Background = color.RGBA{0, 0, 0, 0}
	} else {
		out.FillRect(bounds, style.Rounding, background.Data.Color)
		text.Background = background.Data.Color
	}

	widgetText(out, bounds, str, &text, align, font)
}

///////////////////////////////////////////////////////////////////////////////////
// SCROLLBARS
///////////////////////////////////////////////////////////////////////////////////

func drawScrollbar(win *Window, state nstyle.WidgetStates, style *nstyle.Scrollbar, bounds, scroll rect.Rect) {
	out := &win.cmds
	if style.DrawBegin != nil {
		style.DrawBegin(out)
	}
	if style.DrawEnd != nil {
		defer style.DrawEnd(out)
	}
	if style.Draw != nil {
		style.Draw(out, state, style, bounds.Rectangle(), scroll.Rectangle())
		return
	}

	/* select correct colors/images to draw */
	var background *nstyle.Item
	var cursorstyle *nstyle.Item
	if state&nstyle.WidgetStateActive != 0 {
		background = &style.Active
		cursorstyle = &style.CursorActive
	} else if state&nstyle.WidgetStateHovered != 0 {
		background = &style.Hover
		cursorstyle = &style.CursorHover
	} else {
		background = &style.Normal
		cursorstyle = &style.CursorNormal
	}

	/* draw background */
	if background.Type == nstyle.ItemColor {
		out.FillRect(bounds, style.Rounding, style.BorderColor)
		out.FillRect(shrinkRect(bounds, style.Border), style.Rounding, background.Data.Color)
	} else {
		out.DrawImage(bounds, background.Data.Image)
	}

	/* draw cursor */
	if cursorstyle.Type == nstyle.ItemImage {
		out.DrawImage(scroll, cursorstyle.Data.Image)
	} else {
		out.FillRect(scroll, style.Rounding, cursorstyle.Data.Color)
	}
}

///////////////////////////////////////////////////////////////////////////////////
// TOGGLE BOXES
///////////////////////////////////////////////////////////////////////////////////

func drawTogglebox(win *Window, type_ toggleType, state nstyle.WidgetStates, style *nstyle.Toggle, active bool, labelrect, select_, cursor rect.Rect, str string) {
	out := &win.cmds
	font := win.ctx.Style.Font

	if style.DrawBegin != nil {
		style.DrawBegin(out)
	}
	if style.DrawEnd != nil {
		defer style.DrawEnd(out)
	}
	switch type_ {
	case toggleCheck:
		if style.Draw.Checkbox != nil {
			style.Draw.Checkbox(out, state, style, active, labelrect.Rectangle(), select_.Rectangle(), cursor.Rectangle(), str, font)
			return
		}
	default:
		if style.Draw.Radio != nil {
			style.Draw.Radio(out, state, style, active, labelrect.Rectangle(), select_.Rectangle(), cursor.Rectangle(), str, font)
			return
		}
	}

	/* select correct colors/images */
	var background *nstyle.Item
	var cursorstyle *nstyle.Item
	var text textWidget
	if state&nstyle.WidgetStateHovered != 0 {
		background = &style.Hover
		cursorstyle = &style.CursorHover
		text.Text = style.TextHover
	} else if state&nstyle.WidgetStateActive != 0 {
		background = &style.Hover
		cursorstyle = &style.CursorHover
		text.Text = style.TextActive
	} else {
		background = &style.Normal
		cursorstyle = &style.CursorNormal
		text.Text = style.TextNormal
	}

	/* draw background and cursor */
	if background.Type == nstyle.ItemImage {
		out.DrawImage(select_, background.Data.Image)
	} else {
		switch type_ {
		case toggleCheck:
			out.FillRect(select_, 0, background.Data.Color)
		default:
			out.FillCircle(select_, background.Data.Color)
		}
	}
	if active {
		if cursorstyle.Type == nstyle.ItemImage {
			out.DrawImage(cursor, cursorstyle.Data.Image)
		} else {
			switch type_ {
			case toggleCheck:
				out.FillRect(cursor, 0, cursorstyle.Data.Color)
			default:
				out.FillCircle(cursor, cursorstyle.Data.Color)
			}
		}
	}

	text.Padding.X = 0
	text.Padding.Y = 0
	text.Background = style.TextBackground
	widgetText(out, labelrect, str, &text, "LC", font)
}

///////////////////////////////////////////////////////////////////////////////////
// PROGRESS BAR
///////////////////////////////////////////////////////////////////////////////////

func drawProgress(win *Window, state nstyle.WidgetStates, style *nstyle.Progress, bounds, scursor rect.Rect, value, maxval int) {
	out := &win.cmds
	if style.DrawBegin != nil {
		style.DrawBegin(out)
	}
	if style.DrawEnd != nil {
		defer style.DrawEnd(out)
	}

	if style.Draw != nil {
		style.Draw(out, state, style, bounds.Rectangle(), scursor.Rectangle(), value, maxval)
		return
	}

	var background *nstyle.Item
	var cursor *nstyle.Item

	/* select correct colors/images to draw */
	if state&nstyle.WidgetStateActive != 0 {
		background = &style.Active
		cursor = &style.CursorActive
	} else if state&nstyle.WidgetStateHovered != 0 {
		background = &style.Hover
		cursor = &style.CursorHover
	} else {
		background = &style.Normal
		cursor = &style.CursorNormal
	}

	/* draw background */
	if background.Type == nstyle.ItemImage {
		out.DrawImage(bounds, background.Data.Image)
	} else {
		out.FillRect(bounds, style.Rounding, background.Data.Color)
	}

	/* draw cursor */
	if cursor.Type == nstyle.ItemImage {
		out.DrawImage(scursor, cursor.Data.Image)
	} else {
		out.FillRect(scursor, style.Rounding, cursor.Data.Color)
	}
}

///////////////////////////////////////////////////////////////////////////////////
// SLIDER
///////////////////////////////////////////////////////////////////////////////////

func drawSlider(win *Window, state nstyle.WidgetStates, style *nstyle.Slider, bounds, virtual_cursor rect.Rect, minval, value, maxval float64) {
	out := &win.cmds
	if style.DrawBegin != nil {
		style.DrawBegin(out)
	}
	if style.DrawEnd != nil {
		defer style.DrawEnd(out)
	}
	if style.Draw != nil {
		style.Draw(out, state, style, bounds.Rectangle(), virtual_cursor.Rectangle(), minval, value, maxval)
		return
	}

	var fill rect.Rect
	var bar rect.Rect
	var scursor rect.Rect
	var background *nstyle.Item
	var bar_color color.RGBA
	var cursor *nstyle.Item

	/* select correct slider images/colors */
	if state&nstyle.WidgetStateActive != 0 {
		background = &style.Active
		bar_color = style.BarActive
		cursor = &style.CursorActive
	} else if state&nstyle.WidgetStateHovered != 0 {
		background = &style.Hover
		bar_color = style.BarHover
		cursor = &style.CursorHover
	} else {
		background = &style.Normal
		bar_color = style.BarNormal
		cursor = &style.CursorNormal
	}

	/* calculate slider background bar */
	bar.X = bounds.X
	bar.Y = (bounds.Y + virtual_cursor.H/2) - virtual_cursor.H/8
	bar.W = bounds.W
	bar.H = bounds.H / 6

	/* resize virtual cursor to given size */
	scursor.H = style.CursorSize.Y
	scursor.W = style.CursorSize.X
	scursor.Y = (bar.Y + bar.H/2.0) - scursor.H/2.0
	scursor.X = virtual_cursor.X - (virtual_cursor.W / 2)

	/* filled background bar style */
	fill.W = (scursor.X + (scursor.W / 2.0)) - bar.X

	fill.X = bar.X
	fill.Y = bar.Y
	fill.H = bar.H

	/* draw background */
	if background.Type == nstyle.ItemImage {
		out.DrawImage(bounds, background.Data.Image)
	} else {
		out.FillRect(bounds, style.Rounding, style.BorderColor)
		out.FillRect(shrinkRect(bounds, style.Border), style.Rounding, background.Data.Color)
	}

	/* draw slider bar */
	out.FillRect(bar, style.Rounding, bar_color)

	out.FillRect(fill, style.Rounding, style.BarFilled)

	/* draw cursor */
	if cursor.Type == nstyle.ItemImage {
		out.DrawImage(scursor, cursor.Data.Image)
	} else {
		out.FillCircle(scursor, cursor.Data.Color)
	}
}

///////////////////////////////////////////////////////////////////////////////////
// PROPERTY
///////////////////////////////////////////////////////////////////////////////////

func drawProperty(win *Window, style *nstyle.Property, bounds, labelrect rect.Rect, ws nstyle.WidgetStates, name string) {
	out := &win.cmds
	font := win.ctx.Style.Font

	if style.DrawBegin != nil {
		style.DrawBegin(out)
	}
	if style.DrawEnd != nil {
		defer style.DrawEnd(out)
	}
	if style.Draw != nil {
		style.Draw(out, style, bounds.Rectangle(), labelrect.Rectangle(), ws, name, font)
		return
	}

	var text textWidget
	var background *nstyle.Item

	// select correct background and text color
	if ws&nstyle.WidgetStateActive != 0 {
		background = &style.Active
		text.Text = style.LabelActive
	} else if ws&nstyle.WidgetStateHovered != 0 {
		background = &style.Hover
		text.Text = style.LabelHover
	} else {
		background = &style.Normal
		text.Text = style.LabelNormal
	}

	// draw background
	if background.Type == nstyle.ItemImage {
		out.DrawImage(bounds, background.Data.Image)
		text.Background = color.RGBA{0, 0, 0, 0}
	} else {
		text.Background = background.Data.Color
		out.FillRect(bounds, style.Rounding, style.BorderColor)
		out.FillRect(shrinkRect(bounds, style.Border), style.Rounding, background.Data.Color)
	}

	// draw label
	widgetText(out, labelrect, name, &text, "CC", font)
}

///////////////////////////////////////////////////////////////////////////////////
// COMBO-BOX
///////////////////////////////////////////////////////////////////////////////////

func drawComboColor(win *Window, state nstyle.WidgetStates, header rect.Rect, is_active bool, color color.RGBA) {
	style := &win.ctx.Style
	out := &win.cmds
	/* draw combo box header background and border */
	var background *nstyle.Item
	if state&nstyle.WidgetStateActive != 0 {
		background = &style.Combo.Active
	} else if state&nstyle.WidgetStateHovered != 0 {
		background = &style.Combo.Hover
	} else {
		background = &style.Combo.Normal
	}

	if background.Type == nstyle.ItemImage {
		out.DrawImage(header, background.Data.Image)
	} else {
		out.FillRect(header, 0, style.Combo.BorderColor)
		out.FillRect(shrinkRect(header, 1), 0, background.Data.Color)
	}
	{
		var content rect.Rect
		var button rect.Rect
		var bounds rect.Rect
		var sym label.SymbolType
		if state&nstyle.WidgetStateHovered != 0 {
			sym = style.Combo.SymHover
		} else if is_active {
			sym = style.Combo.SymActive
		} else {
			sym = style.Combo.SymNormal
		}

		/* calculate button */
		button.W = header.H - 2*style.Combo.ButtonPadding.Y

		button.X = (header.X + header.W - header.H) - style.Combo.ButtonPadding.X
		button.Y = header.Y + style.Combo.ButtonPadding.Y
		button.H = button.W

		content.X = button.X + style.Combo.Button.Padding.X
		content.Y = button.Y + style.Combo.Button.Padding.Y
		content.W = button.W - 2*style.Combo.Button.Padding.X
		content.H = button.H - 2*style.Combo.Button.Padding.Y

		/* draw color */
		bounds.H = header.H - 4*style.Combo.ContentPadding.Y

		bounds.Y = header.Y + 2*style.Combo.ContentPadding.Y
		bounds.X = header.X + 2*style.Combo.ContentPadding.X
		bounds.W = (button.X - (style.Combo.ContentPadding.X + style.Combo.Spacing.X)) - bounds.X
		out.FillRect(bounds, 0, color)

		/* draw open/close button */
		drawSymbolButton(win, button, content, state, &style.Combo.Button, sym)
	}
}

func drawComboSymbol(win *Window, state nstyle.WidgetStates, header rect.Rect, is_active bool, symbol label.SymbolType) {
	style := &win.ctx.Style
	out := &win.cmds
	var background *nstyle.Item
	var sym_background color.RGBA
	var symbol_color color.RGBA

	/* draw combo box header background and border */
	if state&nstyle.WidgetStateActive != 0 {
		background = &style.Combo.Active
		symbol_color = style.Combo.SymbolActive
	} else if state&nstyle.WidgetStateHovered != 0 {
		background = &style.Combo.Hover
		symbol_color = style.Combo.SymbolHover
	} else {
		background = &style.Combo.Normal
		symbol_color = style.Combo.SymbolHover
	}

	if background.Type == nstyle.ItemImage {
		sym_background = color.RGBA{0, 0, 0, 0}
		out.DrawImage(header, background.Data.Image)
	} else {
		sym_background = background.Data.Color
		out.FillRect(header, 0, style.Combo.BorderColor)
		out.FillRect(shrinkRect(header, 1), 0, background.Data.Color)
	}
	{
		var bounds = rect.Rect{0, 0, 0, 0}
		var content rect.Rect
		var button rect.Rect
		var sym label.SymbolType
		if state&nstyle.WidgetStateHovered != 0 {
			sym = style.Combo.SymHover
		} else if is_active {
			sym = style.Combo.SymActive
		} else {
			sym = style.Combo.SymNormal
		}

		/* calculate button */
		button.W = header.H - 2*style.Combo.ButtonPadding.Y

		button.X = (header.X + header.W - header.H) - style.Combo.ButtonPadding.Y
		button.Y = header.Y + style.Combo.ButtonPadding.Y
		button.H = button.W

		content.X = button.X + style.Combo.Button.Padding.X
		content.Y = button.Y + style.Combo.Button.Padding.Y
		content.W = button.W - 2*style.Combo.Button.Padding.X
		content.H = button.H - 2*style.Combo.Button.Padding.Y

		/* draw symbol */
		bounds.H = header.H - 2*style.Combo.ContentPadding.Y

		bounds.Y = header.Y + style.Combo.ContentPadding.Y
		bounds.X = header.X + style.Combo.ContentPadding.X
		bounds.W = (button.X - style.Combo.ContentPadding.Y) - bounds.X
		drawSymbol(out, symbol, bounds, sym_background, symbol_color, 1.0, style.Font)

		/* draw open/close button */
		drawSymbolButton(win, button, content, state, &style.Combo.Button, sym)
	}
}

func drawComboSymbolText(win *Window, state nstyle.WidgetStates, header rect.Rect, is_active bool, symbol label.SymbolType, selected string) {
	style := &win.ctx.Style
	out := &win.cmds
	var background *nstyle.Item
	var symbol_color color.RGBA
	var text textWidget

	/* draw combo box header background and border */
	if state&nstyle.WidgetStateActive != 0 {
		background = &style.Combo.Active
		symbol_color = style.Combo.SymbolActive
		text.Text = style.Combo.LabelActive
	} else if state&nstyle.WidgetStateHovered != 0 {
		background = &style.Combo.Hover
		symbol_color = style.Combo.SymbolHover
		text.Text = style.Combo.LabelHover
	} else {
		background = &style.Combo.Normal
		symbol_color = style.Combo.SymbolNormal
		text.Text = style.Combo.LabelNormal
	}

	if background.Type == nstyle.ItemImage {
		text.Background = color.RGBA{0, 0, 0, 0}
		out.DrawImage(header, background.Data.Image)
	} else {
		text.Background = background.Data.Color
		out.FillRect(header, 0, style.Combo.BorderColor)
		out.FillRect(shrinkRect(header, 1), 0, background.Data.Color)
	}
	{
		var content rect.Rect
		var button rect.Rect
		var imrect rect.Rect
		var sym label.SymbolType
		if state&nstyle.WidgetStateHovered != 0 {
			sym = style.Combo.SymHover
		} else if is_active {
			sym = style.Combo.SymActive
		} else {
			sym = style.Combo.SymNormal
		}

		/* calculate button */
		button.W = header.H - 2*style.Combo.ButtonPadding.Y

		button.X = (header.X + header.W - header.H) - style.Combo.ButtonPadding.X
		button.Y = header.Y + style.Combo.ButtonPadding.Y
		button.H = button.W

		content.X = button.X + style.Combo.Button.Padding.X
		content.Y = button.Y + style.Combo.Button.Padding.Y
		content.W = button.W - 2*style.Combo.Button.Padding.X
		content.H = button.H - 2*style.Combo.Button.Padding.Y
		drawSymbolButton(win, button, content, state, &style.Combo.Button, sym)

		/* draw symbol */
		imrect.X = header.X + style.Combo.ContentPadding.X

		imrect.Y = header.Y + style.Combo.ContentPadding.Y
		imrect.H = header.H - 2*style.Combo.ContentPadding.Y
		imrect.W = imrect.H
		drawSymbol(out, symbol, imrect, text.Background, symbol_color, 1.0, style.Font)

		/* draw label */
		text.Padding = image.Point{0, 0}

		var lblrect rect.Rect
		lblrect.X = imrect.X + imrect.W + style.Combo.Spacing.X + style.Combo.ContentPadding.X
		lblrect.Y = header.Y + style.Combo.ContentPadding.Y
		lblrect.W = (button.X - style.Combo.ContentPadding.X) - lblrect.X
		lblrect.H = header.H - 2*style.Combo.ContentPadding.Y
		widgetText(out, lblrect, selected, &text, "LC", style.Font)
	}
}

func drawComboImage(win *Window, state nstyle.WidgetStates, header rect.Rect, is_active bool, img *image.RGBA) {
	style := &win.ctx.Style
	out := &win.cmds
	var background *nstyle.Item

	/* draw combo box header background and border */
	if state&nstyle.WidgetStateActive != 0 {
		background = &style.Combo.Active
	} else if state&nstyle.WidgetStateHovered != 0 {
		background = &style.Combo.Hover
	} else {
		background = &style.Combo.Normal
	}

	if background.Type == nstyle.ItemImage {
		out.DrawImage(header, background.Data.Image)
	} else {
		out.FillRect(header, 0, style.Combo.BorderColor)
		out.FillRect(shrinkRect(header, 1), 0, background.Data.Color)
	}
	{
		var bounds = rect.Rect{0, 0, 0, 0}
		var content rect.Rect
		var button rect.Rect
		var sym label.SymbolType
		if state&nstyle.WidgetStateHovered != 0 {
			sym = style.Combo.SymHover
		} else if is_active {
			sym = style.Combo.SymActive
		} else {
			sym = style.Combo.SymNormal
		}

		/* calculate button */
		button.W = header.H - 2*style.Combo.ButtonPadding.Y
		button.X = (header.X + header.W - header.H) - style.Combo.ButtonPadding.Y
		button.Y = header.Y + style.Combo.ButtonPadding.Y
		button.H = button.W

		content.X = button.X + style.Combo.Button.Padding.X
		content.Y = button.Y + style.Combo.Button.Padding.Y
		content.W = button.W - 2*style.Combo.Button.Padding.X
		content.H = button.H - 2*style.Combo.Button.Padding.Y

		/* draw image */
		bounds.H = header.H - 2*style.Combo.ContentPadding.Y
		bounds.Y = header.Y + style.Combo.ContentPadding.Y
		bounds.X = header.X + style.Combo.ContentPadding.X
		bounds.W = (button.X - style.Combo.ContentPadding.Y) - bounds.X
		out.DrawImage(bounds, img)

		/* draw open/close button */
		drawSymbolButton(win, button, content, state, &style.Combo.Button, sym)
	}
}

func drawComboImageText(win *Window, state nstyle.WidgetStates, header rect.Rect, is_active bool, selected string, img *image.RGBA) {
	style := &win.ctx.Style
	out := &win.cmds
	var background *nstyle.Item
	var text textWidget

	/* draw combo box header background and border */
	if state&nstyle.WidgetStateActive != 0 {
		background = &style.Combo.Active
		text.Text = style.Combo.LabelActive
	} else if state&nstyle.WidgetStateHovered != 0 {
		background = &style.Combo.Hover
		text.Text = style.Combo.LabelHover
	} else {
		background = &style.Combo.Normal
		text.Text = style.Combo.LabelNormal
	}

	if background.Type == nstyle.ItemImage {
		text.Background = color.RGBA{0, 0, 0, 0}
		out.DrawImage(header, background.Data.Image)
	} else {
		text.Background = background.Data.Color
		out.FillRect(header, 0, style.Combo.BorderColor)
		out.FillRect(shrinkRect(header, 1), 0, background.Data.Color)
	}
	{
		var content rect.Rect
		var button rect.Rect
		var imrect rect.Rect
		var sym label.SymbolType
		if state&nstyle.WidgetStateHovered != 0 {
			sym = style.Combo.SymHover
		} else if is_active {
			sym = style.Combo.SymActive
		} else {
			sym = style.Combo.SymNormal
		}

		/* calculate button */
		button.W = header.H - 2*style.Combo.ButtonPadding.Y

		button.X = (header.X + header.W - header.H) - style.Combo.ButtonPadding.X
		button.Y = header.Y + style.Combo.ButtonPadding.Y
		button.H = button.W

		content.X = button.X + style.Combo.Button.Padding.X
		content.Y = button.Y + style.Combo.Button.Padding.Y
		content.W = button.W - 2*style.Combo.Button.Padding.X
		content.H = button.H - 2*style.Combo.Button.Padding.Y
		drawSymbolButton(win, button, content, state, &style.Combo.Button, sym)

		/* draw image */
		imrect.X = header.X + style.Combo.ContentPadding.X
		imrect.Y = header.Y + style.Combo.ContentPadding.Y
		imrect.H = header.H - 2*style.Combo.ContentPadding.Y
		imrect.W = imrect.H
		out.DrawImage(imrect, img)

		/* draw label */
		text.Padding = image.Point{0, 0}

		var lblrect rect.Rect
		lblrect.X = imrect.X + imrect.W + style.Combo.Spacing.X + style.Combo.ContentPadding.X
		lblrect.Y = header.Y + style.Combo.ContentPadding.Y
		lblrect.W = (button.X - style.Combo.ContentPadding.X) - lblrect.X
		lblrect.H = header.H - 2*style.Combo.ContentPadding.Y
		widgetText(out, lblrect, selected, &text, "LC", style.Font)
	}
}

func drawComboText(win *Window, state nstyle.WidgetStates, header rect.Rect, is_active bool, selected string) {
	style := &win.ctx.Style
	out := &win.cmds
	/* draw combo box header background and border */
	var background *nstyle.Item
	var text textWidget
	if state&nstyle.WidgetStateActive != 0 {
		background = &style.Combo.Active
		text.Text = style.Combo.LabelActive
	} else if state&nstyle.WidgetStateHovered != 0 {
		background = &style.Combo.Hover
		text.Text = style.Combo.LabelHover
	} else {
		background = &style.Combo.Normal
		text.Text = style.Combo.LabelNormal
	}

	if background.Type == nstyle.ItemImage {
		text.Background = color.RGBA{0, 0, 0, 0}
		out.DrawImage(header, background.Data.Image)
	} else {
		text.Background = background.Data.Color
		out.FillRect(header, style.Combo.Rounding, style.Combo.BorderColor)
		out.FillRect(shrinkRect(header, 1), style.Combo.Rounding, background.Data.Color)
	}

	var button rect.Rect
	var content rect.Rect
	/* print currently selected text item */

	var sym label.SymbolType
	if state&nstyle.WidgetStateHovered != 0 {
		sym = style.Combo.SymHover
	} else if is_active {
		sym = style.Combo.SymActive
	} else {
		sym = style.Combo.SymNormal
	}

	/* calculate button */
	button.W = header.H - 2*style.Combo.ButtonPadding.Y

	button.X = (header.X + header.W - header.H) - style.Combo.ButtonPadding.X
	button.Y = header.Y + style.Combo.ButtonPadding.Y
	button.H = button.W

	content.X = button.X + style.Combo.Button.Padding.X
	content.Y = button.Y + style.Combo.Button.Padding.Y
	content.W = button.W - 2*style.Combo.Button.Padding.X
	content.H = button.H - 2*style.Combo.Button.Padding.Y

	/* draw selected label */
	text.Padding = image.Point{0, 0}

	var lblrect rect.Rect
	lblrect.X = header.X + style.Combo.ContentPadding.X
	lblrect.Y = header.Y + style.Combo.ContentPadding.Y
	lblrect.W = button.X - (style.Combo.ContentPadding.X + style.Combo.Spacing.X) - lblrect.X
	lblrect.H = header.H - 2*style.Combo.ContentPadding.Y
	widgetText(out, lblrect, selected, &text, "LC", style.Font)

	/* draw open/close button */
	drawSymbolButton(win, button, content, state, &style.Combo.Button, sym)
}
