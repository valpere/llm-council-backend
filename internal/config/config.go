package config

import (
	"os"
	"strings"
)

type Config struct {
	OpenRouterAPIKey string
	CouncilModels    []string
	ChairmanModel    string
	DataDir          string
	Port             string
}

func Load() *Config {
	councilModels := []string{
		"openai/gpt-5.1",
		"google/gemini-3-pro-preview",
		"anthropic/claude-sonnet-4.5",
		"x-ai/grok-4",
	}
	if v := os.Getenv("COUNCIL_MODELS"); v != "" {
		councilModels = strings.Split(v, ",")
	}

	chairmanModel := "google/gemini-3-pro-preview"
	if v := os.Getenv("CHAIRMAN_MODEL"); v != "" {
		chairmanModel = v
	}

	dataDir := "data/conversations"
	if v := os.Getenv("DATA_DIR"); v != "" {
		dataDir = v
	}

	port := "8001"
	if v := os.Getenv("PORT"); v != "" {
		port = v
	}

	return &Config{
		OpenRouterAPIKey: os.Getenv("OPENROUTER_API_KEY"),
		CouncilModels:    councilModels,
		ChairmanModel:    chairmanModel,
		DataDir:          dataDir,
		Port:             port,
	}
}
