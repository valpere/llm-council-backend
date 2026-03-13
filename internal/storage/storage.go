package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Conversation struct {
	ID        string            `json:"id"`
	CreatedAt string            `json:"created_at"`
	Title     string            `json:"title"`
	Messages  []json.RawMessage `json:"messages"`
}

type ConversationMeta struct {
	ID           string `json:"id"`
	CreatedAt    string `json:"created_at"`
	Title        string `json:"title"`
	MessageCount int    `json:"message_count"`
}

type Store struct {
	dataDir string
}

func New(dataDir string) *Store {
	return &Store{dataDir: dataDir}
}

func (s *Store) ensureDir() error {
	return os.MkdirAll(s.dataDir, 0755)
}

func (s *Store) path(id string) string {
	return filepath.Join(s.dataDir, id+".json")
}

func (s *Store) Create(id string) (*Conversation, error) {
	if err := s.ensureDir(); err != nil {
		return nil, err
	}
	conv := &Conversation{
		ID:        id,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Title:     "New Conversation",
		Messages:  []json.RawMessage{},
	}
	return conv, s.save(conv)
}

func (s *Store) Get(id string) (*Conversation, error) {
	data, err := os.ReadFile(s.path(id))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var conv Conversation
	return &conv, json.Unmarshal(data, &conv)
}

func (s *Store) save(conv *Conversation) error {
	data, err := json.MarshalIndent(conv, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(conv.ID), data, 0644)
}

func (s *Store) AddMessage(id string, msg any) error {
	conv, err := s.Get(id)
	if err != nil {
		return err
	}
	if conv == nil {
		return fmt.Errorf("conversation %s not found", id)
	}
	raw, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	conv.Messages = append(conv.Messages, raw)
	return s.save(conv)
}

func (s *Store) UpdateTitle(id, title string) error {
	conv, err := s.Get(id)
	if err != nil {
		return err
	}
	if conv == nil {
		return fmt.Errorf("conversation %s not found", id)
	}
	conv.Title = title
	return s.save(conv)
}

func (s *Store) List() ([]ConversationMeta, error) {
	if err := s.ensureDir(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		return nil, err
	}
	var metas []ConversationMeta
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dataDir, entry.Name()))
		if err != nil {
			continue
		}
		var conv Conversation
		if err := json.Unmarshal(data, &conv); err != nil {
			continue
		}
		metas = append(metas, ConversationMeta{
			ID:           conv.ID,
			CreatedAt:    conv.CreatedAt,
			Title:        conv.Title,
			MessageCount: len(conv.Messages),
		})
	}
	sort.Slice(metas, func(i, j int) bool {
		return metas[i].CreatedAt > metas[j].CreatedAt
	})
	return metas, nil
}
