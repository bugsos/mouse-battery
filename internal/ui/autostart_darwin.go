//go:build darwin

package ui

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const darwinLaunchAgentLabel = "app.mousebattery.desktop"

func darwinLaunchAgentPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", darwinLaunchAgentLabel+".plist"), nil
}

func darwinExecutablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Clean(exe), nil
}

func isAutoStartEnabled() bool {
	plistPath, err := darwinLaunchAgentPath()
	if err != nil {
		return false
	}
	exe, err := darwinExecutablePath()
	if err != nil {
		return false
	}
	data, err := os.ReadFile(plistPath)
	if err != nil {
		return false
	}
	var escaped bytes.Buffer
	if err := xml.EscapeText(&escaped, []byte(exe)); err != nil {
		return false
	}
	return strings.Contains(string(data), "<string>"+escaped.String()+"</string>")
}

func setAutoStart(enable bool) error {
	plistPath, err := darwinLaunchAgentPath()
	if err != nil {
		return err
	}
	if !enable {
		if err := os.Remove(plistPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}

	exe, err := darwinExecutablePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
		return err
	}

	var escaped bytes.Buffer
	if err := xml.EscapeText(&escaped, []byte(exe)); err != nil {
		return err
	}
	content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`, darwinLaunchAgentLabel, escaped.String())

	return os.WriteFile(plistPath, []byte(content), 0o644)
}
