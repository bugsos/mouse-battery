package ui

import (
	"image/color"
	"testing"

	"mouse-battery/internal/battery"
)

func TestDisplayFormattingAndTone(t *testing.T) {
	cases := []struct {
		name     string
		snap     battery.Snapshot
		desc     string
		digits   string
		badge    string
		wantTone color.RGBA
	}{
		{
			name:     "reading without device",
			snap:     battery.Snapshot{Status: battery.Reading},
			desc:     "鼠标电量 · 读取中",
			digits:   "—",
			badge:    "—",
			wantTone: color.RGBA{103, 113, 122, 255},
		},
		{
			name:     "critical red below 10",
			snap:     battery.Snapshot{Selected: battery.Entry{Name: "VK M3 Lite"}, Status: battery.Ready, Percent: 9},
			desc:     "VK M3 Lite · 9%",
			digits:   "9",
			badge:    "9%",
			wantTone: color.RGBA{190, 44, 42, 255},
		},
		{
			name:     "warning yellow 10 to 19",
			snap:     battery.Snapshot{Selected: battery.Entry{Name: "VK M3 Lite"}, Status: battery.Ready, Percent: 10},
			desc:     "VK M3 Lite · 10%",
			digits:   "10",
			badge:    "10%",
			wantTone: color.RGBA{159, 105, 0, 255},
		},
		{
			name:     "healthy green 20 and above",
			snap:     battery.Snapshot{Selected: battery.Entry{Name: "VK M3 Lite"}, Status: battery.Ready, Percent: 100},
			desc:     "VK M3 Lite · 100%",
			digits:   "100",
			badge:    "100%",
			wantTone: color.RGBA{24, 129, 67, 255},
		},
		{
			name:     "disconnected",
			snap:     battery.Snapshot{Selected: battery.Entry{Name: "VK M3 Lite"}, Status: battery.Disconnected},
			desc:     "VK M3 Lite · 未连接",
			digits:   "—",
			badge:    "—",
			wantTone: color.RGBA{103, 113, 122, 255},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Description(tc.snap); got != tc.desc {
				t.Fatalf("Description() = %q, want %q", got, tc.desc)
			}
			if got := digits(tc.snap); got != tc.digits {
				t.Fatalf("digits() = %q, want %q", got, tc.digits)
			}
			if got := badgeText(tc.snap); got != tc.badge {
				t.Fatalf("badgeText() = %q, want %q", got, tc.badge)
			}
			if got := tone(tc.snap); got != tc.wantTone {
				t.Fatalf("tone() = %v, want %v", got, tc.wantTone)
			}
		})
	}
}

