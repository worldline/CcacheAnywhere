package backend

import (
	"encoding/binary"
	"fmt"

	"ccache-backend-client/internal/constants"
	//lint:ignore ST1001 do want pretty LOG function
	. "ccache-backend-client/internal/logger"
	"ccache-backend-client/internal/tlv"
)

type Response struct {
	message []byte
	status  StatusCode
}

// Message interface defines the methods for messages
type Message interface {
	Write(b Backend) error
	Read() ([]byte, StatusCode)
	Create(*tlv.Message) error
	RespType() uint16
}

type TestMessage struct {
	mid      string
	response Response
}

type SetupMessage struct {
	mid      string
	response Response
}

type GetMessage struct {
	key      []byte
	mid      string
	response Response
}

type PutMessage struct {
	key           []byte
	value         []byte
	onlyIfMissing bool
	mid           string
	response      Response
}

type RmMessage struct {
	key      []byte
	mid      string
	response Response
}

func (m *TestMessage) RespType() uint16 {
	return 0
}

func (m *TestMessage) Create(body *tlv.Message) error {
	m.mid = "Test message"
	return nil
}

func (m *TestMessage) Write(b Backend) error {
	if b != nil {
		LOG("Backend running successfully!")
	}

	m.response.message = []byte{0, 1, 2, 3, 4, 5, 0, 0, 0}
	m.response.status = SUCCESS
	return nil
}

func (m *TestMessage) Read() ([]byte, StatusCode) {
	return m.response.message, m.response.status
}

func (m *SetupMessage) RespType() uint16 {
	return constants.MsgTypeSetupReponse
}

func (m *SetupMessage) Create(body *tlv.Message) error {
	// Parse them
	// SetupTypeVersion check if we can do this
	field := body.FindField(uint8(constants.SetupTypeVersion))
	if field != nil {
		value := binary.LittleEndian.Uint16(field.Data)
		if value != 0x01 {
			m.response.message = append(m.response.message, 0x01)
			m.response.status = REDIRECT
		}
	}
	// SetupTypeConnectTimeout configure the local timeout
	field = body.FindField(uint8(constants.SetupTypeConnectTimeout))
	if field != nil {
		m.response.message = append(m.response.message, 0x01)
		m.response.status = REDIRECT
	}
	// SetupTypeOperationTimeout configure this too
	field = body.FindField(uint8(constants.SetupTypeOperationTimeout))
	if field != nil {
		m.response.message = append(m.response.message, 0x01)
		m.response.status = REDIRECT
	}
	m.mid = "Setup message"
	return nil
}

func (m *SetupMessage) Write(b Backend) error {
	if m.response.status == REDIRECT {
		return fmt.Errorf("request change in configuration")
	}
	m.response.status = SUCCESS
	return nil
}

func (m *SetupMessage) Read() ([]byte, StatusCode) {
	return m.response.message, m.response.status
}

func (m *GetMessage) RespType() uint16 {
	return constants.MsgTypeGetResponse
}

func (m *GetMessage) Create(body *tlv.Message) error {
	m.mid = "Get Message"
	m.key = body.FindField(constants.TypeKey).Data
	return nil
}

func (m *GetMessage) Write(b Backend) (err error) {
	_resp, err := b.Get(m.key)
	if err != nil {
		if bf, ok := err.(*BackendFailure); ok {
			m.response.status = b.ResolveProtocolCode(bf.Code)
		}
	} else {
		m.response.status = SUCCESS
	}

	m.response.message = _resp
	return err
}

func (m *GetMessage) Read() ([]byte, StatusCode) {
	if len(m.response.message) == 0 {
		m.response.message = []byte("No data found!")
	}

	return m.response.message, m.response.status
}

func (m *PutMessage) RespType() uint16 {
	return constants.MsgTypePutResponse
}

func (m *PutMessage) Create(body *tlv.Message) error {
	m.mid = "Put Message"
	m.key = body.FindField(constants.TypeKey).Data
	m.value = body.FindField(constants.TypeValue).Data

	flags := body.FindField(constants.TypeFlags).Data[0]
	m.onlyIfMissing = flags&constants.OverwriteFlag == 0x0
	return nil
}

func (m *PutMessage) Write(b Backend) (err error) {
	_resp, err := b.Put(m.key, m.value, m.onlyIfMissing)
	if err != nil {
		if bf, ok := err.(*BackendFailure); ok {
			m.response.status = b.ResolveProtocolCode(bf.Code)
		}
	} else {
		m.response.status = SUCCESS
	}

	if _resp {
		m.response.message = []byte{0x01}
	} else {
		m.response.message = []byte{0x00}
	}
	return err
}

func (m *PutMessage) Read() ([]byte, StatusCode) {
	return m.response.message, m.response.status
}

func (m *RmMessage) RespType() uint16 {
	return constants.MsgTypeDeleteResponse
}

func (m *RmMessage) Create(body *tlv.Message) error {
	m.mid = "Remove Message"
	m.key = body.FindField(constants.TypeKey).Data
	return nil
}

func (m *RmMessage) Write(b Backend) (err error) {
	_resp, err := b.Remove(m.key)
	if err != nil {
		if bf, ok := err.(*BackendFailure); ok {
			m.response.status = b.ResolveProtocolCode(bf.Code)
		}
	} else {
		m.response.status = SUCCESS
	}

	if _resp {
		m.response.message = []byte{0x01}
	} else {
		m.response.message = []byte{0x00}
	}
	return err
}

func (m *RmMessage) Read() ([]byte, StatusCode) {
	return m.response.message, m.response.status
}

func Assemble(p tlv.Message) (Message, error) {
	var resultMessage Message
	switch p.Type { // TODO create the messages
	case constants.MsgTypeGet:
		resultMessage = &GetMessage{}
	case constants.MsgTypePut:
		resultMessage = &PutMessage{}
	case constants.MsgTypeDelete:
		resultMessage = &RmMessage{}
	case constants.MsgTypeSetup:
		resultMessage = &SetupMessage{}
	default:
		return nil, fmt.Errorf("message type is not protocol coherent")
	}

	resultMessage.Create(&p)
	return resultMessage, nil
}
