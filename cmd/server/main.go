package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"llm-council/internal/api"
	"llm-council/internal/config"
	"llm-council/internal/council"
	"llm-council/internal/openrouter"
	"llm-council/internal/storage"

	"github.com/joho/godotenv"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	godotenv.Load() // ignore error if .env not present

	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	client := openrouter.New(cfg.OpenRouterAPIKey)
	c := council.New(client, cfg.CouncilModels, cfg.ChairmanModel, cfg.TitleModel)
	store := storage.New(cfg.DataDir)
	handler := api.New(c, store, cfg.DataDir)

	addr := ":" + cfg.Port
	srv := &http.Server{Addr: addr, Handler: handler.Routes()}

	go func() {
		fmt.Printf("LLM Council API listening on %s\n", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server")
	// Worst-case request duration: Stage1 + Stage2 + Stage3, each 120 s → 360 s total.
	// Allow a generous margin so in-flight council requests can complete.
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}
