package style

import (
	"image"
	"image/color"

	"github.com/aarzilli/nucular/command"
	"github.com/aarzilli/nucular/font"
	"github.com/aarzilli/nucular/label"
)

type WidgetStates int

const (
	WidgetStateInactive WidgetStates = iota
	WidgetStateHovered
	WidgetStateActive
)

type Style struct {
	Scaling          float64
	unscaled         *Style
	defaultFont      font.Face
	Font             font.Face
	Text             Text
	Button           Button
	ContextualButton Button
	MenuButton       Button
	Option           Toggle
	Checkbox         Toggle
	Selectable       Selectable
	Slider           Slider
	Progress         Progress
	Property         Property
	Edit             Edit
	Scrollh          Scrollbar
	Scrollv          Scrollbar
	Tab              Tab
	Combo            Combo
	NormalWindow     Window
	MenuWindow       Window
	TooltipWindow    Window
	ComboWindow      Window
	ContextualWindow Window
	GroupWindow      Window
}

type ColorTable struct {
	ColorText                  color.RGBA
	ColorWindow                color.RGBA
	ColorHeader                color.RGBA
	ColorHeaderFocused         color.RGBA
	ColorBorder                color.RGBA
	ColorButton                color.RGBA
	ColorButtonHover           color.RGBA
	ColorButtonActive          color.RGBA
	ColorToggle                color.RGBA
	ColorToggleHover           color.RGBA
	ColorToggleCursor          color.RGBA
	ColorSelect                color.RGBA
	ColorSelectActive          color.RGBA
	ColorSlider                color.RGBA
	ColorSliderCursor          color.RGBA
	ColorSliderCursorHover     color.RGBA
	ColorSliderCursorActive    color.RGBA
	ColorProperty              color.RGBA
	ColorEdit                  color.RGBA
	ColorEditCursor            color.RGBA
	ColorCombo                 color.RGBA
	ColorChart                 color.RGBA
	ColorChartColor            color.RGBA
	ColorChartColorHighlight   color.RGBA
	ColorScrollbar             color.RGBA
	ColorScrollbarCursor       color.RGBA
	ColorScrollbarCursorHover  color.RGBA
	ColorScrollbarCursorActive color.RGBA
	ColorTabHeader             color.RGBA
}

type ItemType int

const (
	ItemColor ItemType = iota
	ItemImage
)

type ItemData struct {
	Image *image.RGBA
	Color color.RGBA
}

type Item struct {
	Type ItemType
	Data ItemData
}

type Text struct {
	Color   color.RGBA
	Padding image.Point
}

type Button struct {
	Normal            Item
	Hover             Item
	Active            Item
	BorderColor       color.RGBA
	TextBackground    color.RGBA
	TextNormal        color.RGBA
	TextHover         color.RGBA
	TextActive        color.RGBA
	Border            int
	Rounding          uint16
	Padding           image.Point
	ImagePadding      image.Point
	TouchPadding      image.Point
	DrawBegin         func(*command.Buffer)
	Draw              CustomButtonDrawing
	DrawEnd           func(*command.Buffer)
	SymbolBorderWidth int
}

type CustomButtonDrawing struct {
	ButtonText       func(*command.Buffer, image.Rectangle, image.Rectangle, WidgetStates, *Button, string, label.Align, font.Face)
	ButtonSymbol     func(*command.Buffer, image.Rectangle, image.Rectangle, WidgetStates, *Button, label.SymbolType, font.Face)
	ButtonImage      func(*command.Buffer, image.Rectangle, image.Rectangle, WidgetStates, *Button, *image.RGBA)
	ButtonTextSymbol func(*command.Buffer, image.Rectangle, image.Rectangle, image.Rectangle, WidgetStates, *Button, string, label.SymbolType, font.Face)
	ButtonTextImage  func(*command.Buffer, image.Rectangle, image.Rectangle, image.Rectangle, WidgetStates, *Button, string, font.Face, *image.RGBA)
}

type Toggle struct {
	Normal         Item
	Hover          Item
	Active         Item
	CursorNormal   Item
	CursorHover    Item
	TextNormal     color.RGBA
	TextHover      color.RGBA
	TextActive     color.RGBA
	TextBackground color.RGBA
	Padding        image.Point
	TouchPadding   image.Point
	DrawBegin      func(*command.Buffer)
	Draw           CustomToggleDrawing
	DrawEnd        func(*command.Buffer)
}

type CustomToggleDrawing struct {
	Radio    func(*command.Buffer, WidgetStates, *Toggle, bool, image.Rectangle, image.Rectangle, image.Rectangle, string, font.Face)
	Checkbox func(*command.Buffer, WidgetStates, *Toggle, bool, image.Rectangle, image.Rectangle, image.Rectangle, string, font.Face)
}

type Selectable struct {
	Normal            Item
	Hover             Item
	Pressed           Item
	NormalActive      Item
	HoverActive       Item
	PressedActive     Item
	TextNormal        color.RGBA
	TextHover         color.RGBA
	TextPressed       color.RGBA
	TextNormalActive  color.RGBA
	TextHoverActive   color.RGBA
	TextPressedActive color.RGBA
	TextBackground    color.RGBA
	TextAlignment     uint32
	Rounding          uint16
	Padding           image.Point
	TouchPadding      image.Point
	DrawBegin         func(*command.Buffer)
	Draw              func(*command.Buffer, WidgetStates, *Selectable, bool, image.Rectangle, string, label.Align, font.Face)
	DrawEnd           func(*command.Buffer)
}

