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
