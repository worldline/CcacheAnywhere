package tlv

import (
	"bytes"
	"encoding/binary"
	"testing"

	"ccache-backend-client/internal/constants"
)

// Helper function to create test TLV data
func createTestTLVData(msgType uint16, fields []testField) []byte {
	buf := make([]byte, 4)
	pos := 0

	// Write Header (4 bytes)
	binary.LittleEndian.PutUint16(buf, 0x01)
	binary.LittleEndian.PutUint16(buf[2:], msgType)
	pos += 4

	// Write fields
	for _, field := range fields {
		buf = append(buf, field.tag)
		pos++
		lengthEncSize := lengthEncodingSize(uint32(len(field.data)))
		tmp := make([]byte, lengthEncSize)
		pos += encodeLength(tmp, uint32(len(field.data)))

		buf = append(buf, tmp...)
		buf = append(buf, field.data...)
	}

	return buf
}

type testField struct {
	tag  uint8
	data []byte
}

func TestNewParser(t *testing.T) {
	parser := NewParser()

	if parser == nil {
		t.Fatal("NewParser() returned nil")
	}

	if parser.fields == nil {
		t.Error("Parser fields not initialized")
	}

	if cap(parser.fields) != 4 {
		t.Errorf("Expected fields capacity 4, got %d", cap(parser.fields))
	}

	if len(parser.fields) != 0 {
		t.Errorf("Expected fields length 0, got %d", len(parser.fields))
	}
}

func TestParser_ParseValidMessage(t *testing.T) {
	parser := NewParser()

	// Create test data with multiple fields
	testFields := []testField{
		{tag: 1, data: []byte("hello")},
		{tag: 2, data: []byte{0x42}},
		{tag: 3, data: []byte{0x01, 0x02, 0x03, 0x04}},
		{tag: 3, data: make([]byte, 100000)},
	}

	data := createTestTLVData(constants.MsgTypeGetResponse, testFields)

	msg, err := parser.Parse(data)
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	if msg == nil {
		t.Fatal("Parse() returned nil message")
	}

	if msg.Type != constants.MsgTypeGetResponse {
		t.Errorf("Expected message type %d, got %d", constants.MsgTypeGetResponse, msg.Type)
	}

	if len(msg.Fields) != 4 {
		t.Errorf("Expected 4 fields, got %d", len(msg.Fields))
	}

	// Check each field
	for i, expectedField := range testFields {
		if i >= len(msg.Fields) {
			t.Errorf("Missing field %d", i)
			continue
		}

		field := msg.Fields[i]
		if field.Tag != expectedField.tag {
			t.Errorf("Field %d: expected tag %d, got %d", i, expectedField.tag, field.Tag)
		}

		if field.Length != uint32(len(expectedField.data)) {
			t.Errorf("Field %d: expected length %d, got %d", i, len(expectedField.data), field.Length)
		}

		if !bytes.Equal(field.Data, expectedField.data) {
			t.Errorf("Field %d: expected data %v, got %v", i, expectedField.data, field.Data)
		}
	}
}

func TestParser_ParseEmptyMessage(t *testing.T) {
	parser := NewParser()

	// Create message with no fields
	data := createTestTLVData(constants.MsgTypeGetResponse, []testField{})

	msg, err := parser.Parse(data)
	if err != nil {
		t.Fatalf("Parse() failed for empty message: %v", err)
	}

	if len(msg.Fields) != 0 {
		t.Errorf("Expected 0 fields, got %d", len(msg.Fields))
	}
}

func TestParser_ParseInvalidData(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name string
		data []byte
		want error
	}{
		{
			name: "empty data",
			data: []byte{},
			want: constants.ErrInvalidMessage,
		},
		{
			name: "too short for header",
			data: []byte{0x01},
			want: constants.ErrInvalidMessage,
		},
		{
			name: "header only",
			data: []byte{0x01, 0x00, 0x02, 0x00}, // version + msgtype
			want: nil,                            // Should succeed with no fields
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := parser.Parse(tt.data)

			if tt.want != nil {
				if err == nil {
					t.Errorf("Parse() expected error %v, got nil", tt.want)
				} else if err != tt.want {
					t.Errorf("Parse() error = %v, want %v", err, tt.want)
				}
			} else {
				if err != nil {
					t.Errorf("Parse() unexpected error: %v", err)
				}
				if msg == nil {
					t.Error("Parse() returned nil message when expecting success")
				}
			}
		})
	}
}

