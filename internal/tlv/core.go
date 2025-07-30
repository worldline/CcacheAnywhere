package tlv

import (
	"ccache-backend-client/internal/constants"
	"encoding/binary"
)

var (
	SOCKET_PATH    string
	FIXED_BUF_SIZE int
	BACKEND_TYPE   string
)

type TLVField struct {
	Tag    uint8
	Length uint32
	Data   []byte // Slice pointing to original buffer
}

type Message struct {
	Type   uint16
	Fields []TLVField
}

type Parser struct {
	fields []TLVField // Reused slice to avoid allocations
}

type Serializer struct {
	buffer []byte
	pos    int
}

// encodeLength encodes a length using NDN variable-length encoding
func encodeLength(buf []byte, length uint32) int {
	if length <= uint32(constants.Length1ByteMax) {
		buf[0] = uint8(length)
		return 1
	} else if length <= 0xFFFF {
		buf[0] = constants.Length3ByteFlag
		binary.LittleEndian.PutUint16(buf[1:], uint16(length))
		return 3
	} else {
		buf[0] = constants.Length5ByteFlag
		binary.LittleEndian.PutUint32(buf[1:], length)
		return 5
	}
}

// decodeLength decodes NDN variable-length encoding
// Returns (length, bytesConsumed, error)
func decodeLength(buf []byte) (uint32, int, error) {
	if len(buf) < 1 {
		return 0, 0, constants.ErrTruncatedData
	}

	firstByte := buf[0]

	if firstByte <= constants.Length1ByteMax {
		return uint32(firstByte), 1, nil
	} else if firstByte == constants.Length3ByteFlag {
		if len(buf) < 3 {
			return 0, 0, constants.ErrTruncatedData
		}
		length := binary.LittleEndian.Uint16(buf[1:3])
		return uint32(length), 3, nil
	} else if firstByte == constants.Length5ByteFlag {
		if len(buf) < 5 {
			return 0, 0, constants.ErrTruncatedData
		}
		length := binary.LittleEndian.Uint32(buf[1:5])
		return length, 5, nil
	}

	return 0, 0, constants.ErrInvalidLength
}

// lengthEncodingSize returns how many bytes are needed to encode a length
func lengthEncodingSize(length uint32) int {
	if length <= uint32(constants.Length1ByteMax) {
		return 1
	} else if length <= 0xFFFF {
		return 3
	} else {
		return 5
	}
}
