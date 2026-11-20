// Package transport contains OS I/O shared across model drivers.
package transport

import (
	"context"
	"errors"
	"sync"

	"github.com/telesma-app/hid"
)

// Scan is created for each discovery pass. HID enumeration is lazy and shared.
type Scan struct {
	once      sync.Once
	endpoints []*hid.DeviceInfo
	err       error
}

func (s *Scan) HID() ([]*hid.DeviceInfo, error) {
	s.once.Do(func() {
		for info, err := range hid.Enumerate() {
			if info != nil {
				s.endpoints = append(s.endpoints, info)
			} else if err != nil {
				s.err = errors.Join(s.err, err)
			}
		}
	})
	return s.endpoints, s.err
}

// Exchange writes one HID output report, then waits for the model's input report.
// Non-matching traffic is skipped until the supplied context expires.
func Exchange(ctx context.Context, path string, request []byte, accept func([]byte) bool) error {
	d, err := hid.OpenPath(path)
	if err != nil {
		return err
	}
	defer d.Close()
	if _, err = d.Write(ctx, request); err != nil {
		return err
	}
	buf := make([]byte, 1024)
	for ctx.Err() == nil {
		n, err := d.Read(ctx, buf)
		if err != nil {
			return err
		}
		if accept(buf[:n]) {
			return nil
		}
	}
	return ctx.Err()
}
