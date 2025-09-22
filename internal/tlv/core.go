package tlv

import (
	"ccache-backend-client/internal/constants"
	"encoding/binary"
	"fmt"
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

type MessageHeader struct {
	Version   uint8
	NumFields uint8
	MsgType   uint16
}

type Message struct {
	Type   uint16
	Fields []TLVField
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

func (fld *TLVField) String() string {
	num := min(len(fld.Data), 20)
	send := max(len(fld.Data)-20, 20)
	if send > len(fld.Data) {
		return fmt.Sprintf("FLD{Tag: %v, Len: %v, Data: %v}", fld.Tag, fld.Length, fld.Data[:num])
	}
	return fmt.Sprintf("FLD{Tag: %v, Len: %v, Data: %v..%v}", fld.Tag, fld.Length, fld.Data[:num], fld.Data[send:])
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

// lengthEncodingSize returns how many bytes are needed to encode a length
func lengthEncodingSize(length uint64) int {
	if length <= uint64(constants.Length1ByteMax) {
		return 1
	} else if length <= 0xFFFF {
		return 3
	} else if length <= 0xFFFFFFFF {
		return 5
	} else {
		return 9
	}
}
