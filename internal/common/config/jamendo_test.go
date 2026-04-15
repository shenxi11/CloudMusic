package config

import "testing"

func TestResolveJamendoConfigUsesDefaultsAndEnv(t *testing.T) {
	t.Setenv("JAMENDO_CLIENT_ID", " env-client ")

	cfg := &Config{
		External: ExternalConfig{
			Jamendo: JamendoExternalConfig{
				Enabled:      true,
				BaseURL:      " https://api.example.test/v3.0/ ",
				TimeoutSec:   3,
				DefaultLimit: 5,
			},
		},
	}

	got := ResolveJamendoConfig(cfg)
	if !got.Enabled {
		t.Fatal("expected provider to be enabled")
	}
	if got.ClientID != "env-client" {
		t.Fatalf("expected env client id override, got %q", got.ClientID)
	}
	if got.BaseURL != "https://api.example.test/v3.0" {
		t.Fatalf("unexpected base url: %q", got.BaseURL)
	}
	if got.TimeoutSec != 3 || got.DefaultLimit != 5 {
		t.Fatalf("unexpected timeout/default limit: %+v", got)
	}
}
