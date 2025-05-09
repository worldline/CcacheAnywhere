package main

import (
	"ccache-backend-client/com"
	"fmt"
	"net"
	"sync"
)

type Messenger struct {
	mu      sync.Mutex
	curID   uint8
	mtype   uint8
	lastId  uint8
	packets []com.Packet
}

func CreateMessenger() Messenger {
	return Messenger{
		curID:  0,
		mtype:  0,
		lastId: 0,
	}
}

func (m *Messenger) AcknowledgeOnly(conn net.Conn) error {
	data := []byte{m.mtype, 0x00, m.curID, m.lastId,
		/*  Reserved */ 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		/* MsgLength */ 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		/*    Offset */ 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

	conn.Write([]byte(com.Serialize(data)))
	m.curID++
	return nil
}

// returns empty if packets are incomplete otherwise returns string of assembled body
func (m *Messenger) AssembleMessage(p com.Packet) (com.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.packets) == 0 {
		m.mtype = p.MsgType
	}

	if p.MsgType != m.mtype {
		return nil, fmt.Errorf("packet type mismatch: expected %v, got %v", m.mtype, p.MsgType)
	}

	m.packets = append(m.packets, p)
	m.lastId = p.MsgID

	if p.Rest != 0 { // wait for final fragment
		return nil, nil
	}

	var finalMessage com.Message
	switch m.mtype {
	case 1:
		finalMessage = &com.GetMessage{}
	case 2:
		finalMessage = &com.PutMessage{}
	case 3:
		finalMessage = &com.RmMessage{}
	case 4:
		finalMessage = &com.GetMessage{}
	default:
		return nil, fmt.Errorf("message type is not protocol coherent")
	}

	var data []byte
	for _, pck := range m.packets {
		data = append(data, pck.Body...)
	}

	if err := finalMessage.Read(data); err != nil {
		return nil, fmt.Errorf("reading final message failed: %w", err)
	}

	return finalMessage, nil
}
