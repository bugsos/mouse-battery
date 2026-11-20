package drivers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/telesma-app/hid"

	"mouse-battery/internal/battery"
	"mouse-battery/internal/transport"
)

func init() {
	driver := &vkM3Lite{}
	register(driver.discover)
}

// A plugged-in receiver alone does not identify the paired mouse.
type vkM3Lite struct{ verified map[string]bool }

func vkConnection(info *hid.DeviceInfo) string {
	if info.VendorID.Value == nil || info.ProductID.Value == nil {
		return ""
	}
	vid, pid := *info.VendorID.Value, *info.ProductID.Value
	control := strings.Contains(strings.ToLower(info.Path), "&mi_02") ||
		(info.UsagePage == 1 && info.Usage == 0) || (info.UsagePage == 0xff01 && info.Usage == 0x10)
	if !control {
		return ""
	}
	switch {
	case vid == 0x248a && (pid == 0x5d2e || pid == 0x5c2e):
		return "USB"
	case vid == 0x249a && pid == 0x5c2f:
		return "2.4 GHz"
	}
	return ""
}

var errDifferentVK = errors.New("receiver reported a different mouse model")

func vkPercent(report []byte) (int, bool, error) {
	body := bytes.TrimPrefix(report, []byte{0})
	if len(body) < 14 || body[0] != 0x10 || body[1] != 0 {
		return 0, false, nil
	}
	if string(body[4:8]) != "M220" {
		return 0, true, errDifferentVK
	}
	if body[13] > 100 {
		return 0, true, fmt.Errorf("VK M3 Lite 电量字段无效 (%d)", body[13])
	}
	return int(body[13]), true, nil
}

func readVK(ctx context.Context, path string) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 700*time.Millisecond)
	defer cancel()
	req := make([]byte, 33)
	req[1] = 0x10
	var percent int
	var replyErr error
	err := transport.Exchange(ctx, path, req, func(report []byte) bool {
		var matched bool
		percent, matched, replyErr = vkPercent(report)
		return matched
	})
	return percent, errors.Join(err, replyErr)
}

func (v *vkM3Lite) discover(ctx context.Context, scan *transport.Scan) ([]battery.Device, error) {
	infos, err := scan.HID()
	next := make(map[string]bool)
	var found []battery.Device
	var probeErr error
	for _, info := range infos {
		connection := vkConnection(info)
		if connection == "" {
			continue
		}
		path := info.Path
		percent, readErr := readVK(ctx, path)
		if errors.Is(readErr, errDifferentVK) {
			continue
		}
		if readErr != nil && !v.verified[path] {
			probeErr = errors.Join(probeErr, readErr)
			continue
		}
		next[path] = true
		found = append(found, battery.Device{
			ID: "vk-m3-lite/" + path, Name: "VK M3 Lite", Connection: connection,
			// This protocol returns model identity and charge in the same reply.
			Read: func(context.Context) (int, error) { return percent, readErr },
		})
	}
	v.verified = next
	sort.Slice(found, func(i, j int) bool {
		if found[i].Connection != found[j].Connection {
			return found[i].Connection == "USB"
		}
		return found[i].ID < found[j].ID
	})
	return found, errors.Join(err, probeErr)
}
