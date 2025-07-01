package main

import (
	"bytes"
	"fmt"
	"strconv"
	"time"

	"github.com/tidwall/resp"
)

/*
Protocol Implementation for Redis Clone

This file implements the Redis communication protocol and command structures.
It handles parsing Redis commands and formatting responses according to the
RESP (Redis Serialization Protocol) specification.

Key concepts:
- Command Pattern: Each Redis command is represented as a struct implementing the Command interface
- RESP Protocol: Redis uses a specific text protocol for client-server communication
- Command Categories: Commands are organized by functionality (string, numeric, utility, etc.)
- Storage Integration: Commands interact with the storage layer to perform operations
*/

/*
Redis command constants

These constants define all the Redis commands supported by our implementation.
They're organized by category to make it easier to understand what each command does.
*/
const (
	// Basic string commands - fundamental Redis operations
	CommandSET    = "SET"
	CommandGET    = "GET"
	CommandDEL    = "DEL"
	CommandEXISTS = "EXISTS"

	// String manipulation commands - modify existing string values
	CommandAPPEND   = "APPEND"
	CommandSTRLEN   = "STRLEN"
	CommandGETRANGE = "GETRANGE"
	CommandSETRANGE = "SETRANGE"

	// Numeric commands - work with integer values
	CommandINCR   = "INCR"
	CommandDECR   = "DECR"
	CommandINCRBY = "INCRBY"
	CommandDECRBY = "DECRBY"

	// Multiple key commands - batch operations
	CommandMGET = "MGET"
	CommandMSET = "MSET"

	// Utility commands - administrative and helper operations
	CommandGETSET   = "GETSET"
	CommandKEYS     = "KEYS"
	CommandFLUSHALL = "FLUSHALL"

	// Connection commands - client interaction
	CommandHELLO  = "HELLO"
	CommandCLIENT = "CLIENT"
	CommandPING   = "PING"
)

/*
Command interface - all commands must implement this

This is the core of our command pattern implementation. Every Redis command
is represented as a struct that implements this interface.

The Execute method:
  - Takes a storage instance to perform operations
  - Returns the response as bytes (for RESP protocol)
  - Returns an error if the operation fails

This design allows us to:
  - Add new commands easily by creating new structs
  - Handle all commands uniformly in the server
  - Test commands independently
  - Separate command logic from protocol handling
*/
type Command interface {
	Execute(storage *Storage) ([]byte, error)
}

/*
=== BASIC STRING COMMANDS ===

These are the fundamental Redis operations that most applications use.
They provide basic key-value storage functionality.
*/

/*
SetCommand represents the SET command

SET is the most basic Redis command - it stores a value for a given key.
This implementation supports optional TTL (Time To Live) for automatic expiration.

Redis syntax: SET key value [EX seconds]
Example: SET name "John" EX 300 (sets name to John, expires in 5 minutes)
*/
type SetCommand struct {
	key    []byte
	val    []byte
	expiry time.Duration
}

/*
Execute performs the SET operation

If expiry is specified (> 0), uses SetWithExpiry to automatically delete
the key after the specified duration. Otherwise, uses regular Set.
Always returns "OK" on success, matching Redis behavior.
*/
func (c SetCommand) Execute(storage *Storage) ([]byte, error) {
	if c.expiry > 0 {
		err := storage.SetWithExpiry(c.key, c.val, c.expiry)
		return []byte("OK"), err
	}
	err := storage.Set(c.key, c.val)
	return []byte("OK"), err
}

/*
GetCommand represents the GET command

GET retrieves the value stored at a key. If the key doesn't exist or has
expired, Redis returns a null response.

Redis syntax: GET key
Example: GET name (returns "John" if key exists)
*/
type GetCommand struct {
	key []byte
}

/*
Execute performs the GET operation

Returns the stored value if the key exists and hasn't expired.
Returns an error if the key doesn't exist - this gets converted to
a null response in the RESP protocol.
*/
func (c GetCommand) Execute(storage *Storage) ([]byte, error) {
	val, ok := storage.Get(c.key)
	if !ok {
		return nil, fmt.Errorf("key not found")
	}
	return val, nil
}

/*
DelCommand represents the DEL command

DEL removes one or more keys from storage. It returns the number of keys
that were actually deleted (keys that didn't exist are not counted).

Redis syntax: DEL key1 key2 key3...
Example: DEL name age city (might return 2 if only name and age existed)
*/
type DelCommand struct {
	keys [][]byte
}

/*
Execute performs the DEL operation

Iterates through all provided keys and attempts to delete each one.
Counts how many keys were actually deleted and returns that count.
This matches Redis behavior exactly.
*/
func (c DelCommand) Execute(storage *Storage) ([]byte, error) {
	count := 0
	for _, key := range c.keys {
		if storage.Delete(key) {
			count++
		}
	}
	return []byte(strconv.Itoa(count)), nil
}

type ExistsCommand struct {
	keys [][]byte
}

func (c ExistsCommand) Execute(storage *Storage) ([]byte, error) {
	count := 0
	for _, key := range c.keys {
		if storage.Exists(key) {
			count++
		}
	}
	return []byte(strconv.Itoa(count)), nil
}

type AppendCommand struct {
	key []byte
	val []byte
}

