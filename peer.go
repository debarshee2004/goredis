package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/tidwall/resp"
)

/*
Peer Connection Management for Redis Clone

This file handles individual client connections to our Redis server.
Each client that connects gets represented by a Peer struct, which manages:
- Reading commands from the client using RESP protocol
- Parsing those commands into our internal Command structures
- Sending responses back to the client
- Handling client disconnections gracefully

Key concepts:
- RESP Protocol: Redis uses a specific text-based protocol for communication
- Command Parsing: Converting raw RESP data into typed Command structs
- Goroutine per Client: Each client runs in its own goroutine for concurrency
- Channel Communication: Peers communicate with the main server via channels
*/

/*
Peer represents a client connection

This struct encapsulates everything needed to handle a single client connection.
It acts as a bridge between the network connection and the main server.

Architecture:
  - connect: The actual TCP connection to the client
  - messageChannel: Channel to send parsed commands to the main server
  - deleteChannel: Channel to notify the server when this client disconnects

The peer runs in its own goroutine and continuously:
 1. Reads RESP data from the client
 2. Parses it into Command structs
 3. Sends commands to the server via messageChannel
 4. Sends responses back to the client
*/
type Peer struct {
	connect        net.Conn
	messageChannel chan Message
	deleteChannel  chan *Peer
}

/*
NewPeer creates a new peer instance

This is the constructor for Peer objects. It takes the network connection
and the channels needed to communicate with the main server.

Parameters:
  - connect: The TCP connection from net.Accept()
  - messageChannel: Channel where parsed commands will be sent
  - deleteChannel: Channel to notify when this peer disconnects
*/
func NewPeer(connect net.Conn, messageChannel chan Message, deleteChannel chan *Peer) *Peer {
	return &Peer{
		connect:        connect,
		messageChannel: messageChannel,
		deleteChannel:  deleteChannel,
	}
}

/*
Send sends a message to the client

This method writes response data back to the client over the TCP connection.
It's used to send command results, errors, and other responses.

Parameters:
  - message: The response data to send (usually RESP-formatted)

Returns: number of bytes written and any error
*/
func (p *Peer) Send(message []byte) (int, error) {
	return p.connect.Write(message)
}

/*
readLoop reads commands from the client connection

This is the main method that runs in a goroutine for each client.
It continuously reads RESP data from the client, parses it into commands,
and forwards those commands to the main server for processing.

The loop continues until:
  - The client disconnects (EOF)
  - A network error occurs
  - The connection is closed

Error handling:
  - EOF: Normal client disconnection
  - Parse errors: Send error response to client, continue reading
  - Network errors: Log and disconnect the client
*/
func (p *Peer) readLoop() error {
	// Create RESP reader for parsing Redis protocol data
	rd := resp.NewReader(p.connect)

	for {
		// Read RESP value from client. This blocks until data arrives or connection closes
		v, _, err := rd.ReadValue()
		if err == io.EOF {
			p.deleteChannel <- p
			break
		}
		if err != nil {
			log.Fatal("Error reading from peer: ", err)
			p.deleteChannel <- p
			break
		}

		// Parse the RESP value into a Command struct
		cmd, err := p.parseCommand(v)
		if err != nil {
			errorResp := respWriteError(fmt.Sprintf("ERR %s", err.Error()))
			p.Send(errorResp)
			continue
		}

		// Send successfully parsed command to server for processing
		// The server will execute the command and send a response back
		p.messageChannel <- Message{
			cmd:  cmd,
			peer: p,
		}
	}

	return nil
}

