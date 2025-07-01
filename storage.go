package main

import (
	"sync"
	"time"
)

type Storage struct {
	mu       sync.Mutex
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
