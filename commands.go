package main

import (
	"bytes"
	"fmt"
	"strconv"
	"time"

	"github.com/tidwall/resp"
)

const (
	// Basic string commands
	CommandSET    = "SET"
	CommandGET    = "GET"
	CommandDEL    = "DEL"
	CommandEXISTS = "EXISTS"

	// String manipulation commands
	CommandAPPEND   = "APPEND"
	CommandSTRLEN   = "STRLEN"
	CommandGETRANGE = "GETRANGE"
	CommandSETRANGE = "SETRANGE"

	// Numeric commands
	CommandINCR   = "INCR"
	CommandDECR   = "DECR"
	CommandINCRBY = "INCRBY"
	CommandDECRBY = "DECRBY"

	// Multiple key commands
	CommandMGET = "MGET"
	CommandMSET = "MSET"

	// Utility commands
	CommandGETSET   = "GETSET"
	CommandKEYS     = "KEYS"
	CommandFLUSHALL = "FLUSHALL"

	// Connection commands
	CommandHELLO  = "HELLO"
	CommandCLIENT = "CLIENT"
	CommandPING   = "PING"
)

type Command struct {
	// Execute(storage *Storage) ([]byte, error)
}

type SetCommand struct {
	key    []byte
	val    []byte
	expiry time.Duration
}

func (c SetCommand) Execute(storage *Storage) ([]byte, error) {
	if c.expiry > 0 {
		err := storage.SetWithExpiry(c.key, c.val, c.expiry)
		return []byte("OK"), err
	}
	err := storage.Set(c.key, c.val)
	return []byte("OK"), err
}

type GetCommand struct {
	key []byte
}

func (c GetCommand) Execute(storage *Storage) ([]byte, error) {
	val, ok := storage.Get(c.key)
	if !ok {
		return nil, fmt.Errorf("key not found")
	}
	return val, nil
}

type DelCommand struct {
	keys [][]byte
}

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

type MSetCommand struct {
	pairs map[string][]byte
}

type GetSetCommand struct {
	key []byte
	val []byte
}

type KeysCommand struct {
	pattern string
}

type FlushAllCommand struct{}

type HelloCommand struct {
	value string
}

type ClientCommand struct {
	value string
}

type PingCommand struct {
	message string
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
