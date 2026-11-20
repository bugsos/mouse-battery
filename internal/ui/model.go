package ui

import (
	"fmt"
	"image/color"

	"mouse-battery/internal/battery"
)

type MenuItemKind uint8

const (
	MenuLabel MenuItemKind = iota
	MenuSeparator
	MenuRadio
	MenuCheck
	MenuAction
)

type ActionKind uint8

const (
	ActionNone ActionKind = iota
	ActionSelect
	ActionRefresh
	ActionToggleOverlay
	ActionHideOverlay
	ActionQuit
)

const firstMenuID uint32 = 1001

type MenuItem struct {
	ID      uint32
	Kind    MenuItemKind
	Label   string
	Checked bool
	Action  ActionKind
	Device  battery.Entry
}

type ViewState struct {
	Tooltip   string
	TrayText  string
	BadgeText string
	Tone      color.RGBA
	TrayMenu  []MenuItem
	BallMenu  []MenuItem
}

func buildViewState(s battery.Snapshot, hasOverlay, overlayVisible bool) ViewState {
	nextID := firstMenuID
	allocID := func() uint32 {
		id := nextID
		nextID++
		return id
	}

	trayMenu := []MenuItem{
		{ID: allocID(), Kind: MenuLabel, Label: Description(s)},
	}
	if len(s.Devices) > 0 {
		trayMenu = append(trayMenu, MenuItem{Kind: MenuSeparator})
		counts := make(map[string]int, len(s.Devices))
		for _, e := range s.Devices {
			counts[deviceLabel(e)]++
		}
		seen := make(map[string]int, len(s.Devices))
		for _, e := range s.Devices {
			label := deviceLabel(e)
			if counts[label] > 1 {
				seen[label]++
				label = fmt.Sprintf("%s (%d)", label, seen[label])
			}
			trayMenu = append(trayMenu, MenuItem{
				ID:      allocID(),
				Kind:    MenuRadio,
				Label:   label,
				Checked: e.ID == s.Selected.ID,
				Action:  ActionSelect,
				Device:  e,
			})
		}
	}
	trayMenu = append(trayMenu,
		MenuItem{Kind: MenuSeparator},
		MenuItem{ID: allocID(), Kind: MenuAction, Label: "立即刷新", Action: ActionRefresh},
	)
	if hasOverlay {
		trayMenu = append(trayMenu, MenuItem{
			ID:      allocID(),
			Kind:    MenuCheck,
			Label:   "桌面悬浮球",
			Checked: overlayVisible,
			Action:  ActionToggleOverlay,
		})
	}
	trayMenu = append(trayMenu, MenuItem{ID: allocID(), Kind: MenuAction, Label: "退出", Action: ActionQuit})

	var ballMenu []MenuItem
	if hasOverlay {
		ballMenu = []MenuItem{
			{ID: allocID(), Kind: MenuAction, Label: "立即刷新", Action: ActionRefresh},
			{ID: allocID(), Kind: MenuAction, Label: "隐藏悬浮球", Action: ActionHideOverlay},
			{ID: allocID(), Kind: MenuAction, Label: "退出", Action: ActionQuit},
		}
	}

	return ViewState{
		Tooltip:   Description(s),
		TrayText:  digits(s),
		BadgeText: badgeText(s),
		Tone:      tone(s),
		TrayMenu:  trayMenu,
		BallMenu:  ballMenu,
	}
}

func findMenuItem(items []MenuItem, id uint32) (MenuItem, bool) {
	if id == 0 {
		return MenuItem{}, false
	}
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return MenuItem{}, false
}

func deviceLabel(e battery.Entry) string {
	if e.Connection == "" {
		return e.Name
	}
	return e.Name + " · " + e.Connection
}

func snapAxis(pos, size, start, extent, threshold int) int {
	end := start + extent - size
	if absInt(pos-start) <= threshold {
		return start
	}
	if absInt(end-pos) <= threshold {
		return end
	}
	return pos
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
