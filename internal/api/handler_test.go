package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/valpere/llm-council/internal/council"
	"github.com/valpere/llm-council/internal/storage"
)

// ── mocks ────────────────────────────────────────────────────────────────────

type mockStorer struct {
	listConversations    func() ([]storage.ConversationMeta, error)
	createConversation   func() (*storage.Conversation, error)
	getConversation      func(string) (*storage.Conversation, error)
	saveUserMessage      func(string, string) error
	saveAssistantMessage func(string, council.AssistantMessage) error
	saveTitle            func(string, string) error
}

func (m *mockStorer) ListConversations() ([]storage.ConversationMeta, error) {
	if m.listConversations != nil {
		return m.listConversations()
	}
	return nil, nil
}
func (m *mockStorer) CreateConversation() (*storage.Conversation, error) {
	if m.createConversation != nil {
		return m.createConversation()
	}
	return &storage.Conversation{ID: testConvID}, nil
}
func (m *mockStorer) GetConversation(id string) (*storage.Conversation, error) {
	if m.getConversation != nil {
		return m.getConversation(id)
	}
	return &storage.Conversation{ID: id}, nil
}
func (m *mockStorer) SaveUserMessage(id, content string) error {
	if m.saveUserMessage != nil {
		return m.saveUserMessage(id, content)
	}
	return nil
}
func (m *mockStorer) SaveAssistantMessage(id string, msg council.AssistantMessage) error {
	if m.saveAssistantMessage != nil {
		return m.saveAssistantMessage(id, msg)
	}
	return nil
}
func (m *mockStorer) SaveTitle(id, title string) error {
	if m.saveTitle != nil {
		return m.saveTitle(id, title)
	}
	return nil
}

type mockRunner struct {
	runFull func(context.Context, string, string, council.EventFunc) error
}

func (m *mockRunner) RunFull(ctx context.Context, query, councilType string, onEvent council.EventFunc) error {
	if m.runFull != nil {
		return m.runFull(ctx, query, councilType, onEvent)
	}
	return nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

// testConvID is a canonical UUID v4 used across tests.
const testConvID = "00000000-0000-4000-8000-000000000001"

// newTestHandler builds a Handler with no-op defaults and a silent logger.
func newTestHandler(storer *mockStorer, runner *mockRunner) *Handler {
	return NewHandler(runner, storer, nil, "standard")
}

// parseSSEEventTypes returns the "type" field from every SSE data line in body.
func parseSSEEventTypes(body string) []string {
	var types []string
	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var env struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(line[6:]), &env); err == nil && env.Type != "" {
			types = append(types, env.Type)
		}
	}
	return types
}

//── ListConversations ────────────────────────────────────────────────────────

func TestListConversations(t *testing.T) {
	tests := []struct {
		name     string
		storer   *mockStorer
		wantCode int
		check    func(t *testing.T, body string)
	}{
		{
			name: "happy path",
			storer: &mockStorer{
				listConversations: func() ([]storage.ConversationMeta, error) {
					return []storage.ConversationMeta{{ID: testConvID, Title: "Test"}}, nil
				},
			},
			wantCode: http.StatusOK,
			check: func(t *testing.T, body string) {
				var convs []storage.ConversationMeta
				if err := json.Unmarshal([]byte(strings.TrimSpace(body)), &convs); err != nil {
					t.Fatalf("parse body: %v", err)
				}
				if len(convs) != 1 || convs[0].ID != testConvID {
					t.Errorf("body: got %v, want 1 item with ID %q", convs, testConvID)
				}
			},
		},
		{
			name: "empty list returns [] not null",
			storer: &mockStorer{
				listConversations: func() ([]storage.ConversationMeta, error) {
					return nil, nil // storage returns nil slice → handler converts to []
				},
			},
			wantCode: http.StatusOK,
			check: func(t *testing.T, body string) {
				trimmed := strings.TrimSpace(body)
				if !strings.HasPrefix(trimmed, "[") {
					t.Errorf("body: got %q, want JSON array (not null)", trimmed)
				}
			},
		},
		{
			name: "storage error returns 500",
			storer: &mockStorer{
				listConversations: func() ([]storage.ConversationMeta, error) {
					return nil, errors.New("disk failure")
				},
			},
			wantCode: http.StatusInternalServerError,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHandler(tc.storer, &mockRunner{})
			req := httptest.NewRequest(http.MethodGet, "/api/conversations", nil)
			w := httptest.NewRecorder()
			h.listConversations(w, req)
			if w.Code != tc.wantCode {
				t.Errorf("status: got %d, want %d", w.Code, tc.wantCode)
			}
			if tc.check != nil {
				tc.check(t, w.Body.String())
			}
		})
	}
}

