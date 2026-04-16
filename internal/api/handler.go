package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"time"
	"unicode/utf8"

	"github.com/valpere/llm-council/internal/council"
	"github.com/valpere/llm-council/internal/storage"
)

const (
	maxRequestBodyBytes = 1 << 20 // 1 MiB
	maxTitleRunes       = 50
)

var uuidRE = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

var allowedOrigins = map[string]bool{
	"http://localhost:5173": true,
	"http://localhost:3000": true,
}

// Handler holds the dependencies for all API handlers.
type Handler struct {
	runner             council.Runner
	storage            storage.Storer
	logger             *slog.Logger
	defaultCouncilType string
}

// NewHandler constructs a Handler. A nil logger defaults to slog.Default().
func NewHandler(runner council.Runner, store storage.Storer, logger *slog.Logger, defaultCouncilType string) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{
		runner:             runner,
		storage:            store,
		logger:             logger,
		defaultCouncilType: defaultCouncilType,
	}
}

// RegisterRoutes attaches all API routes to mux wrapped with CORS and security middleware.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("GET /health/live", h.wrap(h.healthLive))
	mux.Handle("GET /health/ready", h.wrap(h.healthReady))
	mux.Handle("GET /api/conversations", h.wrap(h.listConversations))
	mux.Handle("POST /api/conversations", h.wrap(h.createConversation))
	mux.Handle("GET /api/conversations/{id}", h.wrap(h.getConversation))
	mux.Handle("POST /api/conversations/{id}/message", h.wrap(h.sendMessage))
	mux.Handle("POST /api/conversations/{id}/message/stream", h.wrap(h.sendMessageStream))
}

// wrap applies CORS and security headers to every route.
func (h *Handler) wrap(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'none'")

		if origin := r.Header.Get("Origin"); allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Add("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	})
}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		h.logger.Error("write JSON response", "error", err)
	}
}

func (h *Handler) writeError(w http.ResponseWriter, status int, msg string) {
	h.writeJSON(w, status, map[string]string{"error": msg})
}

