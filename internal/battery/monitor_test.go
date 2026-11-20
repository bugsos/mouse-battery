package battery

import (
	"context"
	"errors"
	"testing"
)

func TestSelectedMouseAcrossDisconnectAndReadFailure(t *testing.T) {
	ctx := context.Background()
	a := Device{ID: "a", Name: "Twin", Read: func(context.Context) (int, error) { return 84, nil }}
	b := Device{ID: "b", Name: "Twin", Read: func(context.Context) (int, error) { return 0, nil }}
	devices := []Device{a, b}
	m := NewMonitor(func(context.Context) ([]Device, error) { return devices, nil })
	selected := Entry{ID: "b", Name: "Twin"}
	read := func() Snapshot { s, _ := m.sample(ctx, selected); return s }
	if s := read(); s.Status != Ready || s.Percent != 0 || s.Selected.ID != "b" {
		t.Fatalf("selected empty battery: %+v", s)
	}
	devices = []Device{a}
	if s := read(); s.Status != Disconnected || s.Selected.ID != "b" {
		t.Fatalf("disconnected selection: %+v", s)
	}
	b.Read = func(context.Context) (int, error) { return 0, errors.New("timeout") }
	devices = []Device{a, b}
	if s := read(); s.Status != Unavailable {
		t.Fatalf("failed read: %+v", s)
	}
}
