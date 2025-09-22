package backend

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"

	"ccache-backend-client/internal/constants"
	"ccache-backend-client/internal/tlv"
)

type Response struct {
	status StatusCode
	_done  bool
}

// Message interface defines the methods for messages
type Message interface {
	Create(*tlv.Message) error
	ReadStatus() StatusCode
	RespType() uint16
	WriteToBackend(b Backend) error
	WriteToSocket(conn net.Conn, s *tlv.Serializer) error
}

type SetupMessage struct {
	mid      string
	fields   []tlv.UintField
	response Response
}

type GetMessage struct {
	key      []byte
	mid      string
	data     io.ReadCloser
	dataSize int64
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
	m.response.status = SUCCESS
	field := body.FindField(constants.SetupTagVersion)
	if field != nil && false {
		value := binary.LittleEndian.Uint16(field.Data)
		if value != 0x01 {
			m.fields = append(m.fields, tlv.NewUintField(constants.SetupTagVersion, uint8(0x01)))
			m.response.status = REDIRECT
		}
	}
	// SetupTypeConnectTimeout configure the local timeout
	field = body.FindField(constants.SetupTagBufferSize)
	if field != nil && false {
		m.fields = append(m.fields, tlv.NewUintField(constants.SetupTagBufferSize, uint32(1024)))
		m.response.status = REDIRECT
	}
	// SetupTypeOperationTimeout configure this too
	field = body.FindField(constants.SetupTagOperationTimeout)
	if field != nil && false {
		m.fields = append(m.fields, tlv.NewUintField(constants.SetupTagOperationTimeout, uint32(1500)))
		m.response.status = REDIRECT
	}
	m.mid = "Setup message"
	return nil
}

func (m *SetupMessage) WriteToSocket(conn net.Conn, s *tlv.Serializer) error {
	s.BeginMessage(0x01, uint8(len(m.fields))+1, constants.MsgTypeSetupReponse)
	if m.ReadStatus() == REDIRECT {
		// TODO add in the setup data
	} else {
		s.AddUint8Field(constants.TypeStatusCode, uint8(m.ReadStatus()))
	}

	conn.Write(s.Bytes())
	s.Reset()
	return nil
}

func (m *SetupMessage) WriteToBackend(b Backend) error {
	return nil
}

func (m *SetupMessage) ReadStatus() StatusCode {
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

func (m *GetMessage) WriteToBackend(b Backend) (err error) {
	m.data, m.dataSize, err = b.Get(m.key)
	if err != nil {
		if bf, ok := err.(*BackendFailure); ok {
			m.response.status = b.ResolveProtocolCode(bf.Code)
		}
	} else {
		m.response.status = SUCCESS
	}

	return err
}

func (m *GetMessage) WriteToSocket(conn net.Conn, s *tlv.Serializer) error {
	s.BeginMessage(0x01, 2, constants.MsgTypeGetResponse)
	s.AddUint8Field(constants.TypeStatusCode, uint8(m.ReadStatus()))
	if m.ReadStatus() == SUCCESS {
		s.Finalize(conn, m.data, uint64(m.dataSize))
	}

	s.Reset()
	return nil
}

func (m *GetMessage) ReadStatus() StatusCode {
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

func (m *PutMessage) WriteToSocket(conn net.Conn, s *tlv.Serializer) error {
	s.BeginMessage(0x01, 1, constants.MsgTypeGetResponse)
	s.AddUint8Field(constants.TypeStatusCode, uint8(m.ReadStatus()))
	conn.Write(s.Bytes())
	s.Reset()
	return nil
}

func (m *PutMessage) WriteToBackend(b Backend) (err error) {
	_resp, err := b.Put(m.key, m.value, m.onlyIfMissing)
	if err != nil {
		if bf, ok := err.(*BackendFailure); ok {
			m.response.status = b.ResolveProtocolCode(bf.Code)
		}
	} else {
		m.response.status = SUCCESS
	}

	m.response._done = _resp
	return err
}

func (m *PutMessage) ReadStatus() StatusCode {
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

func (m *RmMessage) WriteToSocket(conn net.Conn, s *tlv.Serializer) error {
	s.BeginMessage(0x01, 1, constants.MsgTypeGetResponse)
	s.AddUint8Field(constants.TypeStatusCode, uint8(m.ReadStatus()))
	conn.Write(s.Bytes())
	s.Reset()
	return nil
}

func (m *RmMessage) WriteToBackend(b Backend) (err error) {
	_resp, err := b.Remove(m.key)
	if err != nil {
		if bf, ok := err.(*BackendFailure); ok {
			m.response.status = b.ResolveProtocolCode(bf.Code)
		}
	} else {
		m.response.status = SUCCESS
	}

	m.response._done = _resp
	return err
}

func (m *RmMessage) ReadStatus() StatusCode {
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
