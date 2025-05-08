package com

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sync"
)

// The reserved bytes may be used in the future for passing the
// 0         8        16        24        32
// +---------+---------+---------+---------+
// | msgtype |  rest?  |     reserved      |
// +---------+---------+---------+---------+
// | msg id  |   ACK   |     reserved      |
// +---------+---------+---------+---------+
// |              msg length               |
// +---------+---------+---------+---------+
// |             bit of offset             |
// +---------+---------+---------+---------+
// |                                       |
// .            ...  body ...              .
// .              4080 Bytes               .
// .                                       .
// +---------------------------------------+

type Packet struct {
	MsgType   uint8  // 8 bits
	Rest      uint8  // 8 bits
	Reserved1 uint16 // 16 bits
	MsgID     uint8  // 8 bits
	Ack       uint8  // 8 bits
	Reserved2 uint16 // 16 bits
	MsgLength uint32 // 32 bits
	Offset    uint32 // 32 bits
	Body      []byte // 4080 bytes
}

func ReadFields(buffer *bytes.Buffer, fields ...any) error {
	for _, field := range fields {
		if err := binary.Read(buffer, binary.BigEndian, field); err != nil {
			return err
		}
	}
	return nil
}

func ParsePacket(data []byte) (*Packet, error) {
	if len(data) < 16 {
		return nil, fmt.Errorf("data too short, header alone must be 16 bytes")
	}

	buffer := bytes.NewBuffer(data)
	packet := &Packet{}

	if err := ReadFields(buffer,
		&packet.MsgType,
		&packet.Rest,
		&packet.Reserved1,
		&packet.MsgID,
		&packet.Ack,
		&packet.Reserved2,
		&packet.MsgLength,
		&packet.Offset); err != nil {
		return nil, err
	}

	packet.Body = make([]byte, packet.MsgLength)
	if _, err := buffer.Read(packet.Body); err != nil {
		return nil, err
	}

	return packet, nil
}

func Serialize(data []byte) string {
	return fmt.Sprintf("%X", data)
}

func Deserialize(data string) ([]byte, error) {
	bytes, err := hex.DecodeString(data)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

type MessageQueue = []Messenger

type Messenger struct {
	mu      sync.Mutex
	mtype   uint8
	packets []Packet
}

func CreateMessenger() Messenger {
	return Messenger{
		mtype: 0,
	}
}

// there are 3 types of main messages
// Each packet already done with the deserialisation part
// so the following function just combines the bodies or returns error if all packets not here yet
// PUT key, value, only_if_missing
// GET key
// RM key

// returns empty if packets are incomplete otherwise returns string of assembled body
func (m *Messenger) AssembleMessage(p Packet) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.packets) == 0 {
		m.mtype = p.MsgType
	}

	if p.MsgType != m.mtype {
		return "", fmt.Errorf("packet type mismatch: expected %v, got %v", m.mtype, p.MsgType)
	}

	m.packets = append(m.packets, p)

	if p.Rest == 0 { // this is the final fragment
		finalMessage := ""
		for _, packet := range m.packets {
			finalMessage += string(packet.Body) + " "
		}
		return finalMessage, nil
	}

	return "", nil
}

// func TEST1() {
// 	data := []byte{0x01, 0x00, 0x02, 0x03, 0x04, 0x05, 0x00, 0x06, 0x00, 0x00, 0x00, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x1}
// 	packet, err := ParsePacket(data)
// 	if err != nil {
// 		fmt.Println("Error parsing packet:", err)
// 		return
// 	}
// 	if packet.MsgLength != len(packet.Body) {
//  	panic("failed with parsing")
// 	}
// }
//
// func TEST2() {
// 	originalData := []byte{0x01, 0x2A, 0x57} // Sample byte slice
// 	serialized := Serialize(originalData)
// 	fmt.Println("Serialized:", serialized)

// 	deserialized, err := Deserialize(serialized)
// 	if err != nil {
// 		fmt.Println("Error deserializing:", err)
// 		return
// 	}
// 	fmt.Println("Deserialized:", deserialized)
//
//  if (deserialised != originalData) {
//		panic ("failed with serialisation and deserialisation")
//	}
// }
