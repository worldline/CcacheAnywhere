package app

import (
	"fmt"
	"net/url"
	"strings"

	//lint:ignore ST1001 do want pretty LOG function
	. "ccache-backend-client/internal/logger"
	storage "ccache-backend-client/internal/storage"
)

type Handler interface {
	Handle(storage.Message)
}

type BackendHandler struct {
	node storage.Backend
}

// The URL's prefix (scheme) determines which backend implementation to instantiate.
//
// Supported schemes:
//   - "http": Creates an HTTP backend.
//   - "gs": Creates a Google Cloud Storage (GCS) backend.
func NewBackendHandler(storage_url string) (*BackendHandler, error) {
	prefix := strings.Split(storage_url, ":")[0]

	furl, err := url.Parse(storage_url)
	if err != nil {
		return nil, err
	}

	switch prefix {
	case "http":
		return &BackendHandler{
			node: storage.GetHttpBackend(furl, storage.BackendAttributes)}, nil
	case "gs":
		return &BackendHandler{
			node: storage.GetGCSBackend(furl, storage.BackendAttributes)}, nil
	default:
		return nil, fmt.Errorf("backend not implemented for prefix: %s", prefix)
	}
}

// Propagate message receoved to the backend server
func (h *BackendHandler) Handle(msg storage.Message) {
	err := msg.WriteToBackend(h.node)

	if err != nil {
		LOG("Handling message failed for backend: %v", err.Error())
	}
}
