package backend

import (
	// "ccache-backend-client/storage"
	"fmt"
)

// Message interface defines the methods for messages
type Message interface {
	Write(b Backend) error
	Read() []byte
	Create([]byte) error
}

type TestMessage struct {
	mc string
}

func (m TestMessage) Create(body []byte) error {
	return nil
}

func (m TestMessage) Write(b Backend) error {
	return m.writeImpl(b)
}

func (m TestMessage) Read() []byte {
	return m.readImpl()
}

func (m TestMessage) writeImpl(any interface{}) error {
	return fmt.Errorf("writing TestMessage to backend")
}

func (m TestMessage) readImpl() []byte {
	return []byte{}
}

type SetupMessage struct {
	mc string
}

func (m SetupMessage) Create(body []byte) error {
	return nil
}

func (m SetupMessage) Write(b Backend) error {
	return m.writeImpl(b)
}

func (m SetupMessage) Read() []byte {
	return m.readImpl()
}

func (m SetupMessage) writeImpl(b Backend) error {
	return fmt.Errorf("writing SetupMessage to backend")
}

func (m SetupMessage) readImpl() []byte {
	return []byte{}
}

type GetMessage struct {
	key []byte
	mc  string
}

func (m *GetMessage) Create(body []byte) error {
	m.mc = "Get Message"
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
		return fmt.Errorf("writing GetMessage to backend")
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
	mc            string
}

func (m *PutMessage) Create(body []byte) error {
	m.mc = "Put Message"
	m.key = body[:20]
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
	mc  string
}

func (m *RmMessage) Create(body []byte) error {
	m.mc = "Remove Message"
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
