package utils

import (
	"ccache-backend-client/com"
	storage "ccache-backend-client/storage"
	"crypto/rand"
	"fmt"
	"log"
	"net"
	"net/url"
	"strings"
	"sync"
)

func CreateSocketHandler(bufferSize int, conn *net.Conn) SocketHandler {
	return SocketHandler{node: *conn, PacketSize: bufferSize}
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
	mu         sync.Mutex
	node       net.Conn
	curID      uint8
	respCode   uint8
	packets    []com.Packet
	PacketSize int
}

func (h *SocketHandler) fragment(msg *storage.Message) []com.Packet {
	var packets []com.Packet
	data, status := (*msg).Read()
	h.respCode = uint8(status)
	msgType := (*msg).Type()

	if msgType > 3 {
		data = make([]byte, 400)
		rand.Read(data)
	}

	bodySize := h.PacketSize - 16
	chunksNum := (len(data) + bodySize - 1) / bodySize
	for i := range chunksNum {
		start := i * bodySize
		end := min(start+bodySize, len(data))

		packet := com.CreatePacket(data[start:end], msgType, h.respCode, h.curID, uint8(chunksNum-i-1))
		packets = append(packets, packet)
	}

	return packets
}

// deparse message and send it over network
func (h *SocketHandler) Handle(msg storage.Message) {
	h.mu.Lock()
	defer h.mu.Unlock()

	packets := h.fragment(&msg)

	log.Printf("Send %d number of packets", len(packets))
	for _, p := range packets {
		formedPacket := p.Deparse()
		h.node.Write(formedPacket)
	}
}

func (h *SocketHandler) Assemble(p com.Packet) (storage.Message, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.packets = append(h.packets, p)
	h.respCode = p.MsgID

	if p.Rest != 0 { // wait for final fragment
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
		// TODO make this loggable within the LOG file in ccache?
		log.Printf("Handling message failed for backend: %v\n", err.Error())
	}
}
