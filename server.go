package main

import (
	"ccache-backend-client/com"
	"ccache-backend-client/utils"
	"fmt"
	"log"
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
	wg              sync.WaitGroup
}

func newServer(socketPath string, bufferSize int) (*SocketServer, error) {
	if _, err := os.Stat(socketPath); err == nil {
		log.Println("try os.Stat")
		conn, err := net.Dial("unix", socketPath)
		if err == nil {
			conn.Close()
			return nil, fmt.Errorf("socket already in use")
		}
		os.Remove(socketPath) // exists but can't connect (stale)
	}
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
	log.Println("Server started, listening on:", s.socketPath)
	log.Printf("Limiting connections to a maximum of %d clients!\n", com.MAX_PARALLEL_CLIENTS)

	go s.monitorInactivity()
	semaphore := make(chan struct{}, com.MAX_PARALLEL_CLIENTS)

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				return
			}
			log.Println("Error accepting connection:", err)
			continue
		}

		fd, err := conn.(*net.UnixConn).File()
		if err != nil {
			return
		}
		log.Println("Request from client over fd:", fd)

		s.resetInactivityTimer()
		semaphore <- struct{}{}
		s.wg.Add(1)
		log.Println("Accepted new connection from:", fd)

		go func(c net.Conn) {
			defer func() {
				c.Close()
				<-semaphore // Release semaphore
				s.wg.Done()
			}()

			s.handleConnection(c)
		}(conn)
	}
}

func (s *SocketServer) handleConnection(conn net.Conn) {
	defer conn.Close()
	fd, err := conn.(*net.UnixConn).File()
	if err != nil {
		return
	}
	socketInterface := utils.CreateSocketHandler(com.PACK_SIZE, &conn)
	backendInterface, err := utils.CreateBackend(BACKEND_TYPE)

	if err != nil {
		log.Println(err.Error())
		return
	}

	buf := make([]byte, com.FIXED_BUF_SIZE)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			log.Println("Connection closed:", fd)
			return
		}

		if n > 0 {
			packet, err := com.ParsePacket(buf[:n]) // should provide option serialized=true/false
			if err != nil {
				log.Println("Error with packet format: ", err.Error())
				continue
			}

			receivedMessage, err := socketInterface.Assemble(*packet)

			if err != nil {
				log.Println("Connection closing!", err)
				return
			}

			if receivedMessage != nil {
				log.Println("Server: Handle packet")
				backendInterface.Handle(receivedMessage)
				log.Println("Server: Socket send")
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
			log.Printf("No activity for %v Minutes. Shutting down.\n", inactivityTimeout.Minutes())
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
		log.Printf("Error removing socket file: %s\n", err)
	}
}
