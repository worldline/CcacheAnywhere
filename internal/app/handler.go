package app

import (
	"ccache-backend-client/internal/constants"
	"fmt"
	"net/url"
	"strings"

	//lint:ignore ST1001 do want pretty LOG function
	. "ccache-backend-client/internal/logger"
	storage "ccache-backend-client/internal/storage"
	"ccache-backend-client/internal/tlv"
)

type Handler interface {
	Handle(storage.Message)
}

type BackendHandler struct {
	node       storage.Backend
	serializer tlv.Serializer
}

// The URL's prefix (scheme) determines which backend implementation to instantiate.
//
// Supported schemes:
//   - "http": Creates an HTTP backend.
//   - "gs": Creates a Google Cloud Storage (GCS) backend.
func NewBackendHandler(url string) (*BackendHandler, error) {
	prefix := strings.Split(url, ":")[0]
	furl, _ := parseUrl(url)
	switch prefix {
	case "http":
		return &BackendHandler{
			node:       storage.GetHttpBackend(furl, storage.BackendAttributes),
			serializer: *tlv.NewSerializer(int(constants.MaxFieldSize))}, nil
	case "gs":
		return &BackendHandler{
			node:       storage.NewGCSBackend(furl, storage.BackendAttributes),
			serializer: *tlv.NewSerializer(int(constants.MaxFieldSize))}, nil
	default:
		return nil, fmt.Errorf("backend not implemented for prefix: %s", prefix)
	}
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

// Propagate message receoved to the backend server
func (h *BackendHandler) Handle(msg storage.Message) {
	h.serializer.BeginMessage(uint16(0x01), msg.RespType())
	err := msg.Write(h.node, &h.serializer)
	status := msg.Read()
	h.serializer.AddUint8Field(constants.TypeStatusCode, uint8(status))

	if err != nil {
		LOG("Handling message failed for backend: %v", err.Error())
	}
}
