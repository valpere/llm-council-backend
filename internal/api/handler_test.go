package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"llm-council/internal/council"
	"llm-council/internal/storage"
)

// ---- fakeStore ---------------------------------------------------------------

type fakeStore struct {
	mu    sync.Mutex
	convs map[string]*storage.Conversation
}

var _ storage.Storer = (*fakeStore)(nil)

func newFakeStore() *fakeStore {
	return &fakeStore{convs: make(map[string]*storage.Conversation)}
}

func (f *fakeStore) Create(id string) (*storage.Conversation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	conv := &storage.Conversation{
		ID:        id,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Title:     "New Conversation",
		Messages:  []json.RawMessage{},
	}
	f.convs[id] = conv
	return conv, nil
}

func (f *fakeStore) Get(id string) (*storage.Conversation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.convs[id], nil
}

func (f *fakeStore) AddMessage(id string, msg any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	conv, ok := f.convs[id]
	if !ok {
		return fmt.Errorf("conversation %s not found", id)
	}
	raw, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	conv.Messages = append(conv.Messages, raw)
	return nil
}

func (f *fakeStore) UpdateTitle(id, title string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	conv, ok := f.convs[id]
	if !ok {
		return fmt.Errorf("conversation %s not found", id)
	}
	conv.Title = title
	return nil
}

func (f *fakeStore) List() ([]storage.ConversationMeta, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	metas := make([]storage.ConversationMeta, 0, len(f.convs))
	for _, c := range f.convs {
		metas = append(metas, storage.ConversationMeta{
			ID: c.ID, Title: c.Title, CreatedAt: c.CreatedAt,
			MessageCount: len(c.Messages),
		})
	}
	return metas, nil
}

// ---- fakeCouncil -------------------------------------------------------------

type fakeCouncil struct {
	result council.Result
	err    error
}

var _ council.Runner = (*fakeCouncil)(nil)

func (f *fakeCouncil) RunFull(_ context.Context, _ string) (council.Result, error) {
	return f.result, f.err
}

func (f *fakeCouncil) GenerateTitle(_ context.Context, _ string) string { return "Test Title" }

func (f *fakeCouncil) Stage1CollectResponses(_ context.Context, _ string) ([]council.StageOneResult, error) {
	return nil, nil
}

func (f *fakeCouncil) Stage2CollectRankings(_ context.Context, _ string, _ []council.StageOneResult) ([]council.StageTwoResult, map[string]string, error) {
	return nil, nil, nil
}

func (f *fakeCouncil) Stage3SynthesizeFinal(_ context.Context, _ string, _ []council.StageOneResult, _ []council.StageTwoResult) (council.StageThreeResult, error) {
	return council.StageThreeResult{}, nil
}

func (f *fakeCouncil) CalculateAggregateRankings(_ []council.StageTwoResult, _ map[string]string) []council.AggregateRanking {
	return nil
}

// ---- helpers -----------------------------------------------------------------

func newTestHandler(store storage.Storer, c council.Runner) http.Handler {
	return New(c, store, "", nil).Routes()
}

func newTestHandlerWithDataDir(store storage.Storer, c council.Runner, dataDir string) http.Handler {
	return New(c, store, dataDir, nil).Routes()
}

func do(t *testing.T, handler http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var b bytes.Buffer
	if body != nil {
		json.NewEncoder(&b).Encode(body)
	}
	req := httptest.NewRequest(method, path, &b)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.NewDecoder(w.Body).Decode(&m); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	return m
}

// ---- tests -------------------------------------------------------------------

func TestHealthLive(t *testing.T) {
	h := newTestHandler(newFakeStore(), &fakeCouncil{})
	w := do(t, h, http.MethodGet, "/health/live", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusOK)
	}
	body := decodeBody(t, w)
	if body["status"] != "ok" {
		t.Errorf("status field: got %v, want \"ok\"", body["status"])
	}
}

