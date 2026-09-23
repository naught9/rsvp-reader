//go:build !darwin

package ui

// setCursorVisible is a no-op off macOS.
func setCursorVisible(bool) {}