type Slider struct {
	Normal       Item
	Hover        Item
	Active       Item
	BorderColor  color.RGBA
	BarNormal    color.RGBA
	BarHover     color.RGBA
	BarActive    color.RGBA
	BarFilled    color.RGBA
	CursorNormal Item
	CursorHover  Item
	CursorActive Item
	Border       int
	Rounding     uint16
	BarHeight    int
	Padding      image.Point
	Spacing      image.Point
	CursorSize   image.Point
	ShowButtons  bool
	IncButton    Button
	DecButton    Button
	IncSymbol    label.SymbolType
	DecSymbol    label.SymbolType
	DrawBegin    func(*command.Buffer)
	Draw         func(*command.Buffer, WidgetStates, *Slider, image.Rectangle, image.Rectangle, float64, float64, float64)
	DrawEnd      func(*command.Buffer)
}

type Progress struct {
	Normal       Item
	Hover        Item
	Active       Item
	CursorNormal Item
	CursorHover  Item
	CursorActive Item
	Rounding     uint16
	Padding      image.Point
	DrawBegin    func(*command.Buffer)
	Draw         func(*command.Buffer, WidgetStates, *Progress, image.Rectangle, image.Rectangle, int, int)
	DrawEnd      func(*command.Buffer)
}

type Scrollbar struct {
	Normal       Item
	Hover        Item
	Active       Item
	BorderColor  color.RGBA
	CursorNormal Item
	CursorHover  Item
	CursorActive Item
	Border       int
	Rounding     uint16
	Padding      image.Point
	ShowButtons  bool
	IncButton    Button
	DecButton    Button
	IncSymbol    label.SymbolType
	DecSymbol    label.SymbolType
	DrawBegin    func(*command.Buffer)
	Draw         func(*command.Buffer, WidgetStates, *Scrollbar, image.Rectangle, image.Rectangle)
	DrawEnd      func(*command.Buffer)
}

type Edit struct {
	Normal             Item
	Hover              Item
	Active             Item
	BorderColor        color.RGBA
	Scrollbar          Scrollbar
	CursorNormal       color.RGBA
	CursorHover        color.RGBA
	CursorTextNormal   color.RGBA
	CursorTextHover    color.RGBA
	TextNormal         color.RGBA
	TextHover          color.RGBA
	TextActive         color.RGBA
	SelectedNormal     color.RGBA
	SelectedHover      color.RGBA
	SelectedTextNormal color.RGBA
	SelectedTextHover  color.RGBA
	Border             int
	Rounding           uint16
	ScrollbarSize      image.Point
	Padding            image.Point
	RowPadding         int
}

type Property struct {
	Normal      Item
	Hover       Item
	Active      Item
	BorderColor color.RGBA
	LabelNormal color.RGBA
	LabelHover  color.RGBA
	LabelActive color.RGBA
	SymLeft     label.SymbolType
	SymRight    label.SymbolType
	Border      int
	Rounding    uint16
	Padding     image.Point
	Edit        Edit
	IncButton   Button
	DecButton   Button
	DrawBegin   func(*command.Buffer)
	Draw        func(*command.Buffer, *Property, image.Rectangle, image.Rectangle, WidgetStates, string, font.Face)
	DrawEnd     func(*command.Buffer)
}

type Chart struct {
	Background    Item
	BorderColor   color.RGBA
	SelectedColor color.RGBA
	Color         color.RGBA
	Border        int
	Rounding      uint16
	Padding       image.Point
}

type Combo struct {
	Normal         Item
	Hover          Item
	Active         Item
	BorderColor    color.RGBA
	LabelNormal    color.RGBA
	LabelHover     color.RGBA
	LabelActive    color.RGBA
	SymbolNormal   color.RGBA
	SymbolHover    color.RGBA
	SymbolActive   color.RGBA
	Button         Button
	SymNormal      label.SymbolType
	SymHover       label.SymbolType
	SymActive      label.SymbolType
	Border         int
	Rounding       uint16
	ContentPadding image.Point
	ButtonPadding  image.Point
	Spacing        image.Point
}

type Tab struct {
	Background  Item
	BorderColor color.RGBA
	Text        color.RGBA
	TabButton   Button
	NodeButton  Button
	SymMinimize label.SymbolType
	SymMaximize label.SymbolType
	Border      int
	Rounding    uint16
	Padding     image.Point
	Spacing     image.Point
	Indent      int
}

type HeaderAlign int

const (
	HeaderLeft HeaderAlign = iota
	HeaderRight
)

type WindowHeader struct {
	Normal         Item
	Hover          Item
	Active         Item
	CloseButton    Button
	MinimizeButton Button
	CloseSymbol    label.SymbolType
	MinimizeSymbol label.SymbolType
	MaximizeSymbol label.SymbolType
	LabelNormal    color.RGBA
	LabelHover     color.RGBA
	LabelActive    color.RGBA
	Align          HeaderAlign
	Padding        image.Point
	LabelPadding   image.Point
	Spacing        image.Point
}

type Window struct {
	Header          WindowHeader
	FixedBackground Item
	Background      color.RGBA
	BorderColor     color.RGBA
	Scaler          Item
	FooterPadding   image.Point
	Border          int
	Rounding        uint16
	ScalerSize      image.Point
	Padding         image.Point
	Spacing         image.Point
	ScrollbarSize   image.Point
	MinSize         image.Point
}

var defaultThemeTable = ColorTable{color.RGBA{175, 175, 175, 255}, color.RGBA{45, 45, 45, 255}, color.RGBA{40, 40, 40, 255}, color.RGBA{40, 40, 40, 255}, color.RGBA{65, 65, 65, 255}, color.RGBA{50, 50, 50, 255}, color.RGBA{40, 40, 40, 255}, color.RGBA{35, 35, 35, 255}, color.RGBA{100, 100, 100, 255}, color.RGBA{120, 120, 120, 255}, color.RGBA{45, 45, 45, 255}, color.RGBA{45, 45, 45, 255}, color.RGBA{35, 35, 35, 255}, color.RGBA{38, 38, 38, 255}, color.RGBA{100, 100, 100, 255}, color.RGBA{120, 120, 120, 255}, color.RGBA{150, 150, 150, 255}, color.RGBA{38, 38, 38, 255}, color.RGBA{38, 38, 38, 255}, color.RGBA{175, 175, 175, 255}, color.RGBA{45, 45, 45, 255}, color.RGBA{120, 120, 120, 255}, color.RGBA{45, 45, 45, 255}, color.RGBA{255, 0, 0, 255}, color.RGBA{40, 40, 40, 255}, color.RGBA{100, 100, 100, 255}, color.RGBA{120, 120, 120, 255}, color.RGBA{150, 150, 150, 255}, color.RGBA{40, 40, 40, 255}}

