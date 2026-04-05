package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"llm-council/internal/council"
	"llm-council/internal/storage"

	"github.com/google/uuid"
)

type Handler struct {
	council     council.Runner
	store       storage.Storer
	dataDir     string
	corsOrigins []string
}

func New(c council.Runner, s storage.Storer, dataDir string, corsOrigins []string) *Handler {
	return &Handler{council: c, store: s, dataDir: dataDir, corsOrigins: corsOrigins}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", h.healthCheck)
	mux.HandleFunc("GET /health/live", h.healthLive)
	mux.HandleFunc("GET /health/ready", h.healthReady)
	mux.HandleFunc("GET /api/conversations", h.listConversations)
	mux.HandleFunc("POST /api/conversations", h.createConversation)
	mux.HandleFunc("GET /api/conversations/{id}", h.getConversation)
	mux.HandleFunc("POST /api/conversations/{id}/message", h.sendMessage)
	mux.HandleFunc("POST /api/conversations/{id}/message/stream", h.sendMessageStream)
	return h.corsMiddleware(mux)
}

func (h *Handler) corsMiddleware(next http.Handler) http.Handler {
	allowed := make(map[string]bool, len(h.corsOrigins))
	for _, o := range h.corsOrigins {
		allowed[o] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowed[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Add("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Warn("writeJSON: encode failed", "status", status, "error", err)
	}
}

func (h *Handler) healthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "LLM Council API"})
}

func (h *Handler) healthLive(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) healthReady(w http.ResponseWriter, r *http.Request) {
	unavailable := func(err error) {
		slog.Error("healthReady: data directory check failed", "error", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "not ready",
			"error":  "data directory unavailable",
		})
	}
	if err := os.MkdirAll(h.dataDir, 0700); err != nil {
		unavailable(err)
		return
	}
	info, err := os.Stat(h.dataDir)
	if err != nil {
		unavailable(err)
		return
	}
	if !info.IsDir() {
		unavailable(fmt.Errorf("%s is not a directory", h.dataDir))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) listConversations(w http.ResponseWriter, r *http.Request) {
	metas, err := h.store.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if metas == nil {
		metas = []storage.ConversationMeta{}
	}
	writeJSON(w, http.StatusOK, metas)
}

func (h *Handler) createConversation(w http.ResponseWriter, r *http.Request) {
	conv, err := h.store.Create(uuid.New().String())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, conv)
}

func (h *Handler) getConversation(w http.ResponseWriter, r *http.Request) {
	conv, err := h.store.Get(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if conv == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "conversation not found"})
		return
	}
	writeJSON(w, http.StatusOK, conv)
}

