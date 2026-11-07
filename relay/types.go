// Package relay implements the byte-transparent relay-tunnel v1 data plane.
package relay

import (
	"context"
	"errors"
	"time"
)

var (
	ErrDuplicateChannel = errors.New("duplicate channel")
	ErrUnknownLocator   = errors.New("unknown locator")
	ErrInstanceOffline  = errors.New("instance offline")
)

// TicketAuthenticator validates the one-time registration credential and its
// signature. Implementations belong to the relay control plane; the data plane
// deliberately has no knowledge of ticket contents.
type TicketAuthenticator interface {
	Authenticate(context.Context, DesktopHello) error
}

// DesktopHello is the bounded desktop registration control frame.
type DesktopHello struct {
	Type               string `json:"type"`
	Version            int    `json:"version"`
	InstanceID         string `json:"instance_id"`
	Locator            string `json:"locator"`
	ChannelID          string `json:"channel_id"`
	RegistrationTicket string `json:"registration_ticket"`
	KeyProof           string `json:"key_proof"`
}

type MobileOpen struct {
	Type     string `json:"type"`
	Version  int    `json:"version"`
	Locator  string `json:"locator"`
	StreamID uint32 `json:"stream_id"`
}

// Config contains data-plane limits. It is intentionally provider-neutral;
// deployers supply a shared Store for multi-replica routing.
type Config struct {
	HandshakeTimeout            time.Duration
	StreamOpenTimeout           time.Duration
	Lease                       time.Duration
	MaxStreams                  int
	MaxFramePayload             uint32
	MaxBufferedBytes            int64
	WriteTimeout                time.Duration
	MaxDesktopChannels          int
	MaxDesktopAttemptsPerMinute int
	MaxMobileOpensPerMinute     int
	MaxSourceOpensPerMinute     int
	RateLimitWindow             time.Duration
}

func DefaultConfig() Config {
	return Config{
		HandshakeTimeout:            5 * time.Second,
		StreamOpenTimeout:           10 * time.Second,
		Lease:                       45 * time.Second,
		MaxStreams:                  8,
		MaxFramePayload:             64 << 10,
		MaxBufferedBytes:            1 << 20,
		WriteTimeout:                15 * time.Second,
		MaxDesktopChannels:          10_000,
		MaxDesktopAttemptsPerMinute: 10,
		MaxMobileOpensPerMinute:     30,
		MaxSourceOpensPerMinute:     120,
		RateLimitWindow:             time.Minute,
	}
}