func TestBuildViewStateDuplicateDevicesAndMenus(t *testing.T) {
	dev1 := battery.Entry{ID: "usb-1", Name: "VK M3 Lite", Connection: "2.4G"}
	dev2 := battery.Entry{ID: "usb-2", Name: "VK M3 Lite", Connection: "2.4G"}
	dev3 := battery.Entry{ID: "usb-3", Name: "VK M3 Lite", Connection: "有线"}

	snap := battery.Snapshot{
		Devices:  []battery.Entry{dev1, dev2, dev3},
		Selected: dev2,
		Status:   battery.Ready,
		Percent:  85,
	}

	vs := buildViewState(snap, uiOptions{
		hasOverlay:     true,
		overlayVisible: true,
		hasAutoStart:   true,
		autoStart:      true,
		hasShortcut:    true,
		shortcut:       true,
	})
	if vs.Tooltip != "VK M3 Lite · 85%" {
		t.Fatalf("Tooltip = %q", vs.Tooltip)
	}
	if vs.TrayText != "85" || vs.BadgeText != "85%" {
		t.Fatalf("TrayText=%q BadgeText=%q", vs.TrayText, vs.BadgeText)
	}

	var radios []MenuItem
	var hasOverlayToggle, hasTrayAutoStart, hasBallAutoStart, hasTrayShortcut, hasBallShortcut bool
	for _, item := range vs.TrayMenu {
		if item.Kind == MenuRadio {
			radios = append(radios, item)
		}
		if item.Kind == MenuCheck && item.Action == ActionToggleOverlay {
			hasOverlayToggle = true
			if !item.Checked {
				t.Fatalf("expected overlay toggle checked")
			}
		}
		if item.Kind == MenuCheck && item.Action == ActionToggleAutoStart {
			hasTrayAutoStart = true
			if !item.Checked {
				t.Fatalf("expected tray auto-start checked")
			}
		}
		if item.Kind == MenuCheck && item.Action == ActionToggleShortcut {
			hasTrayShortcut = true
			if !item.Checked {
				t.Fatalf("expected tray shortcut checked")
			}
		}
	}
	if len(radios) != 3 {
		t.Fatalf("got %d radio items, want 3", len(radios))
	}
	if radios[0].Label != "VK M3 Lite · 2.4G (1)" || radios[0].Checked {
		t.Fatalf("radio[0] = %+v", radios[0])
	}
	if radios[1].Label != "VK M3 Lite · 2.4G (2)" || !radios[1].Checked {
		t.Fatalf("radio[1] = %+v", radios[1])
	}
	if radios[2].Label != "VK M3 Lite · 有线" || radios[2].Checked {
		t.Fatalf("radio[2] = %+v", radios[2])
	}
	if !hasOverlayToggle {
		t.Fatalf("missing overlay toggle menu item")
	}
	if !hasTrayAutoStart {
		t.Fatalf("missing tray auto-start menu item")
	}
	if !hasTrayShortcut {
		t.Fatalf("missing tray shortcut menu item")
	}
	if len(vs.BallMenu) != 5 {
		t.Fatalf("BallMenu len = %d, want 5", len(vs.BallMenu))
	}
	for _, item := range vs.BallMenu {
		if item.Kind == MenuCheck && item.Action == ActionToggleAutoStart && item.Checked {
			hasBallAutoStart = true
		}
		if item.Kind == MenuCheck && item.Action == ActionToggleShortcut && item.Checked {
			hasBallShortcut = true
		}
	}
	if !hasBallAutoStart {
		t.Fatalf("missing checked auto-start item in BallMenu")
	}
	if !hasBallShortcut {
		t.Fatalf("missing checked shortcut item in BallMenu")
	}

	// Verify macOS mode (hasOverlay = false, hasAutoStart = true)
	macVS := buildViewState(snap, uiOptions{hasAutoStart: true, autoStart: false})
	if len(macVS.BallMenu) != 0 {
		t.Fatalf("macOS should have empty BallMenu")
	}
	var hasMacAutoStart bool
	for _, item := range macVS.TrayMenu {
		if item.Action == ActionToggleOverlay {
			t.Fatalf("macOS tray menu should not have overlay toggle")
		}
		if item.Kind == MenuCheck && item.Action == ActionToggleAutoStart {
			hasMacAutoStart = true
			if item.Checked {
				t.Fatalf("expected macOS auto-start unchecked")
			}
		}
	}
	if !hasMacAutoStart {
		t.Fatalf("missing macOS auto-start menu item")
	}
}

func TestSnapAxisMultiMonitor(t *testing.T) {
	// Primary monitor: [0, 1920), window width 56, threshold 12
	if got := snapAxis(10, 56, 0, 1920, 12); got != 0 {
		t.Fatalf("near left edge: got %d, want 0", got)
	}
	if got := snapAxis(1855, 56, 0, 1920, 12); got != 1864 {
		t.Fatalf("near right edge: got %d, want 1864", got)
	}
	if got := snapAxis(500, 56, 0, 1920, 12); got != 500 {
		t.Fatalf("middle: got %d, want 500", got)
	}

	// Secondary monitor to the left: [-1920, 0), threshold 18 (150% DPI)
	if got := snapAxis(-1905, 84, -1920, 1920, 18); got != -1920 {
		t.Fatalf("negative left edge: got %d, want -1920", got)
	}
	if got := snapAxis(-95, 84, -1920, 1920, 18); got != -84 {
		t.Fatalf("negative right edge: got %d, want -84", got)
	}
}
