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

/*
ExistsCommand represents the EXISTS command

EXISTS checks if one or more keys exist in storage. It returns the count
of keys that exist (not a boolean).

Redis syntax: EXISTS key1 key2 key3...
Example: EXISTS name age city (might return 2 if only name and age exist)
*/
type ExistsCommand struct {
	keys [][]byte
}

/*
Execute performs the EXISTS check

Iterates through all provided keys and counts how many exist.
Takes into account key expiration - expired keys are considered non-existent.
*/
func (c ExistsCommand) Execute(storage *Storage) ([]byte, error) {
	count := 0
	for _, key := range c.keys {
		if storage.Exists(key) {
			count++
		}
	}
	return []byte(strconv.Itoa(count)), nil
}

/*
=== STRING MANIPULATION COMMANDS ===

These commands work with string values, allowing you to modify parts of
strings without retrieving and setting the entire value.
*/

/*
AppendCommand represents the APPEND command

APPEND adds a value to the end of an existing string. If the key doesn't
exist, it creates a new key with the appended value.

Redis syntax: APPEND key value
Example: APPEND greeting " World" (if greeting was "Hello", becomes "Hello World")
*/
type AppendCommand struct {
	key []byte
	val []byte
}

/*
Execute performs the APPEND operation

Returns the new length of the string after appending.
If the key didn't exist, the new length equals the length of the appended value.
*/
func (c AppendCommand) Execute(storage *Storage) ([]byte, error) {
	length := storage.Append(c.key, c.val)
	return []byte(strconv.Itoa(length)), nil
}

/*
StrlenCommand represents the STRLEN command

STRLEN returns the length of the string stored at a key.
Returns 0 if the key doesn't exist.

Redis syntax: STRLEN key
Example: STRLEN name (returns 4 if name is "John")
*/
type StrlenCommand struct {
	key []byte
}

func (c StrlenCommand) Execute(storage *Storage) ([]byte, error) {
	length := storage.Strlen(c.key)
	return []byte(strconv.Itoa(length)), nil
}

/*
GetRangeCommand represents the GETRANGE command

GETRANGE extracts a substring from the string stored at a key.
Supports negative indices (counting from the end).

Redis syntax: GETRANGE key start end
Example: GETRANGE name 0 2 (returns "Joh" if name is "John")
*/
type GetRangeCommand struct {
	key   []byte
	start int
	end   int
}

func (c GetRangeCommand) Execute(storage *Storage) ([]byte, error) {
	result := storage.GetRange(c.key, c.start, c.end)
	return result, nil
}

/*
SetRangeCommand represents the SETRANGE command

SETRANGE overwrites part of a string at a specified offset.
If the string is shorter than the offset, it's padded with null bytes.

Redis syntax: SETRANGE key offset value
Example: SETRANGE name 0 "Jane" (changes "John" to "Jane")
*/
type SetRangeCommand struct {
	key    []byte
	offset int
	value  []byte
}

func (c SetRangeCommand) Execute(storage *Storage) ([]byte, error) {
	length := storage.SetRange(c.key, c.offset, c.value)
	return []byte(strconv.Itoa(length)), nil
}

/*
=== NUMERIC COMMANDS ===

These commands treat string values as integers and perform arithmetic operations.
They're atomic operations - useful for counters, IDs, and other numeric data.
*/

/*
IncrCommand represents the INCR command

INCR increments the integer value stored at a key by 1.
If the key doesn't exist, it's treated as 0 before incrementing.

Redis syntax: INCR key
Example: INCR counter (if counter was 5, becomes 6)
*/
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

/*
DecrCommand represents the DECR command

DECR decrements the integer value stored at a key by 1.
If the key doesn't exist, it's treated as 0 before decrementing.

Redis syntax: DECR key
Example: DECR counter (if counter was 5, becomes 4)
*/
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

/*
IncrByCommand represents the INCRBY command

INCRBY increments the integer value by a specified amount.
Can be negative to effectively decrement by a larger amount.

Redis syntax: INCRBY key increment
Example: INCRBY score 10 (adds 10 to the score)
*/
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

/*
DecrByCommand represents the DECRBY command

DECRBY decrements the integer value by a specified amount.
This is essentially INCRBY with a positive decrement value.

Redis syntax: DECRBY key decrement
Example: DECRBY score 5 (subtracts 5 from the score)
*/
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

/*
=== MULTIPLE KEY COMMANDS ===

These commands operate on multiple keys in a single operation.
They're more efficient than multiple individual commands and can be atomic.
*/

/*
MGetCommand represents the MGET command

MGET retrieves values for multiple keys in a single operation.
Returns an array where each position corresponds to the requested key.
Non-existent keys return null in their position.

Redis syntax: MGET key1 key2 key3...
Example: MGET name age city (returns ["John", "25", null] if city doesn't exist)
*/
type MGetCommand struct {
	keys [][]byte
}

/*
Execute performs the MGET operation

Uses the storage's MGet method to retrieve all values efficiently.
Formats the result as a RESP array for proper protocol compliance.
*/
func (c MGetCommand) Execute(storage *Storage) ([]byte, error) {
	results := storage.MGet(c.keys)
	return respWriteArray(results), nil
}

