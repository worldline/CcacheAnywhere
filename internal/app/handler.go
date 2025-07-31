package app

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"ccache-backend-client/internal/constants"
	"ccache-backend-client/internal/logger"
	storage "ccache-backend-client/internal/storage"
	"ccache-backend-client/internal/tlv"
)

func CreateSocketHandler(conn *net.Conn) SocketHandler {
	return SocketHandler{node: *conn, serializer: tlv.NewSerializer(int(constants.MaxFieldSize))}
}

// Format of inputted url http://secret-key@domainname.com/path/to/folder|attribute=value
//
// Attributes might contain one or multiple key=value pairs separated by '|'
func parseUrl(input string) (*url.URL, []storage.Attribute) {
	parts := strings.Split(input, "|")
	var attributes []storage.Attribute

	parsedUrl, err := url.Parse(parts[0])
	if err != nil {
		return nil, nil
	}

	for _, attrStr := range parts[1:] {
		if attrStr != "" {
			attrs := strings.Split(attrStr, "=")
			if len(attrs) == 2 {
				attributes = append(attributes, storage.Attribute{Key: attrs[0], Value: attrs[1]})
			}
		}
	}

	return parsedUrl, attributes
}

func CreateBackend(url string) (*BackendHandler, error) {
	prefix := strings.Split(url, ":")[0]
	furl, _ := parseUrl(url)
	switch prefix {
	case "http":
		return &BackendHandler{node: storage.CreateHTTPBackend(furl, storage.BackendAttributes)}, nil
	case "gs":
		return &BackendHandler{node: storage.CreateGCSBackend(furl, storage.BackendAttributes)}, nil
	default:
		return nil, fmt.Errorf("backend not implemented for prefix: %s", prefix)
	}
}

type Handler interface {
	Handle(storage.Message)
}

type SocketHandler struct {
	node       net.Conn
	serializer *tlv.Serializer
}

// deparse message and send it over network
func (h *SocketHandler) Handle(msg storage.Message) {
	data, status := msg.Read()
	msgType := msg.RespType()

	h.serializer.BeginMessage(uint16(0x01), uint16(msgType))
	h.serializer.AddUint8Field(constants.TypeStatusCode, uint8(status))
	switch msgType {
	case constants.MsgTypeGetResponse:
		h.serializer.AddField(constants.TypeValue, data)
	case constants.MsgTypePutResponse:
		// do we want to say we put?
	case constants.MsgTypeDeleteResponse:
		// do we want to say we deleted?
	case constants.MsgTypeSetupReponse:
		// if there is something to configure send it
	}

	h.node.Write(h.serializer.Bytes())
	h.serializer.Reset()
}

type BackendHandler struct {
	node storage.Backend
}

func (h *BackendHandler) Handle(msg storage.Message) {
	err := msg.Write(h.node)

	if err != nil {
		logger.LOG("Handling message failed for backend: %v", err.Error())
	}
}