func TestHealthReady_ok(t *testing.T) {
	h := newTestHandlerWithDataDir(newFakeStore(), &fakeCouncil{}, t.TempDir())
	w := do(t, h, http.MethodGet, "/health/ready", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHealthReady_unavailable(t *testing.T) {
	// Create a file (not a dir) at the path — MkdirAll will fail because a file blocks it.
	blockingFile := t.TempDir() + "/blocking-file"
	if err := os.WriteFile(blockingFile, []byte{}, 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}
	h := newTestHandlerWithDataDir(newFakeStore(), &fakeCouncil{}, blockingFile+"/subdir")
	w := do(t, h, http.MethodGet, "/health/ready", nil)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
	body := decodeBody(t, w)
	if body["status"] != "not ready" {
		t.Errorf("status field: got %v, want \"not ready\"", body["status"])
	}
	if body["error"] != "data directory unavailable" {
		t.Errorf("error field: got %v, want \"data directory unavailable\"", body["error"])
	}
}

func TestHealthCheck(t *testing.T) {
	h := newTestHandler(newFakeStore(), &fakeCouncil{})
	w := do(t, h, http.MethodGet, "/", nil)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusOK)
	}
	body := decodeBody(t, w)
	if body["status"] != "ok" {
		t.Errorf("status field: got %v, want \"ok\"", body["status"])
	}
}

func TestCreateConversation(t *testing.T) {
	h := newTestHandler(newFakeStore(), &fakeCouncil{})
	w := do(t, h, http.MethodPost, "/api/conversations", nil)

	if w.Code != http.StatusCreated {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusCreated)
	}
	body := decodeBody(t, w)
	id, ok := body["id"].(string)
	if !ok || id == "" {
		t.Errorf("response id field missing or not a non-empty string: got %T (%v)", body["id"], body["id"])
	}
}

func TestListConversations(t *testing.T) {
	store := newFakeStore()
	store.Create("aaaaaaaa-0000-0000-0000-000000000001")
	store.Create("aaaaaaaa-0000-0000-0000-000000000002")
	h := newTestHandler(store, &fakeCouncil{})

	w := do(t, h, http.MethodGet, "/api/conversations", nil)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusOK)
	}
	var list []map[string]any
	if err := json.NewDecoder(w.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("list length: got %d, want 2", len(list))
	}
}

func TestGetConversation_found(t *testing.T) {
	store := newFakeStore()
	conv, _ := store.Create("aaaaaaaa-0000-0000-0000-000000000001")
	h := newTestHandler(store, &fakeCouncil{})

	w := do(t, h, http.MethodGet, "/api/conversations/"+conv.ID, nil)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusOK)
	}
	body := decodeBody(t, w)
	if body["id"] != conv.ID {
		t.Errorf("id: got %v, want %v", body["id"], conv.ID)
	}
}

func TestGetConversation_notFound(t *testing.T) {
	h := newTestHandler(newFakeStore(), &fakeCouncil{})
	w := do(t, h, http.MethodGet, "/api/conversations/aaaaaaaa-0000-0000-0000-000000000099", nil)

	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestSendMessage_success(t *testing.T) {
	store := newFakeStore()
	id := "aaaaaaaa-0000-0000-0000-000000000001"
	store.Create(id)

	c := &fakeCouncil{result: council.Result{
		Stage3: council.StageThreeResult{Model: "test-model", Response: "final answer"},
	}}
	h := newTestHandler(store, c)

	w := do(t, h, http.MethodPost, "/api/conversations/"+id+"/message",
		map[string]string{"content": "what is 2+2?"})

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusOK)
	}
	body := decodeBody(t, w)
	if body["stage3"] == nil {
		t.Error("response missing stage3 field")
	}
}

func TestSendMessage_runFullError(t *testing.T) {
	store := newFakeStore()
	id := "aaaaaaaa-0000-0000-0000-000000000001"
	store.Create(id)

	c := &fakeCouncil{err: context.DeadlineExceeded}
	h := newTestHandler(store, c)

	w := do(t, h, http.MethodPost, "/api/conversations/"+id+"/message",
		map[string]string{"content": "hello"})

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestSendMessage_conversationNotFound(t *testing.T) {
	h := newTestHandler(newFakeStore(), &fakeCouncil{})
	w := do(t, h, http.MethodPost, "/api/conversations/aaaaaaaa-0000-0000-0000-000000000099/message",
		map[string]string{"content": "hello"})

	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusNotFound)
	}
}