func (h *Handler) sendMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB
	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	conv, err := h.store.Get(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if conv == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "conversation not found"})
		return
	}

	isFirst := len(conv.Messages) == 0
	if err := h.store.AddMessage(id, map[string]string{"role": "user", "content": req.Content}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Start title generation concurrently so it doesn't block RunFull.
	// Detached from the request context so it completes even if the client disconnects.
	var awaitTitle func() string
	if isFirst {
		ch := make(chan string, 1)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			ch <- h.council.GenerateTitle(ctx, req.Content)
		}()
		awaitTitle = func() string { return <-ch }
	}

	result, err := h.council.RunFull(r.Context(), req.Content)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if awaitTitle != nil {
		if err := h.store.UpdateTitle(id, awaitTitle()); err != nil {
			slog.Error("sendMessage: UpdateTitle failed", "conversation_id", id, "error", err)
		}
	}

	if err := h.store.AddMessage(id, map[string]any{
		"role":   "assistant",
		"stage1": result.Stage1,
		"stage2": result.Stage2,
		"stage3": result.Stage3,
	}); err != nil {
		slog.Error("sendMessage: AddMessage (assistant) failed", "conversation_id", id, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to persist response"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) sendMessageStream(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB
	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	conv, err := h.store.Get(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if conv == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "conversation not found"})
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming not supported"})
		return
	}

	isFirst := len(conv.Messages) == 0

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	send := func(v any) {
		data, err := json.Marshal(v)
		if err != nil {
			slog.Error("sendMessageStream: marshal failed", "conversation_id", id, "error", err)
			fmt.Fprintf(w, "data: {\"type\":\"error\",\"message\":\"internal serialization error\"}\n\n")
			flusher.Flush()
			return
		}
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	ctx := r.Context()

	if err := h.store.AddMessage(id, map[string]string{"role": "user", "content": req.Content}); err != nil {
		slog.Error("sendMessageStream: AddMessage (user) failed", "conversation_id", id, "error", err)
		send(map[string]string{"type": "error", "message": "Failed to save your message. Please try again."})
		return
	}

	// Start title generation concurrently with Stage 1.
	// Detached from the request context so it completes even if the client disconnects.
	type titleMsg struct{ title string }
	titleCh := make(chan titleMsg, 1)
	if isFirst {
		go func() {
			tCtx, tCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer tCancel()
			titleCh <- titleMsg{h.council.GenerateTitle(tCtx, req.Content)}
		}()
	}

	// Stage 1
	send(map[string]string{"type": "stage1_start"})
	stage1, err := h.council.Stage1CollectResponses(ctx, req.Content)
	if err != nil {
		slog.Error("sendMessageStream: Stage1 failed", "conversation_id", id, "error", err)
		send(map[string]string{"type": "error", "message": "The council failed to collect responses. Please try again."})
		return
	}
	if len(stage1) == 0 {
		send(map[string]string{"type": "error", "message": "All models failed to respond. Please try again."})
		return
	}
	send(map[string]any{"type": "stage1_complete", "data": stage1})

	// Stage 2
	send(map[string]string{"type": "stage2_start"})
	stage2, labelToModel, err := h.council.Stage2CollectRankings(ctx, req.Content, stage1)
	if err != nil {
		slog.Error("sendMessageStream: Stage2 failed", "conversation_id", id, "error", err)
		send(map[string]string{"type": "error", "message": "The council failed to collect peer rankings. Please try again."})
		return
	}
	aggregateRankings, consensusW := h.council.CalculateAggregateRankings(stage2, labelToModel)
	send(map[string]any{
		"type": "stage2_complete",
		"data": stage2,
		"metadata": map[string]any{
			"label_to_model":     labelToModel,
			"aggregate_rankings": aggregateRankings,
			"consensus_w":        consensusW,
		},
	})

	// Stage 3
	send(map[string]string{"type": "stage3_start"})
	stage3, err := h.council.Stage3SynthesizeFinal(ctx, req.Content, stage1, stage2, consensusW)
	if err != nil {
		slog.Error("sendMessageStream: Stage3 failed", "conversation_id", id, "error", err)
		send(map[string]string{"type": "error", "message": "The chairman failed to synthesize a final answer. Please try again."})
		return
	}
	send(map[string]any{"type": "stage3_complete", "data": stage3})

	// Wait for title
	if isFirst {
		tm := <-titleCh
		if err := h.store.UpdateTitle(id, tm.title); err != nil {
			slog.Error("sendMessageStream: UpdateTitle failed", "conversation_id", id, "error", err)
		}
		send(map[string]any{"type": "title_complete", "data": map[string]string{"title": tm.title}})
	}

	// Save assistant message
	if err := h.store.AddMessage(id, map[string]any{
		"role":   "assistant",
		"stage1": stage1,
		"stage2": stage2,
		"stage3": stage3,
	}); err != nil {
		slog.Error("sendMessageStream: AddMessage (assistant) failed", "conversation_id", id, "error", err)
		send(map[string]string{"type": "error", "message": "failed to persist conversation"})
		return
	}

	send(map[string]string{"type": "complete"})
}
