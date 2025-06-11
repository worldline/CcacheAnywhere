package main

import (
	"ccache-backend-client/com"
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
	wg              sync.WaitGroup
}

func newServer(socketPath string, bufferSize int) (*SocketServer, error) {
	if _, err := os.Stat(socketPath); err == nil {
		LOG("try os.Stat for %v\n", socketPath)
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
	LOG("Server started, listening on: %v\n", s.socketPath)
	LOG("Limiting connections to a maximum of %d clients!\n", com.MAX_PARALLEL_CLIENTS)

	go s.monitorInactivity()
	semaphore := make(chan struct{}, com.MAX_PARALLEL_CLIENTS)

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				return
			}
			LOG("Error accepting connection: %v\n", err)
			continue
		}

		fd, err := conn.(*net.UnixConn).File()
		if err != nil {
			return
		}
		LOG("Request from client over fd: %d\n", fd.Fd())

		s.resetInactivityTimer()
		semaphore <- struct{}{}
		s.wg.Add(1)
		LOG("Accepted new connection from: %d\n", fd.Fd())

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
	socketInterface := CreateSocketHandler(com.PACK_SIZE, &conn)
	backendInterface, err := CreateBackend(BACKEND_TYPE)
	if err != nil {
		WARN("%v\n", err.Error())
		return
	}

	buf := make([]byte, com.FIXED_BUF_SIZE)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			LOG("Connection closed: %d\n", fd.Fd())
			return
		}

		if n > 0 {
			packet, err := com.ParsePacket(buf[:n]) // should provide option serialized=true/false
			if err != nil {
				LOG("Error with packet format: %v\n", err.Error())
				continue
			}

			receivedMessage, err := socketInterface.Assemble(*packet)

			if err != nil {
				LOG("Connection closing! %v\n", err)
				return
			}

			if receivedMessage != nil {
				LOG("Server: Handle packet\n")
				backendInterface.Handle(receivedMessage)
				LOG("Server: Socket send\n")
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
			LOG("No activity for %v Minutes. Shutting down!\n", inactivityTimeout.Minutes())
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
		LOG("Error removing socket file: %w\n", err)
	}
}
