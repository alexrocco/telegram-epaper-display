// Package store keeps the most recent channel messages and the Telegram poll
// offset, persisting them to a JSON file so they survive restarts.
package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// Store is a concurrency-safe ring buffer of recent messages plus the last
// processed Telegram update offset.
type Store struct {
	mu       sync.RWMutex
	path     string
	capacity int

	data persisted
}

type persisted struct {
	// Offset is the next getUpdates offset to request.
	Offset int `json:"offset"`
	// Messages are kept newest-last internally; Recent() returns newest-first.
	Messages []Message `json:"messages"`
}

// New creates a Store backed by path, retaining up to capacity messages.
// If the file exists it is loaded; a missing file starts empty.
func New(path string, capacity int) (*Store, error) {
	if capacity < 1 {
		capacity = 1
	}
	s := &Store{path: path, capacity: capacity}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	b, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(b, &s.data)
}

func (s *Store) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(&s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Offset returns the next getUpdates offset.
func (s *Store) Offset() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.Offset
}

// Add stores a message (deduplicated by ID), trims to capacity, advances the
// offset to at least nextOffset, and persists.
func (s *Store) Add(m Message, nextOffset int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	replaced := false
	for i := range s.data.Messages {
		if s.data.Messages[i].ID == m.ID {
			s.data.Messages[i] = m
			replaced = true
			break
		}
	}
	if !replaced {
		s.data.Messages = append(s.data.Messages, m)
	}
	if len(s.data.Messages) > s.capacity {
		s.data.Messages = s.data.Messages[len(s.data.Messages)-s.capacity:]
	}
	if nextOffset > s.data.Offset {
		s.data.Offset = nextOffset
	}
	return s.save()
}

// SetOffset advances the offset (used when an update isn't a channel post we
// keep, but still needs to be acknowledged) and persists.
func (s *Store) SetOffset(nextOffset int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if nextOffset <= s.data.Offset {
		return nil
	}
	s.data.Offset = nextOffset
	return s.save()
}

// Recent returns up to n messages, newest first.
func (s *Store) Recent(n int) []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	msgs := make([]Message, len(s.data.Messages))
	copy(msgs, s.data.Messages)
	sort.SliceStable(msgs, func(i, j int) bool {
		return msgs[i].Date.After(msgs[j].Date)
	})
	if n > 0 && len(msgs) > n {
		msgs = msgs[:n]
	}
	return msgs
}
