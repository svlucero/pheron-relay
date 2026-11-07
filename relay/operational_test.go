package relay

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gorilla/websocket"
)

func TestDrainRejectsNewAdmissionsAndKeepsMetricsAggregate(t *testing.T) {
	s := NewServer(DefaultConfig(), NewMemoryStore(), testAuth{accept: true})
	h := httptest.NewServer(s.Handler())
	defer h.Close()
	s.Drain()
	response, err := http.Get(h.URL + "/tunnel/v1/desktop")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", response.StatusCode)
	}
	metrics := s.Metrics()
	if !metrics.Draining || metrics.DesktopChannels != 0 || metrics.MobileStreams != 0 {
		t.Fatalf("unexpected metrics: %#v", metrics)
	}
}

func TestConcurrentDesktopAdmissionsStayWithinConfiguredLimit(t *testing.T) {
	config := DefaultConfig()
	config.MaxDesktopChannels = 16
	s := NewServer(config, NewMemoryStore(), testAuth{accept: true})
	h := httptest.NewServer(s.Handler())
	defer h.Close()
	const clients = 16
	connections := make(chan *websocket.Conn, clients)
	errors := make(chan error, clients)
	var group sync.WaitGroup
	for n := 0; n < clients; n++ {
		group.Add(1)
		go func(n int) {
			defer group.Done()
			url := "ws" + h.URL[4:] + "/tunnel/v1/desktop"
			connection, _, err := websocket.DefaultDialer.Dial(url, nil)
			if err != nil {
				errors <- err
				return
			}
			hello := DesktopHello{Type: "desktop_hello", Version: 1, InstanceID: fmt.Sprintf("10000000-0000-4000-8000-%012d", n), Locator: fmt.Sprintf("LOC-%d", n), ChannelID: fmt.Sprintf("20000000-0000-4000-8000-%012d", n), RegistrationTicket: "ticket", KeyProof: "proof"}
			if err := connection.WriteJSON(hello); err != nil {
				errors <- err
				return
			}
			_, raw, err := connection.ReadMessage()
			if err != nil {
				errors <- err
				return
			}
			var ready struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(raw, &ready); err != nil || ready.Type != "desktop_ready" {
				errors <- fmt.Errorf("ready = %q, error = %v", ready.Type, err)
				return
			}
			connections <- connection
		}(n)
	}
	group.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	if got := s.Metrics().DesktopChannels; got != clients {
		t.Fatalf("desktop channels = %d", got)
	}
	close(connections)
	for connection := range connections {
		_ = connection.Close()
	}
}

func TestMobileOpenRateLimitDoesNotCreateStreams(t *testing.T) {
	config := DefaultConfig()
	config.MaxMobileOpensPerMinute = 1
	s := NewServer(config, NewMemoryStore(), testAuth{accept: true})
	h := httptest.NewServer(s.Handler())
	defer h.Close()
	first := dialMobile(t, h.URL, "UNREGISTERED", 1)
	defer first.Close()
	if got := readCode(t, first); got != "unknown_locator" {
		t.Fatalf("first code = %q", got)
	}
	second := dialMobile(t, h.URL, "UNREGISTERED", 2)
	defer second.Close()
	if got := readCode(t, second); got != "rate_limited" {
		t.Fatalf("second code = %q", got)
	}
	metrics := s.Metrics()
	if metrics.MobileStreams != 0 || metrics.Events["error_rate_limited"] != 1 {
		t.Fatalf("unexpected metrics: %#v", metrics)
	}
}
