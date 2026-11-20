// Package drivers collects built-in model drivers. Each model registers its own discovery function.
package drivers

import (
	"context"
	"errors"

	"mouse-battery/internal/battery"
	"mouse-battery/internal/transport"
)

type discoverFunc func(context.Context, *transport.Scan) ([]battery.Device, error)

var installed []discoverFunc

func register(driver discoverFunc) { installed = append(installed, driver) }

func Discover(ctx context.Context) ([]battery.Device, error) {
	var devices []battery.Device
	var combined error
	scan := new(transport.Scan)
	for _, discover := range installed {
		if err := ctx.Err(); err != nil {
			return devices, err
		}
		found, err := discover(ctx, scan)
		devices = append(devices, found...)
		combined = errors.Join(combined, err)
	}
	return devices, combined
}
