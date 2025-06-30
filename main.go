package main

import (
	"net"
)

const defaultListenPortAddress = ":6379"

type Config struct {
	listenPortAddress string
}

type Message struct {
	cmd  Command
	peer *Peer
}

type Server struct {
	Config
	peers             map[*Peer]bool
	ln                net.Listener
	addPeerChannel    chan *Peer
	deleteChannelPeer chan *Peer
	quitChannel       chan struct{}
	messageChannel    chan Message

	storage *Storage
}

func main() {

}
