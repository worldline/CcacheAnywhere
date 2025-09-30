package backend

import (
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
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
	Get(key []byte) (io.ReadCloser, int64, error)
	Put([]byte, []byte, bool) (bool, error)
	Remove([]byte) (bool, error)
	ResolveProtocolCode(int) StatusCode
}

var BackendAttributes []Attribute

func formatDigest(data []byte) (string, error) {
	const base16Bytes = 2

	if len(data) < base16Bytes {
		return "", fmt.Errorf("data size must be at least %d bytes", base16Bytes)
	}

	base16Part := hex.EncodeToString(data[:base16Bytes])
	base32Part := strings.ToLower(base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString(data[base16Bytes:]))

	return base16Part + base32Part, nil
}

func parseTimeout(value string) time.Duration {
	var timeout time.Duration
	fmt.Sscanf(value, "%d", &timeout)
	return time.Duration(timeout.Seconds())
}

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
