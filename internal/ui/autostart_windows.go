//go:build windows

package ui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	windowsRunKeyPath   = `Software\Microsoft\Windows\CurrentVersion\Run`
	windowsRunValueName = "MouseBattery"
)

func currentExecutablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Clean(exe), nil
}

func formatRunCommand(exe string) string {
	return fmt.Sprintf(`"%s"`, filepath.Clean(exe))
}

func parseRunCommandPath(cmd string) string {
	s := strings.TrimSpace(cmd)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, `"`) {
		if end := strings.Index(s[1:], `"`); end >= 0 {
			return filepath.Clean(s[1 : 1+end])
		}
		return filepath.Clean(strings.Trim(s, `"`))
	}
	return filepath.Clean(s)
}

func isAutoStartEnabled() bool {
	exe, err := currentExecutablePath()
	if err != nil {
		return false
	}
	return isAutoStartEnabledInKey(registry.CURRENT_USER, windowsRunKeyPath, windowsRunValueName, exe)
}

func setAutoStart(enable bool) error {
	exe, err := currentExecutablePath()
	if err != nil {
		return err
	}
	return setAutoStartInKey(registry.CURRENT_USER, windowsRunKeyPath, windowsRunValueName, exe, enable)
}

func isAutoStartEnabledInKey(root registry.Key, subkey, name, exe string) bool {
	k, err := registry.OpenKey(root, subkey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	val, _, err := k.GetStringValue(name)
	if err != nil {
		return false
	}
	return strings.EqualFold(parseRunCommandPath(val), filepath.Clean(exe))
}

func setAutoStartInKey(root registry.Key, subkey, name, exe string, enable bool) error {
	if enable {
		k, _, err := registry.CreateKey(root, subkey, registry.SET_VALUE)
		if err != nil {
			return err
		}
		defer k.Close()
		return k.SetStringValue(name, formatRunCommand(exe))
	}

	k, err := registry.OpenKey(root, subkey, registry.SET_VALUE)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		return err
	}
	defer k.Close()

	if err := k.DeleteValue(name); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	return nil
}
