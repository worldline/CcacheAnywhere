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
	"ccache-backend-client/internal/logger"
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

	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	var workerCount int64

	// Launch goroutine to listen for signals
	go func() {
		sig := <-sigChan
		logger.LOG("Received signal: %s, shutting down gracefully...\n", sig)
		cancel()
		s.listener.Close()
	}()

	logger.LOG("Server started, listening on: %v\n", s.socketPath)
	logger.LOG("Limiting connections to a maximum of %d clients!\n", constants.MAX_PARALLEL_CLIENTS)

	go s.monitorInactivity(ctx, cancel)

	semaphore := make(chan struct{}, constants.MAX_PARALLEL_CLIENTS)

	for {
		select {
		case <-ctx.Done():
			logger.LOG("Shutdown signal received! Exiting main loop.\n")
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
				logger.LOG("Accept error: %v ...continuing!\n", err)
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
			atomic.AddInt64(&workerCount, 1)
			logger.LOG("Accepted connection: %d TOTAL=%v\n", fd.Fd(), workerCount)

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
	socketInterface := CreateSocketHandler(&conn)
	backendInterface, err := CreateBackend(s.backendType)
	tlv_parser := tlv.NewParser()

	if err != nil {
		logger.WARN("%v\n", err.Error())
		return
	}

	persistentBuffer := make([]byte, 0)
	buf := make([]byte, 1024)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			logger.LOG("Connection closed: %d\n", fd.Fd())
			return
		}

		if n > 0 {
			persistentBuffer = append(persistentBuffer, buf[:n]...)
			packet, err := tlv_parser.Parse(persistentBuffer)
			if err != nil {
				continue
			}
			logger.LOG("Received %v\n", packet.Fields)

			receivedMessage, err := storage.Assemble(*packet)
			persistentBuffer = persistentBuffer[:0]

			if err != nil {
				logger.LOG("Connection closing! %v\n", err)
				return
			}

			if receivedMessage != nil { // handling
				logger.LOG("Server: Handle packet\n")
				backendInterface.Handle(receivedMessage)
				logger.LOG("Server: Socket send\n")
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
			logger.LOG("Inactivity monitor received shutdown signal. Exiting.\n")
			return
		case <-s.inactivityTimer.C:
			cancel()
			logger.LOG("No activity for %v Minutes. Shutting down!\n", constants.INACTIVITY_TIMEOUT.Minutes())
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
		logger.LOG("Error removing socket file: %v\n", err.Error())
	}
}
