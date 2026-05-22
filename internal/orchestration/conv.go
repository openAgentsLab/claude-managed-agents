package orchestration

import (
	"forge/internal/config"
	appstore "forge/internal/gateway/store"
)

// configSettingsToStore converts config.TenantSettings to the flat appstore.Settings
// used by the store layer.
func configSettingsToStore(s config.TenantSettings) appstore.Settings {
	st := appstore.Settings{
		AllowRules: s.AllowRules,
		DenyRules:  s.DenyRules,
		MemoryBytes:    s.ResourceQuota.MemoryBytes,
		NanoCPUs:       s.ResourceQuota.NanoCPUs,
	}
	if s.ModelOverride != nil {
		st.ModelOverride = &appstore.ModelSettings{
			Provider:   s.ModelOverride.Provider,
			APIKey:     s.ModelOverride.APIKey,
			BaseURL:    s.ModelOverride.BaseURL,
			Model:      s.ModelOverride.Model,
			ByAzure:    s.ModelOverride.ByAzure,
			APIVersion: s.ModelOverride.APIVersion,
		}
	}
	if s.BrainOverride != nil {
		st.BrainOverride = &appstore.BrainSettings{
			Effort:     s.BrainOverride.Effort,
			Thinking:   s.BrainOverride.Thinking,
			MaxRetries: s.BrainOverride.MaxRetries,
		}
	}
	return st
}
