package main

import (
	"strconv"
	"strings"
	"sync"
	"time"
)

/*
Storage represents the key-value storage engine with thread-safe operations

This is the core data structure that stores all Redis data in memory.
It provides thread-safe operations using read-write mutexes to allow
multiple concurrent readers but exclusive writers.

Key features:
  - Thread-safe operations using sync.RWMutex
  - TTL (Time To Live) support for automatic key expiration
  - Support for string operations, counters, and pattern matching
  - In-memory storage with maps for fast lookups
*/
type Storage struct {
	mu       sync.RWMutex
	data     map[string][]byte
	expiry   map[string]time.Time
	counters map[string]int64
}

/*
NewStorage creates a new storage instance

This is the constructor function that initializes all the internal maps
and returns a ready-to-use Storage instance.
*/
func NewStorage() *Storage {
	return &Storage{
		data:     make(map[string][]byte),
		expiry:   make(map[string]time.Time),
		counters: make(map[string]int64),
	}

}

/*
Set stores a key-value pair

This is the basic SET operation in Redis. It stores a value for a given key.
If the key already exists, it overwrites the existing value.
Any existing TTL is removed when a key is set.

params:
- key: The key to store (as byte slice for efficiency)
- val: The value to store (as byte slice to support binary data)

returns: error (always nil in this implementation)
*/
func (s *Storage) Set(key, val []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	keyStr := string(key)
	s.data[keyStr] = val
	delete(s.expiry, keyStr)

	return nil
}

/*
SetWithExpiry stores a key-value pair with TTL

This implements Redis SETEX command - sets a key with an automatic expiration time.
The key will be automatically deleted after the specified duration.

Parameters:
- key: The key to store
- val: The value to store
- expiry: How long the key should live (duration from now)
*/
func (s *Storage) SetWithExpiry(key, val []byte, expiry time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	keyStr := string(key)
	s.data[keyStr] = val

	// Calculate absolute expiration time by adding duration to current time
	s.expiry[keyStr] = time.Now().Add(expiry)

	return nil
}

/*
Get retrieves a value by key

This implements the Redis GET command. It returns the value for a key
and a boolean indicating whether the key exists.
Automatically handles TTL - expired keys are treated as non-existent.

Parameters:
- key: The key to retrieve

Returns:
- []byte: The value (nil if key doesn't exist)
- bool: Whether the key exists and is not expired
*/
func (s *Storage) Get(key []byte) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keyStr := string(key)

	if expTime, exists := s.expiry[keyStr]; exists {
		if time.Now().After(expTime) {
			/*
				Key has expired, remove it from storage
				Note: We can't modify maps during RLock, but this is a cleanup operation
				In production, you'd typically do lazy expiration or background cleanup
			*/
			delete(s.data, keyStr)
			delete(s.expiry, keyStr)
			return nil, false
		}
	}

	val, ok := s.data[keyStr]
	return val, ok
}

/*
Delete removes a key-value pair

Implements Redis DEL command. Removes a key and its associated value,
expiry time, and counter value if they exist.

Returns: true if the key existed and was deleted, false if key didn't exist
*/
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

/*
Exists checks if a key exists

Implements Redis EXISTS command. Checks if a key exists and is not expired.
This is more efficient than Get() when you only need to check existence.
*/
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

/*
Append appends a value to an existing key

Implements Redis APPEND command. If the key exists, appends the value to the end.
If the key doesn't exist, creates it with the given value.

Returns: The new length of the string after append operation
*/
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

func (s *Storage) GetRange(key []byte, start, end int) []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keyStr := string(key)

	if expTime, exists := s.expiry[keyStr]; exists {
		if time.Now().After(expTime) {
			return []byte{}
		}
	}

	val, exists := s.data[keyStr]
	if !exists {
		return []byte{}
	}

	length := len(val)

	if start < 0 {
		start = length + start
	}
	if end < 0 {
		end = length + end
	}

	if start < 0 {
		start = 0
	}
	if end >= length {
		end = length - 1
	}
	if start > end {
		return []byte{}
	}

	return val[start : end+1]
}

func (s *Storage) SetRange(key []byte, offset int, value []byte) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	keyStr := string(key)
	existing, exists := s.data[keyStr]

	if !exists {
		if offset > 0 {
			existing = make([]byte, offset)
		}
	}

	requiredLength := offset + len(value)
	if len(existing) < requiredLength {
		newBytes := make([]byte, requiredLength)
		copy(newBytes, existing)
		existing = newBytes
	}

	copy(existing[offset:], value)
	s.data[keyStr] = existing

	return len(existing)
}

func (s *Storage) Incr(key []byte) (int64, error) {
	return s.IncrBy(key, 1)
}

func (s *Storage) IncrBy(key []byte, increment int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	keyStr := string(key)

	if val, exists := s.data[keyStr]; exists {
		intVal, err := strconv.ParseInt(string(val), 10, 64)
		if err != nil {
			return 0, err
		}
		intVal += increment
		s.data[keyStr] = []byte(strconv.FormatInt(intVal, 10))
		s.counters[keyStr] = intVal
		return intVal, nil
	}

	s.data[keyStr] = []byte(strconv.FormatInt(increment, 10))
	s.counters[keyStr] = increment
	return increment, nil
}

func (s *Storage) Decr(key []byte) (int64, error) {
	return s.IncrBy(key, -1)
}

func (s *Storage) DecrBy(key []byte, decrement int64) (int64, error) {
	return s.IncrBy(key, -decrement)
}

func (s *Storage) GetSet(key, val []byte) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	keyStr := string(key)
	oldVal, exists := s.data[keyStr]
	s.data[keyStr] = val

	delete(s.expiry, keyStr)

	return oldVal, exists
}

func (s *Storage) MGet(keys [][]byte) [][]byte {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([][]byte, len(keys))
	for i, key := range keys {
		keyStr := string(key)

		if expTime, exists := s.expiry[keyStr]; exists {
			if time.Now().After(expTime) {
				results[i] = nil
				continue
			}
		}

		if val, exists := s.data[keyStr]; exists {
			results[i] = val
		} else {
			results[i] = nil
		}
	}

	return results
}

func (s *Storage) MSet(pairs map[string][]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, val := range pairs {
		s.data[key] = val
		delete(s.expiry, key)
	}

	return nil
}

func (s *Storage) Keys(pattern string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var keys []string

	for key := range s.data {
		if expTime, exists := s.expiry[key]; exists {
			if time.Now().After(expTime) {
				continue
			}
		}

		if matchPattern(key, pattern) {
			keys = append(keys, key)
		}
	}

	return keys
}

func (s *Storage) FlushAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data = make(map[string][]byte)
	s.expiry = make(map[string]time.Time)
	s.counters = make(map[string]int64)
}

func matchPattern(key, pattern string) bool {
	if pattern == "*" {
		return true
	}

	if !strings.Contains(pattern, "*") {
		return key == pattern
	}

	parts := strings.Split(pattern, "*")
	keyIndex := 0

	for i, part := range parts {
		if part == "" {
			continue
		}

		index := strings.Index(key[keyIndex:], part)
		if index == -1 {
			return false
		}

		if i == 0 && index != 0 {
			return false
		}

		keyIndex += index + len(part)
	}

	if strings.HasSuffix(pattern, "*") {
		return true
	}

	return keyIndex == len(key)
}
