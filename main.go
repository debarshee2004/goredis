package main

import (
	"flag"
	"log"
	"log/slog"
	"net"
)

const defaultListenPortAddress = ":5555"

/*
Config holds the server configuration
This struct contains all the settings needed to configure our Redis server
*/
type Config struct {
	listenPortAddress string
}

/*
Message represents a command message from a client
This is used to pass commands through channels between goroutines
It contains both the command and which peer (client) sent it
*/
type Message struct {
	cmd  Command
	peer *Peer
}

/*
Server represents the main Redis server
This is the core structure that manages all client connections,
handles commands, and maintains the key-value storage
*/
type Server struct {
	Config                           // Embedded config struct - contains server settings
	peers             map[*Peer]bool // Map of all currently connected clients (peers)
	ln                net.Listener   // TCP listener that accepts new connections
	addPeerChannel    chan *Peer     // Channel for notifying when a new client connects
	deletePeerChannel chan *Peer     // Channel for notifying when a client disconnects
	quitChannel       chan struct{}  // Channel for gracefully shutting down the server
	messageChannel    chan Message   // Channel for receiving commands from all clients

	// The key-value storage engine that holds our data
	storage *Storage
}

/*
NewServer creates a new Redis server instance
This is a constructor function that initializes all the server components
*/
func NewServer(cfg Config) *Server {
	// If no listen address is provided, use the default
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

/*
handleConnection handles a new connection by creating a peer and starting its read loop
This function is called in a goroutine for each new client connection
It creates a Peer object to represent the client and starts reading commands from them
*/
func (s *Server) handleConnection(connection net.Conn) {
	/*
		Create a new Peer object to represent this client connection
		The peer will send messages to msgCh and notify delPeerCh when it disconnects
	*/
	peer := NewPeer(connection, s.messageChannel, s.deletePeerChannel)

	// Notify the main server loop that a new peer has connected
	s.addPeerChannel <- peer

	// Start reading commands from this client. This blocks until the client disconnects or an error occurs
	if err := peer.readLoop(); err != nil {
		slog.Error("peer read error", "err", err, "remoteAddress", connection.RemoteAddr())
	}
}

/*
loop is the main server event loop

This is the heart of the server - it processes all events using Go channels
It runs in its own goroutine and handles:
  - Incoming commands from clients
  - New client connections
  - Client disconnections
  - Server shutdown signals
*/
func (s *Server) loop() {
	for {
		/* Use select to listen on multiple channels simultaneously
		   This is Go's way of handling multiple concurrent events */
		select {
		case message := <-s.messageChannel:
			// A command message arrived from a client
			if err := s.handleMessage(message); err != nil {
				slog.Error("message handling error", "err", err)
			}

		case <-s.quitChannel:
			// Server shutdown signal received - Exit the loop and stop the server
			slog.Info("quiting the messaging channel")
			return

		case peer := <-s.addPeerChannel:
			// A new client has connected - Add them to our list of active clients
			slog.Info("peer connected", "remoteAddress", peer.connect.RemoteAddr())

		case peer := <-s.deletePeerChannel:
			// A client has disconnected - Remove them from our list of active clients
			slog.Info("peer disconnected", "remoteAddress", peer.connect.RemoteAddr())
			delete(s.peers, peer)
		}
	}
}

/*
acceptLoop accepts incoming connections

This method runs in a loop, waiting for new TCP connections
Each time a client connects, it creates a new goroutine to handle that client
*/
func (s *Server) acceptLoop() error {
	for {
		// Accept blocks until a new connection arrives
		connection, err := s.ln.Accept()
		if err != nil {
			slog.Error("accept error", "err", err)
			continue
		}

		/* Handle each connection in a separate goroutine
		   This allows the server to handle multiple clients simultaneously */
		go s.handleConnection(connection)
	}
}

/*
Start starts the Redis server
This method begins listening for connections and starts the main server loop
*/
func (s *Server) Start() error {
	// Create a TCP listener on the specified address
	ln, err := net.Listen("tcp", s.listenPortAddress)
	if err != nil {
		log.Fatal("Error starting server:", err)
		return err
	}

	s.ln = ln

	/* Start the main server loop in a goroutine
	   This runs concurrently and handles all server events */
	go s.loop()

	slog.Info("Redis clone server running", "listenPortAddress", s.listenPortAddress)

	// Start accepting connections (this blocks the main thread)
	return s.acceptLoop()
}

func main() {
	/*
		Parse command line flags - This allows users to specify a custom listen address when starting the server
		Example: ./gotrsredis -listenAddr=":6379"
	*/
	listenAddress := flag.String("listenAddress", defaultListenPortAddress, "listen address of the Redis server")
	flag.Parse()

	// Create a new server instance with the provided configuration
	server := NewServer(Config{
		listenPortAddress: *listenAddress,
	})

	log.Fatal(server.Start())
}
