package tlv

import (
	"ccache-backend-client/internal/constants"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
)

var parserPool = sync.Pool{
	New: func() any {
		return NewParser()
	},
}

func GetParser() *Parser {
	return parserPool.Get().(*Parser)
}

func PutParser(s *Parser) {
	s.Cleanup()
	parserPool.Put(s)
}

type ParseResult struct {
	ok            bool
	ParsedMessage *Message
	ValueReader   io.Reader
}

func NewParser() *Parser {
	return &Parser{
		fields: make([]TLVField, 0, 4), // Pre-allocate for common case
	}
}

type Parser struct {
	result ParseResult
	fields []TLVField // Reused slice to avoid allocations
}

// decodeLength decodes NDN variable-length encoding
// Returns (length, bytesConsumed, error)
func decodeLength(buf []byte) (uint64, int, error) {
	if len(buf) < 1 {
		return 0, 0, constants.ErrTruncatedData
	}

	firstByte := buf[0]

	if firstByte <= constants.Length1ByteMax {
		return uint64(firstByte), 1, nil
	} else if firstByte == constants.Length3ByteFlag {
		if len(buf) < 3 {
			return 0, 0, constants.ErrTruncatedData
		}
		length := binary.LittleEndian.Uint16(buf[1:3])
		return uint64(length), 3, nil
	} else if firstByte == constants.Length5ByteFlag {
		if len(buf) < 5 {
			return 0, 0, constants.ErrTruncatedData
		}
		length := binary.LittleEndian.Uint32(buf[1:5])
		return uint64(length), 5, nil
	} else if firstByte == constants.Length9ByteFlag {
		if len(buf) < 9 {
			return 0, 0, constants.ErrTruncatedData
		}
		length := binary.LittleEndian.Uint64(buf[1:5])
		return length, 5, nil // TODO change length to uint64
	}

	return 0, 0, constants.ErrInvalidLength
}

func (p *Parser) Cleanup() {
	p.result.ok = false
	p.result.ParsedMessage = nil
	p.result.ValueReader = nil
	p.fields = p.fields[:0]
}

// Parse parses a TLV message from the given buffer
// Uses zero-copy approach - returned fields reference original buffer
func (p *Parser) Parse(data []byte) (*Message, error) {
	if len(data) < constants.TLVHeaderSize {
		return nil, constants.ErrInvalidMessage
	}
	pos := 0
	p.fields = p.fields[:0]

	// version := binary.LittleEndian.Uint16(data[0:2])
	msgType := binary.LittleEndian.Uint16(data[2:4])
	pos = 4

	for pos < len(data) {
		fieldType := data[pos]
		pos += 1

		length, lengthBytes, err := decodeLength(data[pos:])
		if err != nil {
			return nil, fmt.Errorf("failed to decode length: %w", err)
		}
		pos += lengthBytes

		if uint64(pos)+uint64(length) > uint64(len(data)) {
			return nil, constants.ErrTruncatedData
		}

		field := TLVField{
			Tag:    fieldType,
			Length: length,
			Data:   data[pos : pos+int(length)], // Zero-copy
		}

		p.fields = append(p.fields, field)
		pos += int(length)
	}

	return &Message{
		Type:   msgType,
		Fields: p.fields,
	}, nil
}
