package backend

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestHttpStorageBackend_Get(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test data"))
	}))
	defer server.Close()

	u, _ := url.Parse(server.URL)
	backend := NewHTTPBackend(u, []Attribute{})

	respBody, respLen, err := backend.Get([]byte{0x01, 0x02})
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}

	if respLen == 0 {
		t.Error("Expected response to contain data")
	}

	buf := make([]byte, 20)
	n, err := io.ReadAtLeast(respBody, buf, int(respLen))

	if err != nil || respLen != int64(n) {
		t.Errorf("Should read correct length: respLen=%d and n=%d", int(respLen), n)
	}

	if !bytes.Contains(buf, []byte("test data")) {
		t.Error("Did not get correct data!")
	}
}