func FromTable(table ColorTable, scaling float64) *Style {
	var text *Text
	var button *Button
	var toggle *Toggle
	var select_ *Selectable
	var slider *Slider
	var prog *Progress
	var scroll *Scrollbar
	var edit *Edit
	var property *Property
	var combo *Combo
	var tab *Tab
	var win *Window

	style := &Style{Scaling: 1.0}

	/* default text */
	text = &style.Text

	text.Color = table.ColorText
	text.Padding = image.Point{4, 4}

	/* default button */
	button = &style.Button

	*button = Button{}
	button.Normal = MakeItemColor(table.ColorButton)
	button.Hover = MakeItemColor(table.ColorButtonHover)
	button.Active = MakeItemColor(table.ColorButtonActive)
	button.BorderColor = table.ColorBorder
	button.TextBackground = table.ColorButton
	button.TextNormal = table.ColorText
	button.TextHover = table.ColorText
	button.TextActive = table.ColorText
	button.Padding = image.Point{4.0, 4.0}
	button.ImagePadding = image.Point{0.0, 0.0}
	button.TouchPadding = image.Point{0.0, 0.0}
	button.Border = 1
	button.SymbolBorderWidth = 1
	button.Rounding = 4
	button.DrawBegin = nil
	button.DrawEnd = nil

	/* contextual button */
	button = &style.ContextualButton

	*button = Button{}
	button.Normal = MakeItemColor(table.ColorWindow)
	button.Hover = MakeItemColor(table.ColorButtonHover)
	button.Active = MakeItemColor(table.ColorButtonActive)
	button.BorderColor = table.ColorWindow
	button.TextBackground = table.ColorWindow
	button.TextNormal = table.ColorText
	button.TextHover = table.ColorText
	button.TextActive = table.ColorText
	button.Padding = image.Point{4.0, 4.0}
	button.TouchPadding = image.Point{0.0, 0.0}
	button.Border = 0
	button.SymbolBorderWidth = 1
	button.Rounding = 0
	button.DrawBegin = nil
	button.DrawEnd = nil

	/* menu button */
	button = &style.MenuButton

	*button = Button{}
	button.Normal = MakeItemColor(table.ColorWindow)
	button.Hover = MakeItemColor(table.ColorWindow)
	button.Active = MakeItemColor(table.ColorWindow)
	button.BorderColor = table.ColorWindow
	button.TextBackground = table.ColorWindow
	button.TextNormal = table.ColorText
	button.TextHover = table.ColorText
	button.TextActive = table.ColorText
	button.Padding = image.Point{4.0, 4.0}
	button.TouchPadding = image.Point{0.0, 0.0}
	button.Border = 0
	button.SymbolBorderWidth = 1
	button.Rounding = 1
	button.DrawBegin = nil
	button.DrawEnd = nil

	/* checkbox toggle */
	toggle = &style.Checkbox

	*toggle = Toggle{}
	toggle.Normal = MakeItemColor(table.ColorToggle)
	toggle.Hover = MakeItemColor(table.ColorToggleHover)
	toggle.Active = MakeItemColor(table.ColorToggleHover)
	toggle.CursorNormal = MakeItemColor(table.ColorToggleCursor)
	toggle.CursorHover = MakeItemColor(table.ColorToggleCursor)
	toggle.TextBackground = table.ColorWindow
	toggle.TextNormal = table.ColorText
	toggle.TextHover = table.ColorText
	toggle.TextActive = table.ColorText
	toggle.Padding = image.Point{4.0, 4.0}
	toggle.TouchPadding = image.Point{0, 0}

	/* option toggle */
	toggle = &style.Option

	*toggle = Toggle{}
	toggle.Normal = MakeItemColor(table.ColorToggle)
	toggle.Hover = MakeItemColor(table.ColorToggleHover)
	toggle.Active = MakeItemColor(table.ColorToggleHover)
	toggle.CursorNormal = MakeItemColor(table.ColorToggleCursor)
	toggle.CursorHover = MakeItemColor(table.ColorToggleCursor)
	toggle.TextBackground = table.ColorWindow
	toggle.TextNormal = table.ColorText
	toggle.TextHover = table.ColorText
	toggle.TextActive = table.ColorText
	toggle.Padding = image.Point{4.0, 4.0}
	toggle.TouchPadding = image.Point{0, 0}

	/* selectable */
	select_ = &style.Selectable

	*select_ = Selectable{}
	select_.Normal = MakeItemColor(table.ColorSelect)
	select_.Hover = MakeItemColor(table.ColorSelect)
	select_.Pressed = MakeItemColor(table.ColorSelect)
	select_.NormalActive = MakeItemColor(table.ColorSelectActive)
	select_.HoverActive = MakeItemColor(table.ColorSelectActive)
	select_.PressedActive = MakeItemColor(table.ColorSelectActive)
	select_.TextNormal = table.ColorText
	select_.TextHover = table.ColorText
	select_.TextPressed = table.ColorText
	select_.TextNormalActive = table.ColorText
	select_.TextHoverActive = table.ColorText
	select_.TextPressedActive = table.ColorText
	select_.Padding = image.Point{4.0, 4.0}
	select_.TouchPadding = image.Point{0, 0}
	select_.Rounding = 0.0
	select_.DrawBegin = nil
	select_.Draw = nil
	select_.DrawEnd = nil

	/* slider */
	slider = &style.Slider

	*slider = Slider{}
	slider.Normal = ItemHide()
	slider.Hover = ItemHide()
	slider.Active = ItemHide()
	slider.BarNormal = table.ColorSlider
	slider.BarHover = table.ColorSlider
	slider.BarActive = table.ColorSlider
	slider.BarFilled = table.ColorSliderCursor
	slider.CursorNormal = MakeItemColor(table.ColorSliderCursor)
	slider.CursorHover = MakeItemColor(table.ColorSliderCursorHover)
	slider.CursorActive = MakeItemColor(table.ColorSliderCursorActive)
	slider.IncSymbol = label.SymbolTriangleRight
	slider.DecSymbol = label.SymbolTriangleLeft
	slider.CursorSize = image.Point{16, 16}
	slider.Padding = image.Point{4, 4}
	slider.Spacing = image.Point{4, 4}
	slider.ShowButtons = false
	slider.BarHeight = 8
	slider.Rounding = 0
	slider.DrawBegin = nil
	slider.Draw = nil
	slider.DrawEnd = nil

	/* slider buttons */
	button = &style.Slider.IncButton

	button.Normal = MakeItemColor(color.RGBA{40, 40, 40, 0xff})
	button.Hover = MakeItemColor(color.RGBA{42, 42, 42, 0xff})
	button.Active = MakeItemColor(color.RGBA{44, 44, 44, 0xff})
	button.BorderColor = color.RGBA{65, 65, 65, 0xff}
	button.TextBackground = color.RGBA{40, 40, 40, 0xff}
	button.TextNormal = color.RGBA{175, 175, 175, 0xff}
	button.TextHover = color.RGBA{175, 175, 175, 0xff}
	button.TextActive = color.RGBA{175, 175, 175, 0xff}
	button.Padding = image.Point{8.0, 8.0}
	button.TouchPadding = image.Point{0.0, 0.0}
	button.Border = 1.0
	button.Rounding = 0.0
	button.DrawBegin = nil
	button.DrawEnd = nil
	style.Slider.DecButton = style.Slider.IncButton

	/* progressbar */
	prog = &style.Progress

	*prog = Progress{}
	prog.Normal = MakeItemColor(table.ColorSlider)
	prog.Hover = MakeItemColor(table.ColorSlider)
	prog.Active = MakeItemColor(table.ColorSlider)
	prog.CursorNormal = MakeItemColor(table.ColorSliderCursor)
	prog.CursorHover = MakeItemColor(table.ColorSliderCursorHover)
	prog.CursorActive = MakeItemColor(table.ColorSliderCursorActive)
	prog.Padding = image.Point{4, 4}
	prog.Rounding = 0
	prog.DrawBegin = nil
	prog.Draw = nil
	prog.DrawEnd = nil

	/* scrollbars */
	scroll = &style.Scrollh

	*scroll = Scrollbar{}
	scroll.Normal = MakeItemColor(table.ColorScrollbar)
	scroll.Hover = MakeItemColor(table.ColorScrollbar)
	scroll.Active = MakeItemColor(table.ColorScrollbar)
	scroll.CursorNormal = MakeItemColor(table.ColorScrollbarCursor)
	scroll.CursorHover = MakeItemColor(table.ColorScrollbarCursorHover)
	scroll.CursorActive = MakeItemColor(table.ColorScrollbarCursorActive)
	scroll.DecSymbol = label.SymbolCircleFilled
	scroll.IncSymbol = label.SymbolCircleFilled
	scroll.BorderColor = color.RGBA{65, 65, 65, 0xff}
	scroll.Padding = image.Point{4, 4}
	scroll.ShowButtons = false
	scroll.Border = 0
	scroll.Rounding = 0
	scroll.DrawBegin = nil
	scroll.Draw = nil
	scroll.DrawEnd = nil
	style.Scrollv = style.Scrollh

	/* scrollbars buttons */
	button = &style.Scrollh.IncButton

	button.Normal = MakeItemColor(color.RGBA{40, 40, 40, 0xff})
	button.Hover = MakeItemColor(color.RGBA{42, 42, 42, 0xff})
	button.Active = MakeItemColor(color.RGBA{44, 44, 44, 0xff})
	button.BorderColor = color.RGBA{65, 65, 65, 0xff}
	button.TextBackground = color.RGBA{40, 40, 40, 0xff}
	button.TextNormal = color.RGBA{175, 175, 175, 0xff}
	button.TextHover = color.RGBA{175, 175, 175, 0xff}
	button.TextActive = color.RGBA{175, 175, 175, 0xff}
	button.Padding = image.Point{4.0, 4.0}
	button.TouchPadding = image.Point{0.0, 0.0}
	button.Border = 1.0
	button.Rounding = 0.0
	button.DrawBegin = nil
	button.DrawEnd = nil
	style.Scrollh.DecButton = style.Scrollh.IncButton
	style.Scrollv.IncButton = style.Scrollh.IncButton
	style.Scrollv.DecButton = style.Scrollh.IncButton

	/* edit */
	edit = &style.Edit

	*edit = Edit{}
	edit.Normal = MakeItemColor(table.ColorEdit)
	edit.Hover = MakeItemColor(table.ColorEdit)
	edit.Active = MakeItemColor(table.ColorEdit)
	edit.Scrollbar = *scroll
	edit.CursorNormal = table.ColorText
	edit.CursorHover = table.ColorText
	edit.CursorTextNormal = table.ColorEdit
	edit.CursorTextHover = table.ColorEdit
	edit.BorderColor = table.ColorBorder
	edit.TextNormal = table.ColorText
	edit.TextHover = table.ColorText
	edit.TextActive = table.ColorText
	edit.SelectedNormal = table.ColorText
	edit.SelectedHover = table.ColorText
	edit.SelectedTextNormal = table.ColorEdit
	edit.SelectedTextHover = table.ColorEdit
	edit.RowPadding = 2
	edit.Padding = image.Point{4, 4}
	edit.ScrollbarSize = image.Point{4, 4}
	edit.Border = 1
	edit.Rounding = 0

	/* property */
	property = &style.Property

	*property = Property{}
	property.Normal = MakeItemColor(table.ColorProperty)
	property.Hover = MakeItemColor(table.ColorProperty)
	property.Active = MakeItemColor(table.ColorProperty)
	property.BorderColor = table.ColorBorder
	property.LabelNormal = table.ColorText
	property.LabelHover = table.ColorText
	property.LabelActive = table.ColorText
	property.SymLeft = label.SymbolTriangleLeft
	property.SymRight = label.SymbolTriangleRight
	property.Padding = image.Point{4, 4}
	property.Border = 1
	property.Rounding = 10
	property.DrawBegin = nil
	property.Draw = nil
	property.DrawEnd = nil

	/* property buttons */
	button = &style.Property.DecButton

	*button = Button{}
	button.Normal = MakeItemColor(table.ColorProperty)
	button.Hover = MakeItemColor(table.ColorProperty)
	button.Active = MakeItemColor(table.ColorProperty)
	button.BorderColor = color.RGBA{0, 0, 0, 0}
	button.TextBackground = table.ColorProperty
	button.TextNormal = table.ColorText
	button.TextHover = table.ColorText
	button.TextActive = table.ColorText
	button.Padding = image.Point{0.0, 0.0}
	button.TouchPadding = image.Point{0.0, 0.0}
	button.Border = 0.0
	button.SymbolBorderWidth = 1
	button.Rounding = 0.0
	button.DrawBegin = nil
	button.DrawEnd = nil
	style.Property.IncButton = style.Property.DecButton

	/* property edit */
	edit = &style.Property.Edit

	*edit = Edit{}
	edit.Normal = MakeItemColor(table.ColorProperty)
	edit.Hover = MakeItemColor(table.ColorProperty)
	edit.Active = MakeItemColor(table.ColorProperty)
	edit.BorderColor = color.RGBA{0, 0, 0, 0}
	edit.CursorNormal = table.ColorText
	edit.CursorHover = table.ColorText
	edit.CursorTextNormal = table.ColorEdit
	edit.CursorTextHover = table.ColorEdit
	edit.TextNormal = table.ColorText
	edit.TextHover = table.ColorText
	edit.TextActive = table.ColorText
	edit.SelectedNormal = table.ColorText
	edit.SelectedHover = table.ColorText
	edit.SelectedTextNormal = table.ColorEdit
	edit.SelectedTextHover = table.ColorEdit
	edit.Padding = image.Point{0, 0}
	edit.Border = 0
	edit.Rounding = 0

	/* combo */
	combo = &style.Combo

	combo.Normal = MakeItemColor(table.ColorCombo)
	combo.Hover = MakeItemColor(table.ColorCombo)
	combo.Active = MakeItemColor(table.ColorCombo)
	combo.BorderColor = table.ColorBorder
	combo.LabelNormal = table.ColorText
	combo.LabelHover = table.ColorText
	combo.LabelActive = table.ColorText
	combo.SymNormal = label.SymbolTriangleDown
	combo.SymHover = label.SymbolTriangleDown
	combo.SymActive = label.SymbolTriangleDown
	combo.ContentPadding = image.Point{4, 4}
	combo.ButtonPadding = image.Point{0, 4}
	combo.Spacing = image.Point{4, 0}
	combo.Border = 1
	combo.Rounding = 0

	/* combo button */
	button = &style.Combo.Button

	*button = Button{}
	button.Normal = MakeItemColor(table.ColorCombo)
	button.Hover = MakeItemColor(table.ColorCombo)
	button.Active = MakeItemColor(table.ColorCombo)
	button.BorderColor = color.RGBA{0, 0, 0, 0}
	button.TextBackground = table.ColorCombo
	button.TextNormal = table.ColorText
	button.TextHover = table.ColorText
	button.TextActive = table.ColorText
	button.Padding = image.Point{2.0, 2.0}
	button.TouchPadding = image.Point{0.0, 0.0}
	button.Border = 0.0
	button.SymbolBorderWidth = 1
	button.Rounding = 0.0
	button.DrawBegin = nil
	button.DrawEnd = nil

	/* tab */
	tab = &style.Tab

	tab.Background = MakeItemColor(table.ColorTabHeader)
	tab.BorderColor = table.ColorBorder
	tab.Text = table.ColorText
	tab.SymMinimize = label.SymbolTriangleDown
	tab.SymMaximize = label.SymbolTriangleRight
	tab.Border = 1
	tab.Rounding = 0
	tab.Padding = image.Point{4, 4}
	tab.Spacing = image.Point{4, 4}

	/* tab button */
	button = &style.Tab.TabButton

	*button = Button{}
	button.Normal = MakeItemColor(table.ColorTabHeader)
	button.Hover = MakeItemColor(table.ColorTabHeader)
	button.Active = MakeItemColor(table.ColorTabHeader)
	button.BorderColor = color.RGBA{0, 0, 0, 0}
	button.TextBackground = table.ColorTabHeader
	button.TextNormal = table.ColorText
	button.TextHover = table.ColorText
	button.TextActive = table.ColorText
	button.Padding = image.Point{2.0, 2.0}
	button.TouchPadding = image.Point{0.0, 0.0}
	button.Border = 0.0
	button.SymbolBorderWidth = 2
	button.Rounding = 0.0
	button.DrawBegin = nil
	button.DrawEnd = nil

	/* node button */
	button = &style.Tab.NodeButton

	*button = Button{}
	button.Normal = MakeItemColor(table.ColorWindow)
	button.Hover = MakeItemColor(table.ColorWindow)
	button.Active = MakeItemColor(table.ColorWindow)
	button.BorderColor = color.RGBA{0, 0, 0, 0}
	button.TextBackground = table.ColorTabHeader
	button.TextNormal = table.ColorText
	button.TextHover = table.ColorText
	button.TextActive = table.ColorText
	button.Padding = image.Point{2.0, 2.0}
	button.TouchPadding = image.Point{0.0, 0.0}
	button.Border = 0.0
	button.SymbolBorderWidth = 2
	button.Rounding = 0.0
	button.DrawBegin = nil
	button.DrawEnd = nil

	/* window header */
	win = &style.NormalWindow

	win.Header.Align = HeaderRight
	win.Header.CloseSymbol = label.SymbolX
	win.Header.MinimizeSymbol = label.SymbolMinus
	win.Header.MaximizeSymbol = label.SymbolPlus
	win.Header.Normal = MakeItemColor(table.ColorHeader)
	win.Header.Hover = MakeItemColor(table.ColorHeader)
	win.Header.Active = MakeItemColor(table.ColorHeaderFocused)
	win.Header.LabelNormal = table.ColorText
	win.Header.LabelHover = table.ColorText
	win.Header.LabelActive = table.ColorText
	win.Header.LabelPadding = image.Point{2, 2}
	win.Header.Padding = image.Point{2, 2}
	win.Header.Spacing = image.Point{0, 0}

	/* window header close button */
	button = &style.NormalWindow.Header.CloseButton

	*button = Button{}
	button.Normal = MakeItemColor(table.ColorHeader)
	button.Hover = MakeItemColor(table.ColorHeader)
	button.Active = MakeItemColor(table.ColorHeaderFocused)
	button.BorderColor = color.RGBA{0, 0, 0, 0}
	button.TextBackground = table.ColorHeader
	button.TextNormal = table.ColorText
	button.TextHover = table.ColorText
	button.TextActive = table.ColorText
	button.Padding = image.Point{0.0, 0.0}
	button.TouchPadding = image.Point{0.0, 0.0}
	button.Border = 0.0
	button.SymbolBorderWidth = 1
	button.Rounding = 0.0
	button.DrawBegin = nil
	button.DrawEnd = nil

	/* window header minimize button */
	button = &style.NormalWindow.Header.MinimizeButton

	*button = Button{}
	button.Normal = MakeItemColor(table.ColorHeader)
	button.Hover = MakeItemColor(table.ColorHeader)
	button.Active = MakeItemColor(table.ColorHeaderFocused)
	button.BorderColor = color.RGBA{0, 0, 0, 0}
	button.TextBackground = table.ColorHeader
	button.TextNormal = table.ColorText
	button.TextHover = table.ColorText
	button.TextActive = table.ColorText
	button.Padding = image.Point{0.0, 0.0}
	button.TouchPadding = image.Point{0.0, 0.0}
	button.Border = 0.0
	button.SymbolBorderWidth = 1
	button.Rounding = 0.0
	button.DrawBegin = nil
	button.DrawEnd = nil

	/* window */
	win.Background = table.ColorWindow
	win.FixedBackground = MakeItemColor(table.ColorWindow)
	win.BorderColor = table.ColorBorder
	win.Scaler = MakeItemColor(table.ColorText)
	win.FooterPadding = image.Point{0, 0}
	win.Rounding = 0.0
	win.ScalerSize = image.Point{9, 9}
	win.Padding = image.Point{4, 4}
	win.Spacing = image.Point{4, 4}
	win.ScrollbarSize = image.Point{10, 10}
	win.MinSize = image.Point{64, 64}
	win.Border = 2.0

	style.MenuWindow = style.NormalWindow
	style.TooltipWindow = style.NormalWindow
	style.ComboWindow = style.NormalWindow
	style.ContextualWindow = style.NormalWindow
	style.GroupWindow = style.NormalWindow

	style.MenuWindow.BorderColor = table.ColorBorder
	style.MenuWindow.Border = 1
	style.MenuWindow.Spacing = image.Point{2, 2}

	style.TooltipWindow.BorderColor = table.ColorBorder
	style.TooltipWindow.Border = 1
	style.TooltipWindow.Padding = image.Point{2, 2}

	style.ComboWindow.BorderColor = table.ColorBorder
	style.ComboWindow.Border = 1

	style.ContextualWindow.BorderColor = table.ColorText
	style.ContextualWindow.Border = 1

	style.GroupWindow.BorderColor = table.ColorBorder
	style.GroupWindow.Border = 1
	style.GroupWindow.Padding = image.Point{2, 2}
	style.GroupWindow.Spacing = image.Point{2, 2}

	style.Scale(scaling)

	return style
}

