package main

import (
	"flag"
	"log"
	"log/slog"
	"net"
)

const defaultListenPortAddress = ":5555"

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
	deletePeerChannel chan *Peer
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
		deletePeerChannel: make(chan *Peer),
		quitChannel:       make(chan struct{}),
		messageChannel:    make(chan Message),
		storage:           NewStorage(),
	}
}

func (s *Server) handleConnection(connection net.Conn) {
	peer := NewPeer(connection, s.messageChannel, s.deletePeerChannel)
	s.addPeerChannel <- peer

	// if err := peer.readLoop(); err != nil {
	// 	slog.Error("peer read error", "err", err, "remoteAddress", connection.RemoteAddr())
	// }
}

func (s *Server) loop() {
	for {
		select {
		// case message := <-s.messageChannel:
		// 	if err := s.handleMessage(message); err != nil {
		// 		slog.Error("message handling error", "err", err)
		// 	}

		case <-s.quitChannel:
			slog.Info("quiting the messaging channel")
			return

		case peer := <-s.addPeerChannel:
			slog.Info("peer connected", "remoteAddress", peer.connect.RemoteAddr())

		case peer := <-s.deletePeerChannel:
			slog.Info("peer disconnected", "remoteAddress", peer.connect.RemoteAddr())
			delete(s.peers, peer)
		}
	}
}

func (s *Server) acceptLoop() error {
	for {
		connection, err := s.ln.Accept()
		if err != nil {
			slog.Error("accept error", "err", err)
			continue
		}

		go s.handleConnection(connection)
	}
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.listenPortAddress)
	if err != nil {
		log.Fatal("Error starting server:", err)
		return err
	}

	s.ln = ln

	go s.loop()

	slog.Info("Redis clone server running", "listenPortAddress", s.listenPortAddress)

	return s.acceptLoop()
}

func main() {
	listenAddress := flag.String("listenAddress", defaultListenPortAddress, "listen address of the Redis server")
	flag.Parse()

	server := NewServer(Config{
		listenPortAddress: *listenAddress,
	})

	log.Fatal(server.Start())
}
