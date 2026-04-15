package storage

import (
	"encoding/json"
	"errors"
	"fmt"
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

// Store is the concrete JSON backend. It satisfies Storer.
// Implementation lives in the L2.2 JSON storage backend issue.
type Store struct{}

var errNotImplemented = errors.New("not implemented")

func (s *Store) CreateConversation() (*Conversation, error)                         { return nil, errNotImplemented }
func (s *Store) GetConversation(id string) (*Conversation, error)                   { return nil, errNotImplemented }
func (s *Store) ListConversations() ([]ConversationMeta, error)                     { return nil, errNotImplemented }
func (s *Store) SaveUserMessage(id, content string) error                           { return errNotImplemented }
func (s *Store) SaveAssistantMessage(id string, msg council.AssistantMessage) error { return errNotImplemented }
func (s *Store) SaveTitle(id, title string) error                                   { return errNotImplemented }

// Compile-time assertion: Store implements Storer.
var _ Storer = (*Store)(nil)
