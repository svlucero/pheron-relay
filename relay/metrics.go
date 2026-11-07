package relay

import "sync"

// MetricsSnapshot contains aggregate operational values only. It deliberately
// has no labels for locators, instance IDs, addresses, tickets, or payloads.
type MetricsSnapshot struct {
	DesktopChannels int64
	MobileStreams   int64
	Draining        bool
	Events          map[string]uint64
}

type metrics struct {
	mu     sync.Mutex
	events map[string]uint64
}

func newMetrics() *metrics { return &metrics{events: make(map[string]uint64)} }

func (m *metrics) add(event string) {
	m.mu.Lock()
	m.events[event]++
	m.mu.Unlock()
}

func (m *metrics) snapshot() map[string]uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make(map[string]uint64, len(m.events))
	for key, value := range m.events {
		result[key] = value
	}
	return result
}