func (h *Handler) healthLive(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) healthReady(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) listConversations(w http.ResponseWriter, r *http.Request) {
	convs, err := h.storage.ListConversations()
	if err != nil {
		h.logger.Error("list conversations", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if convs == nil {
		convs = []storage.ConversationMeta{}
	}
	h.writeJSON(w, http.StatusOK, convs)
}

func (h *Handler) createConversation(w http.ResponseWriter, r *http.Request) {
	conv, err := h.storage.CreateConversation()
	if err != nil {
		h.logger.Error("create conversation", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	h.writeJSON(w, http.StatusCreated, conv)
}

func (h *Handler) getConversation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidRE.MatchString(id) {
		h.writeError(w, http.StatusBadRequest, "invalid conversation id")
		return
	}
	conv, err := h.storage.GetConversation(id)
	if err != nil {
		var nfe *storage.NotFoundError
		if errors.As(err, &nfe) {
			h.writeError(w, http.StatusNotFound, "not found")
			return
		}
		h.logger.Error("get conversation", "id", id, "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	h.writeJSON(w, http.StatusOK, conv)
}

func (h *Handler) sendMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidRE.MatchString(id) {
		h.writeError(w, http.StatusBadRequest, "invalid conversation id")
		return
	}

	var body struct {
		Content     string `json:"content"`
		CouncilType string `json:"council_type"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Content == "" {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.storage.SaveUserMessage(id, body.Content); err != nil {
		var nfe *storage.NotFoundError
		if errors.As(err, &nfe) {
			h.writeError(w, http.StatusNotFound, "not found")
			return
		}
		h.logger.Error("save user message", "id", id, "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	councilType := body.CouncilType
	if councilType == "" {
		councilType = h.defaultCouncilType
	}

	var (
		stage1Results []council.StageOneResult
		stage2Data    council.Stage2CompleteData
		stage3Result  council.StageThreeResult
	)
	onEvent := func(eventType string, data any) {
		switch eventType {
		case "stage1_complete":
			if r, ok := data.([]council.StageOneResult); ok {
				stage1Results = r
			}
		case "stage2_complete":
			if d, ok := data.(council.Stage2CompleteData); ok {
				stage2Data = d
			}
		case "stage3_complete":
			if r, ok := data.(council.StageThreeResult); ok {
				stage3Result = r
			}
		}
	}

	if err := h.runner.RunFull(r.Context(), body.Content, councilType, onEvent); err != nil {
		var qe *council.QuorumError
		if errors.As(err, &qe) {
			h.logger.Warn("council quorum not met", "id", id, "got", qe.Got, "need", qe.Need)
			h.writeError(w, http.StatusServiceUnavailable, "council quorum not met")
		} else {
			h.logger.Error("council run", "id", id, "error", err)
			h.writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	msg := council.AssistantMessage{
		Role:     "assistant",
		Stage1:   stage1Results,
		Stage2:   stage2Data.Results,
		Stage3:   stage3Result,
		Metadata: stage2Data.Metadata,
	}

	if err := h.storage.SaveAssistantMessage(id, msg); err != nil {
		h.logger.Error("save assistant message", "id", id, "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	title := msg.Stage3.Content
	if utf8.RuneCountInString(title) > maxTitleRunes {
		runes := []rune(title)
		title = string(runes[:maxTitleRunes])
	}
	if err := h.storage.SaveTitle(id, title); err != nil {
		h.logger.Warn("save title", "id", id, "error", err)
	}

	h.writeJSON(w, http.StatusOK, msg)
}

// sseEnvelope is the JSON shape of every SSE data line.
type sseEnvelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

func (h *Handler) sendMessageStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		h.writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	id := r.PathValue("id")
	if !uuidRE.MatchString(id) {
		h.writeError(w, http.StatusBadRequest, "invalid conversation id")
		return
	}

	var body struct {
		Message     string `json:"message"`
		CouncilType string `json:"council_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Message == "" {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.storage.SaveUserMessage(id, body.Message); err != nil {
		var nfe *storage.NotFoundError
		if errors.As(err, &nfe) {
			h.writeError(w, http.StatusNotFound, "not found")
			return
		}
		h.logger.Error("save user message", "id", id, "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// SSE headers must be set before any write.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")

	councilType := body.CouncilType
	if councilType == "" {
		councilType = h.defaultCouncilType
	}

	var (
		stage1Results []council.StageOneResult
		stage2Data    council.Stage2CompleteData
		stage3Result  council.StageThreeResult
	)

	// sendSSE emits a standard {type, data} SSE event and flushes.
	sendSSE := func(eventType string, data any) {
		dataJSON, err := json.Marshal(data)
		if err != nil {
			h.logger.Error("marshal SSE event data", "type", eventType, "error", err)
			return
		}
		env := sseEnvelope{Type: eventType, Data: json.RawMessage(dataJSON)}
		envJSON, err := json.Marshal(env)
		if err != nil {
			h.logger.Error("marshal SSE envelope", "type", eventType, "error", err)
			return
		}
		fmt.Fprintf(w, "data: %s\n\n", envJSON)
		flusher.Flush()
	}

	// sendStage2SSE emits the spec-correct stage2_complete shape:
	// { "type": "stage2_complete", "data": [...], "metadata": {...} }
	// metadata is a top-level field, not nested under data.
	sendStage2SSE := func(d council.Stage2CompleteData) {
		type stage2Payload struct {
			Type     string                 `json:"type"`
			Data     []council.StageTwoResult `json:"data"`
			Metadata council.Metadata       `json:"metadata"`
		}
		b, err := json.Marshal(stage2Payload{
			Type:     "stage2_complete",
			Data:     d.Results,
			Metadata: d.Metadata,
		})
		if err != nil {
			h.logger.Error("marshal stage2 SSE payload", "error", err)
			return
		}
		fmt.Fprintf(w, "data: %s\n\n", b)
		flusher.Flush()
	}

	// sendErrorSSE emits { "type": "error", "message": "..." } per the SSE spec.
	sendErrorSSE := func(msg string) {
		type errPayload struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		}
		b, err := json.Marshal(errPayload{Type: "error", Message: msg})
		if err != nil {
			h.logger.Error("marshal error SSE payload", "error", err)
			return
		}
		fmt.Fprintf(w, "data: %s\n\n", b)
		flusher.Flush()
	}

	onEvent := func(eventType string, data any) {
		switch eventType {
		case "stage1_complete":
			if results, ok := data.([]council.StageOneResult); ok {
				stage1Results = results
			}
			sendSSE(eventType, data)
		case "stage2_complete":
			if d, ok := data.(council.Stage2CompleteData); ok {
				stage2Data = d
				sendStage2SSE(d)
			}
		case "stage3_complete":
			if result, ok := data.(council.StageThreeResult); ok {
				stage3Result = result
			}
			sendSSE(eventType, data)
		default:
			sendSSE(eventType, data)
		}
	}

	if err := h.runner.RunFull(r.Context(), body.Message, councilType, onEvent); err != nil {
		var qe *council.QuorumError
		if errors.As(err, &qe) {
			h.logger.Warn("council quorum not met", "id", id, "got", qe.Got, "need", qe.Need)
			sendErrorSSE("council quorum not met")
		} else {
			h.logger.Error("council run", "id", id, "error", err)
			sendErrorSSE("internal server error")
		}
		return
	}

	msg := council.AssistantMessage{
		Role:     "assistant",
		Stage1:   stage1Results,
		Stage2:   stage2Data.Results,
		Stage3:   stage3Result,
		Metadata: stage2Data.Metadata,
	}

	if err := h.storage.SaveAssistantMessage(id, msg); err != nil {
		h.logger.Error("save assistant message", "id", id, "error", err)
		sendErrorSSE("internal server error")
		return
	}

	// Title generation: run in a goroutine to avoid blocking the ResponseWriter.
	// A buffered channel of size 1 prevents goroutine leak if the select times out.
	titleCh := make(chan string, 1)
	go func() {
		content := msg.Stage3.Content
		if len(content) > 50 {
			content = content[:50]
		}
		titleCh <- content
	}()

	select {
	case title := <-titleCh:
		if err := h.storage.SaveTitle(id, title); err != nil {
			h.logger.Warn("save title", "id", id, "error", err)
		}
		sendSSE("title_complete", map[string]string{"title": title})
	case <-time.After(30 * time.Second):
		h.logger.Warn("title generation timed out", "id", id)
	}

	// Spec: { "type": "complete" } with no payload.
	fmt.Fprintf(w, "data: {\"type\":\"complete\"}\n\n")
	flusher.Flush()
}
