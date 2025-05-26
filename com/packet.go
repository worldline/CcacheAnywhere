package com

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

var FIXED_BUF_SIZE int
var PACK_SIZE int
var SOCKET_PATH string
var MAX_PARALLEL_CLIENTS = 32

// The reserved bytes may be used in the future for passing the fd
// 0         8        16        24        32
// +---------+---------+---------+---------+
// | msgtype |  rest?  | msg id  |   ACK   |
// +---------+---------+---------+---------+
// |     reserved      |     reserved      |
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
	RespCode  uint8  // 8 bits
	Reserved2 uint16 // 16 bits
	MsgLength uint32 // 32 bits
	Offset    uint32 // 32 bits
	Body      []byte // 4080 bytes
}

func (p *Packet) Print() {
	fmt.Println("Head:   ", p.MsgType, p.Rest, p.MsgID, p.RespCode)
	fmt.Println("Unused: ", p.Reserved1, p.Reserved2)
	fmt.Println("Length: ", p.MsgLength)
	fmt.Println("Offset: ", p.Offset)
	fmt.Println("Body:   ", p.Body[:p.MsgLength])
}

func readFields(buffer *bytes.Buffer, fields ...any) error {
	for _, field := range fields {
		if err := binary.Read(buffer, binary.BigEndian, field); err != nil {
			return err
		}
	}
	return nil
}

func padBytes(data []byte, bufferSize int) []byte {
	if len(data) >= bufferSize {
		return data[:bufferSize]
	}

	paddedData := make([]byte, bufferSize)
	copy(paddedData, data)

	return paddedData
}

func CreatePacket(data []byte, msgtype uint8, respCode uint8, msgId uint8, remainder uint8) Packet {
	// TODO some checks to the inputs
	pdata := padBytes(data, PACK_SIZE-16)
	return Packet{
		MsgType:   msgtype,
		Rest:      remainder,
		MsgID:     msgId,
		RespCode:  respCode,
		Reserved1: 0,
		Reserved2: 0,
		MsgLength: uint32(len(data)),
		Offset:    0,
		Body:      pdata,
	}
}

func (p *Packet) Deparse() []byte {
	var deparsedMessage []byte

	deparsedMessage = append(deparsedMessage, Serialize(p.MsgType)...)
	deparsedMessage = append(deparsedMessage, Serialize(p.Rest)...)
	deparsedMessage = append(deparsedMessage, Serialize(p.MsgID)...)
	deparsedMessage = append(deparsedMessage, Serialize(p.RespCode)...)
	deparsedMessage = append(deparsedMessage, Serialize(p.Reserved1)...)
	deparsedMessage = append(deparsedMessage, Serialize(p.Reserved2)...)
	deparsedMessage = append(deparsedMessage, Serialize(p.MsgLength)...)
	deparsedMessage = append(deparsedMessage, Serialize(p.Offset)...)
	for _, octet := range p.Body {
		deparsedMessage = append(deparsedMessage, Serialize(octet)...)
	}

	return deparsedMessage
}

func ParsePacket(data []byte) (*Packet, error) {
	if len(data) < 16 {
		return nil, fmt.Errorf("data too short, header alone must be 16 bytes")
	}

	data, err := Deserialize(string(data))
	if err != nil {
		return nil, fmt.Errorf("deserialization of received data failed: %w", err)
	}
	buffer := bytes.NewBuffer(data)
	packet := &Packet{}

	if err := readFields(buffer,
		&packet.MsgType,
		&packet.Rest,
		&packet.MsgID,
		&packet.RespCode,
		&packet.Reserved1,
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
