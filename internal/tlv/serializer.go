package tlv

import (
	"ccache-backend-client/internal/constants"
	"encoding/binary"
)

// creates a new TLV-protocol serializer with the given capacity
func NewSerializer(capacity int) *Serializer {
	return &Serializer{
		buffer: make([]byte, capacity),
		pos:    0,
	}
}

// resets the serializer for reuse
//
// sets pointer to the beginning of the buffer
func (s *Serializer) Reset() {
	s.pos = 0
}

// BeginMessage starts a new message with the given type
func (s *Serializer) BeginMessage(version uint16, msgType uint16) error {
	if len(s.buffer) < 1 {
		return constants.ErrFieldTooLarge
	}

	binary.LittleEndian.PutUint16(s.buffer, version)
	binary.LittleEndian.PutUint16(s.buffer, msgType)
	s.pos += 4
	return nil
}

// AddField adds a field with raw byte data
func (s *Serializer) AddField(fieldTag uint8, data []byte) error {
	return s.addFieldInternal(fieldTag, data)
}

// AddStringField adds a string field
func (s *Serializer) AddStringField(fieldTag uint8, value string) error {
	return s.addFieldInternal(fieldTag, []byte(value))
}

// AddBoolField adds a boolean field
func (s *Serializer) AddBoolField(fieldTag uint8, value bool) error {
	data := []byte{0}
	if value {
		data[0] = 1
	}
	return s.addFieldInternal(fieldTag, data)
}

// AddUint32Field adds a uint32 field
func (s *Serializer) AddUint32Field(fieldTag uint8, value uint32) error {
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, value)
	return s.addFieldInternal(fieldTag, data)
}

// addFieldInternal handles the actual field serialization
func (s *Serializer) addFieldInternal(fieldTag uint8, data []byte) error {
	dataLen := uint32(len(data))

	if dataLen > constants.MaxFieldSize {
		return constants.ErrFieldTooLarge
	}

	// Calculate space needed: 2 bytes type + variable length + data
	lengthEncSize := lengthEncodingSize(dataLen)
	needed := 2 + lengthEncSize + len(data)

	if s.pos+needed > len(s.buffer) {
		return constants.ErrFieldTooLarge
	}

	// Write field type (2 bytes, big endian)
	s.buffer[s.pos] = fieldTag
	s.pos += 1

	// Write variable length
	s.pos += encodeLength(s.buffer[s.pos:], dataLen)

	// Write data
	copy(s.buffer[s.pos:], data)
	s.pos += len(data)

	return nil
}

// Bytes returns the serialized message as a byte slice
func (s *Serializer) Bytes() []byte {
	return s.buffer[:s.pos]
}

// Len returns the current message length
func (s *Serializer) Len() int {
	return s.pos
}