func TestParser_ParseTruncatedData(t *testing.T) {
	parser := NewParser()

	// Create valid data first
	testFields := []testField{
		{tag: 1, data: []byte("hello world")},
	}

	validData := createTestTLVData(constants.MsgTypeGetResponse, testFields)

	// Truncate the data (remove last few bytes)
	truncatedData := validData[:len(validData)-5]

	_, err := parser.Parse(truncatedData)
	if err == nil {
		t.Error("Parse() should fail on truncated data")
	}

	if err != constants.ErrTruncatedData {
		t.Errorf("Parse() error = %v, want %v", err, constants.ErrTruncatedData)
	}
}

func TestParser_ZeroCopyBehavior(t *testing.T) {
	parser := NewParser()

	testData := []byte("test data for zero copy")
	testFields := []testField{
		{tag: 1, data: testData},
	}

	data := createTestTLVData(constants.MsgTypeGetResponse, testFields)

	msg, err := parser.Parse(data)
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	if len(msg.Fields) == 0 {
		t.Fatal("No fields parsed")
	}

	// Check that field data points to original buffer (zero-copy)
	fieldData := msg.Fields[0].Data

	// Find where our test data should be in the original buffer
	dataStart := -1
	for i := 0; i <= len(data)-len(testData); i++ {
		if bytes.Equal(data[i:i+len(testData)], testData) {
			dataStart = i
			break
		}
	}

	if dataStart == -1 {
		t.Fatal("Test data not found in original buffer")
	}

	// Verify zero-copy: field data should point to the same memory
	expectedPtr := &data[dataStart]
	actualPtr := &fieldData[0]

	if expectedPtr != actualPtr {
		t.Error("Field data is not zero-copy (different memory addresses)")
	}

	// Modify original data and verify field data changes (proving zero-copy)
	originalByte := data[dataStart]
	data[dataStart] = 0xFF

	if fieldData[0] != 0xFF {
		t.Error("Field data didn't change when original buffer changed (not zero-copy)")
	}

	// Restore original data
	data[dataStart] = originalByte
}

func TestMessage_FindField(t *testing.T) {
	parser := NewParser()

	testFields := []testField{
		{tag: 1, data: []byte("first")},
		{tag: 2, data: []byte("second")},
		{tag: 3, data: []byte("third")},
		{tag: 2, data: []byte("duplicate tag")}, // Duplicate tag
	}

	data := createTestTLVData(constants.MsgTypeGetResponse, testFields)

	msg, err := parser.Parse(data)
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	// Test finding existing fields
	field1 := msg.FindField(1)
	if field1 == nil {
		t.Error("FindField(1) returned nil")
	} else if !bytes.Equal(field1.Data, []byte("first")) {
		t.Errorf("FindField(1) data = %v, want %v", field1.Data, []byte("first"))
	}

	// Test finding duplicate tag (should return first occurrence)
	field2 := msg.FindField(2)
	if field2 == nil {
		t.Error("FindField(2) returned nil")
	} else if !bytes.Equal(field2.Data, []byte("second")) {
		t.Errorf("FindField(2) data = %v, want %v", field2.Data, []byte("second"))
	}

	// Test finding non-existent field
	fieldNotFound := msg.FindField(99)
	if fieldNotFound != nil {
		t.Error("FindField(99) should return nil for non-existent field")
	}
}

func TestTLVField_GetString(t *testing.T) {
	field := &TLVField{
		Tag:    1,
		Length: 5,
		Data:   []byte("hello"),
	}

	result := field.GetString()
	if result != "hello" {
		t.Errorf("GetString() = %q, want %q", result, "hello")
	}

	// Test empty string
	emptyField := &TLVField{Data: []byte{}}
	if emptyField.GetString() != "" {
		t.Error("GetString() should return empty string for empty data")
	}

	// Test string with null bytes
	nullField := &TLVField{Data: []byte{'h', 'e', 0, 'l', 'o'}}
	result = nullField.GetString()
	expected := "he\x00lo"
	if result != expected {
		t.Errorf("GetString() = %q, want %q", result, expected)
	}
}

