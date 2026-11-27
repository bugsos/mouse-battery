//go:build windows

package ui

import (
	"testing"

	"golang.org/x/sys/windows/registry"
)

func TestWindowsAutoStartRegistryRoundTrip(t *testing.T) {
	const testKey = `Software\MouseBatteryAutoStartTest`
	const testVal = "MouseBattery"

	t.Cleanup(func() {
		_ = registry.DeleteKey(registry.CURRENT_USER, testKey)
	})

	exeA := `C:\Program Files\Mouse Battery\mouse-battery.exe`
	exeB := `D:\Tools\mouse-battery.exe`

	if isAutoStartEnabledInKey(registry.CURRENT_USER, testKey, testVal, exeA) {
		t.Fatalf("expected disabled before creation")
	}

	if err := setAutoStartInKey(registry.CURRENT_USER, testKey, testVal, exeA, true); err != nil {
		t.Fatalf("enable failed: %v", err)
	}
	if !isAutoStartEnabledInKey(registry.CURRENT_USER, testKey, testVal, exeA) {
		t.Fatalf("expected enabled for exeA")
	}
	// Case-insensitive match on Windows paths.
	if !isAutoStartEnabledInKey(registry.CURRENT_USER, testKey, testVal, `c:\program files\mouse battery\mouse-battery.exe`) {
		t.Fatalf("expected case-insensitive path match")
	}
	// Different path should report false so user can update it cleanly.
	if isAutoStartEnabledInKey(registry.CURRENT_USER, testKey, testVal, exeB) {
		t.Fatalf("expected false for a different executable path")
	}

	// Updating to exeB replaces the entry.
	if err := setAutoStartInKey(registry.CURRENT_USER, testKey, testVal, exeB, true); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if !isAutoStartEnabledInKey(registry.CURRENT_USER, testKey, testVal, exeB) {
		t.Fatalf("expected enabled for exeB after update")
	}

	// Disabling removes the value and is idempotent.
	if err := setAutoStartInKey(registry.CURRENT_USER, testKey, testVal, exeB, false); err != nil {
		t.Fatalf("disable failed: %v", err)
	}
	if isAutoStartEnabledInKey(registry.CURRENT_USER, testKey, testVal, exeB) {
		t.Fatalf("expected disabled after removal")
	}
	if err := setAutoStartInKey(registry.CURRENT_USER, testKey, testVal, exeB, false); err != nil {
		t.Fatalf("second disable should be idempotent, got: %v", err)
	}
}

func TestParseRunCommandPath(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`"C:\Program Files\Mouse Battery\mouse-battery.exe"`, `C:\Program Files\Mouse Battery\mouse-battery.exe`},
		{`"C:\Tools\app.exe" --flag`, `C:\Tools\app.exe`},
		{`C:\Tools\app.exe`, `C:\Tools\app.exe`},
		{"", ""},
	}
	for _, tc := range cases {
		if got := parseRunCommandPath(tc.in); got != tc.want {
			t.Fatalf("parseRunCommandPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
