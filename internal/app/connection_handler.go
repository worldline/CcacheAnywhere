package app

import (
	"bufio"
	"fmt"
	"net"
	"sync"

	//lint:ignore ST1001 for clean LOG operations
	. "ccache-backend-client/internal/logger"
	storage "ccache-backend-client/internal/storage"
	"ccache-backend-client/internal/tlv"
)

// ConnectionHandler manages the lifecycle of a single client connection
type ConnectionHandler struct {
	conn           net.Conn
	backendHandler *BackendHandler
	serializer     *tlv.Serializer
	parser         *tlv.Parser
	buffer         []byte
	reader         *bufio.Reader
	resetTimer     func() // callback to reset server's inactivity timer
}

// ConnectionHandlerFactory creates connection handlers with proper resource management
type ConnectionHandlerFactory struct {
	backendType string
}

var readerPool = sync.Pool{
	New: func() any {
		return bufio.NewReaderSize(nil, tlv.FIXED_BUF_SIZE)
	},
}

func GetBufioReader(conn net.Conn) *bufio.Reader {
	reader := readerPool.Get().(*bufio.Reader)
	reader.Reset(conn)
	return reader
}

func PutBufioReader(reader *bufio.Reader) {
	// reader.Reset(nil) // this causes reallocation
	readerPool.Put(reader)
}

func NewConnectionHandlerFactory(backendstr string) *ConnectionHandlerFactory {
	return &ConnectionHandlerFactory{backendType: backendstr}
}

func (f *ConnectionHandlerFactory) CreateHandler(conn net.Conn, resetTimer func()) (*ConnectionHandler, error) {
	backendHandler, err := NewBackendHandler(f.backendType)
	if err != nil {
		return nil, err
	}

	return &ConnectionHandler{
		conn:           conn,
		backendHandler: backendHandler,
		serializer:     tlv.GetSerializer(),
		parser:         tlv.NewParser(),
		reader:         GetBufioReader(conn),
		resetTimer:     resetTimer,
	}, nil
}

type BufferedStreamReader struct {
	conn   net.Conn
	reader *bufio.Reader
	// timeout time.Duration // Not now
}

func NewBufferedStreamReader(conn net.Conn) *BufferedStreamReader {
	return &BufferedStreamReader{
		conn:   conn,
		reader: bufio.NewReaderSize(conn, tlv.FIXED_BUF_SIZE),
	}
}

// Process handles the complete request/response cycle for this connection
func (h *ConnectionHandler) Process() {
	defer h.Cleanup()

	fd := h.getConnectionID()
	LOG("Processing connection: %s", fd)

	readBuffer := make([]byte, tlv.FIXED_BUF_SIZE)

	for {
		if !h.readAndAccumulate(readBuffer) {
			LOG("Connection closed: %s", fd)
			return
		}

		if h.processAccumulatedData() {
			go h.resetTimer()
		}
	}
}

// readN reads from connection and only returns true if read bytes equals N
func (h *ConnectionHandler) ReadN(N int, readBuffer []byte) bool {
	n, err := h.reader.Read(readBuffer)
	if err != nil || N != n {
		return false
	}

	h.buffer = append(h.buffer, readBuffer[:n]...)
	return N == n
}

// readAndAccumulate reads data from connection and accumulates it in the persistent buffer
func (h *ConnectionHandler) readAndAccumulate(readBuffer []byte) bool {
	n, err := h.reader.Read(readBuffer)
	if err != nil {
		return false
	}

	if n > 0 {
		h.buffer = append(h.buffer, readBuffer[:n]...)
	}

	return true
}

// processAccumulatedData attempts to parse and handle complete packets
func (h *ConnectionHandler) processAccumulatedData() bool {
	packet, err := h.parser.Parse(h.buffer)
	if err != nil {
		// Not enough data yet, continue reading
		return false
	}

	LOG("Received packet: %v", packet.Fields)

	if h.handlePacket(packet) {
		h.buffer = h.buffer[:0] // Reset buffer after successful processing
		return true
	}

	return false
}

// handlePacket processes a complete TLV packet
func (h *ConnectionHandler) handlePacket(packet *tlv.Message) bool {
	message, err := storage.Assemble(packet)
	if err != nil {
		LOG("Failed to assemble message: %v", err)
		return false
	}

	if message == nil {
		return false
	}

	// TODO:
	// Next optimisation my be to add the reader to the Handle
	// such that the bytes can be instantly copied to backend
	// connection
	LOG("Handling packet via backend")
	h.backendHandler.Handle(message)

	LOG("Sending response")
	return h.sendResponse(message)
}

// sendResponse serializes and sends the response back to the client
func (h *ConnectionHandler) sendResponse(message storage.Message) bool {
	err := message.WriteToSocket(h.conn, h.serializer)
	if err != nil {
		LOG("Failed to send response: %v", err)
		return false
	}
	return true
}

// getConnectionID returns a string identifier for this connection
func (h *ConnectionHandler) getConnectionID() string {
	if unixConn, ok := h.conn.(*net.UnixConn); ok {
		if file, err := unixConn.File(); err == nil {
			return fmt.Sprintf("fd-%d", file.Fd())
		}
	}
	return h.conn.RemoteAddr().String()
}

// cleanup releases all resources associated with this connection handler
func (h *ConnectionHandler) Cleanup() {
	if h.serializer != nil {
		tlv.PutSerializer(h.serializer)
	}

	if h.reader != nil {
		readerPool.Put(h.reader)
	}
}
