package main

import (
	"fmt"
	"io"
	"log"
	"net"

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
			errorResp := respWriteError(fmt.Sprintf("ERR %s", err.Error()))
			p.Send(errorResp)
			continue
		}

		p.messageChannel <- Message{
			cmd:  cmd,
			peer: p,
		}
	}

	return nil
}