func TestTLVField_GetBool(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"true - non-zero byte", []byte{1}, true},
		{"true - multiple bytes", []byte{1, 2, 3}, true},
		{"true - 255", []byte{255}, true},
		{"false - zero byte", []byte{0}, false},
		{"false - zero with more bytes", []byte{0, 1, 2}, false},
		{"false - empty data", []byte{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := &TLVField{Data: tt.data}
			if got := field.GetBool(); got != tt.want {
				t.Errorf("GetBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTLVField_GetUint32(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want uint32
	}{
		{
			name: "valid uint32",
			data: []byte{0x01, 0x02, 0x03, 0x04}, // Little endian: 0x04030201
			want: 0x04030201,
		},
		{
			name: "zero value",
			data: []byte{0x00, 0x00, 0x00, 0x00},
			want: 0,
		},
		{
			name: "max uint32",
			data: []byte{0xFF, 0xFF, 0xFF, 0xFF},
			want: 0xFFFFFFFF,
		},
		{
			name: "too short data",
			data: []byte{0x01, 0x02},
			want: 0,
		},
		{
			name: "empty data",
			data: []byte{},
			want: 0,
		},
		{
			name: "longer data (should use first 4 bytes)",
			data: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
			want: 0x04030201,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := &TLVField{Data: tt.data}
			if got := field.GetUint32(); got != tt.want {
				t.Errorf("GetUint32() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTLVField_GetBytes(t *testing.T) {
	testData := []byte{1, 2, 3, 4, 5}
	field := &TLVField{Data: testData}

	result := field.GetBytes()

	if !bytes.Equal(result, testData) {
		t.Errorf("GetBytes() = %v, want %v", result, testData)
	}

	// Verify it's the same slice (not a copy)
	if &result[0] != &testData[0] {
		t.Error("GetBytes() should return the same slice, not a copy")
	}

	// Test empty data
	emptyField := &TLVField{Data: []byte{}}
	emptyResult := emptyField.GetBytes()
	if len(emptyResult) != 0 {
		t.Errorf("GetBytes() for empty data = %v, want empty slice", emptyResult)
	}
}

func TestParser_ParseMultipleMessages(t *testing.T) {
	parser := NewParser()

	// Parse first message
	fields1 := []testField{{tag: 1, data: []byte("first message")}}
	data1 := createTestTLVData(constants.MsgTypeGetResponse, fields1)

	msg1, err := parser.Parse(data1)
	if err != nil {
		t.Fatalf("First Parse() failed: %v", err)
	}

	// Parse second message (should reuse parser)
	fields2 := []testField{{tag: 2, data: []byte("second message")}}
	data2 := createTestTLVData(constants.MsgTypePutResponse, fields2)

	msg2, err := parser.Parse(data2)
	if err != nil {
		t.Fatalf("Second Parse() failed: %v", err)
	}

	// Verify message headers are independent
	if msg1.Type == msg2.Type {
		t.Error("Messages should have different types")
	}

	if len(msg1.Fields) != 1 || len(msg2.Fields) != 1 {
		t.Error("Each message should have exactly 1 field")
	}

	// This is currently the desired behaviour. Parser returns a pointer to fields not a copy!
	if !bytes.Equal(msg1.Fields[0].Data, msg2.Fields[0].Data) {
		t.Error("Messages should point to the same data")
	}
}

// Benchmark tests
func BenchmarkParser_Parse(b *testing.B) {
	parser := NewParser()

	// Create test data with multiple fields
	testFields := []testField{
		{tag: 1, data: []byte("field one data")},
		{tag: 2, data: []byte("field two data with more content")},
		{tag: 3, data: make([]byte, constants.MaxFieldSize)}, // Larger field
	}

	data := createTestTLVData(constants.MsgTypeGetResponse, testFields)

	b.ResetTimer()
	for range b.N {
		_, err := parser.Parse(data)
		if err != nil {
			b.Fatalf("Parse failed: %v", err)
		}
	}
}

func BenchmarkMessage_FindField(b *testing.B) {
	parser := NewParser()

	// Create message with many fields
	var testFields []testField
	for i := range 20 {
		testFields = append(testFields, testField{
			tag:  uint8(i),
			data: []byte("field data"),
		})
	}

	data := createTestTLVData(constants.MsgTypeGetResponse, testFields)
	msg, _ := parser.Parse(data)

	b.ResetTimer()
	for range b.N {
		// Find field in the middle
		msg.FindField(10)
	}
}
