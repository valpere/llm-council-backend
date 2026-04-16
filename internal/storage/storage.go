package storage

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/valpere/llm-council/internal/council"
)

// NotFoundError is returned when a requested conversation does not exist.
type NotFoundError struct {
	ID string
}

func (e NotFoundError) Error() string {
	return fmt.Sprintf("conversation not found: %s", e.ID)
}

// ConversationMeta holds lightweight metadata for list responses.
type ConversationMeta struct {
	ID           string    `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	Title        string    `json:"title"`
	MessageCount int       `json:"message_count"`
}

// Conversation is the full stored record including the message history.
// Messages is []json.RawMessage so the heterogeneous user/assistant array
// survives round-trips without losing type information; callers demux by
// inspecting the "role" field of each element.
type Conversation struct {
	ID        string            `json:"id"`
	CreatedAt time.Time         `json:"created_at"`
	Title     string            `json:"title"`
	Messages  []json.RawMessage `json:"messages"`
}

// Storer is the persistence interface. The handler depends only on this
// interface — never on a concrete implementation.
type Storer interface {
	CreateConversation() (*Conversation, error)
	GetConversation(id string) (*Conversation, error)
	ListConversations() ([]ConversationMeta, error)
	SaveUserMessage(id, content string) error
	SaveAssistantMessage(id string, msg council.AssistantMessage) error
	SaveTitle(id, title string) error
}

// Store is the JSON file backend. One file per conversation under dataDir.
type Store struct {
	dataDir string
	logger  *slog.Logger
}

// NewStore creates the data directory if needed and returns a Store.
func NewStore(dataDir string, logger *slog.Logger) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	return &Store{dataDir: dataDir, logger: logger}, nil
}

// Compile-time assertion: Store implements Storer.
var _ Storer = (*Store)(nil)

func (s *Store) filePath(id string) string {
	return filepath.Join(s.dataDir, id+".json")
}

func (s *Store) readConversation(id string) (*Conversation, error) {
	data, err := os.ReadFile(s.filePath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &NotFoundError{ID: id}
		}
		return nil, fmt.Errorf("read %s: %w", id, err)
	}
	var c Conversation
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("unmarshal %s: %w", id, err)
	}
	return &c, nil
}

func (s *Store) writeConversation(c *Conversation) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal conversation: %w", err)
	}
	if err := os.WriteFile(s.filePath(c.ID), data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", c.ID, err)
	}
	return nil
}

func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func (s *Store) CreateConversation() (*Conversation, error) {
	id, err := newUUID()
	if err != nil {
		return nil, fmt.Errorf("generate id: %w", err)
	}
	c := &Conversation{
		ID:        id,
		CreatedAt: time.Now().UTC(),
		Messages:  []json.RawMessage{},
	}
	if err := s.writeConversation(c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Store) GetConversation(id string) (*Conversation, error) {
	return s.readConversation(id)
}

func (s *Store) ListConversations() ([]ConversationMeta, error) {
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}
	var metas []ConversationMeta
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		id := e.Name()[:len(e.Name())-5] // strip .json
		data, err := os.ReadFile(filepath.Join(s.dataDir, e.Name()))
		if err != nil {
			s.logger.Warn("skipping unreadable conversation file", "file", e.Name(), "error", err)
			continue
		}
		var c Conversation
		if err := json.Unmarshal(data, &c); err != nil {
			s.logger.Warn("skipping corrupt conversation file", "file", e.Name(), "error", err)
			continue
		}
		metas = append(metas, ConversationMeta{
			ID:           id,
			CreatedAt:    c.CreatedAt,
			Title:        c.Title,
			MessageCount: len(c.Messages),
		})
	}
	sort.Slice(metas, func(i, j int) bool {
		return metas[i].CreatedAt.After(metas[j].CreatedAt)
	})
	return metas, nil
}

func (s *Store) SaveUserMessage(id, content string) error {
	c, err := s.readConversation(id)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}{Role: "user", Content: content})
	if err != nil {
		return fmt.Errorf("marshal user message: %w", err)
	}
	c.Messages = append(c.Messages, raw)
	return s.writeConversation(c)
}

func (s *Store) SaveAssistantMessage(id string, msg council.AssistantMessage) error {
	c, err := s.readConversation(id)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal assistant message: %w", err)
	}
	c.Messages = append(c.Messages, raw)
	return s.writeConversation(c)
}

func (s *Store) SaveTitle(id, title string) error {
	c, err := s.readConversation(id)
	if err != nil {
		return err
	}
	c.Title = title
	return s.writeConversation(c)
}
