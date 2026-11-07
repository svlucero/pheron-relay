package relay

import (
	"strings"
	"sync"
	"time"
)

// Route is the opaque, live routing record used by the data plane. A durable
// implementation must make Claim atomic across replicas.
type Route struct {
	InstanceID   string
	Locator      string
	ConnectionID string
	ExpiresAt    time.Time
}

// Store is the shared-state boundary required for a multi-replica deployment.
// The in-memory implementation is only appropriate for a single process/tests.
type Store interface {
	Claim(Route) error
	Resolve(locator string, now time.Time) (Route, error)
	Touch(connectionID string, expiresAt time.Time)
	Remove(connectionID string)
}

type MemoryStore struct {
	mu        sync.Mutex
	routes    map[string]Route
	instances map[string]string
	known     map[string]bool
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{routes: map[string]Route{}, instances: map[string]string{}, known: map[string]bool{}}
}

func (s *MemoryStore) Claim(route Route) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expireLocked(time.Now())
	locator := strings.ToLower(route.Locator)
	if _, ok := s.routes[locator]; ok || s.instances[route.InstanceID] != "" {
		return ErrDuplicateChannel
	}
	s.routes[locator] = route
	s.instances[route.InstanceID] = route.ConnectionID
	s.known[locator] = true
	return nil
}

func (s *MemoryStore) Resolve(locator string, now time.Time) (Route, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expireLocked(now)
	locator = strings.ToLower(locator)
	route, ok := s.routes[locator]
	if ok {
		return route, nil
	}
	if s.known[locator] {
		return Route{}, ErrInstanceOffline
	}
	return Route{}, ErrUnknownLocator
}

func (s *MemoryStore) Touch(connectionID string, expiresAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for locator, route := range s.routes {
		if route.ConnectionID == connectionID {
			route.ExpiresAt = expiresAt
			s.routes[locator] = route
			return
		}
	}
}

func (s *MemoryStore) Remove(connectionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for locator, route := range s.routes {
		if route.ConnectionID == connectionID {
			delete(s.routes, locator)
			delete(s.instances, route.InstanceID)
			return
		}
	}
}

func (s *MemoryStore) expireLocked(now time.Time) {
	for locator, route := range s.routes {
		if !now.Before(route.ExpiresAt) {
			delete(s.routes, locator)
			delete(s.instances, route.InstanceID)
		}
	}
}
