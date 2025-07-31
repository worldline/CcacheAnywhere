package app

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"ccache-backend-client/internal/constants"
	//lint:ignore ST1001 do want nice LOG operations
	. "ccache-backend-client/internal/logger"
	storage "ccache-backend-client/internal/storage"
	"ccache-backend-client/internal/tlv"
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
		WARN("Socket file exists at %v", socketPath)
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

	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	var workerCount int64

	// Launch goroutine to listen for signals
	go func() {
		sig := <-sigChan
		LOG("Received signal: %s, shutting down gracefully...", sig)
		cancel()
		s.listener.Close()
	}()

	LOG("Server started, listening on: %v", s.socketPath)
	LOG("Limiting connections to a maximum of %d clients!", constants.MAX_PARALLEL_CLIENTS)

	go s.monitorInactivity(ctx, cancel)

	semaphore := make(chan struct{}, constants.MAX_PARALLEL_CLIENTS)

	for {
		select {
		case <-ctx.Done():
			LOG("Shutdown signal received! Exiting main loop.")
			s.wg.Wait()
			return
		default:
			conn, err := s.listener.Accept()
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					return
				} else if opErr, ok := err.(*net.OpError); ok {
					if strings.Contains(opErr.Err.Error(), "use of closed network connection") {
						return
					}
				}
				LOG("Accept error: %v ...continuing!", err)
				continue
			}

			fd, err := conn.(*net.UnixConn).File()
			if err != nil {
				return
			}
			LOG("Request from client over fd: %d", fd.Fd())

			s.resetInactivityTimer()

			semaphore <- struct{}{}
			s.wg.Add(1)
			atomic.AddInt64(&workerCount, 1)
			LOG("Accepted connection: %d TOTAL=%v", fd.Fd(), workerCount)

			go func(c net.Conn) {
				defer func() {
					c.Close()
					<-semaphore // Release semaphore
					s.wg.Done()
					atomic.AddInt64(&workerCount, -1)
				}()

				select {
				case <-ctx.Done():
					return
				default:
					s.handleConnection(c)
				}
			}(conn)
		}
	}
}

func (s *SocketServer) handleConnection(conn net.Conn) {
	defer conn.Close()
	fd, err := conn.(*net.UnixConn).File()
	if err != nil {
		return
	}
	socketInterface := NewSocketHandler(&conn)
	backendInterface, err := NewBackendHandler(s.backendType)
	tlv_parser := tlv.NewParser()

	if err != nil {
		WARN("%v", err.Error())
		return
	}

	persistentBuffer := make([]byte, 0)
	buf := make([]byte, 1024)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			LOG("Connection closed: %d", fd.Fd())
			return
		}

		if n > 0 {
			persistentBuffer = append(persistentBuffer, buf[:n]...)
			packet, err := tlv_parser.Parse(persistentBuffer)
			if err != nil {
				continue
			}
			LOG("Received %v", packet.Fields)

			receivedMessage, err := storage.Assemble(*packet)
			persistentBuffer = persistentBuffer[:0]

			if err != nil {
				LOG("Connection closing! %v", err)
				return
			}

			if receivedMessage != nil { // handling
				LOG("Server: Handle packet")
				backendInterface.Handle(receivedMessage)
				LOG("Server: Socket send")
				socketInterface.Handle(receivedMessage)
			}
		}

		s.resetInactivityTimer()
	}
}

func (s *SocketServer) monitorInactivity(ctx context.Context, cancel context.CancelFunc) {
	for {
		select {
		case <-ctx.Done():
			LOG("Inactivity monitor received shutdown signal. Exiting.")
			return
		case <-s.inactivityTimer.C:
			cancel()
			LOG("No activity for %v Minutes. Shutting down!", constants.INACTIVITY_TIMEOUT.Minutes())
			s.listener.Close()
			return
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
		LOG("Error removing socket file: %v", err.Error())
	}
}
