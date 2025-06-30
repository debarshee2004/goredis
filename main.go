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

func NewServer(cfg Config) *Server {
	if len(cfg.listenPortAddress) == 0 {
		cfg.listenPortAddress = defaultListenPortAddress
	}

	return &Server{
		Config:            cfg,
		peers:             make(map[*Peer]bool),
		addPeerChannel:    make(chan *Peer),
		deleteChannelPeer: make(chan *Peer),
		quitChannel:       make(chan struct{}),
		messageChannel:    make(chan Message),
		storage:           NewStorage(),
	}

}

func main() {

}
