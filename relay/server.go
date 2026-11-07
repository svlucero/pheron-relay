package relay

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const (
	frameHeader = 12
	dataFrame   = 1
	finFrame    = 2
	rstFrame    = 3
)

type Server struct {
	config          Config
	store           Store
	auth            TicketAuthenticator
	upgrader        websocket.Upgrader
	mu              sync.Mutex
	desktops        map[string]*desktop
	limiter         *rateLimiter
	metrics         *metrics
	draining        atomic.Bool
	desktopChannels atomic.Int64
	mobileStreams   atomic.Int64
}

func NewServer(config Config, store Store, auth TicketAuthenticator) *Server {
	defaults := DefaultConfig()
	if config.HandshakeTimeout == 0 {
		config.HandshakeTimeout = defaults.HandshakeTimeout
	}
	if config.StreamOpenTimeout == 0 {
		config.StreamOpenTimeout = defaults.StreamOpenTimeout
	}
	if config.Lease == 0 {
		config.Lease = defaults.Lease
	}
	if config.MaxStreams == 0 {
		config.MaxStreams = defaults.MaxStreams
	}
	if config.MaxFramePayload == 0 {
		config.MaxFramePayload = defaults.MaxFramePayload
	}
	if config.MaxBufferedBytes == 0 {
		config.MaxBufferedBytes = defaults.MaxBufferedBytes
	}
	if config.WriteTimeout == 0 {
		config.WriteTimeout = defaults.WriteTimeout
	}
	if config.MaxDesktopChannels == 0 {
		config.MaxDesktopChannels = defaults.MaxDesktopChannels
	}
	if config.MaxDesktopAttemptsPerMinute == 0 {
		config.MaxDesktopAttemptsPerMinute = defaults.MaxDesktopAttemptsPerMinute
	}
	if config.MaxMobileOpensPerMinute == 0 {
		config.MaxMobileOpensPerMinute = defaults.MaxMobileOpensPerMinute
	}
	if config.MaxSourceOpensPerMinute == 0 {
		config.MaxSourceOpensPerMinute = defaults.MaxSourceOpensPerMinute
	}
	if config.RateLimitWindow == 0 {
		config.RateLimitWindow = defaults.RateLimitWindow
	}
	return &Server{config: config, store: store, auth: auth, desktops: map[string]*desktop{}, limiter: newRateLimiter(), metrics: newMetrics(), upgrader: websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}}
}

// Drain stops new channel and stream admission while allowing established
// streams to finish until the hosting process reaches its drain deadline.
func (s *Server) Drain() { s.draining.Store(true) }

func (s *Server) Draining() bool { return s.draining.Load() }

func (s *Server) Metrics() MetricsSnapshot {
	return MetricsSnapshot{DesktopChannels: s.desktopChannels.Load(), MobileStreams: s.mobileStreams.Load(), Draining: s.Draining(), Events: s.metrics.snapshot()}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/tunnel/v1/desktop", s.handleDesktop)
	mux.HandleFunc("/tunnel/v1/mobile", s.handleMobile)
	return mux
}

