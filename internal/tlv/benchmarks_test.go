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
		s.BeginMessage(1, 1)
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
		s.BeginMessage(1, 1)
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
		encodeLength(buf, uint32(i%1000))
	}
}