/*
parseCommand parses a RESP value into a Command

This is the main command parsing dispatcher. It takes raw RESP data
and converts it into one of our typed Command structs.

RESP Command Format:
Commands come as arrays: ["SET", "key", "value"]
  - First element is the command name (case-insensitive)
  - Remaining elements are the arguments
  - Different commands have different argument requirements

Parameters:
  - v: The RESP value from the client (should be an array)

Returns: A Command interface implementation, or error if parsing fails

Error cases:
  - Not an array: Commands must be RESP arrays
  - Empty array: Must have at least a command name
  - Unknown command: Command name not recognized
  - Wrong arguments: Command has wrong number/type of arguments
*/
func (p *Peer) parseCommand(v resp.Value) (Command, error) {
	// Commands must be arrays in RESP protocol
	if v.Type() != resp.Array {
		return nil, fmt.Errorf("expected array")
	}

	arr := v.Array()
	if len(arr) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	// Get command name (case-insensitive)
	cmdName := strings.ToUpper(arr[0].String())

	/*
		Dispatch to specific parsing method based on command name
		Each command has its own parsing logic due to different argument patterns
	*/
	switch cmdName {
	case CommandSET:
		return p.parseSetCommand(arr)
	case CommandGET:
		return p.parseGetCommand(arr)
	case CommandDEL:
		return p.parseDelCommand(arr)
	case CommandEXISTS:
		return p.parseExistsCommand(arr)
	case CommandAPPEND:
		return p.parseAppendCommand(arr)
	case CommandSTRLEN:
		return p.parseStrlenCommand(arr)
	case CommandGETRANGE:
		return p.parseGetRangeCommand(arr)
	case CommandSETRANGE:
		return p.parseSetRangeCommand(arr)
	case CommandINCR:
		return p.parseIncrCommand(arr)
	case CommandDECR:
		return p.parseDecrCommand(arr)
	case CommandINCRBY:
		return p.parseIncrByCommand(arr)
	case CommandDECRBY:
		return p.parseDecrByCommand(arr)
	case CommandMGET:
		return p.parseMGetCommand(arr)
	case CommandMSET:
		return p.parseMSetCommand(arr)
	case CommandGETSET:
		return p.parseGetSetCommand(arr)
	case CommandKEYS:
		return p.parseKeysCommand(arr)
	case CommandFLUSHALL:
		return p.parseFlushAllCommand(arr)
	case CommandHELLO:
		return p.parseHelloCommand(arr)
	case CommandCLIENT:
		return p.parseClientCommand(arr)
	case CommandPING:
		return p.parsePingCommand(arr)
	default:
		return nil, fmt.Errorf("unknown command '%s'", cmdName)
	}
}

/*
=== COMMAND PARSING METHODS ===

Each method below parses a specific Redis command from RESP array format
into our internal Command struct. They follow a common pattern:

1. Validate argument count (Redis is strict about this)
2. Extract and validate each argument
3. Convert types as needed (strings to integers, etc.)
4. Return the appropriate Command struct

Common validation patterns:
- Argument count checking (too few/too many arguments)
- Type conversion with error handling
- Optional parameter parsing
*/

/*
parseSetCommand parses SET command: SET key value [EX seconds]

SET is one of the most complex basic commands because it supports optional TTL.

Formats:
  - SET key value (basic set)
  - SET key value EX seconds (set with expiration)

Validation:
  - Must have at least 3 arguments (SET, key, value)
  - If EX is present, must have exactly 5 arguments
  - EX parameter must be followed by a valid integer

Examples:
  - ["SET", "name", "John"] -> SetCommand{key: "name", val: "John"}
  - ["SET", "temp", "data", "EX", "300"] -> SetCommand with 5-minute TTL
*/
func (p *Peer) parseSetCommand(arr []resp.Value) (Command, error) {
	if len(arr) < 3 {
		return nil, fmt.Errorf("wrong number of arguments for 'SET' command")
	}

	cmd := SetCommand{
		key: arr[1].Bytes(),
		val: arr[2].Bytes(),
	}

	/*
		Parse optional EX parameter for TTL
		Format: SET key value EX seconds
	*/
	if len(arr) >= 5 && strings.ToUpper(arr[3].String()) == "EX" {
		seconds, err := strconv.Atoi(arr[4].String())
		if err != nil {
			return nil, fmt.Errorf("invalid expire time")
		}
		cmd.expiry = time.Duration(seconds) * time.Second
	}

	return cmd, nil
}

