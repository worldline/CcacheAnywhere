package com

import (
	"encoding/hex"
	"fmt"
)

func Serialize(data any) []byte {
	switch any(data).(type) {
	case uint8:
		return fmt.Appendf(nil, "%02X", data)
	case uint16:
		return fmt.Appendf(nil, "%04X", data)
	case uint32:
		return fmt.Appendf(nil, "%08X", data)
	case uint64:
		return fmt.Appendf(nil, "%016X", data)
	default:
		panic("unsupported type")
	}
}

func Deserialize(data any) ([]byte, error) {
	switch v := data.(type) {
	case string:
		return DeserializeString(v)
	case []byte:
		return DeserializeBytes(v)
	default:
		return nil, fmt.Errorf("deserialize invoked with unsupported type '%T", v)
	}
}

func DeserializeString(data string) ([]byte, error) {
	bytes, err := hex.DecodeString(data)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

func DeserializeBytes(data []byte) ([]byte, error) {
	if len(data)%2 != 0 {
		return nil, fmt.Errorf("input byte slice must have an even length")
	}

	decodedBytes := make([]byte, len(data)/2)
	for i := 0; i < len(data); i += 2 {
		// Parse two bytes (one hex byte)
		byteValue, err := hex.DecodeString(string(data[i : i+2]))
		if err != nil {
			return nil, err // Return nil and the error if decoding fails
		}
		decodedBytes[i/2] = byteValue[0] // Append to decoded bytes
	}

	return decodedBytes, nil // Return the decoded bytes
}
