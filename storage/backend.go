package backend

import (
	"fmt"
)

var RedactedPassword = "********"

type Attribute struct {
	RawValue string
	Value    string
	Key      string
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