/*
parseGetCommand parses GET command: GET key

GET is simple - just a key lookup.

Validation:
  - Must have exactly 2 arguments (GET, key)

Example: ["GET", "name"] -> GetCommand{key: "name"}
*/
func (p *Peer) parseGetCommand(arr []resp.Value) (Command, error) {
	if len(arr) != 2 {
		return nil, fmt.Errorf("wrong number of arguments for 'GET' command")
	}

	return GetCommand{
		key: arr[1].Bytes(),
	}, nil
}

/*
parseDelCommand parses DEL command: DEL key [key ...]

DEL can delete multiple keys in one command.

Validation:
  - Must have at least 2 arguments (DEL, key1, ...)
  - Each additional argument is another key to delete

Examples:
  - ["DEL", "key1"] -> delete one key
  - ["DEL", "key1", "key2", "key3"] -> delete three keys
*/
func (p *Peer) parseDelCommand(arr []resp.Value) (Command, error) {
	if len(arr) < 2 {
		return nil, fmt.Errorf("wrong number of arguments for 'DEL' command")
	}

	// Extract all keys (everything after the command name)
	keys := make([][]byte, len(arr)-1)
	for i := 1; i < len(arr); i++ {
		keys[i-1] = arr[i].Bytes()
	}

	return DelCommand{keys: keys}, nil
}

/*
parseExistsCommand parses EXISTS command: EXISTS key [key ...]

Like DEL, EXISTS can check multiple keys at once.

Validation:
  - Must have at least 2 arguments (EXISTS, key1, ...)
  - Returns count of how many keys exist

Examples:
  - ["EXISTS", "key1"] -> check one key
  - ["EXISTS", "key1", "key2"] -> check two keys, return count
*/
func (p *Peer) parseExistsCommand(arr []resp.Value) (Command, error) {
	if len(arr) < 2 {
		return nil, fmt.Errorf("wrong number of arguments for 'EXISTS' command")
	}

	keys := make([][]byte, len(arr)-1)
	for i := 1; i < len(arr); i++ {
		keys[i-1] = arr[i].Bytes()
	}

	return ExistsCommand{keys: keys}, nil
}

/*
parseAppendCommand parses APPEND command: APPEND key value

APPEND adds to the end of an existing string value.

Validation:
  - Must have exactly 3 arguments (APPEND, key, value)

Example: ["APPEND", "greeting", " World"] -> append " World" to greeting
*/
func (p *Peer) parseAppendCommand(arr []resp.Value) (Command, error) {
	if len(arr) != 3 {
		return nil, fmt.Errorf("wrong number of arguments for 'APPEND' command")
	}

	return AppendCommand{
		key: arr[1].Bytes(),
		val: arr[2].Bytes(),
	}, nil
}

/*
parseStrlenCommand parses STRLEN command: STRLEN key

STRLEN returns the length of a string value.

Validation:
  - Must have exactly 2 arguments (STRLEN, key)

Example: ["STRLEN", "name"] -> return length of value at "name"
*/
func (p *Peer) parseStrlenCommand(arr []resp.Value) (Command, error) {
	if len(arr) != 2 {
		return nil, fmt.Errorf("wrong number of arguments for 'STRLEN' command")
	}

	return StrlenCommand{
		key: arr[1].Bytes(),
	}, nil
}

/*
parseGetRangeCommand parses GETRANGE command: GETRANGE key start end

GETRANGE extracts a substring from a stored string.

Validation:
  - Must have exactly 4 arguments (GETRANGE, key, start, end)
  - start and end must be valid integers
  - Supports negative indices (count from end)

Example: ["GETRANGE", "name", "0", "2"] -> get characters 0-2 from "name"
*/
func (p *Peer) parseGetRangeCommand(arr []resp.Value) (Command, error) {
	if len(arr) != 4 {
		return nil, fmt.Errorf("wrong number of arguments for 'GETRANGE' command")
	}

	// Parse start index
	start, err := strconv.Atoi(arr[2].String())
	if err != nil {
		return nil, fmt.Errorf("invalid start index")
	}

	// Parse end index
	end, err := strconv.Atoi(arr[3].String())
	if err != nil {
		return nil, fmt.Errorf("invalid end index")
	}

	return GetRangeCommand{
		key:   arr[1].Bytes(),
		start: start,
		end:   end,
	}, nil
}

