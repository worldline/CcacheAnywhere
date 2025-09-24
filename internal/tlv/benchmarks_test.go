package tlv

import (
	"ccache-backend-client/internal/constants"
	"crypto/rand"
	"testing"
)

const LARGE_BUFFER = 1048575

type SimpleRandomReader struct{}

func NewSimpleRandomReader() *SimpleRandomReader {
	return &SimpleRandomReader{}
}

func (r *SimpleRandomReader) Read(p []byte) (n int, err error) {
	return rand.Read(p)
}

func BenchmarkSerializerPool(b *testing.B) {
	b.ResetTimer()
	token := make([]byte, LARGE_BUFFER)
	for i := 0; i < b.N; i++ {
		s := GetSerializer()
		s.BeginMessage(1, 1, 1)
		s.AddUint8Field(1, 42)
		rand.Read(token)
		s.AddField(constants.TypeValue, token)
		PutSerializer(s)
	}
}

func BenchmarkNoCopySerializerPool(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := GetSerializer()
		s.BeginMessage(1, 1, 1)
		s.AddUint8Field(1, 42)
		reader := NewSimpleRandomReader()
		s.AddFieldFromReader(constants.TypeValue, reader, LARGE_BUFFER)
		PutSerializer(s)
	}
}

func BenchmarkEncodeLength(b *testing.B) {
	buf := make([]byte, 8)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encodeLength(buf, uint64(i%1000))
	}
}

// Benchmark tests
func BenchmarkParser_Parse(b *testing.B) {
	parser := NewParser()

	// Create test data with multiple fields
	testFields := []testField{
		{tag: 1, data: []byte("field one data")},
		{tag: 2, data: []byte("field two data with more content")},
		{tag: 3, data: make([]byte, LARGE_BUFFER)},
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