/*
MSetCommand represents the MSET command

MSET sets multiple key-value pairs in a single atomic operation.
Either all keys are set or none are (atomic operation).

Redis syntax: MSET key1 value1 key2 value2...
Example: MSET name "John" age "25" city "NYC"
*/
type MSetCommand struct {
	pairs map[string][]byte
}

func (c MSetCommand) Execute(storage *Storage) ([]byte, error) {
	err := storage.MSet(c.pairs)
	return []byte("OK"), err
}

/*
=== UTILITY COMMANDS ===

These commands provide additional functionality for key management,
atomic operations, and administrative tasks.
*/

/*
GetSetCommand represents the GETSET command

GETSET atomically sets a key to a new value and returns the old value.
This is useful for implementing atomic counters, flags, or swapping values.

Redis syntax: GETSET key newvalue
Example: GETSET counter 0 (sets counter to 0 and returns previous value)
*/
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

/*
KeysCommand represents the KEYS command

KEYS returns all keys matching a given pattern. Uses simple glob-style
pattern matching with * wildcards.

WARNING: This command can be slow with large datasets as it scans all keys.
In production Redis, this command is often disabled.

Redis syntax: KEYS pattern
Examples:
- KEYS * (returns all keys)
- KEYS user:* (returns all keys starting with "user:")
*/
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

/*
FlushAllCommand represents the FLUSHALL command

FLUSHALL removes all keys from the database. This is equivalent to
restarting with an empty database.

WARNING: This operation is destructive and cannot be undone.

Redis syntax: FLUSHALL
*/
type FlushAllCommand struct{}

func (c FlushAllCommand) Execute(storage *Storage) ([]byte, error) {
	storage.FlushAll()
	return []byte("OK"), nil
}

/*
=== CONNECTION COMMANDS ===

These commands handle client-server communication, handshakes,
and connection management.
*/

/*
HelloCommand represents the HELLO command

HELLO is used for protocol negotiation and server information.
It returns server details in a structured format.

Redis syntax: HELLO [protover]
*/
type HelloCommand struct {
	value string
}

/*
Execute returns server information

Returns a map with server details formatted according to RESP protocol.
This helps clients understand what server they're connected to.
*/
func (c HelloCommand) Execute(storage *Storage) ([]byte, error) {
	spec := map[string]string{
		"server":  "redis-clone",
		"version": "1.0.0",
		"proto":   "2",
		"mode":    "standalone",
	}
	return respWriteMap(spec), nil
}

/*
ClientCommand represents the CLIENT command

CLIENT provides client connection management functionality.
This is a simplified implementation.

Redis syntax: CLIENT subcommand [arguments...]
*/
type ClientCommand struct {
	value string
}

func (c ClientCommand) Execute(storage *Storage) ([]byte, error) {
	return []byte("OK"), nil
}

/*
PingCommand represents the PING command

PING tests the connection to the server. It can optionally echo a message.
This is commonly used for health checks and connection testing.

Redis syntax: PING [message]
Examples:
- PING (returns "PONG")
- PING "hello" (returns "hello")
*/
type PingCommand struct {
	message string
}

func (c PingCommand) Execute(storage *Storage) ([]byte, error) {
	if c.message == "" {
		return []byte("PONG"), nil
	}
	return []byte(c.message), nil
}

/*
=== RESP PROTOCOL HELPER FUNCTIONS ===

RESP (Redis Serialization Protocol) is the protocol Redis uses for
client-server communication. It's a simple text-based protocol.

RESP Data Types:
- Simple Strings: +OK\r\n
- Errors: -Error message\r\n
- Integers: :123\r\n
- Bulk Strings: $5\r\nhello\r\n
- Arrays: *2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n
- Maps: %2\r\n+key1\r\n+value1\r\n+key2\r\n+value2\r\n

These helper functions format our response data according to RESP.
*/

/*
respWriteMap writes a map as RESP format

Maps are used for structured data like server information.
Format: %<count>\r\n followed by alternating keys and values.

Example: {"name": "redis", "version": "1.0"}
Becomes: %2\r\n+name\r\n+redis\r\n+version\r\n+1.0\r\n
*/
func respWriteMap(m map[string]string) []byte {
	buf := &bytes.Buffer{}
	buf.WriteString("%" + fmt.Sprintf("%d\r\n", len(m)))
	for k, v := range m {
		buf.WriteString("+" + k + "\r\n")
		buf.WriteString("+" + v + "\r\n")
	}
	return buf.Bytes()
}

/*
respWriteArray writes an array as RESP format

Arrays are used for multi-value responses like MGET results.
Format: *<count>\r\n followed by each element.

Example: ["hello", "world", null]
Uses the resp library for proper null handling and bulk string formatting.
*/
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

/*
respWriteInteger writes an integer as RESP format

Integers are prefixed with : and followed by \r\n
Example: 42 becomes :42\r\n
*/
func respWriteInteger(num int64) []byte {
	buf := &bytes.Buffer{}
	buf.WriteString(":" + strconv.FormatInt(num, 10) + "\r\n")
	return buf.Bytes()
}

/*
respWriteError writes an error as RESP format

Errors are prefixed with - and followed by \r\n
Example: "key not found" becomes -key not found\r\n
*/
func respWriteError(err string) []byte {
	buf := &bytes.Buffer{}
	buf.WriteString("-" + err + "\r\n")
	return buf.Bytes()
}
