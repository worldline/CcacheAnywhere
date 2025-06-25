package app

import (
	"crypto/rand"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"

	"ccache-backend-client/internal/com"
	"ccache-backend-client/internal/logger"
	storage "ccache-backend-client/internal/storage"
)

func CreateSocketHandler(conn *net.Conn) SocketHandler {
	return SocketHandler{node: *conn}
}

// Format of inputted url http://secret-key@domainname.com/path/to/folder|attribute=value
//
// Attributes might contain one or multiple key=value pairs separated by '|'
func parseUrl(input string) (*url.URL, []storage.Attribute) {
	parts := strings.Split(input, "|")
	var attributes []storage.Attribute

	parsedUrl, err := url.Parse(parts[0])
	if err != nil {
		return nil, nil
	}

	for _, attrStr := range parts[1:] {
		if attrStr != "" {
			attrs := strings.Split(attrStr, "=")
			if len(attrs) == 2 {
				attributes = append(attributes, storage.Attribute{Key: attrs[0], Value: attrs[1]})
			}
		}
	}

	return parsedUrl, attributes
}

func CreateBackend(url string) (*BackendHandler, error) {
	prefix := strings.Split(url, ":")[0]
	furl, attributes := parseUrl(url)
	switch prefix {
	case "http":
		return &BackendHandler{node: storage.CreateHTTPBackend(furl, attributes)}, nil
	case "gs":
		return &BackendHandler{node: storage.CreateGCSBackend(furl, attributes)}, nil
	default:
		return nil, fmt.Errorf("backend not implemented for prefix: %s", prefix)
	}
}

type Handler interface {
	Handle(storage.Message)
}

type SocketHandler struct {
	mu      sync.Mutex
	node    net.Conn
	curID   uint8
	packets []com.Packet
}

// deparse message and send it over network
func (h *SocketHandler) Handle(msg storage.Message) {
	data, status := msg.Read()
	msgType := msg.Type()

	if msgType > 3 {
		data = make([]byte, 40)
		rand.Read(data)
	}

	packet := com.CreatePacket(data, msgType, uint8(status), h.curID, 0)
	formedPacket := packet.Deparse()
	logger.LOG("Emit %v", string(formedPacket))
	h.node.Write(formedPacket)
	// fmt.Printf("Emit timepoint: %s\n", time.Now().Format("2006-01-02 15:04:05.000"))
}

// Deprecated: Used to be necessary for fragmentation. Use assemble method from messege.go
func (h *SocketHandler) Assemble(p com.Packet) (storage.Message, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.packets = append(h.packets, p)

	if p.FDesc != 0 { // wait for final fragment
		return nil, nil
	}

	var data []byte
	for _, pck := range h.packets {
		data = append(data, pck.Body[:pck.MsgLength]...)
	}

	var resultMessage storage.Message
	switch p.MsgType { // TODO create the messages
	case 1:
		resultMessage = &storage.GetMessage{}
	case 2:
		resultMessage = &storage.PutMessage{}
	case 3:
		resultMessage = &storage.RmMessage{}
	case 4:
		resultMessage = &storage.TestMessage{}
	default:
		return nil, fmt.Errorf("message type is not protocol coherent")
	}

	resultMessage.Create(data)
	h.packets = nil // reset after assembling a message successfully
	return resultMessage, nil
}

type BackendHandler struct {
	node storage.Backend
}

func (h *BackendHandler) Handle(msg storage.Message) {
	err := msg.Write(h.node)

	if err != nil {
		logger.LOG("Handling message failed for backend: %v\n", err.Error())
	}
}
