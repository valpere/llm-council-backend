package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	OpenRouterAPIKey string
	CouncilModels    []string
	ChairmanModel    string
	TitleModel       string
	DataDir          string
	Port             string
	CORSOrigins      []string
}

// Validate returns an error if the configuration is missing required values.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.OpenRouterAPIKey) == "" {
		return fmt.Errorf("OPENROUTER_API_KEY is required but not set")
	}
	return nil
}

func Load() *Config {
	councilModels := []string{
		"openai/gpt-5.1",
		"google/gemini-3-pro-preview",
		"anthropic/claude-sonnet-4.5",
		"x-ai/grok-4",
	}
	if v := os.Getenv("COUNCIL_MODELS"); v != "" {
		var models []string
		for _, m := range strings.Split(v, ",") {
			if m = strings.TrimSpace(m); m != "" {
				models = append(models, m)
			}
		}
		if len(models) > 0 {
			councilModels = models
		}
	}

	chairmanModel := "google/gemini-3-pro-preview"
	if v := os.Getenv("CHAIRMAN_MODEL"); v != "" {
		chairmanModel = v
	}

	titleModel := "google/gemini-2.5-flash"
	if v := os.Getenv("TITLE_MODEL"); v != "" {
		titleModel = v
	}

	dataDir := "data/conversations"
	if v := os.Getenv("DATA_DIR"); v != "" {
		dataDir = v
	}

	port := "8001"
	if v := os.Getenv("PORT"); v != "" {
		port = v
	}

	corsOrigins := []string{"http://localhost:5173", "http://localhost:3000"}
	if v := os.Getenv("CORS_ORIGINS"); v != "" {
		var origins []string
		for _, o := range strings.Split(v, ",") {
			if o = strings.TrimSpace(o); o != "" {
				origins = append(origins, o)
			}
		}
		if len(origins) > 0 {
			corsOrigins = origins
		}
	}

	return &Config{
		OpenRouterAPIKey: strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")),
		CouncilModels:    councilModels,
		ChairmanModel:    chairmanModel,
		TitleModel:       titleModel,
		DataDir:          dataDir,
		Port:             port,
		CORSOrigins:      corsOrigins,
	}
}
