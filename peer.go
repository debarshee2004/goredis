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

type Peer struct {
	connect        net.Conn
	messageChannel chan Message
	deleteChannel  chan *Peer
}

func NewPeer(connect net.Conn, messageChannel chan Message, deleteChannel chan *Peer) *Peer {
	return &Peer{
		connect:        connect,
		messageChannel: messageChannel,
		deleteChannel:  deleteChannel,
	}
}

func (p *Peer) Send(message []byte) (int, error) {
	return p.connect.Write(message)
}

func (p *Peer) readLoop() error {
	rd := resp.NewReader(p.connect)

	for {
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

		cmd, err := p.parseCommand(v)
		if err != nil {
			// errorResp := respWriteError(fmt.Sprintf("ERR %s", err.Error()))
			// p.Send(errorResp)
			continue
		}

		p.messageChannel <- Message{
			cmd:  cmd,
			peer: p,
		}
	}

	return nil
}

func (p *Peer) parseCommand(v resp.Value) (Command, error) {
	if v.Type() != resp.Array {
		return nil, fmt.Errorf("expected array")
	}

	arr := v.Array()
	if len(arr) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	cmdName := strings.ToUpper(arr[0].String())

	switch cmdName {
	default:
		return nil, fmt.Errorf("unknown commands '%s'", cmdName)
	}
}

func (p *Peer) parseSetCommand(arr []resp.Value) (Command, error) {
	if len(arr) < 3 {
		return nil, fmt.Errorf("wrong number of arguments for 'SET' command")
	}

	cmd := SetCommand{
		key: arr[1].Bytes(),
		val: arr[2].Bytes(),
	}

	if len(arr) >= 5 && strings.ToUpper(arr[3].String()) == "EX" {
		seconds, err := strconv.Atoi(arr[4].String())
		if err != nil {
			return nil, fmt.Errorf("invalid expire time")
		}
		cmd.expiry = time.Duration(seconds) * time.Second
	}

	return cmd, nil
}

func (p *Peer) parseGetCommand(arr []resp.Value) (Command, error) {
	if len(arr) != 2 {
		return nil, fmt.Errorf("wrong number of arguments for 'GET' command")
	}

	return GetCommand{
		key: arr[1].Bytes(),
	}, nil
}

func (p *Peer) parseDelCommand(arr []resp.Value) (Command, error) {
	if len(arr) < 2 {
		return nil, fmt.Errorf("wrong number of arguments for 'DEL' command")
	}

	keys := make([][]byte, len(arr)-1)
	for i := 1; i < len(arr); i++ {
		keys[i-1] = arr[i].Bytes()
	}

	return DelCommand{keys: keys}, nil
}

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

func (p *Peer) parseAppendCommand(arr []resp.Value) (Command, error) {
	if len(arr) != 3 {
		return nil, fmt.Errorf("wrong number of arguments for 'APPEND' command")
	}

	return AppendCommand{
		key: arr[1].Bytes(),
		val: arr[2].Bytes(),
	}, nil
}

func (p *Peer) parseStrlenCommand(arr []resp.Value) (Command, error) {
	if len(arr) != 2 {
		return nil, fmt.Errorf("wrong number of arguments for 'STRLEN' command")
	}

	return StrlenCommand{
		key: arr[1].Bytes(),
	}, nil
}

func (p *Peer) parseGetRangeCommand(arr []resp.Value) (Command, error) {
	if len(arr) != 4 {
		return nil, fmt.Errorf("wrong number of arguments for 'GETRANGE' command")
	}

	start, err := strconv.Atoi(arr[2].String())
	if err != nil {
		return nil, fmt.Errorf("invalid start index")
	}

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

func (p *Peer) parseSetRangeCommand(arr []resp.Value) (Command, error) {
	if len(arr) != 4 {
		return nil, fmt.Errorf("wrong number of arguments for 'SETRANGE' command")
	}

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

func (p *Peer) parseDecrCommand(arr []resp.Value) (Command, error) {
	if len(arr) != 2 {
		return nil, fmt.Errorf("wrong number of arguments for 'DECR' command")
	}

	return DecrCommand{
		key: arr[1].Bytes(),
	}, nil
}

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

func (p *Peer) parseMSetCommand(arr []resp.Value) (Command, error) {
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

func (p *Peer) parseGetSetCommand(arr []resp.Value) (Command, error) {
	if len(arr) != 3 {
		return nil, fmt.Errorf("wrong number of arguments for 'GETSET' command")
	}

	return GetSetCommand{
		key: arr[1].Bytes(),
		val: arr[2].Bytes(),
	}, nil
}

func (p *Peer) parseKeysCommand(arr []resp.Value) (Command, error) {
	if len(arr) != 2 {
		return nil, fmt.Errorf("wrong number of arguments for 'KEYS' command")
	}

	return KeysCommand{
		pattern: arr[1].String(),
	}, nil
}

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