func (c AppendCommand) Execute(storage *Storage) ([]byte, error) {
	length := storage.Append(c.key, c.val)
	return []byte(strconv.Itoa(length)), nil
}

type StrlenCommand struct {
	key []byte
}

func (c StrlenCommand) Execute(storage *Storage) ([]byte, error) {
	length := storage.Strlen(c.key)
	return []byte(strconv.Itoa(length)), nil
}

type GetRangeCommand struct {
	key   []byte
	start int
	end   int
}

func (c GetRangeCommand) Execute(storage *Storage) ([]byte, error) {
	result := storage.GetRange(c.key, c.start, c.end)
	return result, nil
}

type SetRangeCommand struct {
	key    []byte
	offset int
	value  []byte
}

func (c SetRangeCommand) Execute(storage *Storage) ([]byte, error) {
	length := storage.SetRange(c.key, c.offset, c.value)
	return []byte(strconv.Itoa(length)), nil
}

type IncrCommand struct {
	key []byte
}

func (c IncrCommand) Execute(storage *Storage) ([]byte, error) {
	result, err := storage.Incr(c.key)
	if err != nil {
		return nil, err
	}
	return []byte(strconv.FormatInt(result, 10)), nil
}

type DecrCommand struct {
	key []byte
}

func (c DecrCommand) Execute(storage *Storage) ([]byte, error) {
	result, err := storage.Decr(c.key)
	if err != nil {
		return nil, err
	}
	return []byte(strconv.FormatInt(result, 10)), nil
}

type IncrByCommand struct {
	key       []byte
	increment int64
}

func (c IncrByCommand) Execute(storage *Storage) ([]byte, error) {
	result, err := storage.IncrBy(c.key, c.increment)
	if err != nil {
		return nil, err
	}
	return []byte(strconv.FormatInt(result, 10)), nil
}

type DecrByCommand struct {
	key       []byte
	decrement int64
}

func (c DecrByCommand) Execute(storage *Storage) ([]byte, error) {
	result, err := storage.DecrBy(c.key, c.decrement)
	if err != nil {
		return nil, err
	}
	return []byte(strconv.FormatInt(result, 10)), nil
}

type MGetCommand struct {
	keys [][]byte
}

func (c MGetCommand) Execute(storage *Storage) ([]byte, error) {
	results := storage.MGet(c.keys)
	return respWriteArray(results), nil
}

type MSetCommand struct {
	pairs map[string][]byte
}

func (c MSetCommand) Execute(storage *Storage) ([]byte, error) {
	err := storage.MSet(c.pairs)
	return []byte("OK"), err
}

type GetSetCommand struct {
	key []byte
	val []byte
}

func (c GetSetCommand) Execute(storage *Storage) ([]byte, error) {
	oldVal, exists := storage.GetSet(c.key, c.val)
	if !exists {
		return nil, fmt.Errorf("key not found")
	}
	return oldVal, nil
}

type KeysCommand struct {
	pattern string
}

func (c KeysCommand) Execute(storage *Storage) ([]byte, error) {
	keys := storage.Keys(c.pattern)
	keyBytes := make([][]byte, len(keys))
	for i, key := range keys {
		keyBytes[i] = []byte(key)
	}
	return respWriteArray(keyBytes), nil
}

type FlushAllCommand struct{}

func (c FlushAllCommand) Execute(storage *Storage) ([]byte, error) {
	storage.FlushAll()
	return []byte("OK"), nil
}

type HelloCommand struct {
	value string
}

func (c HelloCommand) Execute(storage *Storage) ([]byte, error) {
	spec := map[string]string{
		"server":  "redis-clone",
		"version": "1.0.0",
		"proto":   "2",
		"mode":    "standalone",
	}
	return respWriteMap(spec), nil
}

type ClientCommand struct {
	value string
}

func (c ClientCommand) Execute(storage *Storage) ([]byte, error) {
	return []byte("OK"), nil
}

type PingCommand struct {
	message string
}

func (c PingCommand) Execute(storage *Storage) ([]byte, error) {
	if c.message == "" {
		return []byte("PONG"), nil
	}
	return []byte(c.message), nil
}

func respWriteMap(m map[string]string) []byte {
	buf := &bytes.Buffer{}
	buf.WriteString("%" + fmt.Sprintf("%d\r\n", len(m)))
	for k, v := range m {
		buf.WriteString("+" + k + "\r\n")
		buf.WriteString("+" + v + "\r\n")
	}
	return buf.Bytes()
}

func respWriteArray(arr [][]byte) []byte {
	buf := &bytes.Buffer{}
	buf.WriteString("*" + fmt.Sprintf("%d\r\n", len(arr)))
	rw := resp.NewWriter(buf)
	for _, item := range arr {
		if item == nil {
			rw.WriteNull()
		} else {
			rw.WriteBytes(item)
		}
	}
	return buf.Bytes()
}

func respWriteInteger(num int64) []byte {
	buf := &bytes.Buffer{}
	buf.WriteString(":" + strconv.FormatInt(num, 10) + "\r\n")
	return buf.Bytes()
}

func respWriteError(err string) []byte {
	buf := &bytes.Buffer{}
	buf.WriteString("-" + err + "\r\n")
	return buf.Bytes()
}