func (s *Server) handleDesktop(w http.ResponseWriter, r *http.Request) {
	if s.Draining() {
		http.Error(w, "relay unavailable", http.StatusServiceUnavailable)
		return
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.SetReadLimit(4096)
	_ = conn.SetReadDeadline(time.Now().Add(s.config.HandshakeTimeout))
	messageType, raw, err := conn.ReadMessage()
	if err != nil {
		return
	}
	var hello DesktopHello
	if messageType != websocket.TextMessage || json.Unmarshal(raw, &hello) != nil || hello.Type != "desktop_hello" {
		s.error(conn, "invalid_handshake", false)
		return
	}
	if hello.Version != 1 {
		s.error(conn, "unsupported_version", false)
		return
	}
	if !validDesktopHello(hello) || s.auth == nil || s.auth.Authenticate(r.Context(), hello) != nil {
		s.error(conn, "registration_invalid", false)
		return
	}
	if !s.allowDesktop(hello.InstanceID, r.RemoteAddr) || !s.reserveDesktop() {
		s.error(conn, "rate_limited", true)
		return
	}
	connectionID := randomID()
	route := Route{InstanceID: hello.InstanceID, Locator: hello.Locator, ConnectionID: connectionID, ExpiresAt: time.Now().Add(s.config.Lease)}
	if err := s.store.Claim(route); err != nil {
		s.desktopChannels.Add(-1)
		s.error(conn, codeFor(err), false)
		return
	}
	d := &desktop{server: s, conn: conn, route: route, streams: map[uint32]*stream{}}
	s.mu.Lock()
	s.desktops[connectionID] = d
	s.mu.Unlock()
	s.metrics.add("desktop_ready")
	defer s.removeDesktop(d)
	if !d.control(map[string]any{"type": "desktop_ready", "version": 1, "lease_seconds": int(s.config.Lease.Seconds()), "connection_id": connectionID}) {
		return
	}
	_ = conn.SetReadDeadline(time.Time{})
	d.readLoop()
}

func (s *Server) handleMobile(w http.ResponseWriter, r *http.Request) {
	if s.Draining() {
		http.Error(w, "relay unavailable", http.StatusServiceUnavailable)
		return
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.SetReadLimit(1024)
	_ = conn.SetReadDeadline(time.Now().Add(s.config.HandshakeTimeout))
	messageType, raw, err := conn.ReadMessage()
	if err != nil {
		return
	}
	var open MobileOpen
	if messageType != websocket.TextMessage || json.Unmarshal(raw, &open) != nil || open.Type != "mobile_open" || open.Version != 1 || open.Locator == "" || open.StreamID == 0 {
		s.error(conn, "invalid_handshake", false)
		return
	}
	if !s.allowMobile(open.Locator, r.RemoteAddr) {
		s.error(conn, "rate_limited", true)
		return
	}
	route, err := s.store.Resolve(open.Locator, time.Now())
	if err != nil {
		s.error(conn, codeFor(err), true)
		return
	}
	s.mu.Lock()
	d := s.desktops[route.ConnectionID]
	s.mu.Unlock()
	if d == nil || !d.reserve(open.StreamID, conn) {
		s.error(conn, "stream_limit", true)
		return
	}
	s.mobileStreams.Add(1)
	s.metrics.add("stream_open")
	stream := d.get(open.StreamID)
	defer func() { s.mobileStreams.Add(-1); d.closeStream(open.StreamID, true) }()
	if !d.control(map[string]any{"type": "stream_open", "version": 1, "stream_id": open.StreamID}) {
		return
	}
	select {
	case accepted := <-stream.accepted:
		if !accepted {
			s.error(conn, "stream_rejected", true)
			return
		}
	case <-time.After(s.config.StreamOpenTimeout):
		s.error(conn, "stream_rejected", true)
		return
	}
	d.markReady(open.StreamID)
	if !writeJSON(conn, map[string]any{"type": "stream_ready", "version": 1, "stream_id": open.StreamID}, s.config.WriteTimeout) {
		return
	}
	_ = conn.SetReadDeadline(time.Time{})
	for {
		messageType, bytes, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if messageType != websocket.BinaryMessage || !validFrame(bytes, open.StreamID, s.config.MaxFramePayload) {
			d.reset(open.StreamID)
			return
		}
		if !d.toDesktop(open.StreamID, bytes) {
			return
		}
		if bytes[1] == finFrame || bytes[1] == rstFrame {
			return
		}
	}
}

type desktop struct {
	server  *Server
	conn    *websocket.Conn
	route   Route
	write   sync.Mutex
	mu      sync.Mutex
	streams map[uint32]*stream
}
type stream struct {
	mobile   *websocket.Conn
	accepted chan bool
	ready    bool
}

func (d *desktop) readLoop() {
	for {
		kind, raw, err := d.conn.ReadMessage()
		if err != nil {
			return
		}
		d.server.store.Touch(d.route.ConnectionID, time.Now().Add(d.server.config.Lease))
		if kind == websocket.TextMessage {
			var msg struct {
				Type     string `json:"type"`
				StreamID uint32 `json:"stream_id"`
			}
			if json.Unmarshal(raw, &msg) != nil {
				d.closeAll()
				return
			}
			if msg.Type == "stream_accept" || msg.Type == "stream_reject" {
				d.answer(msg.StreamID, msg.Type == "stream_accept")
				continue
			}
			if msg.Type == "ping" {
				d.control(map[string]any{"type": "pong", "version": 1})
				continue
			}
			d.closeAll()
			return
		}
		if kind != websocket.BinaryMessage || len(raw) < frameHeader {
			d.closeAll()
			return
		}
		id := binary.BigEndian.Uint32(raw[4:8])
		if !validFrame(raw, id, d.server.config.MaxFramePayload) || !d.toMobile(id, raw) {
			d.reset(id)
		}
	}
}

func (d *desktop) reserve(id uint32, mobile *websocket.Conn) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.streams) >= d.server.config.MaxStreams {
		return false
	}
	if _, exists := d.streams[id]; exists {
		return false
	}
	d.streams[id] = &stream{mobile: mobile, accepted: make(chan bool, 1)}
	return true
}
func (d *desktop) get(id uint32) *stream { d.mu.Lock(); defer d.mu.Unlock(); return d.streams[id] }
func (d *desktop) answer(id uint32, accepted bool) {
	if st := d.get(id); st != nil {
		select {
		case st.accepted <- accepted:
		default:
		}
	}
}
func (d *desktop) control(value any) bool {
	d.write.Lock()
	defer d.write.Unlock()
	return writeJSON(d.conn, value, d.server.config.WriteTimeout)
}
func (d *desktop) toMobile(id uint32, bytes []byte) bool {
	d.mu.Lock()
	st := d.streams[id]
	ready := st != nil && st.ready
	d.mu.Unlock()
	if !ready {
		return false
	}
	if int64(len(bytes)) > d.server.config.MaxBufferedBytes || !writeBinary(st.mobile, bytes, d.server.config.WriteTimeout) {
		return false
	}
	return true
}
func (d *desktop) markReady(id uint32) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if st := d.streams[id]; st != nil {
		st.ready = true
	}
}
func (d *desktop) toDesktop(id uint32, bytes []byte) bool {
	if d.get(id) == nil || int64(len(bytes)) > d.server.config.MaxBufferedBytes {
		return false
	}
	d.write.Lock()
	defer d.write.Unlock()
	return writeBinary(d.conn, bytes, d.server.config.WriteTimeout)
}
func (d *desktop) reset(id uint32) {
	if st := d.get(id); st != nil {
		_ = writeBinary(st.mobile, frame(id, rstFrame, nil), d.server.config.WriteTimeout)
	}
	d.closeStream(id, false)
}
func (d *desktop) closeStream(id uint32, notify bool) {
	d.mu.Lock()
	st := d.streams[id]
	delete(d.streams, id)
	d.mu.Unlock()
	if st != nil && notify {
		_ = d.control(map[string]any{"type": "stream_closed", "version": 1, "stream_id": id})
	}
}
func (d *desktop) closeAll() {
	d.mu.Lock()
	ids := make([]uint32, 0, len(d.streams))
	for id := range d.streams {
		ids = append(ids, id)
	}
	d.mu.Unlock()
	for _, id := range ids {
		d.reset(id)
	}
}

