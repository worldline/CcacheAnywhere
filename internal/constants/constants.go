package constants

import (
	"errors"
	"time"
)

const (
	INACTIVITY_TIMEOUT   = 300 * time.Second
	MAX_PARALLEL_CLIENTS = 128
)

// Message types
const (
	MsgTypeSetup          uint16 = 0x01
	MsgTypeGet            uint16 = 0x02
	MsgTypePut            uint16 = 0x03
	MsgTypeDelete         uint16 = 0x04
	MsgTypeSetupReponse   uint16 = 0x8001
	MsgTypeGetResponse    uint16 = 0x8002
	MsgTypePutResponse    uint16 = 0x8003
	MsgTypeDeleteResponse uint16 = 0x8004
)

// Field types
const (
	SetupTypeVersion          uint16 = 0x01
	SetupTypeConnectTimeout   uint16 = 0x02
	SetupTypeOperationTimeout uint16 = 0x03
)

const (
	TypeKey        uint8 = 0x081
	TypeValue      uint8 = 0x082
	TypeTimetamp   uint8 = 0x083
	TypeStatusCode uint8 = 0x084
	TypeErrorMsg   uint8 = 0x085
	TypeFlags      uint8 = 0x086
)

// Flags
const OverwriteFlag uint8 = 0x01

// Status codes
const (
	LOCAL_ERROR uint8 = 0x00
	NO_FILE     uint8 = 0x01
	TIMEOUT     uint8 = 0x02
	SIGWAIT     uint8 = 0x03
	SUCCESS     uint8 = 0x04
	REDIRECT    uint8 = 0x05
	ERROR       uint8 = 0x06
)

// NDN Length encoding constants
const (
	Length1ByteMax  uint8  = 252 // 0xFC
	Length3ByteFlag uint8  = 253 // 0xFD
	Length5ByteFlag uint8  = 254 // 0xFE
	TLVHeaderSize   int    = 4
	MaxFieldSize    uint32 = 0xFFFFF
)

// Errors
var (
	ErrInvalidLength  = errors.New("invalid length encoding")
	ErrTruncatedData  = errors.New("truncated data")
	ErrInvalidMessage = errors.New("invalid message format")
	ErrFieldTooLarge  = errors.New("field too large")
)

var DEBUG_ENABLED = false
