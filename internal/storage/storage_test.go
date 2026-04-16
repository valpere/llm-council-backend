package storage_test

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/valpere/llm-council/internal/council"
	"github.com/valpere/llm-council/internal/storage"
)

func newTestStore(t *testing.T) *storage.Store {
	t.Helper()
	s, err := storage.NewStore(t.TempDir(), slog.Default())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s
}

func TestCreateGetRoundTrip(t *testing.T) {
	s := newTestStore(t)

	c, err := s.CreateConversation()
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	if c.ID == "" {
		t.Fatal("expected non-empty ID")
	}

	got, err := s.GetConversation(c.ID)
	if err != nil {
		t.Fatalf("GetConversation: %v", err)
	}
	if got.ID != c.ID {
		t.Errorf("ID: got %q, want %q", got.ID, c.ID)
	}
	if !got.CreatedAt.Equal(c.CreatedAt) {
		t.Errorf("CreatedAt: got %v, want %v", got.CreatedAt, c.CreatedAt)
	}
}

func TestListNewestFirst(t *testing.T) {
	s := newTestStore(t)

	c1, err := s.CreateConversation()
	if err != nil {
		t.Fatalf("CreateConversation c1: %v", err)
	}
	time.Sleep(2 * time.Millisecond) // ensure distinct timestamps
	c2, err := s.CreateConversation()
	if err != nil {
		t.Fatalf("CreateConversation c2: %v", err)
	}

	list, err := s.ListConversations()
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 conversations, got %d", len(list))
	}
	if list[0].ID != c2.ID {
		t.Errorf("expected newest first: got %q, want %q", list[0].ID, c2.ID)
	}
	if list[1].ID != c1.ID {
		t.Errorf("expected oldest last: got %q, want %q", list[1].ID, c1.ID)
	}
}

func TestSaveAssistantMessageRoundTrip(t *testing.T) {
	s := newTestStore(t)

	c, err := s.CreateConversation()
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	msg := council.AssistantMessage{
		Role: "assistant",
		Stage3: council.StageThreeResult{
			Content:    "synthesised answer",
			Model:      "openai/gpt-4o",
			DurationMs: 1234,
		},
		Metadata: council.Metadata{
			CouncilType: "default",
			LabelToModel: map[string]string{
				"Response A": "openai/gpt-4o",
				"Response B": "anthropic/claude-haiku-4-5",
			},
			AggregateRankings: []council.RankedModel{
				{Model: "openai/gpt-4o", Score: 1.5},
				{Model: "anthropic/claude-haiku-4-5", Score: 2.5},
			},
			ConsensusW: 0.72,
		},
	}

	if err := s.SaveAssistantMessage(c.ID, msg); err != nil {
		t.Fatalf("SaveAssistantMessage: %v", err)
	}

	got, err := s.GetConversation(c.ID)
	if err != nil {
		t.Fatalf("GetConversation: %v", err)
	}
	if len(got.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(got.Messages))
	}

	var gotMsg council.AssistantMessage
	if err := json.Unmarshal(got.Messages[0], &gotMsg); err != nil {
		t.Fatalf("unmarshal assistant message: %v", err)
	}

	if gotMsg.Metadata.ConsensusW != 0.72 {
		t.Errorf("ConsensusW: got %v, want 0.72", gotMsg.Metadata.ConsensusW)
	}
	if gotMsg.Metadata.CouncilType != "default" {
		t.Errorf("CouncilType: got %q, want %q", gotMsg.Metadata.CouncilType, "default")
	}
	if len(gotMsg.Metadata.AggregateRankings) != 2 {
		t.Errorf("AggregateRankings len: got %d, want 2", len(gotMsg.Metadata.AggregateRankings))
	}
	if gotMsg.Stage3.DurationMs != 1234 {
		t.Errorf("Stage3.DurationMs: got %d, want 1234", gotMsg.Stage3.DurationMs)
	}
}

func TestMissingMetadataUnmarshalsToZero(t *testing.T) {
	dir := t.TempDir()
	s, err := storage.NewStore(dir, slog.Default())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	c, err := s.CreateConversation()
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	// Overwrite the file with a legacy message that has no metadata field.
	type legacyConv struct {
		ID        string            `json:"id"`
		CreatedAt time.Time         `json:"created_at"`
		Messages  []json.RawMessage `json:"messages"`
	}
	legacyMsg := json.RawMessage(`{"role":"assistant","stage1":[],"stage2":[],"stage3":{"content":"old","model":"gpt-3","duration_ms":0}}`)
	lc := legacyConv{
		ID:        c.ID,
		CreatedAt: c.CreatedAt,
		Messages:  []json.RawMessage{legacyMsg},
	}
	data, _ := json.Marshal(lc)
	if err := os.WriteFile(filepath.Join(dir, c.ID+".json"), data, 0644); err != nil {
		t.Fatalf("write legacy file: %v", err)
	}

	got, err := s.GetConversation(c.ID)
	if err != nil {
		t.Fatalf("GetConversation: %v", err)
	}
	if len(got.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(got.Messages))
	}

	var gotMsg council.AssistantMessage
	if err := json.Unmarshal(got.Messages[0], &gotMsg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if gotMsg.Metadata.ConsensusW != 0 {
		t.Errorf("ConsensusW: got %v, want 0", gotMsg.Metadata.ConsensusW)
	}
	if gotMsg.Metadata.CouncilType != "" {
		t.Errorf("CouncilType: got %q, want empty", gotMsg.Metadata.CouncilType)
	}
	if gotMsg.Metadata.LabelToModel != nil {
		t.Errorf("LabelToModel: got %v, want nil", gotMsg.Metadata.LabelToModel)
	}
}

func TestNotFoundError(t *testing.T) {
	s := newTestStore(t)

	_, err := s.GetConversation("00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var nfe *storage.NotFoundError
	if !errors.As(err, &nfe) {
		t.Errorf("expected *storage.NotFoundError, got %T: %v", err, err)
	}
	if nfe.ID != "00000000-0000-0000-0000-000000000000" {
		t.Errorf("NotFoundError.ID: got %q", nfe.ID)
	}
}

func TestCorruptFileSkippedInList(t *testing.T) {
	dir := t.TempDir()
	s, err := storage.NewStore(dir, slog.Default())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	c, err := s.CreateConversation()
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	// Plant a corrupt JSON file alongside the valid one.
	if err := os.WriteFile(filepath.Join(dir, "corrupt.json"), []byte("{not valid json{{"), 0644); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}

	list, err := s.ListConversations()
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 conversation (corrupt skipped), got %d", len(list))
	}
	if len(list) > 0 && list[0].ID != c.ID {
		t.Errorf("ID: got %q, want %q", list[0].ID, c.ID)
	}
}
