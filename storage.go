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