// ── CreateConversation ───────────────────────────────────────────────────────

func TestCreateConversation(t *testing.T) {
	tests := []struct {
		name     string
		storer   *mockStorer
		wantCode int
	}{
		{
			name: "happy path returns 201",
			storer: &mockStorer{
				createConversation: func() (*storage.Conversation, error) {
					return &storage.Conversation{ID: testConvID, Title: "New Conversation"}, nil
				},
			},
			wantCode: http.StatusCreated,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHandler(tc.storer, &mockRunner{})
			req := httptest.NewRequest(http.MethodPost, "/api/conversations", nil)
			w := httptest.NewRecorder()
			h.createConversation(w, req)
			if w.Code != tc.wantCode {
				t.Errorf("status: got %d, want %d", w.Code, tc.wantCode)
			}
		})
	}
}

// ── GetConversation ──────────────────────────────────────────────────────────

func TestGetConversation(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		storer   *mockStorer
		wantCode int
	}{
		{
			name: "200 found",
			id:   testConvID,
			storer: &mockStorer{
				getConversation: func(id string) (*storage.Conversation, error) {
					return &storage.Conversation{ID: id}, nil
				},
			},
			wantCode: http.StatusOK,
		},
		{
			name: "404 not found",
			id:   testConvID,
			storer: &mockStorer{
				getConversation: func(id string) (*storage.Conversation, error) {
					return nil, &storage.NotFoundError{ID: id}
				},
			},
			wantCode: http.StatusNotFound,
		},
		{
			name:     "400 invalid UUID",
			id:       "not-a-uuid",
			storer:   &mockStorer{},
			wantCode: http.StatusBadRequest,
		},
		{
			name: "500 storage error",
			id:   testConvID,
			storer: &mockStorer{
				getConversation: func(id string) (*storage.Conversation, error) {
					return nil, errors.New("db error")
				},
			},
			wantCode: http.StatusInternalServerError,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHandler(tc.storer, &mockRunner{})
			req := httptest.NewRequest(http.MethodGet, "/api/conversations/"+tc.id, nil)
			req.SetPathValue("id", tc.id)
			w := httptest.NewRecorder()
			h.getConversation(w, req)
			if w.Code != tc.wantCode {
				t.Errorf("status: got %d, want %d", w.Code, tc.wantCode)
			}
		})
	}
}

// ── SendMessageStream ────────────────────────────────────────────────────────

// okStorer returns a mockStorer that succeeds silently for all write operations.
func okStorer() *mockStorer {
	return &mockStorer{
		saveUserMessage:      func(string, string) error { return nil },
		saveAssistantMessage: func(string, council.AssistantMessage) error { return nil },
		saveTitle:            func(string, string) error { return nil },
	}
}

