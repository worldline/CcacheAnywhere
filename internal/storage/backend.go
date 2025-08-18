package backend

import (
	"ccache-backend-client/internal/tlv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type StatusCode uint8

const (
	LOCAL_ERR = iota
	NO_FILE
	TIMEOUT
	SIGWAIT
	SUCCESS
	REDIRECT
	ERROR
)

type BackendFailure struct {
	Message string
	Code    int
}

type Attribute struct {
	RawValue string
	Value    string
	Key      string
}

type Backend interface {
	Get(key []byte, serializer *tlv.Serializer) error
	Put([]byte, []byte, bool) (bool, error)
	Remove([]byte) (bool, error)
	ResolveProtocolCode(int) StatusCode
}

var BackendAttributes []Attribute

// ParseAttributes reads a JSON configuration file and extracts attributes into a slice.
//
// It loads the specified file from the "configs" directory, parses its JSON content,
// and appends the key-value pairs as `Attribute` entries into the global `BackendAttributes` slice.
//
// Note:
//   - This function modifies the global variable `BackendAttributes` by appending new entries.
//   - Ensure that concurrent calls are synchronized if necessary, as `BackendAttributes` is shared.
func ParseAttributes(filename string) ([]Attribute, error) {
	filePath := filepath.Join("configs", filename)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %v", filePath, err)
	}

	var attributesMap map[string]any
	if err := json.Unmarshal(data, &attributesMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON data: %v", err)
	}

	for key, value := range attributesMap {
		BackendAttributes = append(BackendAttributes, Attribute{Key: key, Value: fmt.Sprintf("%v", value)})
	}

	return BackendAttributes, nil
}

// Error returns a string formatting of the backend failure.
func (e *BackendFailure) Error() string {
	return fmt.Sprintf("Failure: %s with status code %d", e.Message, e.Code)
}
