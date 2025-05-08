package main

import (
	"ccache-backend-client/com"
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

type server struct {
	bufferSize      int
	socketPath      string
	listener        net.Listener
	inactivityTimer *time.Timer
	mu              sync.Mutex
}

func newServer(socketPath string, bufferSize int) (*server, error) {
	l, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, err
	}

	return &server{
		bufferSize:      bufferSize,
		socketPath:      socketPath,
		listener:        l,
		inactivityTimer: time.NewTimer(inactivityTimeout),
	}, nil
}

func (s *server) start() {
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

func (s *server) handleConnection(conn net.Conn) {
	defer conn.Close()
	msgr := com.CreateMessenger()

	buf := make([]byte, 1024)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Println("Connection closed:", conn.RemoteAddr())
			return
		}

		if n > 0 {
			receivedMessage := string(buf[:n])
			fmt.Printf("Received: %s\n", receivedMessage)

			packet, err := com.ParsePacket(buf[:n])
			if err != nil {
				fmt.Println("Error with packet format!")
				continue
			}

			receivedMessage, err = msgr.AssembleMessage(*packet)

			if err != nil {
				fmt.Println("Connection closing!", err)
				return
			}

			// Check for the "setup" message
			if receivedMessage == "" {
				_, err := conn.Write([]byte("ACK!")) // Send ACK for "setup"
				if err != nil {
					fmt.Printf("Error writing to connection: %v\n", err)
					return
				}
			} else {
				_, err := conn.Write([]byte("Send Me!")) // Send ACK for "setup"
				if err != nil {
					fmt.Printf("Error writing to connection: %v\n", err)
					return
				}
			}
		}

		s.resetInactivityTimer()
	}
}

func (s *server) monitorInactivity() {
	for {
		select {
		case <-s.inactivityTimer.C:
			fmt.Printf("No activity for %v Minutes. Shutting down.\n", inactivityTimeout.Minutes())
			s.cleanup()
			os.Exit(0)
		}
	}
}

func (s *server) resetInactivityTimer() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.inactivityTimer.Stop() {
		<-s.inactivityTimer.C
	}
	s.inactivityTimer.Reset(inactivityTimeout)
}

func (s *server) cleanup() {
	if err := os.Remove(s.socketPath); err != nil && !os.IsNotExist(err) {
		fmt.Printf("Error removing socket file: %s\n", err)
	}
}
