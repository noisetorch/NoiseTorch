/*
Package icccm provides an API for a portion of the ICCCM, namely, getters
and setters for many of the properties specified in the ICCCM. There is also a
smattering of support for other protocols specified by ICCCM. For example, to
satisfy the WM_DELETE_WINDOW protocol, package icccm provides 'IsDeleteProtocol'
which returns whether a ClientMessage event satisfies the WM_DELETE_WINDOW
protocol.

If a property has values that aren't simple strings or integers, struct types
are provided to organize the data. In particular, WM_NORMAL_HINTS and WM_HINTS.

Also note that properties like WM_NORMAL_HINTS and WM_HINTS contain a 'Flags'
field (a bit mask) that specifies which values are "active". This is of
importance when setting and reading WM_NORMAL_HINTS and WM_HINTS; one must make
sure the appropriate bit is set in Flags.

For example, you might want to check if a window has specified a resize
increment in the WM_NORMAL_HINTS property. The values in the corresponding
NormalHints struct are WidthInc and HeightInc. So to check if such values exist
*and* should be used:

	normalHints, err := icccm.WmNormalHintsGet(XUtilValue, window-id)
	if err != nil {
		// handle error
	}
	if normalHints.Flags&icccm.SizeHintPResizeInc > 0 {
		// Use normalHints.WidthInc and normalHints.HeightInc
	}

When you should use icccm

Although the ICCCM is extremely old, a lot of it is still used. In fact, the
EWMH spec itself specifically states that the ICCCM should still be used unless
otherwise noted by the EWMH. For example, WM_HINTS and WM_NORMAL_HINTS are
still used, but _NET_WM_NAME replaces WM_NAME.

With that said, many applications (like xterm or LibreOffice) have not been
updated to be fully EWMH compliant. Therefore, code that finds a window's name
often looks like this:

	winName, err := ewmh.WmNameGet(XUtilValue, window-id)
	if err != nil || winName == "" {
		winName, err = icccm.WmNameGet(XUtilValue, window-id)
		if err != nill || winName == "" {
			winName = "N/A"
		}
	}

Something similar can be said for the _NET_WM_ICON and the IconPixmap field
in WM_HINTS.

Naming scheme

The naming scheme is precisely the same as the one found in the ewmh package.
The documentation for the ewmh package describes the naming scheme in more
detail. The only difference (currently) is that the icccm package only contains
functions ending in "Get" and "Set". It is planned to add "Req" functions. (An
example of a Req function would be to send a ClientMessage implementing the
WM_DELETE_WINDOW protocol to a client window.)
*/
package icccm
