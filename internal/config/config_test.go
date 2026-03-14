package config

import "testing"

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				OpenRouterAPIKey: "sk-or-v1-abc",
				CouncilModels:    []string{"openai/gpt-4o"},
				Port:             "8001",
			},
			wantErr: false,
		},
		{
			name: "missing API key",
			cfg: Config{
				OpenRouterAPIKey: "",
				CouncilModels:    []string{"openai/gpt-4o"},
				Port:             "8001",
			},
			wantErr: true,
		},
		{
			name: "empty council models",
			cfg: Config{
				OpenRouterAPIKey: "sk-or-v1-abc",
				CouncilModels:    []string{},
				Port:             "8001",
			},
			wantErr: true,
		},
		{
			name: "empty port",
			cfg: Config{
				OpenRouterAPIKey: "sk-or-v1-abc",
				CouncilModels:    []string{"openai/gpt-4o"},
				Port:             "",
			},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
