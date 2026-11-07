package main

import (
	"os"
	"testing"
)

func TestLoadConfigDevelopmentRequiresExplicitUnsafeMode(t *testing.T) {
	t.Setenv("RELAY_ENVIRONMENT", "development")
	t.Setenv("RELAY_DEV_ACCEPT_TICKETS", "")
	if _, err := loadConfig(); err == nil {
		t.Fatal("expected explicit development authenticator guard")
	}
	t.Setenv("RELAY_DEV_ACCEPT_TICKETS", "true")
	t.Setenv("RELAY_ALLOW_MEMORY_STORE", "true")
	if _, err := loadConfig(); err != nil {
		t.Fatalf("development config: %v", err)
	}
}

func TestLoadConfigRejectsProductionPlaceholder(t *testing.T) {
	t.Setenv("RELAY_ENVIRONMENT", "production")
	t.Setenv("RELAY_DEV_ACCEPT_TICKETS", "")
	t.Setenv("RELAY_TLS_CERT_FILE", "/run/secrets/cert")
	t.Setenv("RELAY_TLS_KEY_FILE", "/run/secrets/key")
	if _, err := loadConfig(); err == nil {
		t.Fatal("production cannot silently use development integrations")
	}
	_ = os.Unsetenv("RELAY_TLS_CERT_FILE")
}