/*
parseSetRangeCommand parses SETRANGE command: SETRANGE key offset value

SETRANGE overwrites part of a string at a specific position.

Validation:
  - Must have exactly 4 arguments (SETRANGE, key, offset, value)
  - offset must be a valid non-negative integer

Example: ["SETRANGE", "name", "0", "Jane"] -> overwrite starting at position 0
*/
func (p *Peer) parseSetRangeCommand(arr []resp.Value) (Command, error) {
	if len(arr) != 4 {
		return nil, fmt.Errorf("wrong number of arguments for 'SETRANGE' command")
	}

	// Parse offset position
	offset, err := strconv.Atoi(arr[2].String())
	if err != nil {
		return nil, fmt.Errorf("invalid offset")
	}

	return SetRangeCommand{
		key:    arr[1].Bytes(),
		offset: offset,
		value:  arr[3].Bytes(),
	}, nil
}

/*
parseIncrCommand parses INCR command: INCR key

INCR increments an integer value by 1.

Validation:
  - Must have exactly 2 arguments (INCR, key)
  - The stored value must be parseable as an integer

Example: ["INCR", "counter"] -> increment counter by 1
*/
func (p *Peer) parseIncrCommand(arr []resp.Value) (Command, error) {
	if len(arr) != 2 {
		return nil, fmt.Errorf("wrong number of arguments for 'INCR' command")
	}

	return IncrCommand{
		key: arr[1].Bytes(),
	}, nil
}

/*
parseDecrCommand parses DECR command: DECR key

DECR decrements an integer value by 1.

Validation:
  - Must have exactly 2 arguments (DECR, key)
  - The stored value must be parseable as an integer

Example: ["DECR", "counter"] -> decrement counter by 1
*/
func (p *Peer) parseDecrCommand(arr []resp.Value) (Command, error) {
	if len(arr) != 2 {
		return nil, fmt.Errorf("wrong number of arguments for 'DECR' command")
	}

	return DecrCommand{
		key: arr[1].Bytes(),
	}, nil
}

/*
parseIncrByCommand parses INCRBY command: INCRBY key increment

INCRBY increments an integer value by a specified amount.

Validation:
  - Must have exactly 3 arguments (INCRBY, key, increment)
  - increment must be a valid 64-bit integer (can be negative)

Example: ["INCRBY", "score", "10"] -> add 10 to score
*/
func (p *Peer) parseIncrByCommand(arr []resp.Value) (Command, error) {
	if len(arr) != 3 {
		return nil, fmt.Errorf("wrong number of arguments for 'INCRBY' command")
	}

	increment, err := strconv.ParseInt(arr[2].String(), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid increment value")
	}

	return IncrByCommand{
		key:       arr[1].Bytes(),
		increment: increment,
	}, nil
}

/*
parseDecrByCommand parses DECRBY command: DECRBY key decrement

DECRBY decrements an integer value by a specified amount.

Validation:
  - Must have exactly 3 arguments (DECRBY, key, decrement)
  - decrement must be a valid 64-bit integer

Example: ["DECRBY", "score", "5"] -> subtract 5 from score
*/
func (p *Peer) parseDecrByCommand(arr []resp.Value) (Command, error) {
	if len(arr) != 3 {
		return nil, fmt.Errorf("wrong number of arguments for 'DECRBY' command")
	}

	decrement, err := strconv.ParseInt(arr[2].String(), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid decrement value")
	}

	return DecrByCommand{
		key:       arr[1].Bytes(),
		decrement: decrement,
	}, nil
}

