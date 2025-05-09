package com

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

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

	data, err := Deserialize(string(data))
	if err != nil {
		return nil, fmt.Errorf("deserialization of received data failed: %w", err)
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
