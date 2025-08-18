package tlv

import (
	"ccache-backend-client/internal/constants"
	"encoding/binary"
	"fmt"
	"io"
)

// creates a new TLV-protocol serializer with the given capacity
func NewSerializer(capacity int) *Serializer {
	return &Serializer{
		buffer: make([]byte, 1024, capacity),
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
	if len(s.buffer) < 4 {
		return constants.ErrFieldTooLarge
	}

	binary.LittleEndian.PutUint16(s.buffer, version)
	binary.LittleEndian.PutUint16(s.buffer[2:], msgType)
	s.pos += 4
	return nil
}

func (s *Serializer) ensureCapacity(minCapacity int) {
	if minCapacity <= cap(s.buffer) {
		// Extend length if needed
		if minCapacity > len(s.buffer) {
			s.buffer = s.buffer[:minCapacity]
		}
		return
	}

	// Reallocate with growth strategy
	newCap := cap(s.buffer) * 2
	if newCap < minCapacity {
		newCap = minCapacity
	}

	newBuffer := make([]byte, len(s.buffer), newCap)
	copy(newBuffer, s.buffer)
	s.buffer = newBuffer

	// Extend to needed length
	if minCapacity > len(s.buffer) {
		s.buffer = s.buffer[:minCapacity]
	}
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

// AddUint8Field adds a uint8 field
func (s *Serializer) AddUint8Field(fieldTag uint8, value uint8) error {
	data := make([]byte, 1)
	data[0] = value
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
	dataLen := uint32(len(data)) // TODO uint64

	if dataLen > constants.MaxFieldSize {
		return constants.ErrFieldTooLarge
	}

	// Calculate space needed: 1 bytes tag + variable length + data
	lengthEncSize := lengthEncodingSize(dataLen)
	needed := 1 + lengthEncSize + len(data)

	// Ensure sufficient space
	s.ensureCapacity(s.pos + needed)

	// Write field type (1 byte, little endian)
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

// AddFieldFromReader adds a field by reading directly from an io.Reader
// This avoids copying the data through an intermediate buffer
func (s *Serializer) AddFieldFromReader(fieldTag uint8, reader io.Reader, contentLength int64) error {
	if contentLength > 0 && contentLength <= int64(constants.MaxFieldSize) {
		return s.addFieldFromReaderWithLength(fieldTag, reader, uint32(contentLength))
	}

	return constants.ErrFieldTooLarge
}

func (s *Serializer) addFieldFromReaderWithLength(fieldTag uint8, reader io.Reader, dataLen uint32) error {
	if dataLen > constants.MaxFieldSize {
		return constants.ErrFieldTooLarge
	}

	lengthEncSize := lengthEncodingSize(dataLen)
	needed := 1 + lengthEncSize + int(dataLen)
	s.ensureCapacity(s.pos + needed)

	s.buffer[s.pos] = fieldTag
	s.pos += 1

	s.pos += encodeLength(s.buffer[s.pos:], dataLen)

	// Read directly into buffer (zero-copy!)
	totalRead := 0
	for totalRead < int(dataLen) {
		n, err := reader.Read(s.buffer[s.pos : s.pos+int(dataLen)-totalRead])
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}
		s.pos += n
		totalRead += n
	}

	if totalRead != int(dataLen) {
		return fmt.Errorf("expected %d bytes, got %d", dataLen, totalRead)
	}

	return nil
}