func MakeItemImage(img *image.RGBA) Item {
	var i Item
	i.Type = ItemImage
	i.Data.Image = img
	return i
}

func MakeItemColor(col color.RGBA) Item {
	var i Item
	i.Type = ItemColor
	i.Data.Color = col
	return i
}

func ItemHide() Item {
	var i Item
	i.Type = ItemColor
	i.Data.Color = color.RGBA{0, 0, 0, 0}
	return i
}

type Theme int

const (
	DefaultTheme Theme = iota
	WhiteTheme
	RedTheme
	DarkTheme
)

var whiteThemeTable = ColorTable{
	ColorText:                  color.RGBA{70, 70, 70, 255},
	ColorWindow:                color.RGBA{175, 175, 175, 255},
	ColorHeader:                color.RGBA{175, 175, 175, 255},
	ColorHeaderFocused:         color.RGBA{0xc3, 0x9a, 0x9a, 255},
	ColorBorder:                color.RGBA{0, 0, 0, 255},
	ColorButton:                color.RGBA{185, 185, 185, 255},
	ColorButtonHover:           color.RGBA{170, 170, 170, 255},
	ColorButtonActive:          color.RGBA{160, 160, 160, 255},
	ColorToggle:                color.RGBA{150, 150, 150, 255},
	ColorToggleHover:           color.RGBA{120, 120, 120, 255},
	ColorToggleCursor:          color.RGBA{175, 175, 175, 255},
	ColorSelect:                color.RGBA{175, 175, 175, 255},
	ColorSelectActive:          color.RGBA{190, 190, 190, 255},
	ColorSlider:                color.RGBA{190, 190, 190, 255},
	ColorSliderCursor:          color.RGBA{80, 80, 80, 255},
	ColorSliderCursorHover:     color.RGBA{70, 70, 70, 255},
	ColorSliderCursorActive:    color.RGBA{60, 60, 60, 255},
	ColorProperty:              color.RGBA{175, 175, 175, 255},
	ColorEdit:                  color.RGBA{150, 150, 150, 255},
	ColorEditCursor:            color.RGBA{0, 0, 0, 255},
	ColorCombo:                 color.RGBA{175, 175, 175, 255},
	ColorChart:                 color.RGBA{160, 160, 160, 255},
	ColorChartColor:            color.RGBA{45, 45, 45, 255},
	ColorChartColorHighlight:   color.RGBA{255, 0, 0, 255},
	ColorScrollbar:             color.RGBA{180, 180, 180, 255},
	ColorScrollbarCursor:       color.RGBA{140, 140, 140, 255},
	ColorScrollbarCursorHover:  color.RGBA{150, 150, 150, 255},
	ColorScrollbarCursorActive: color.RGBA{160, 160, 160, 255},
	ColorTabHeader:             color.RGBA{180, 180, 180, 255},
}