func TestSendMessageStream(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		storer   *mockStorer
		runner   *mockRunner
		wantCode int
		checkSSE func(t *testing.T, body string)
	}{
		{
			name:   "event sequence with metadata in stage2_complete",
			body:   `{"message":"what is Go?","council_type":"standard"}`,
			storer: okStorer(),
			runner: &mockRunner{
				runFull: func(ctx context.Context, query, ct string, onEvent council.EventFunc) error {
					onEvent("stage1_complete", []council.StageOneResult{
						{Label: "Response A", Content: "Go is a compiled language"},
					})
					onEvent("stage2_complete", council.Stage2CompleteData{
						Results: []council.StageTwoResult{
							{ReviewerLabel: "Response A", Rankings: []string{"Response A"}},
						},
						Metadata: council.Metadata{
							CouncilType:  "standard",
							ConsensusW:   0.9,
							LabelToModel: map[string]string{"Response A": "openai/gpt-4o"},
						},
					})
					onEvent("stage3_complete", council.StageThreeResult{Content: "final answer"})
					return nil
				},
			},
			wantCode: http.StatusOK,
			checkSSE: func(t *testing.T, body string) {
				types := parseSSEEventTypes(body)

				// Verify required events are present.
				for _, want := range []string{"stage1_complete", "stage2_complete", "stage3_complete", "complete"} {
					found := false
					for _, got := range types {
						if got == want {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("missing event %q in sequence %v", want, types)
					}
				}

				// "complete" must be the last event.
				if len(types) == 0 || types[len(types)-1] != "complete" {
					t.Errorf("last event: got %v, want 'complete'", types)
				}

				// stage2_complete must have metadata as a TOP-LEVEL field per the
				// streaming spec: { "type": "stage2_complete", "data": [...], "metadata": {...} }
				for _, line := range strings.Split(body, "\n") {
					if !strings.HasPrefix(line, "data: ") {
						continue
					}
					var env struct {
						Type     string               `json:"type"`
						Data     []council.StageTwoResult `json:"data"`
						Metadata council.Metadata     `json:"metadata"`
					}
					if err := json.Unmarshal([]byte(line[6:]), &env); err != nil || env.Type != "stage2_complete" {
						continue
					}
					if env.Metadata.ConsensusW != 0.9 {
						t.Errorf("consensus_w: got %f, want 0.9", env.Metadata.ConsensusW)
					}
					if env.Metadata.LabelToModel["Response A"] != "openai/gpt-4o" {
						t.Errorf("label_to_model: got %v", env.Metadata.LabelToModel)
					}
					break
				}
			},
		},
		{
			name:   "QuorumError emits error event",
			body:   `{"message":"test","council_type":"standard"}`,
			storer: okStorer(),
			runner: &mockRunner{
				runFull: func(ctx context.Context, query, ct string, onEvent council.EventFunc) error {
					return &council.QuorumError{Got: 1, Need: 3}
				},
			},
			wantCode: http.StatusOK,
			checkSSE: func(t *testing.T, body string) {
				// Error event must be present with a non-empty "message" field
				// per the SSE spec: { "type": "error", "message": "..." }
				for _, line := range strings.Split(body, "\n") {
					if !strings.HasPrefix(line, "data: ") {
						continue
					}
					var env struct {
						Type    string `json:"type"`
						Message string `json:"message"`
					}
					if err := json.Unmarshal([]byte(line[6:]), &env); err != nil || env.Type != "error" {
						continue
					}
					if env.Message == "" {
						t.Errorf("error event missing 'message' field, got: %s", line[6:])
					}
					return
				}
				t.Errorf("expected 'error' event for QuorumError, got:\n%s", body)
			},
		},
		{
			name:     "malformed JSON body returns 400 before SSE starts",
			body:     `not json`,
			storer:   okStorer(),
			runner:   &mockRunner{},
			wantCode: http.StatusBadRequest,
			checkSSE: func(t *testing.T, body string) {
				// Must be a plain JSON error, not an SSE stream.
				var errResp map[string]string
				if err := json.Unmarshal([]byte(strings.TrimSpace(body)), &errResp); err != nil {
					t.Errorf("expected JSON error body, got: %q", body)
				}
				if errResp["error"] == "" {
					t.Errorf("error field missing in response: %v", errResp)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHandler(tc.storer, tc.runner)
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/conversations/"+testConvID+"/message/stream",
				bytes.NewBufferString(tc.body),
			)
			req.SetPathValue("id", testConvID)
			w := httptest.NewRecorder()
			h.sendMessageStream(w, req)
			if w.Code != tc.wantCode {
				t.Errorf("status: got %d, want %d\nbody: %s", w.Code, tc.wantCode, w.Body.String())
			}
			if tc.checkSSE != nil {
				tc.checkSSE(t, w.Body.String())
			}
		})
	}
}
