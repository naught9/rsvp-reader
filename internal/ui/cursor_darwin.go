//go:build darwin

package ui

/*
#cgo LDFLAGS: -framework CoreGraphics
#include <CoreGraphics/CoreGraphics.h>
*/
import "C"

// setCursorVisible shows or hides the system cursor. Calls must stay
// balanced: hide exactly once per show. All callers go through
// showChrome/hideChrome (plus onClose), which are already paired by the
// chromeHidden flag, so balance holds by construction.
//
// Edge: if the process died while hidden the cursor could stick. The
// only paths that hide run on the UI thread of a live app, and onClose
// restores unconditionally.
func setCursorVisible(visible bool) {
	if visible {
		C.CGDisplayShowCursor(C.CGMainDisplayID())
	} else {
		C.CGDisplayHideCursor(C.CGMainDisplayID())
	}
}
