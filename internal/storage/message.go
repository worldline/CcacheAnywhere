package backend

import (
	"encoding/binary"
	"fmt"

	"ccache-backend-client/internal/constants"
	"ccache-backend-client/internal/tlv"
)

type Response struct {
	status StatusCode
}

// Message interface defines the methods for messages
type Message interface {
	Write(b Backend, s *tlv.Serializer) error
	Read() StatusCode
	Create(*tlv.Message) error
	RespType() uint16
}

type SetupMessage struct {
	mid      string
	fields   []byte
	response Response
}

type GetMessage struct {
	key      []byte
	mid      string
	Ser      tlv.Serializer
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
			m.fields = append(m.fields, uint8(constants.SetupTypeVersion))
			m.fields = append(m.fields, 0x01)
			m.response.status = REDIRECT
		}
	}
	// SetupTypeConnectTimeout configure the local timeout
	field = body.FindField(uint8(constants.SetupTypeBufferSize))
	if field != nil {
		m.fields = append(m.fields, uint8(constants.SetupTypeBufferSize))
		m.fields = append(m.fields, 0x01)
		m.response.status = REDIRECT
	}
	// SetupTypeOperationTimeout configure this too
	field = body.FindField(uint8(constants.SetupTypeOperationTimeout))
	if field != nil {
		m.fields = append(m.fields, uint8(constants.SetupTypeOperationTimeout))
		m.fields = append(m.fields, 0x01)
		m.response.status = REDIRECT
	}
	m.mid = "Setup message"
	return nil
}

func (m *SetupMessage) Write(b Backend, s *tlv.Serializer) error {
	if m.response.status == REDIRECT {
		for i := 0; i < len(m.fields); i += 2 {
			s.AddUint8Field(m.fields[i], m.fields[i+1])
		}
		m.response.status = LOCAL_ERR
		return fmt.Errorf("request change in configuration")
	}
	m.response.status = SUCCESS
	return nil
}

func (m *SetupMessage) Read() StatusCode {
	return m.response.status
}

func (m *GetMessage) RespType() uint16 {
	return constants.MsgTypeGetResponse
}

func (m *GetMessage) Create(body *tlv.Message) error {
	m.mid = "Get Message"
	m.key = body.FindField(constants.TypeKey).Data
	return nil
}

func (m *GetMessage) Write(b Backend, s *tlv.Serializer) (err error) {
	err = b.Get(m.key, s)
	if err != nil {
		if bf, ok := err.(*BackendFailure); ok {
			m.response.status = b.ResolveProtocolCode(bf.Code)
		}
	} else {
		m.response.status = SUCCESS
	}

	return err
}

func (m *GetMessage) Read() StatusCode {
	return m.response.status
}

func (m *PutMessage) RespType() uint16 {
	return constants.MsgTypePutResponse
}

func (m *PutMessage) Create(body *tlv.Message) error {
	m.mid = "Put Message"
	m.key = body.FindField(constants.TypeKey).Data
	m.value = body.FindField(constants.TypeValue).Data

	flagsField := body.FindField(constants.TypeFlags)
	if flagsField != nil {
		m.onlyIfMissing = flagsField.Data[0]&constants.OverwriteFlag == 0x0
	} else {
		m.onlyIfMissing = false
	}

	return nil
}

func (m *PutMessage) Write(b Backend, s *tlv.Serializer) (err error) {
	_resp, err := b.Put(m.key, m.value, m.onlyIfMissing)
	if err != nil {
		if bf, ok := err.(*BackendFailure); ok {
			m.response.status = b.ResolveProtocolCode(bf.Code)
		}
	} else {
		m.response.status = SUCCESS
	}

	s.AddBoolField(constants.TypeValue, _resp)
	return err
}

func (m *PutMessage) Read() StatusCode {
	return m.response.status
}

func (m *RmMessage) RespType() uint16 {
	return constants.MsgTypeDeleteResponse
}

func (m *RmMessage) Create(body *tlv.Message) error {
	m.mid = "Remove Message"
	m.key = body.FindField(constants.TypeKey).Data
	return nil
}

func (m *RmMessage) Write(b Backend, s *tlv.Serializer) (err error) {
	_resp, err := b.Remove(m.key)
	if err != nil {
		if bf, ok := err.(*BackendFailure); ok {
			m.response.status = b.ResolveProtocolCode(bf.Code)
		}
	} else {
		m.response.status = SUCCESS
	}

	s.AddBoolField(constants.TypeValue, _resp)
	return err
}

func (m *RmMessage) Read() StatusCode {
	return m.response.status
}

func Assemble(p *tlv.Message) (Message, error) {
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

	resultMessage.Create(p)
	return resultMessage, nil
}
