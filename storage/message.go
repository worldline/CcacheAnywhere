package backend

import (
	// "ccache-backend-client/storage"
	"fmt"
	"strconv"
)

// Message interface defines the methods for messages
type Message interface {
	Write(b Backend) error
	Read() []byte
	Create([]byte) error
}

type TestMessage struct {
	mid string
}

func (m *TestMessage) Create(body []byte) error {
	m.mid = "Test message"
	return nil
}

func (m *TestMessage) Write(b Backend) error {
	return m.writeImpl(b)
}

func (m *TestMessage) Read() []byte {
	return m.readImpl()
}

func (m *TestMessage) writeImpl(b Backend) error {
	return fmt.Errorf("writing TestMessage to backend")
}

func (m *TestMessage) readImpl() []byte {
	return []byte{}
}

type SetupMessage struct {
	mid string
}

func (m *SetupMessage) Create(body []byte) error {
	m.mid = "Setup message"
	return nil
}

func (m *SetupMessage) Write(b Backend) error {
	return m.writeImpl(b)
}

func (m *SetupMessage) Read() []byte {
	return m.readImpl()
}

func (m *SetupMessage) writeImpl(b Backend) error {
	return fmt.Errorf("writing SetupMessage to backend")
}

func (m *SetupMessage) readImpl() []byte {
	return []byte{}
}

type GetMessage struct {
	key []byte
	mid string
}

func (m *GetMessage) Create(body []byte) error {
	m.mid = "Get Message"
	if len(body) < 20 {
		return fmt.Errorf("key should be at least of length 20")
	}
	m.key = body[:20]
	return nil
}

func (m *GetMessage) Write(b Backend) error {
	return m.writeImpl(b)
}

func (m *GetMessage) Read() []byte {
	return m.readImpl()
}

func (m *GetMessage) writeImpl(b Backend) error {
	_, err := b.Get(m.key)
	if err != nil {
		return fmt.Errorf("writing GetMessage to backend: %w", err)
	}

	return nil
}

func (m *GetMessage) readImpl() []byte {
	return []byte{}
}

type PutMessage struct {
	key           []byte
	value         []byte
	onlyIfMissing bool
	mid           string
}

func (m *PutMessage) Create(body []byte) error {
	m.mid = "Put Message"
	if len(body) < 20 {
		return fmt.Errorf("key should be at least of length 20")
	}
	m.key = body[:20]
	m.value = body[20 : len(body)-1]
	tmp, err := strconv.ParseBool(string(body[len(body)-1]))
	if err != nil {
		return err
	}
	m.onlyIfMissing = tmp
	return nil
}

func (m *PutMessage) Write(b Backend) error {
	return m.writeImpl(b)
}

func (m *PutMessage) Read() []byte {
	return m.readImpl()
}

func (m *PutMessage) writeImpl(b Backend) error {
	_, err := b.Put(m.key, m.value, m.onlyIfMissing)
	if err != nil {
		return fmt.Errorf("writing PutMessage to backend")
	}

	return nil
}

func (m *PutMessage) readImpl() []byte {
	return []byte{}
}

type RmMessage struct {
	key []byte
	mid string
}

func (m *RmMessage) Create(body []byte) error {
	m.mid = "Remove Message"
	if len(body) < 20 {
		return fmt.Errorf("key should be at least of length 20")
	}
	m.key = body[:20]
	return nil
}

func (m *RmMessage) Write(b Backend) error {
	return m.writeImpl(b)
}

func (m *RmMessage) Read() []byte {
	return m.readImpl()
}

func (m *RmMessage) writeImpl(b Backend) error {
	_, err := b.Remove(m.key)
	if err != nil {
		return fmt.Errorf("writing RmMessage to backend")
	}

	return nil
}

func (m *RmMessage) readImpl() []byte {
	return []byte{}
}

// func TEST1() {
// 	data := []byte{0x01, 0x00, 0x02, 0x03, 0x04, 0x05, 0x00, 0x06, 0x00, 0x00, 0x00, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x1}
// 	packet, err := ParsePacket(data)
// 	if err != nil {
// 		fmt.Println("Error parsing packet:", err)
// 		return
// 	}
// 	if packet.MsgLength != len(packet.Body) {
//  	panic("failed with parsing")
// 	}
// }
//
// func TEST2() {
// 	originalData := []byte{0x01, 0x2A, 0x57} // Sample byte slice
// 	serialized := Serialize(originalData)
// 	fmt.Println("Serialized:", serialized)

// 	deserialized, err := Deserialize(serialized)
// 	if err != nil {
// 		fmt.Println("Error deserializing:", err)
// 		return
// 	}
// 	fmt.Println("Deserialized:", deserialized)
//
//  if (deserialised != originalData) {
//		panic ("failed with serialisation and deserialisation")
//	}
// }
