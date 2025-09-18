package backend

import (
	"bytes"
	"ccache-backend-client/internal/tlv"
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

	serializer := tlv.NewSerializer(1024)
	_, _, err := backend.Get([]byte{0x01, 0x02})

	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}

	if serializer.Len() == 0 {
		t.Error("Expected serializer to contain data")
	}

	if !bytes.Contains(serializer.Bytes(), []byte("test data")) {
		t.Error("Serializer should contain received data!")
	}
}
