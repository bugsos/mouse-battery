//go:build !windows

package ui

func isDesktopShortcutEnabled() bool { return false }

func setDesktopShortcut(bool) error { return nil }
