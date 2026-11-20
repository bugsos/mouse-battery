// Package battery defines devices and the state shared by all displays.
package battery

import (
	"context"
	"fmt"
)

// Device describes one identifiable mouse. ID is an instance key, not a model name.
// Read is bound to this device and may reuse a sample obtained during discovery.
type Device struct {
	ID, Name, Connection string
	Read                 func(context.Context) (int, error)
}

func (d Device) ReadBattery(ctx context.Context) (int, error) {
	n, err := d.Read(ctx)
	if err == nil && (n < 0 || n > 100) {
		return 0, fmt.Errorf("%s returned invalid battery level %d", d.Name, n)
	}
	return n, err
}

type Discover func(context.Context) ([]Device, error)

type Status uint8

const (
	Reading Status = iota + 1
	Ready
	Disconnected
	Unavailable
)

type Entry struct{ ID, Name, Connection string }

type Snapshot struct {
	Devices  []Entry
	Selected Entry
	Status   Status
	Percent  int // meaningful only when Status == Ready (including 0%)
}
