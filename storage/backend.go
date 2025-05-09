package backend

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
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
	Create(string, Attribute) error
	Handle(Message) (Message, error)
	Get([]byte) (string, error)
	Put([]byte, io.Reader, bool) (bool, error)
	Remove([]byte) (bool, error)
}

func runclient() {
	c := http.Client{Timeout: time.Duration(1) * time.Second}
	resp, err := c.Get("https://www.google.com")
	if err != nil {
		fmt.Printf("Error %s", err)
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)

	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Body : %s", body)
}
