package main

import (
	"bytes"
	"fmt"
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

type GetCommand struct {
	key []byte
}

type DelCommand struct {
	keys [][]byte
}

type ExistsCommand struct {
	keys [][]byte
}

type AppendCommand struct {
	key []byte
	val []byte
}

type StrlenCommand struct {
	key []byte
}

type GetRangeCommand struct {
	key   []byte
	start int
	end   int
}

type SetRangeCommand struct {
	key    []byte
	offset int
	value  []byte
}

type IncrCommand struct {
	key []byte
}

type DecrCommand struct {
	key []byte
}

type IncrByCommand struct {
	key       []byte
	increment int64
}

type DecrByCommand struct {
	key       []byte
	decrement int64
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