var redThemeTable = ColorTable{
	ColorText:                  color.RGBA{190, 190, 190, 255},
	ColorWindow:                color.RGBA{30, 33, 40, 215},
	ColorHeader:                color.RGBA{181, 45, 69, 220},
	ColorHeaderFocused:         color.RGBA{0xb5, 0x0c, 0x2c, 0xdc},
	ColorBorder:                color.RGBA{51, 55, 67, 255},
	ColorButton:                color.RGBA{181, 45, 69, 255},
	ColorButtonHover:           color.RGBA{190, 50, 70, 255},
	ColorButtonActive:          color.RGBA{195, 55, 75, 255},
	ColorToggle:                color.RGBA{51, 55, 67, 255},
	ColorToggleHover:           color.RGBA{45, 60, 60, 255},
	ColorToggleCursor:          color.RGBA{181, 45, 69, 255},
	ColorSelect:                color.RGBA{51, 55, 67, 255},
	ColorSelectActive:          color.RGBA{181, 45, 69, 255},
	ColorSlider:                color.RGBA{51, 55, 67, 255},
	ColorSliderCursor:          color.RGBA{181, 45, 69, 255},
	ColorSliderCursorHover:     color.RGBA{186, 50, 74, 255},
	ColorSliderCursorActive:    color.RGBA{191, 55, 79, 255},
	ColorProperty:              color.RGBA{51, 55, 67, 255},
	ColorEdit:                  color.RGBA{51, 55, 67, 225},
	ColorEditCursor:            color.RGBA{190, 190, 190, 255},
	ColorCombo:                 color.RGBA{51, 55, 67, 255},
	ColorChart:                 color.RGBA{51, 55, 67, 255},
	ColorChartColor:            color.RGBA{170, 40, 60, 255},
	ColorChartColorHighlight:   color.RGBA{255, 0, 0, 255},
	ColorScrollbar:             color.RGBA{30, 33, 40, 255},
	ColorScrollbarCursor:       color.RGBA{64, 84, 95, 255},
	ColorScrollbarCursorHover:  color.RGBA{70, 90, 100, 255},
	ColorScrollbarCursorActive: color.RGBA{75, 95, 105, 255},
	ColorTabHeader:             color.RGBA{181, 45, 69, 220},
}

