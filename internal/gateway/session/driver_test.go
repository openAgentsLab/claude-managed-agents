package session_test

import (
	"testing"

	"forge/internal/config"
	"forge/internal/gateway/session"

	// Register the config.SessionDriverMemory driver for these tests.
	_ "forge/internal/gateway/session/memory"
)

func TestOpen_Memory(t *testing.T) {
	store, err := session.Open(config.SessionConfig{Driver: config.SessionDriverMemory})
	if err != nil {
		t.Fatalf("Open(memory): %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil SessionStore")
	}
	if _, err := store.EmitEvent("sess", session.Event{Role: "user", Content: "hi"}); err != nil {
		t.Fatalf("EmitEvent on opened store: %v", err)
	}
	events, err := store.GetEvents("sess")
	if err != nil {
		t.Fatalf("GetEvents: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("expected 1 event, got %d", len(events))
	}
}

func TestOpen_DefaultDriverIsMemory(t *testing.T) {
	// Empty Driver field defaults to config.SessionDriverMemory via DriverOrDefault.
	store, err := session.Open(config.SessionConfig{})
	if err != nil {
		t.Fatalf("Open(default): %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestOpen_UnknownDriver_ReturnsError(t *testing.T) {
	_, err := session.Open(config.SessionConfig{Driver: "does-not-exist"})
	if err == nil {
		t.Fatal("expected error for unknown driver, got nil")
	}
}

func TestOpen_Memory_WithOptions(t *testing.T) {
	// config.SessionDriverMemory driver ignores options; should still succeed.
	store, err := session.Open(config.SessionConfig{
		Driver:  config.SessionDriverMemory,
		Options: map[string]string{"unused": "value"},
	})
	if err != nil {
		t.Fatalf("Open(memory, opts): %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}
