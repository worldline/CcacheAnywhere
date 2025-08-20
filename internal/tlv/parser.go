package tlv

import (
	"ccache-backend-client/internal/constants"
	"encoding/binary"
	"fmt"
)

func NewParser() *Parser {
	return &Parser{
		fields: make([]TLVField, 0, 4), // Pre-allocate for common case
	}
}

// Parse parses a TLV message from the given buffer
// Uses zero-copy approach - returned fields reference original buffer
func (p *Parser) Parse(data []byte) (*Message, error) {
	if len(data) < constants.TLVHeaderSize {
		return nil, constants.ErrInvalidMessage
	}

	p.fields = p.fields[:0]

	// version := binary.LittleEndian.Uint16(data[0:2])
	msgType := binary.LittleEndian.Uint16(data[2:4])
	pos := 4

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

// FindField finds the first field with the given type
func (m *Message) FindField(fieldTag uint8) *TLVField {
	for i := range m.Fields {
		if m.Fields[i].Tag == fieldTag {
			return &m.Fields[i]
		}
	}
	return nil
}

// GetString extracts a string value from a field
func (f *TLVField) GetString() string {
	return string(f.Data)
}

// GetBool extracts a boolean value from a field
func (f *TLVField) GetBool() bool {
	return len(f.Data) > 0 && f.Data[0] != 0
}

// GetUint32 extracts a uint32 value from a field
func (f *TLVField) GetUint32() uint32 {
	if len(f.Data) < 4 {
		return 0
	}
	return binary.LittleEndian.Uint32(f.Data)
}

// GetBytes returns the raw bytes of the field
func (f *TLVField) GetBytes() []byte {
	return f.Data
}
