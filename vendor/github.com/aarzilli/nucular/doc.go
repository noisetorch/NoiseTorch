/*
Nucular is an immediate mode GUI library for Go, its implementation is a partial source port of Nuklear [0] by Micha Mettke.
For a brief introduction to Immediate Mode GUI see [1]

Opening a Window

A window can be opened with the following three lines of code:

 wnd := nucular.NewMasterWindow(0, "Title", updatefn)
 wnd.SetStyle(style.FromTheme(style.DarkTheme, 1.0))
 wnd.Main()

The first line creates the MasterWindow object and sets its flags (usually 0 is fine) and updatefn as the update function.
Updatefn will be responsible for drawing the contents of the window and handling the GUI logic (see the "Window Update and layout" section).

The second line configures the theme, the font (passing nil will use the default font face) and the default scaling factor (see the "Scaling" section).

The third line opens the window and starts its event loop, updatefn will be called whenever the window needs to be redrawn, this is usually only in response to mouse and keyboard events, if you want the window to be redrawn you can also manually call wnd.Changed().

Window Update and layout

The update function is responsible for drawing the contents of the window as well as handling user events, this is usually done by calling methods of nucular.Window.

For example, drawing a simple text button is done with this code:

 if w.ButtonText("button caption") {
 	// code here only runs once every time the button is clicked
 }

Widgets are laid out left to right and top to bottom, each row has a layout that can be configured calling the methods of nucular.rowConstructor (an instance of which can be obtained by calling the `nucular.Window.Row` or `nucular.Window.RowScaled`). There are three main row layout modes:

 - Static: in this mode the columns of the row have a fixed, user defined, width. This row layout can be selected calling Static or StaticScaled

 - Dynamic: in this mode the columns of the row have a width proportional to the total width of the window. This row layout can be selected calling Dynamic, DynamicScaled or Ratio

 - Space: in this mode widgets are positioned and sized arbitrarily. This row layout can be selected calling SpaceBegin or SpaceBeginRatio, once this row layout is selected widgets can be positioned using LayoutSpacePush or LayoutSpacePushRatio

Scaling

When calling SetStyle you can specify a scaling factor, this will be used to scale the sizes in the style argument and also all the size arguments for the methods of rowConstructor.

Links

 [0] https://github.com/vurtun/nuklear/
 [1] https://mollyrocket.com/861
*/
package nucular
