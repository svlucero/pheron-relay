// Command relay runs the provider-neutral relay data plane.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/svlucero/pheron-relay/relay"
)

type acceptAllAuthenticator struct{}

func (acceptAllAuthenticator) Authenticate(context.Context, relay.DesktopHello) error { return nil }

func main() {
	cfg, err := loadConfig()
	if err != nil {
		slog.Error("invalid relay configuration", "error", err)
		os.Exit(2)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.logLevel}))
	server := relay.NewServer(cfg.relayConfig, relay.NewMemoryStore(), acceptAllAuthenticator{})
	mux := http.NewServeMux()
	mux.Handle("/tunnel/", server.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if server.Draining() {
			http.Error(w, "draining", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		snapshot := server.Metrics()
		fmt.Fprintf(w, "relay_desktop_channels %d\nrelay_mobile_streams %d\nrelay_draining %d\n", snapshot.DesktopChannels, snapshot.MobileStreams, boolNumber(snapshot.Draining))
		for event, count := range snapshot.Events {
			fmt.Fprintf(w, "relay_events_total{event=\"%s\"} %d\n", event, count)
		}
	})

	httpServer := &http.Server{Addr: cfg.listenAddr, Handler: securityHeaders(mux), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 75 * time.Second}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		server.Drain()
		logger.Info("relay draining")
		ctx, cancel := context.WithTimeout(context.Background(), cfg.drainTimeout)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
		}
	}()
	logger.Info("relay listening", "address", cfg.listenAddr, "environment", cfg.environment)
	var serveErr error
	if cfg.tlsCertFile != "" {
		httpServer.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS13}
		serveErr = httpServer.ListenAndServeTLS(cfg.tlsCertFile, cfg.tlsKeyFile)
	} else {
		serveErr = httpServer.ListenAndServe()
	}
	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		logger.Error("relay stopped", "error", serveErr)
		os.Exit(1)
	}
}

type config struct {
	listenAddr, tlsCertFile, tlsKeyFile, environment string
	drainTimeout                                     time.Duration
	logLevel                                         slog.Level
	relayConfig                                      relay.Config
}

func loadConfig() (config, error) {
	cfg := config{listenAddr: env("RELAY_LISTEN_ADDR", ":8443"), environment: env("RELAY_ENVIRONMENT", "development"), drainTimeout: durationEnv("RELAY_DRAIN_TIMEOUT", 30*time.Second), logLevel: slog.LevelInfo, relayConfig: relay.DefaultConfig()}
	cfg.tlsCertFile, cfg.tlsKeyFile = os.Getenv("RELAY_TLS_CERT_FILE"), os.Getenv("RELAY_TLS_KEY_FILE")
	if (cfg.tlsCertFile == "") != (cfg.tlsKeyFile == "") {
		return config{}, errors.New("both RELAY_TLS_CERT_FILE and RELAY_TLS_KEY_FILE are required")
	}
	if cfg.environment != "development" {
		return config{}, errors.New("this binary ships only the development authenticator and memory store; supply production integrations before deployment")
	}
	if cfg.environment == "development" && os.Getenv("RELAY_DEV_ACCEPT_TICKETS") != "true" {
		return config{}, errors.New("development requires RELAY_DEV_ACCEPT_TICKETS=true; never set it outside development")
	}
	if os.Getenv("RELAY_ALLOW_MEMORY_STORE") != "true" {
		return config{}, errors.New("this binary requires RELAY_ALLOW_MEMORY_STORE=true; never set it outside development")
	}
	if value := os.Getenv("RELAY_LOG_LEVEL"); value != "" {
		if err := cfg.logLevel.UnmarshalText([]byte(value)); err != nil {
			return config{}, errors.New("invalid RELAY_LOG_LEVEL")
		}
	}
	setInt := func(name string, target *int) error {
		if raw := os.Getenv(name); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 1 {
				return fmt.Errorf("invalid %s", name)
			}
			*target = value
		}
		return nil
	}
	for _, item := range []struct {
		name   string
		target *int
	}{{"RELAY_MAX_STREAMS", &cfg.relayConfig.MaxStreams}, {"RELAY_MAX_DESKTOP_CHANNELS", &cfg.relayConfig.MaxDesktopChannels}, {"RELAY_MAX_DESKTOP_ATTEMPTS_PER_MINUTE", &cfg.relayConfig.MaxDesktopAttemptsPerMinute}, {"RELAY_MAX_MOBILE_OPENS_PER_MINUTE", &cfg.relayConfig.MaxMobileOpensPerMinute}, {"RELAY_MAX_SOURCE_OPENS_PER_MINUTE", &cfg.relayConfig.MaxSourceOpensPerMinute}} {
		if err := setInt(item.name, item.target); err != nil {
			return config{}, err
		}
	}
	return cfg, nil
}

func env(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
func durationEnv(name string, fallback time.Duration) time.Duration {
	if value := os.Getenv(name); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil && parsed > 0 {
			return parsed
		}
	}
	return fallback
}
func boolNumber(value bool) int {
	if value {
		return 1
	}
	return 0
}
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}
