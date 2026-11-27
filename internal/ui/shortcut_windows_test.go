//go:build windows

package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsDesktopShortcutRoundTrip(t *testing.T) {
	dir := t.TempDir()
	// Use real temporary files as targets because Windows ShellLink resolves existing paths cleanly.
	exeA := filepath.Join(dir, "a", "mouse-battery.exe")
	exeB := filepath.Join(dir, "b", "mouse-battery.exe")
	for _, p := range []string{exeA, exeB} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("stub"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	desktop := filepath.Join(dir, "Desktop")
	if err := os.MkdirAll(desktop, 0o755); err != nil {
		t.Fatal(err)
	}

	if isShortcutPresentInDir(desktop, exeA) {
		t.Fatalf("expected shortcut absent initially")
	}

	if err := setShortcutInDir(desktop, exeA, true); err != nil {
		t.Fatalf("create shortcut failed: %v", err)
	}
	if !isShortcutPresentInDir(desktop, exeA) {
		t.Fatalf("expected shortcut present for exeA")
	}
	if isShortcutPresentInDir(desktop, exeB) {
		t.Fatalf("expected shortcut false for different exeB")
	}

	// Updating shortcut to exeB overwrites cleanly.
	if err := setShortcutInDir(desktop, exeB, true); err != nil {
		t.Fatalf("update shortcut failed: %v", err)
	}
	if !isShortcutPresentInDir(desktop, exeB) {
		t.Fatalf("expected shortcut present for exeB after update")
	}

	// Removing shortcut deletes the .lnk file and is idempotent.
	if err := setShortcutInDir(desktop, exeB, false); err != nil {
		t.Fatalf("remove shortcut failed: %v", err)
	}
	if isShortcutPresentInDir(desktop, exeB) {
		t.Fatalf("expected shortcut absent after removal")
	}
	if err := setShortcutInDir(desktop, exeB, false); err != nil {
		t.Fatalf("second remove should be idempotent, got: %v", err)
	}
}
