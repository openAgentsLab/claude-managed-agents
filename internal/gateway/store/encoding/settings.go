package encoding

import (
	"encoding/json"
	"fmt"

	"forge/internal/gateway/store"
)

// settingsJSON is the on-disk JSON representation of store.Settings.
type settingsJSON struct {
	AllowRules    []string             `json:"allow_rules,omitempty"`
	DenyRules     []string             `json:"deny_rules,omitempty"`
	MemoryBytes   int64                `json:"memory_bytes,omitempty"`
	NanoCPUs      int64                `json:"nano_cpus,omitempty"`
	ModelOverride *store.ModelSettings `json:"model,omitempty"`
	BrainOverride *store.BrainSettings `json:"brain,omitempty"`
}

// MarshalSettings serialises store.Settings to a JSON string.
func MarshalSettings(s store.Settings) (string, error) {
	b, err := json.Marshal(settingsJSON{
		AllowRules:    s.AllowRules,
		DenyRules:     s.DenyRules,
		MemoryBytes:   s.MemoryBytes,
		NanoCPUs:      s.NanoCPUs,
		ModelOverride: s.ModelOverride,
		BrainOverride: s.BrainOverride,
	})
	if err != nil {
		return "", fmt.Errorf("store: marshal settings: %w", err)
	}
	return string(b), nil
}

// UnmarshalSettings deserialises a JSON string to store.Settings.
func UnmarshalSettings(raw string) (store.Settings, error) {
	if raw == "" || raw == "{}" {
		return store.Settings{}, nil
	}
	var js settingsJSON
	if err := json.Unmarshal([]byte(raw), &js); err != nil {
		return store.Settings{}, fmt.Errorf("store: unmarshal settings: %w", err)
	}
	return store.Settings{
		AllowRules:    js.AllowRules,
		DenyRules:     js.DenyRules,
		MemoryBytes:   js.MemoryBytes,
		NanoCPUs:      js.NanoCPUs,
		ModelOverride: js.ModelOverride,
		BrainOverride: js.BrainOverride,
	}, nil
}

// userSettingsJSON is the on-disk JSON representation of store.UserSettings.
type userSettingsJSON struct {
	ModelOverride *store.ModelSettings `json:"model,omitempty"`
	BrainOverride *store.BrainSettings `json:"brain,omitempty"`
}

// MarshalUserSettings serialises store.UserSettings to a JSON string.
func MarshalUserSettings(s store.UserSettings) (string, error) {
	b, err := json.Marshal(userSettingsJSON{
		ModelOverride: s.ModelOverride,
		BrainOverride: s.BrainOverride,
	})
	if err != nil {
		return "", fmt.Errorf("store: marshal user settings: %w", err)
	}
	return string(b), nil
}

// UnmarshalUserSettings deserialises a JSON string to store.UserSettings.
func UnmarshalUserSettings(raw string) (store.UserSettings, error) {
	if raw == "" || raw == "{}" {
		return store.UserSettings{}, nil
	}
	var js userSettingsJSON
	if err := json.Unmarshal([]byte(raw), &js); err != nil {
		return store.UserSettings{}, fmt.Errorf("store: unmarshal user settings: %w", err)
	}
	return store.UserSettings{
		ModelOverride: js.ModelOverride,
		BrainOverride: js.BrainOverride,
	}, nil
}
