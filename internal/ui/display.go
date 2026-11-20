package ui

import (
	"fmt"
	"image/color"

	"mouse-battery/internal/battery"
)

func Description(s battery.Snapshot) string {
	name := s.Selected.Name
	if name == "" {
		name = "鼠标电量"
	}
	switch s.Status {
	case battery.Reading:
		return name + " · 读取中"
	case battery.Ready:
		return fmt.Sprintf("%s · %d%%", name, s.Percent)
	case battery.Unavailable:
		return name + " · 暂不可读"
	default:
		return name + " · 未连接"
	}
}

func digits(s battery.Snapshot) string {
	if s.Status == battery.Ready {
		return fmt.Sprint(s.Percent)
	}
	return "—"
}

func badgeText(s battery.Snapshot) string {
	if s.Status == battery.Ready {
		return fmt.Sprintf("%d%%", s.Percent)
	}
	return "—"
}

func tone(s battery.Snapshot) color.RGBA {
	switch {
	case s.Status != battery.Ready:
		return color.RGBA{103, 113, 122, 255}
	case s.Percent < 10:
		return color.RGBA{190, 44, 42, 255}
	case s.Percent < 20:
		return color.RGBA{159, 105, 0, 255}
	default:
		return color.RGBA{24, 129, 67, 255}
	}
}
