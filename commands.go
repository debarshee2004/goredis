package main

import "time"

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
	expiry time.Duration // Optional TTL
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
