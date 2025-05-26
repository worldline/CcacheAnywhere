package backend

import (
	// "ccache-backend-client/storage"
	"fmt"
	"strconv"
)

type Response struct {
	message string
	status  StatusCode
}

// Message interface defines the methods for messages
type Message interface {
	Write(b Backend) error
	Read() ([]byte, StatusCode)
	Create([]byte) error
	Type() uint8
}

type TestMessage struct {
	mid      string
	response Response
}

func (m *TestMessage) Type() uint8 {
	return 4
}

func (m *TestMessage) Create(body []byte) error {
	m.mid = "Test message"
	return nil
}

func (m *TestMessage) Write(b Backend) error {
	if b != nil {
		fmt.Println("Backend running successfully!")
	}

	// insert trivial string!
	m.response.message = "012345000"
	m.response.status = SUCCESS
	return nil
}

func (m *TestMessage) Read() ([]byte, StatusCode) {
	return []byte(m.response.message), m.response.status
}

type SetupMessage struct {
	mid      string
	response Response
}

func (m *SetupMessage) Type() uint8 {
	return 0
}

func (m *SetupMessage) Create(body []byte) error {
	m.mid = "Setup message"
	return nil
}

func (m *SetupMessage) Write(b Backend) error {
	return fmt.Errorf("writing SetupMessage to backend")
}

func (m *SetupMessage) Read() ([]byte, StatusCode) {
	return []byte(m.response.message), m.response.status
}

type GetMessage struct {
	key      []byte
	mid      string
	response Response
}

func (m *GetMessage) Type() uint8 {
	return 1
}

func (m *GetMessage) Create(body []byte) error {
	m.mid = "Get Message"
	if len(body) < 20 {
		return fmt.Errorf("key should be at least of length 20")
	}
	m.key = body[:20]
	return nil
}

func (m *GetMessage) Write(b Backend) (err error) {
	_resp, err := b.Get(m.key)
	if err != nil {
		if bf, ok := err.(*BackendFailure); ok {
			m.response.status = b.ResolveProtocolCode(bf.Code)
		}
	}

	m.response.message = _resp
	return err
}

func (m *GetMessage) Read() ([]byte, StatusCode) {
	if len(m.response.message) == 0 {
		m.response.message = "No data found!"
	}

	return []byte(m.response.message), m.response.status
}

type PutMessage struct {
	key           []byte
	value         []byte
	onlyIfMissing bool
	mid           string
	response      Response
}

func (m *PutMessage) Type() uint8 {
	return 2
}

func (m *PutMessage) Create(body []byte) error {
	m.mid = "Put Message"
	if len(body) < 20 {
		return fmt.Errorf("key should be at least of length 20")
	}
	m.key = body[:20]
	m.value = body[20 : len(body)-1]
	m.onlyIfMissing = int(body[len(body)-1]) != 0
	return nil
}

func (m *PutMessage) Write(b Backend) (err error) {
	_resp, err := b.Put(m.key, m.value, m.onlyIfMissing)
	if err != nil {
		if bf, ok := err.(*BackendFailure); ok {
			m.response.status = b.ResolveProtocolCode(bf.Code)
		}
	}

	m.response.message = strconv.FormatBool(_resp)
	return err
}

func (m *PutMessage) Read() ([]byte, StatusCode) {
	return []byte(m.response.message), m.response.status
}

type RmMessage struct {
	key      []byte
	mid      string
	response Response
}

func (m *RmMessage) Type() uint8 {
	return 3
}

func (m *RmMessage) Create(body []byte) error {
	m.mid = "Remove Message"
	if len(body) < 20 {
		return fmt.Errorf("key should be at least of length 20")
	}
	m.key = body[:20]
	return nil
}

func (m *RmMessage) Write(b Backend) (err error) {
	_resp, err := b.Remove(m.key)
	if err != nil {
		if bf, ok := err.(*BackendFailure); ok {
			m.response.status = b.ResolveProtocolCode(bf.Code)
		}
	}

	m.response.message = strconv.FormatBool(_resp)
	return err
}

func (m *RmMessage) Read() ([]byte, StatusCode) {
	return []byte(m.response.message), m.response.status
}
