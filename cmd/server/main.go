package main

import (
	"fmt"
	"log"
	"net/http"

	"llm-council/internal/api"
	"llm-council/internal/config"
	"llm-council/internal/council"
	"llm-council/internal/openrouter"
	"llm-council/internal/storage"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load() // ignore error if .env not present

	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("configuration error: %v", err)
	}
	client := openrouter.New(cfg.OpenRouterAPIKey)
	c := council.New(client, cfg.CouncilModels, cfg.ChairmanModel)
	store := storage.New(cfg.DataDir)
	handler := api.New(c, store)

	addr := ":" + cfg.Port
	fmt.Printf("LLM Council API listening on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, handler.Routes()))
}