var darkThemeTable = ColorTable{
	ColorText:                  color.RGBA{210, 210, 210, 255},
	ColorWindow:                color.RGBA{57, 67, 71, 255},
	ColorHeader:                color.RGBA{51, 51, 56, 220},
	ColorHeaderFocused:         color.RGBA{0x29, 0x29, 0x37, 0xdc},
	ColorBorder:                color.RGBA{46, 46, 46, 255},
	ColorButton:                color.RGBA{48, 83, 111, 255},
	ColorButtonHover:           color.RGBA{58, 93, 121, 255},
	ColorButtonActive:          color.RGBA{63, 98, 126, 255},
	ColorToggle:                color.RGBA{50, 58, 61, 255},
	ColorToggleHover:           color.RGBA{45, 53, 56, 255},
	ColorToggleCursor:          color.RGBA{48, 83, 111, 255},
	ColorSelect:                color.RGBA{57, 67, 61, 255},
	ColorSelectActive:          color.RGBA{48, 83, 111, 255},
	ColorSlider:                color.RGBA{50, 58, 61, 255},
	ColorSliderCursor:          color.RGBA{48, 83, 111, 245},
	ColorSliderCursorHover:     color.RGBA{53, 88, 116, 255},
	ColorSliderCursorActive:    color.RGBA{58, 93, 121, 255},
	ColorProperty:              color.RGBA{50, 58, 61, 255},
	ColorEdit:                  color.RGBA{50, 58, 61, 225},
	ColorEditCursor:            color.RGBA{210, 210, 210, 255},
	ColorCombo:                 color.RGBA{50, 58, 61, 255},
	ColorChart:                 color.RGBA{50, 58, 61, 255},
	ColorChartColor:            color.RGBA{48, 83, 111, 255},
	ColorChartColorHighlight:   color.RGBA{255, 0, 0, 255},
	ColorScrollbar:             color.RGBA{50, 58, 61, 255},
	ColorScrollbarCursor:       color.RGBA{48, 83, 111, 255},
	ColorScrollbarCursorHover:  color.RGBA{53, 88, 116, 255},
	ColorScrollbarCursorActive: color.RGBA{58, 93, 121, 255},
	ColorTabHeader:             color.RGBA{48, 83, 111, 255},
}