func (s *Server) removeDesktop(d *desktop) {
	s.store.Remove(d.route.ConnectionID)
	s.mu.Lock()
	delete(s.desktops, d.route.ConnectionID)
	s.mu.Unlock()
	s.desktopChannels.Add(-1)
	s.metrics.add("desktop_closed")
	d.closeAll()
}
func (s *Server) error(conn *websocket.Conn, code string, retryable bool) {
	s.metrics.add("error_" + code)
	_ = writeJSON(conn, map[string]any{"type": "error", "version": 1, "code": code, "retryable": retryable}, s.config.WriteTimeout)
	_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, code), time.Now().Add(s.config.WriteTimeout))
}

func (s *Server) allowDesktop(instanceID, remote string) bool {
	now := time.Now()
	return s.limiter.allow("desktop-instance", instanceID, s.config.MaxDesktopAttemptsPerMinute, s.config.RateLimitWindow, now) &&
		s.limiter.allow("desktop-source", sourceIP(remote), s.config.MaxSourceOpensPerMinute, s.config.RateLimitWindow, now)
}

func (s *Server) reserveDesktop() bool {
	for {
		current := s.desktopChannels.Load()
		if current >= int64(s.config.MaxDesktopChannels) {
			return false
		}
		if s.desktopChannels.CompareAndSwap(current, current+1) {
			return true
		}
	}
}

func (s *Server) allowMobile(locator, remote string) bool {
	now := time.Now()
	return s.limiter.allow("mobile-locator", locator, s.config.MaxMobileOpensPerMinute, s.config.RateLimitWindow, now) &&
		s.limiter.allow("mobile-source", sourceIP(remote), s.config.MaxSourceOpensPerMinute, s.config.RateLimitWindow, now)
}

func sourceIP(remote string) string {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return remote
	}
	return host
}
func codeFor(err error) string {
	switch {
	case errors.Is(err, ErrDuplicateChannel):
		return "duplicate_channel"
	case errors.Is(err, ErrInstanceOffline):
		return "instance_offline"
	default:
		return "unknown_locator"
	}
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

func validDesktopHello(h DesktopHello) bool {
	return uuidPattern.MatchString(h.InstanceID) && h.Locator != "" && uuidPattern.MatchString(h.ChannelID) && h.RegistrationTicket != "" && h.KeyProof != ""
}
func validFrame(b []byte, id, limit uint32) bool {
	return len(b) >= frameHeader && b[0] == 1 && b[1] >= dataFrame && b[1] <= 5 && b[2] == 0 && b[3] == 0 && binary.BigEndian.Uint32(b[4:8]) == id && id != 0 && binary.BigEndian.Uint32(b[8:12]) <= limit && int(binary.BigEndian.Uint32(b[8:12])) == len(b)-frameHeader && (b[1] == dataFrame || len(b) == frameHeader)
}
func frame(id uint32, typ byte, payload []byte) []byte {
	out := make([]byte, frameHeader+len(payload))
	out[0] = 1
	out[1] = typ
	binary.BigEndian.PutUint32(out[4:8], id)
	binary.BigEndian.PutUint32(out[8:12], uint32(len(payload)))
	copy(out[12:], payload)
	return out
}
func writeJSON(c *websocket.Conn, value any, timeout time.Duration) bool {
	bytes, err := json.Marshal(value)
	if err != nil {
		return false
	}
	_ = c.SetWriteDeadline(time.Now().Add(timeout))
	return c.WriteMessage(websocket.TextMessage, bytes) == nil
}
func writeBinary(c *websocket.Conn, bytes []byte, timeout time.Duration) bool {
	_ = c.SetWriteDeadline(time.Now().Add(timeout))
	return c.WriteMessage(websocket.BinaryMessage, bytes) == nil
}
func randomID() string {
	bytes := make([]byte, 16)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
