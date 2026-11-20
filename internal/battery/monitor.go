package battery

import (
	"context"
	"slices"
	"sync"
	"time"
)

// Monitor serializes device I/O. Select and Refresh only wake the worker.
type Monitor struct {
	discover   Discover
	wake       chan struct{}
	mu         sync.Mutex
	chosen     Entry
	generation uint64
}

func NewMonitor(discover Discover) *Monitor {
	return &Monitor{discover: discover, wake: make(chan struct{}, 1)}
}

func (m *Monitor) Refresh() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

func (m *Monitor) Select(device Entry) {
	m.mu.Lock()
	m.chosen = device
	m.generation++
	m.mu.Unlock()
	m.Refresh()
}

func (m *Monitor) sample(ctx context.Context, selected Entry) (Snapshot, error) {
	devices, err := m.discover(ctx)
	snapshot := Snapshot{Selected: selected, Status: Disconnected}
	for _, device := range devices {
		snapshot.Devices = append(snapshot.Devices, Entry{device.ID, device.Name, device.Connection})
	}
	// Without a manual selection, use the first device in the driver's preferred order.
	if snapshot.Selected.ID == "" && len(snapshot.Devices) > 0 {
		snapshot.Selected = snapshot.Devices[0]
	}
	if err != nil && len(devices) == 0 {
		snapshot.Status = Unavailable
	}
	for i, device := range devices {
		if device.ID != snapshot.Selected.ID {
			continue
		}
		snapshot.Selected = snapshot.Devices[i]
		n, readErr := device.ReadBattery(ctx)
		if readErr != nil {
			snapshot.Status = Unavailable
			return snapshot, readErr
		}
		snapshot.Status, snapshot.Percent = Ready, n
		break
	}
	return snapshot, err
}

func (m *Monitor) ReadOnce(ctx context.Context) (Snapshot, error) { return m.sample(ctx, Entry{}) }

func (m *Monitor) Run(ctx context.Context, publish func(Snapshot)) {
	for ctx.Err() == nil {
		m.mu.Lock()
		selected, generation := m.chosen, m.generation
		m.mu.Unlock()
		next, _ := m.sample(ctx, selected)
		m.mu.Lock()
		current := generation == m.generation
		m.mu.Unlock()
		if current && ctx.Err() == nil {
			publish(next)
		}
		timer := time.NewTimer(20 * time.Second)
		select {
		case <-ctx.Done():
		case <-m.wake:
		case <-timer.C:
		}
		timer.Stop()
	}
}

func (s Snapshot) Same(other Snapshot) bool {
	return s.Selected == other.Selected && s.Status == other.Status && s.Percent == other.Percent && slices.Equal(s.Devices, other.Devices)
}
