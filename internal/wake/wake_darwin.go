//go:build darwin

package wake

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation
#include <IOKit/pwr_mgt/IOPMLib.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdlib.h>

static int holdNoDisplaySleep(const char *reason, IOPMAssertionID *outID) {
	CFStringRef rs = CFStringCreateWithCString(NULL, reason, kCFStringEncodingUTF8);
	if (!rs) return -1;
	IOReturn ret = IOPMAssertionCreateWithName(
		kIOPMAssertionTypeNoDisplaySleep,
		kIOPMAssertionLevelOn,
		rs,
		outID);
	CFRelease(rs);
	return (int)ret;
}
*/
import "C"

import (
	"sync"
)

var (
	mu      sync.Mutex
	assert  C.IOPMAssertionID
	held    bool
	reasonC = C.CString("RSVP Reader playing")
)

// Hold takes a NoDisplaySleep power assertion: the screen stays on as if
// video were playing. Idempotent; safe for headless test runs (a failing
// create simply leaves the lock unheld).
func Hold() {
	mu.Lock()
	defer mu.Unlock()
	if held {
		return
	}
	var id C.IOPMAssertionID
	if C.holdNoDisplaySleep(reasonC, &id) != 0 {
		return
	}
	assert, held = id, true
}

// Release drops the assertion. Idempotent; releasing an unheld lock is a
// no-op.
func Release() {
	mu.Lock()
	defer mu.Unlock()
	if !held {
		return
	}
	C.IOPMAssertionRelease(assert)
	held = false
}

// Held reports the lock state (for tests).
func Held() bool {
	mu.Lock()
	defer mu.Unlock()
	return held
}
