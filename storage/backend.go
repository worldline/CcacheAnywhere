package backend

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

var RedactedPassword = "********"

type Attribute struct {
	RawValue string
	Value    string
	Key      string
}

func ParseAttributes(filename string) ([]Attribute, error) {
	filePath := filepath.Join("./configs", filename)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %v", filePath, err)
	}

	var attributesMap map[string]any
	if err := json.Unmarshal(data, &attributesMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON data: %v", err)
	}

	var attributes []Attribute
	for key, value := range attributesMap {
		attributes = append(attributes, Attribute{Key: key, Value: fmt.Sprintf("%v", value)})
	}

	return attributes, nil
}

type BackendFailure struct {
	Message string
	Code    int
}

func (e *BackendFailure) Error() string {
	return fmt.Sprintf("backend failure: %s with status code %d", e.Message, e.Code)
}

type Backend interface {
	Get([]byte) (string, error)
	Put([]byte, []byte, bool) (bool, error)
	Remove([]byte) (bool, error)
}