/*
parseMGetCommand parses MGET command: MGET key [key ...]

MGET retrieves multiple values in a single operation.

Validation:
  - Must have at least 2 arguments (MGET, key1, ...)
  - Each additional argument is another key to retrieve

Example: ["MGET", "name", "age", "city"] -> get values for all three keys
*/
func (p *Peer) parseMGetCommand(arr []resp.Value) (Command, error) {
	if len(arr) < 2 {
		return nil, fmt.Errorf("wrong number of arguments for 'MGET' command")
	}

	keys := make([][]byte, len(arr)-1)
	for i := 1; i < len(arr); i++ {
		keys[i-1] = arr[i].Bytes()
	}

	return MGetCommand{keys: keys}, nil
}

/*
parseMSetCommand parses MSET command: MSET key value [key value ...]

MSET sets multiple key-value pairs in one atomic operation.

Validation:
  - Must have at least 3 arguments (MSET, key1, value1, ...)
  - Must have odd number of arguments (command + pairs)
  - Arguments alternate: key, value, key, value, ...

Example: ["MSET", "name", "John", "age", "25"] -> set two key-value pairs
*/
func (p *Peer) parseMSetCommand(arr []resp.Value) (Command, error) {
	// Must have odd number of arguments: MSET key1 value1 key2 value2...
	if len(arr) < 3 || len(arr)%2 == 0 {
		return nil, fmt.Errorf("wrong number of arguments for 'MSET' command")
	}

	pairs := make(map[string][]byte)
	for i := 1; i < len(arr); i += 2 {
		key := arr[i].String()
		val := arr[i+1].Bytes()
		pairs[key] = val
	}

	return MSetCommand{pairs: pairs}, nil
}

/*
parseGetSetCommand parses GETSET command: GETSET key value

GETSET atomically sets a key and returns the old value.

Validation: Must have exactly 3 arguments (GETSET, key, value)

Example: ["GETSET", "counter", "0"] -> set counter to 0, return old value
*/
func (p *Peer) parseGetSetCommand(arr []resp.Value) (Command, error) {
	if len(arr) != 3 {
		return nil, fmt.Errorf("wrong number of arguments for 'GETSET' command")
	}

	return GetSetCommand{
		key: arr[1].Bytes(),
		val: arr[2].Bytes(),
	}, nil
}

/*
parseKeysCommand parses KEYS command: KEYS pattern

KEYS returns all keys matching a glob-style pattern.

Validation:
  - Must have exactly 2 arguments (KEYS, pattern)
  - Pattern supports * wildcard

Example: ["KEYS", "user:*"] -> find all keys starting with "user:"
*/
func (p *Peer) parseKeysCommand(arr []resp.Value) (Command, error) {
	if len(arr) != 2 {
		return nil, fmt.Errorf("wrong number of arguments for 'KEYS' command")
	}

	return KeysCommand{
		pattern: arr[1].String(),
	}, nil
}

/*
parseFlushAllCommand parses FLUSHALL command: FLUSHALL

FLUSHALL removes all keys from the database.

Validation:
  - Must have exactly 1 argument (just FLUSHALL)
  - No parameters allowed

Example: ["FLUSHALL"] -> delete everything
*/
func (p *Peer) parseFlushAllCommand(arr []resp.Value) (Command, error) {
	if len(arr) != 1 {
		return nil, fmt.Errorf("wrong number of arguments for 'FLUSHALL' command")
	}

	return FlushAllCommand{}, nil
}

func (p *Peer) parseHelloCommand(arr []resp.Value) (Command, error) {
	value := "2"
	if len(arr) > 1 {
		value = arr[1].String()
	}

	return HelloCommand{value: value}, nil
}

func (p *Peer) parseClientCommand(arr []resp.Value) (Command, error) {
	value := ""
	if len(arr) > 1 {
		value = arr[1].String()
	}

	return ClientCommand{value: value}, nil
}

func (p *Peer) parsePingCommand(arr []resp.Value) (Command, error) {
	message := ""
	if len(arr) > 1 {
		message = arr[1].String()
	}

	return PingCommand{message: message}, nil
}
