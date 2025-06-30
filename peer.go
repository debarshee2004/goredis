package main

import (
	"net"
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
