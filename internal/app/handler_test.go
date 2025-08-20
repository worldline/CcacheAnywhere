package app

import (
	"errors"
	"testing"

	"ccache-backend-client/internal/constants"
	storage "ccache-backend-client/internal/storage"
	"ccache-backend-client/internal/tlv"
)

// Helper function to find field by type in parsed packet
func getFieldByType(fields []tlv.TLVField, fieldType uint8) *tlv.TLVField {
	for _, field := range fields {
		if field.Tag == fieldType {
			return &field
		}
	}
	return nil
}

// Mock implementations for testing
type mockBackend struct {
	getCalled    bool
	putCalled    bool
	removeCalled bool
	getError     error
	putError     error
	removeError  error
}

func (m *mockBackend) Get(key []byte, serializer *tlv.Serializer) error {
	// Simulate Get call adding data to serializer
	m.getCalled = true
	if m.getError != nil {
		return m.getError
	}
	return serializer.AddField(constants.TypeValue, []byte("mock data"))
}

func (m *mockBackend) Put(key []byte, data []byte, onlyIfMissing bool) (bool, error) {
	m.putCalled = true
	return true, m.putError
}

func (m *mockBackend) Remove(key []byte) (bool, error) {
	m.removeCalled = true
	return true, m.removeError
}

func (m *mockBackend) ResolveProtocolCode(code int) storage.StatusCode {
	return storage.SUCCESS
}

type mockMessage struct {
	writeError   error
	status       storage.StatusCode
	respType     uint16
	writeCalled  bool
	createCalled bool
	readCalled   bool
}

func (m *mockMessage) Create(*tlv.Message) error {
	m.createCalled = true
	return nil
}

func (m *mockMessage) Write(backend storage.Backend, serializer *tlv.Serializer) error {
	m.writeCalled = true
	return m.writeError
}

func (m *mockMessage) Read() storage.StatusCode {
	m.readCalled = true
	return m.status
}

func (m *mockMessage) RespType() uint16 {
	return m.respType
}

// Test NewBackendHandler
func TestNewBackendHandler(t *testing.T) {
	tests := []struct {
		name   string
		url    string
		isErr  bool
		errMsg string
	}{
		{
			name:  "valid http URL",
			url:   "http://example.com",
			isErr: false,
		},
		{
			name:  "valid gs URL",
			url:   "gs://bucket-name",
			isErr: false,
		},
		{
			name:   "invalid scheme",
			url:    "ftp://example.com",
			isErr:  true,
			errMsg: "backend not implemented for prefix: ftp",
		},
		{
			name:   "malformed URL",
			url:    "not-a-url",
			isErr:  true,
			errMsg: "backend not implemented for prefix: not-a-url",
		},
		{
			name:  "empty URL",
			url:   "",
			isErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, err := NewBackendHandler(tt.url)

			if tt.isErr {
				if err == nil {
					t.Errorf("NewBackendHandler() expected error, got nil")
				}
				if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("NewBackendHandler() error = %v, want %v", err.Error(), tt.errMsg)
				}
				if handler != nil {
					t.Errorf("NewBackendHandler() expected nil handler on error, got %v", handler)
				}
			} else {
				if err != nil {
					t.Errorf("NewBackendHandler() unexpected error = %v", err)
				}
				if handler == nil {
					t.Errorf("NewBackendHandler() expected valid handler, got nil")
				}
				if handler != nil && handler.node == nil {
					t.Errorf("NewBackendHandler() handler.node is nil")
				}
			}
		})
	}
}

// Test Handle method
func TestBackendHandler_Handle(t *testing.T) {
	tests := []struct {
		name        string
		writeError  error
		status      storage.StatusCode
		respType    uint16
		expectWrite bool
		expectRead  bool
	}{
		{
			name:        "successful handle",
			writeError:  nil,
			status:      storage.SUCCESS,
			respType:    constants.MsgTypeGetResponse,
			expectWrite: true,
			expectRead:  true,
		},
		{
			name:        "handle with write error",
			writeError:  errors.New("write failed"),
			status:      storage.ERROR,
			respType:    constants.MsgTypePutResponse,
			expectWrite: true,
			expectRead:  true,
		},
		{
			name:        "handle with error status",
			writeError:  nil,
			status:      storage.ERROR,
			respType:    constants.MsgTypeDeleteResponse,
			expectWrite: true,
			expectRead:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock backend and message
			mockBackend := &mockBackend{}
			mockMsg := &mockMessage{
				writeError: tt.writeError,
				status:     tt.status,
				respType:   tt.respType,
			}

			// Create handler with mock backend
			handler := &BackendHandler{
				node:       mockBackend,
				serializer: *tlv.GetSerializer(),
			}

			// Call Handle
			handler.Handle(mockMsg)

			// Verify message methods were called
			if mockMsg.writeCalled != tt.expectWrite {
				t.Errorf("Handle() Write called = %v, want %v", mockMsg.writeCalled, tt.expectWrite)
			}

			if mockMsg.readCalled != tt.expectRead {
				t.Errorf("Handle() Read called = %v, want %v", mockMsg.readCalled, tt.expectRead)
			}

			// Verify serializer has data
			if handler.serializer.Len() == 0 {
				t.Error("Handle() serializer should contain data after handling")
			}

			// Verify serializer contains the status code field
			data := handler.serializer.Bytes()
			if len(data) < 6 { // At least header (4 bytes) + status field (2+ bytes)
				t.Errorf("Handle() serializer data too short: %d bytes", len(data))
			}
		})
	}
}

// Test Handle method integration with real serializer
func TestBackendHandler_HandleIntegration(t *testing.T) {
	// Create a mock backend that will be called
	mockBackend := &mockBackend{}

	// Create a mock message that simulates a GET request
	mockMsg := &mockMessage{
		writeError: nil,
		status:     storage.SUCCESS,
		respType:   constants.MsgTypeGetResponse,
	}

	// Create handler
	handler := &BackendHandler{
		node:       mockBackend,
		serializer: *tlv.GetSerializer(),
	}

	// Handle the message
	handler.Handle(mockMsg)

	// Verify the serializer contains expected structure
	// Should have at least: version(2) + msgtype(2) + status_field_header + status_value
	data := handler.serializer.Bytes()
	if len(data) < 6 {
		t.Errorf("Expected at least 6 bytes in serialized data, got %d", len(data))
	}

	// Parse the message to verify structure
	parser := tlv.NewParser()
	packet, err := parser.Parse(data)
	if err != nil {
		t.Fatalf("Failed to parse serialized data: %v", err)
	}

	// Verify message type
	if packet.Type != constants.MsgTypeGetResponse {
		t.Errorf("Expected message type %d, got %d", constants.MsgTypeGetResponse, packet.Type)
	}

	// Verify status code field exists
	statusField := getFieldByType(packet.Fields, constants.TypeStatusCode)
	if statusField == nil {
		t.Error("Status code field not found in serialized message")
	} else if len(statusField.Data) != 1 {
		t.Errorf("Expected status code field to have 1 byte, got %d", len(statusField.Data))
	} else if statusField.Data[0] != uint8(storage.SUCCESS) {
		t.Errorf("Expected status code %d, got %d", uint8(storage.SUCCESS), statusField.Data[0])
	}
}
