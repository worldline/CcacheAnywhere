package utils

import (
	"ccache-backend-client/com"
	storage "ccache-backend-client/storage"
	"fmt"
	"net"
	"strings"
	"sync"
)

func CreateBackend(url string) BackendHandler {
	var tmp storage.Backend
	prefix := strings.Split(url, ":")[0]
	switch prefix {
	case "http":
		attributes, err := storage.ParseAttributes("http-config.json")
		if err != nil {
			panic("Config file issue!")
		}
		tmp = storage.CreateHTTPBackend(url, attributes)
	default:
		panic("Backend not implemented yet!")
	}
	return BackendHandler{node: tmp}
}

type Handler interface {
	Handle(storage.Message)
}

type SocketHandler struct {
	mu         sync.Mutex
	node       net.Conn
	curID      uint8
	ackID      uint8
	packets    []com.Packet
	BufferSize int
}

func (h *SocketHandler) fragment(msg *storage.Message) []com.Packet {
	var packets []com.Packet
	var msgType uint8
	data := (*msg).Read()
	switch (*msg).(type) {
	case *storage.GetMessage:
		msgType = 1
	case *storage.PutMessage:
		msgType = 2
	case *storage.RmMessage:
		msgType = 3
	case *storage.SetupMessage:
		msgType = 4
	case *storage.TestMessage:
		msgType = 5
	default:
		msgType = 0
	}

	chunksNum := (len(data) + h.BufferSize - 1) / h.BufferSize
	for i := range chunksNum {
		start := i * h.BufferSize
		end := min(start+h.BufferSize, len(data))

		packet := com.CreatePacket(data[start:end], msgType, h.ackID, h.curID, uint8(chunksNum-i))
		packets = append(packets, packet)
	}

	return packets
}

// deparse message and send it over network
func (h *SocketHandler) Handle(msg storage.Message) {
	h.mu.Lock()
	defer h.mu.Unlock()

	packets := h.fragment(&msg)

	for _, p := range packets {
		h.node.Write(p.Deparse())
	}
}

func (h *SocketHandler) Assemble(p com.Packet) (storage.Message, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.packets = append(h.packets, p)
	h.ackID = p.MsgID

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
		resultMessage = &storage.GetMessage{}
	default:
		return nil, fmt.Errorf("message type is not protocol coherent")
	}

	resultMessage.Create(data)
	return resultMessage, nil
}

type BackendHandler struct {
	node storage.Backend
}

func (h *BackendHandler) Handle(msg storage.Message) {
	err := msg.Write(h.node)

	if err != nil {
		fmt.Printf("Handling message failed for backend: %v", err)
	}
}
