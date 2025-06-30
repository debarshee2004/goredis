package main

import (
	"net"
)

type Peer struct {
	connect        net.Conn
	messageChannel chan Message
	deleteChannel  chan *Peer
}
