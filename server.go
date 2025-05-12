package main

import (
	"ccache-backend-client/com"
	"ccache-backend-client/utils"
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

type SocketServer struct {
	bufferSize      int
	socketPath      string
	listener        net.Listener
	inactivityTimer *time.Timer
	mu              sync.Mutex
}

func newServer(socketPath string, bufferSize int) (*SocketServer, error) {
	l, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, err
	}

	return &SocketServer{
		bufferSize:      bufferSize,
		socketPath:      socketPath,
		listener:        l,
		inactivityTimer: time.NewTimer(inactivityTimeout),
	}, nil
}

func (s *SocketServer) start() {
	defer s.listener.Close()
	fmt.Println("Server started, listening on:", s.socketPath)

	go s.monitorInactivity()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				return
			}
			fmt.Println("Error accepting connection:", err)
			continue
		}

		s.resetInactivityTimer()

		fmt.Println("Accepted new connection from:", conn.RemoteAddr())
		go s.handleConnection(conn)
	}
}

func (s *SocketServer) handleConnection(conn net.Conn) {
	defer conn.Close()
	var socketInterface utils.SocketHandler
	backendInterface := utils.CreateBackend("http")

	buf := make([]byte, com.FIXED_BUF_SIZE)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Println("Connection closed:", conn.RemoteAddr())
			return
		}

		if n > 0 {
			messageString := string(buf[:n])
			fmt.Printf("Received: %s\n", messageString)

			packet, err := com.ParsePacket(buf[:n]) // should provide option serialized=true/false
			if err != nil {
				fmt.Println("Error with packet format!")
				continue
			}

			receivedMessage, err := socketInterface.Assemble(*packet)

			if err != nil {
				fmt.Println("Connection closing!", err)
				return
			}

			// Check for the "setup" message
			// maybe construct something like msgr.Handle(receivedMessage, conn)
			if receivedMessage != nil {
				backendInterface.Handle(receivedMessage)
				socketInterface.Handle(receivedMessage)
			}
		}

		s.resetInactivityTimer()
	}
}

func (s *SocketServer) monitorInactivity() {
	for {
		select {
		case <-s.inactivityTimer.C:
			fmt.Printf("No activity for %v Minutes. Shutting down.\n", inactivityTimeout.Minutes())
			s.cleanup()
			os.Exit(0)
		}
	}
}

func (s *SocketServer) resetInactivityTimer() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.inactivityTimer.Stop() {
		<-s.inactivityTimer.C
	}
	s.inactivityTimer.Reset(inactivityTimeout)
}

func (s *SocketServer) cleanup() {
	if err := os.Remove(s.socketPath); err != nil && !os.IsNotExist(err) {
		fmt.Printf("Error removing socket file: %s\n", err)
	}
}
