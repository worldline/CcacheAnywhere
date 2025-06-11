package app

import (
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"ccache-backend-client/internal/com"
	"ccache-backend-client/internal/constants"
	"ccache-backend-client/internal/logger"
)

type SocketServer struct {
	bufferSize      int
	socketPath      string
	backendType     string
	listener        net.Listener
	inactivityTimer *time.Timer
	mu              sync.Mutex
	wg              sync.WaitGroup
}

func NewServer(socketPath string, bufferSize int, btype string) (*SocketServer, error) {
	if _, err := os.Stat(socketPath); err == nil {
		logger.LOG("try os.Stat for %v\n", socketPath)
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
		backendType:     btype,
		listener:        l,
		inactivityTimer: time.NewTimer(constants.INACTIVITY_TIMEOUT),
	}, nil
}

func (s *SocketServer) Start() {
	defer s.listener.Close()
	logger.LOG("Server started, listening on: %v\n", s.socketPath)
	logger.LOG("Limiting connections to a maximum of %d clients!\n", constants.MAX_PARALLEL_CLIENTS)

	go s.monitorInactivity()
	semaphore := make(chan struct{}, constants.MAX_PARALLEL_CLIENTS)

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				return
			}
			logger.LOG("Error accepting connection: %v\n", err)
			continue
		}

		fd, err := conn.(*net.UnixConn).File()
		if err != nil {
			return
		}
		logger.LOG("Request from client over fd: %d\n", fd.Fd())

		s.resetInactivityTimer()
		semaphore <- struct{}{}
		s.wg.Add(1)
		logger.LOG("Accepted new connection from: %d\n", fd.Fd())

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
	backendInterface, err := CreateBackend(s.backendType)
	if err != nil {
		logger.WARN("%v\n", err.Error())
		return
	}

	buf := make([]byte, com.FIXED_BUF_SIZE)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			logger.LOG("Connection closed: %d\n", fd.Fd())
			return
		}

		if n > 0 {
			packet, err := com.ParsePacket(buf[:n]) // should provide option serialized=true/false
			if err != nil {
				logger.LOG("Error with packet format: %v\n", err.Error())
				continue
			}

			receivedMessage, err := socketInterface.Assemble(*packet)

			if err != nil {
				logger.LOG("Connection closing! %v\n", err)
				return
			}

			if receivedMessage != nil {
				logger.LOG("Server: Handle packet\n")
				backendInterface.Handle(receivedMessage)
				logger.LOG("Server: Socket send\n")
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
			logger.LOG("No activity for %v Minutes. Shutting down!\n", constants.INACTIVITY_TIMEOUT.Minutes())
			s.Cleanup()
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
	s.inactivityTimer.Reset(constants.INACTIVITY_TIMEOUT)
}

func (s *SocketServer) Cleanup() {
	if err := os.Remove(s.socketPath); err != nil && !os.IsNotExist(err) {
		logger.LOG("Error removing socket file: %v\n", err.Error())
	}
}
