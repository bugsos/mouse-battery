//go:build !windows && !darwin

package ui

func isAutoStartEnabled() bool { return false }

func setAutoStart(bool) error { return nil }
