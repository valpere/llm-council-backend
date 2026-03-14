package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
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

var validID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type Store struct {
	dataDir string
	locks   sync.Map // map[string]*sync.Mutex — per-conversation write lock; grows with conversation count (one entry per UUID, pointer-sized — acceptable for typical usage)
}

func New(dataDir string) *Store {
	return &Store{dataDir: dataDir}
}

func (s *Store) ensureDir() error {
	return os.MkdirAll(s.dataDir, 0700)
}

func (s *Store) path(id string) string {
	return filepath.Join(s.dataDir, id+".json")
}

// lockConv acquires the per-conversation mutex and returns an unlock function.
func (s *Store) lockConv(id string) func() {
	v, _ := s.locks.LoadOrStore(id, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

func (s *Store) Create(id string) (*Conversation, error) {
	if !validID.MatchString(id) {
		return nil, fmt.Errorf("invalid conversation ID")
	}
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
	if !validID.MatchString(id) {
		return nil, fmt.Errorf("invalid conversation ID")
	}
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

// save writes atomically via a temp file + rename to avoid partial writes.
func (s *Store) save(conv *Conversation) error {
	if err := s.ensureDir(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(conv, "", "  ")
	if err != nil {
		return err
	}
	tmpPath := s.path(conv.ID) + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.path(conv.ID))
}

func (s *Store) AddMessage(id string, msg any) error {
	unlock := s.lockConv(id)
	defer unlock()

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
	unlock := s.lockConv(id)
	defer unlock()

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
		filePath := filepath.Join(s.dataDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			log.Printf("storage: skipping %s: read error: %v", filePath, err)
			continue
		}
		var conv Conversation
		if err := json.Unmarshal(data, &conv); err != nil {
			log.Printf("storage: skipping %s: unmarshal error: %v", filePath, err)
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
