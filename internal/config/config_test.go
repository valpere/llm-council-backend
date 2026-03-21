package config

import (
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{"valid key", "sk-or-v1-abc123", false},
		{"empty key", "", true},
		{"whitespace only", "   ", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{OpenRouterAPIKey: tt.key}
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadTrimsAPIKeyWhitespace(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "  sk-or-v1-abc123  ")
	cfg := Load()
	if cfg.OpenRouterAPIKey != "sk-or-v1-abc123" {
		t.Errorf("expected trimmed key, got %q", cfg.OpenRouterAPIKey)
	}
}