func FromTheme(theme Theme, scaling float64) *Style {
	switch theme {
	case DefaultTheme:
		fallthrough
	default:
		return FromTable(defaultThemeTable, scaling)
	case WhiteTheme:
		return FromTable(whiteThemeTable, scaling)
	case RedTheme:
		return FromTable(redThemeTable, scaling)
	case DarkTheme:
		return FromTable(darkThemeTable, scaling)
	}
}

func (style *Style) Unscaled() *Style {
	if style.unscaled == nil {
		unscaled := &Style{}
		*unscaled = *style
		style.unscaled = unscaled
	}

	return style.unscaled
}

func (style *Style) Scale(scaling float64) {
	unscaled := style.unscaled
	if unscaled != nil {
		*style = *unscaled
		style.unscaled = unscaled
	} else {
		unscaled = &Style{}
		*unscaled = *style
		style.unscaled = unscaled
	}

	style.Scaling = scaling

	if style.Font == (font.Face{}) || style.defaultFont == style.Font {
		style.DefaultFont(scaling)
		style.defaultFont = style.Font
	} else {
		style.defaultFont = font.Face{}
	}

	style.unscaled.Font = style.Font
	style.unscaled.defaultFont = style.defaultFont

	if scaling == 1.0 {
		return
	}

	scale := func(x *int) {
		*x = int(float64(*x) * scaling)
	}

	scaleu := func(x *uint16) {
		*x = uint16(float64(*x) * scaling)
	}

	scalept := func(p *image.Point) {
		if scaling == 1.0 {
			return
		}
		scale(&p.X)
		scale(&p.Y)
	}

	scalebtn := func(button *Button) {
		scalept(&button.Padding)
		scalept(&button.ImagePadding)
		scalept(&button.TouchPadding)
		scale(&button.Border)
		scale(&button.SymbolBorderWidth)
		scaleu(&button.Rounding)
	}

	z := style

	scalept(&z.Text.Padding)

	scalebtn(&z.Button)
	scalebtn(&z.ContextualButton)
	scalebtn(&z.MenuButton)

	scalept(&z.Checkbox.Padding)
	scalept(&z.Checkbox.TouchPadding)

	scalept(&z.Option.Padding)
	scalept(&z.Option.TouchPadding)

	scalept(&z.Selectable.Padding)
	scalept(&z.Selectable.TouchPadding)
	scaleu(&z.Selectable.Rounding)

	scalept(&z.Slider.CursorSize)
	scalept(&z.Slider.Padding)
	scalept(&z.Slider.Spacing)
	scaleu(&z.Slider.Rounding)
	scale(&z.Slider.BarHeight)

	scalebtn(&z.Slider.IncButton)

	scalept(&z.Progress.Padding)
	scaleu(&z.Progress.Rounding)

	scalept(&z.Scrollh.Padding)
	scale(&z.Scrollh.Border)
	scaleu(&z.Scrollh.Rounding)

	scalebtn(&z.Scrollh.IncButton)
	scalebtn(&z.Scrollh.DecButton)
	scalebtn(&z.Scrollv.IncButton)
	scalebtn(&z.Scrollv.DecButton)

	scaleedit := func(edit *Edit) {
		scale(&edit.RowPadding)
		scalept(&edit.Padding)
		scalept(&edit.ScrollbarSize)
		scale(&edit.Border)
		scaleu(&edit.Rounding)
	}

	scaleedit(&z.Edit)

	scalept(&z.Property.Padding)
	scale(&z.Property.Border)
	scaleu(&z.Property.Rounding)

	scalebtn(&z.Property.IncButton)
	scalebtn(&z.Property.DecButton)

	scaleedit(&z.Property.Edit)

	scalept(&z.Combo.ContentPadding)
	scalept(&z.Combo.ButtonPadding)
	scalept(&z.Combo.Spacing)
	scale(&z.Combo.Border)
	scaleu(&z.Combo.Rounding)

	scalebtn(&z.Combo.Button)

	scale(&z.Tab.Border)
	scaleu(&z.Tab.Rounding)
	scalept(&z.Tab.Padding)
	scalept(&z.Tab.Spacing)

	scalebtn(&z.Tab.TabButton)
	scalebtn(&z.Tab.NodeButton)

	scalewin := func(win *Window) {
		scalept(&win.Header.Padding)
		scalept(&win.Header.Spacing)
		scalept(&win.Header.LabelPadding)
		scalebtn(&win.Header.CloseButton)
		scalebtn(&win.Header.MinimizeButton)
		scalept(&win.FooterPadding)
		scaleu(&win.Rounding)
		scalept(&win.ScalerSize)
		scalept(&win.Padding)
		scalept(&win.Spacing)
		scalept(&win.ScrollbarSize)
		scalept(&win.MinSize)
		scale(&win.Border)
	}

	scalewin(&z.NormalWindow)
	scalewin(&z.MenuWindow)
	scalewin(&z.TooltipWindow)
	scalewin(&z.ComboWindow)
	scalewin(&z.ContextualWindow)
	scalewin(&z.GroupWindow)
}

func (style *Style) DefaultFont(scaling float64) {
	style.Font = font.DefaultFont(12, scaling)
	style.defaultFont = style.Font
}

func (style *Style) Defaults() {
	if style.Scaling == 0.0 {
		style.Scaling = 1.0
	}
	if style.Font == (font.Face{}) {
		style.DefaultFont(style.Scaling)
	}
}
