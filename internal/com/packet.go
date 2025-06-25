package com

import (
	"bytes"
	"encoding/binary"
	"fmt"

	. "ccache-backend-client/internal/logger"
)

var FIXED_BUF_SIZE int
var SOCKET_PATH string

// The reserved bytes may be used in the future for passing the fd
// 0         8        16        24        32
// +---------+---------+---------+---------+
// | msgtype |  fdesc  | msg id  |   ACK   |
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
	FDesc     uint8  // 8 bits
	MsgID     uint8  // 8 bits
	RespCode  uint8  // 8 bits
	MsgLength uint32 // 32 bits
	Offset    uint32 // 32 bits
	Body      []byte // 4080 bytes
}

func (p *Packet) Print() {
	LOG("Head:   %v %v %v %v\n", p.MsgType, p.FDesc, p.MsgID, p.RespCode)
	LOG("Length: %v\n", p.MsgLength)
	LOG("Offset: %v\n", p.Offset)
	LOG("Body:   %v\n", p.Body[:p.MsgLength])
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
	return Packet{
		MsgType:   msgtype,
		FDesc:     remainder,
		MsgID:     msgId,
		RespCode:  respCode,
		MsgLength: uint32(len(data)),
		Offset:    0,
		Body:      data,
	}
}

func (p *Packet) Deparse() []byte {
	var deparsedMessage []byte

	deparsedMessage = append(deparsedMessage, Serialize(p.MsgType)...)
	deparsedMessage = append(deparsedMessage, Serialize(p.FDesc)...)
	deparsedMessage = append(deparsedMessage, Serialize(p.MsgID)...)
	deparsedMessage = append(deparsedMessage, Serialize(p.RespCode)...)
	deparsedMessage = append(deparsedMessage, Serialize(p.MsgLength)...)
	deparsedMessage = append(deparsedMessage, Serialize(p.Offset)...)
	for _, octet := range p.Body {
		deparsedMessage = append(deparsedMessage, Serialize(octet)...)
	}
	deparsedMessage = append(deparsedMessage, 0xFF)

	return deparsedMessage
}

func ParsePacket(data []byte) (*Packet, error) {
	if len(data) < 16 {
		return nil, fmt.Errorf("data too short, header alone must be 16 bytes")
	}

	data, err := Deserialize(data)
	if err != nil {
		return nil, fmt.Errorf("deserialization of received data failed: %w", err)
	}
	buffer := bytes.NewBuffer(data)
	packet := &Packet{}

	if err := readFields(buffer,
		&packet.MsgType,
		&packet.FDesc,
		&packet.MsgID,
		&packet.RespCode,
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
