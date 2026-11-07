package relay

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type testAuth struct{ accept bool }

const testInstanceID = "11111111-1111-4111-8111-111111111111"

func (a testAuth) Authenticate(context.Context, DesktopHello) error {
	if !a.accept {
		return context.Canceled
	}
	return nil
}

func TestAuthenticationAndDuplicateIsolation(t *testing.T) {
	s := NewServer(DefaultConfig(), NewMemoryStore(), testAuth{accept: true})
	h := httptest.NewServer(s.Handler())
	defer h.Close()
	first := dialDesktop(t, h.URL, testInstanceID, "LOC-ONE", 1)
	defer first.Close()
	if got := readType(t, first); got != "desktop_ready" {
		t.Fatalf("ready = %q", got)
	}
	duplicate := dialDesktop(t, h.URL, testInstanceID, "LOC-ONE", 2)
	defer duplicate.Close()
	if got := readCode(t, duplicate); got != "duplicate_channel" {
		t.Fatalf("duplicate code = %q", got)
	}
	unknown := dialMobile(t, h.URL, "LOC-TWO", 9)
	defer unknown.Close()
	if got := readCode(t, unknown); got != "unknown_locator" {
		t.Fatalf("unknown code = %q", got)
	}
}

func TestRejectsInvalidAuthentication(t *testing.T) {
	s := NewServer(DefaultConfig(), NewMemoryStore(), testAuth{})
	h := httptest.NewServer(s.Handler())
	defer h.Close()
	c := dialDesktop(t, h.URL, testInstanceID, "LOC-ONE", 1)
	defer c.Close()
	if got := readCode(t, c); got != "registration_invalid" {
		t.Fatalf("code = %q", got)
	}
}

func TestInstancesCannotReceiveEachOthersFrames(t *testing.T) {
	s := NewServer(DefaultConfig(), NewMemoryStore(), testAuth{accept: true})
	h := httptest.NewServer(s.Handler())
	defer h.Close()
	d1 := dialDesktop(t, h.URL, testInstanceID, "LOC-ONE", 1)
	defer d1.Close()
	readType(t, d1)
	d2 := dialDesktop(t, h.URL, "33333333-3333-4333-8333-333333333333", "LOC-TWO", 2)
	defer d2.Close()
	readType(t, d2)
	m := dialMobile(t, h.URL, "LOC-ONE", 7)
	defer m.Close()
	readType(t, d1)
	writeJSONTest(t, d1, map[string]any{"type": "stream_accept", "version": 1, "stream_id": 7})
	readType(t, m)
	if err := m.WriteMessage(websocket.BinaryMessage, frame(7, dataFrame, []byte{9, 8, 7})); err != nil {
		t.Fatal(err)
	}
	if _, _, err := d1.ReadMessage(); err != nil {
		t.Fatal(err)
	}
	_ = d2.SetReadDeadline(time.Now().Add(40 * time.Millisecond))
	if _, _, err := d2.ReadMessage(); err == nil {
		t.Fatal("second instance received another instance's frame")
	}
}

func TestRelaysOpaqueFramesAndReconnects(t *testing.T) {
	s := NewServer(DefaultConfig(), NewMemoryStore(), testAuth{accept: true})
	h := httptest.NewServer(s.Handler())
	defer h.Close()
	d := dialDesktop(t, h.URL, testInstanceID, "LOC-ONE", 1)
	defer d.Close()
	readType(t, d)
	m := dialMobile(t, h.URL, "LOC-ONE", 7)
	defer m.Close()
	if got := readType(t, d); got != "stream_open" {
		t.Fatalf("open=%s", got)
	}
	writeJSONTest(t, d, map[string]any{"type": "stream_accept", "version": 1, "stream_id": 7})
	if got := readType(t, m); got != "stream_ready" {
		t.Fatalf("ready=%s", got)
	}
	payload := []byte{0, 255, 1, 2, 3, 99}
	if err := m.WriteMessage(websocket.BinaryMessage, frame(7, dataFrame, payload)); err != nil {
		t.Fatal(err)
	}
	_, got, err := d.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(frame(7, dataFrame, payload)) {
		t.Fatal("relay changed opaque bytes")
	}
	_ = d.Close()
	_, reset, err := m.ReadMessage()
	if err != nil || string(reset) != string(frame(7, rstFrame, nil)) {
		t.Fatal("mobile should receive a synthetic reset after desktop disconnect")
	}
	reconnected := dialDesktop(t, h.URL, testInstanceID, "LOC-ONE", 2)
	defer reconnected.Close()
	if got := readType(t, reconnected); got != "desktop_ready" {
		t.Fatalf("reconnect=%s", got)
	}
}

func TestExpiryAndStreamLimits(t *testing.T) {
	config := DefaultConfig()
	config.Lease = 20 * time.Millisecond
	config.MaxStreams = 1
	s := NewServer(config, NewMemoryStore(), testAuth{accept: true})
	h := httptest.NewServer(s.Handler())
	defer h.Close()
	d := dialDesktop(t, h.URL, testInstanceID, "LOC-ONE", 1)
	defer d.Close()
	readType(t, d)
	time.Sleep(30 * time.Millisecond)
	m := dialMobile(t, h.URL, "LOC-ONE", 1)
	defer m.Close()
	if got := readCode(t, m); got != "instance_offline" {
		t.Fatalf("expiry=%s", got)
	}
	// A new channel can claim the expired route and enforce its configured cap.
	d2 := dialDesktop(t, h.URL, testInstanceID, "LOC-ONE", 2)
	defer d2.Close()
	readType(t, d2)
	m1 := dialMobile(t, h.URL, "LOC-ONE", 1)
	defer m1.Close()
	readType(t, d2)
	m2 := dialMobile(t, h.URL, "LOC-ONE", 2)
	defer m2.Close()
	if got := readCode(t, m2); got != "stream_limit" {
		t.Fatalf("limit=%s", got)
	}
}

func dialDesktop(t *testing.T, base, instance, locator string, n int) *websocket.Conn {
	t.Helper()
	c := dial(t, base, "/tunnel/v1/desktop")
	writeJSONTest(t, c, DesktopHello{Type: "desktop_hello", Version: 1, InstanceID: instance, Locator: locator, ChannelID: channelID(n), RegistrationTicket: "ticket", KeyProof: "proof"})
	return c
}

func channelID(n int) string { return "22222222-2222-4222-8222-22222222222" + string(rune('0'+n)) }
func dialMobile(t *testing.T, base, locator string, id uint32) *websocket.Conn {
	t.Helper()
	c := dial(t, base, "/tunnel/v1/mobile")
	writeJSONTest(t, c, MobileOpen{Type: "mobile_open", Version: 1, Locator: locator, StreamID: id})
	return c
}
func dial(t *testing.T, base, path string) *websocket.Conn {
	t.Helper()
	url := "ws" + base[4:] + path
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func writeJSONTest(t *testing.T, c *websocket.Conn, v any) {
	t.Helper()
	if err := c.WriteJSON(v); err != nil {
		t.Fatal(err)
	}
}
func readType(t *testing.T, c *websocket.Conn) string {
	t.Helper()
	var v struct {
		Type string `json:"type"`
	}
	_, raw, err := c.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v.Type
}
func readCode(t *testing.T, c *websocket.Conn) string {
	t.Helper()
	var v struct {
		Code string `json:"code"`
	}
	_, raw, err := c.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v.Code
}
