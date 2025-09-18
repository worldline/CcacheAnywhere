package tlv

import (
	"ccache-backend-client/internal/constants"
	"testing"
)

func TestNewSerializer(t *testing.T) {
	s := NewSerializer(1024)
	if s.pos != 0 {
		t.Errorf("Expected pos to be 0, got %d", s.pos)
	}
	if cap(s.buffer) != 1024 {
		t.Errorf("Expected capacity 1024, got %d", cap(s.buffer))
	}
}

func TestAddUint8Field(t *testing.T) {
	s := NewSerializer(1024)
	s.BeginMessage(1, 2, constants.MsgTypeGetResponse)

	err := s.AddUint8Field(1, 42)
	if err != nil {
		t.Fatalf("AddUint8Field failed: %v", err)
	}

	data := s.Bytes()
	if len(data) < 6 {
		t.Errorf("Expected at least 6 bytes, got %d", len(data))
	}
}

// Table-driven test
func TestAddFieldInternal(t *testing.T) {
	tests := []struct {
		name       string
		tag        uint8
		data       []byte
		expectsErr bool
	}{
		{"normal case", 1, []byte("hello"), false},
		{"empty data", 2, []byte{}, false},
		{"too large", 4, make([]byte, constants.MaxFieldSize+1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSerializer(int(constants.MaxFieldSize))
			s.BeginMessage(1, 1, 1)

			err := s.addFieldInternal(tt.tag, tt.data)
			if (err != nil) != tt.expectsErr {
				t.Errorf("addFieldInternal() error = %v, wantErr %v", err, tt.expectsErr)
			}
		})
	}
}
