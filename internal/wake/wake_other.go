//go:build !darwin

package wake

// Hold is a no-op off macOS.
func Hold() {}

// Release is a no-op off macOS.
func Release() {}

// Held always reports false off macOS.
func Held() bool { return false }
