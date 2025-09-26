package tlv

import (
	"encoding/binary"
	"fmt"
)

// TLV uint field interface
type UintField interface {
	GetTag() uint8
	GetLength() uint8
	Serialize() []byte
	String() string
}

// Generic TLV field implementation
type UintFieldRef[T any] struct {
	Tag    uint8
	Length uint8
	Data   T
}

func (f *UintFieldRef[T]) GetTag() uint8 {
	return f.Tag
}

func (f *UintFieldRef[T]) GetLength() uint8 {
	return f.Length
}

func (f *UintFieldRef[T]) String() string {
	return fmt.Sprintf("TLV{Tag:%d, Length:%d, Data:%v}", f.Tag, f.Length, f.Data)
}

// Single Serialize method using type switching just serializes f.Data
func (f *UintFieldRef[T]) Serialize() []byte {
	buf := make([]byte, 0)

	// Type switch on the actual data
	switch data := any(f.Data).(type) {
	case uint8:
		buf = append(buf, data)
	case uint16:
		temp := make([]byte, 2)
		binary.BigEndian.PutUint16(temp, data)
		buf = append(buf, temp...)
	case uint32:
		temp := make([]byte, 4)
		binary.BigEndian.PutUint32(temp, data)
		buf = append(buf, temp...)
	case string:
		buf = append(buf, []byte(data)...)
	default:
		// Handle other types or panic
		panic(fmt.Sprintf("unsupported type: %T", data))
	}

	return buf
}

// Get field with type assertion
func GetFieldByTag(fields []UintField, tag uint8) UintField {
	for _, field := range fields {
		if field.GetTag() == tag {
			return field
		}
	}
	return nil
}

// Factory functions
func NewUintField[T any](tag uint8, data T) UintField {
	var length uint8
	switch any(data).(type) {
	case uint8:
		length = 1
	case uint16:
		length = 2
	case uint32:
		length = 4
	default:
		return nil
	}

	return &UintFieldRef[T]{
		Tag:    tag,
		Length: length,
		Data:   data,
	}
}
