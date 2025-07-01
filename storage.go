package main

import (
	"sync"
	"time"
)

type Storage struct {
	mu       sync.RWMutex
	data     map[string][]byte
	expiry   map[string]time.Time
	counters map[string]int64
}

func NewStorage() *Storage {
	return &Storage{
		data:     make(map[string][]byte),
		expiry:   make(map[string]time.Time),
		counters: make(map[string]int64),
	}

}

func (s *Storage) Set(key, val []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	keyStr := string(key)
	s.data[keyStr] = val
	delete(s.expiry, keyStr)

	return nil
}

func (s *Storage) SetWithExpiry(key, val []byte, expiry time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	keyStr := string(key)
	s.data[keyStr] = val
	s.expiry[keyStr] = time.Now().Add(expiry)

	return nil
}

func (s *Storage) Get(key []byte) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keyStr := string(key)

	if expTime, exists := s.expiry[keyStr]; exists {
		if time.Now().After(expTime) {
			delete(s.data, keyStr)
			delete(s.expiry, keyStr)
			return nil, false
		}
	}

	val, ok := s.data[keyStr]
	return val, ok
}

func (s *Storage) Delete(key []byte) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	keyStr := string(key)
	_, exists := s.data[keyStr]
	if exists {
		delete(s.data, keyStr)
		delete(s.expiry, keyStr)
		delete(s.counters, keyStr)
	}

	return exists
}

func (s *Storage) Exists(key []byte) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keyStr := string(key)

	if expTime, exists := s.expiry[keyStr]; exists {
		if time.Now().After(expTime) {
			return false
		}
	}

	_, exists := s.data[keyStr]
	return exists
}

func (s *Storage) Append(key, val []byte) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	keyStr := string(key)
	existing, exists := s.data[keyStr]

	if !exists {
		s.data[keyStr] = val
		return len(val)
	}

	s.data[keyStr] = append(existing, val...)
	return len(s.data[keyStr])
}

func (s *Storage) Strlen(key []byte) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keyStr := string(key)

	if expTime, exists := s.expiry[keyStr]; exists {
		if time.Now().After(expTime) {
			return 0
		}
	}

	if val, exists := s.data[keyStr]; exists {
		return len(val)
	}

	return 0
}
