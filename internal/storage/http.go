package backend

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	urlib "net/url"
	"strings"
	"sync"
	"time"

	//lint:ignore ST1001 do want nice LOG operations
	. "ccache-backend-client/internal/logger"
)

type HttpStorageBackend struct {
	bearer string
	url    urlib.URL
	client *http.Client
	layout Layout
}

type Layout int

const (
	flat Layout = iota
	bazel
	subdirs
)

type httpHeaders struct {
	headers           map[string]string
	bearerToken       string
	connectionTimeout time.Duration
	operationTimeout  time.Duration
	layout            Layout
}

var (
	httpBackend *HttpStorageBackend
	httpOnce    sync.Once
)

func NewHttpHeaders() *httpHeaders {
	return &httpHeaders{
		headers: make(map[string]string),
	}
}

// TODO define a backendATTributes struct as argument for create
// each create deals with it as it wishes
func NewHTTPBackend(url *urlib.URL, attributes []Attribute) *HttpStorageBackend {
	defaultHeaders := NewHttpHeaders()
	for _, attr := range attributes {
		switch attr.Key {
		case "bearer-token":
			defaultHeaders.bearerToken = attr.Value
		case "connect-timeout":
			defaultHeaders.connectionTimeout = parseTimeout(attr.Value)
		case "keep-alive":
			// TODO
		case "operation-timeout":
			defaultHeaders.operationTimeout = parseTimeout(attr.Value)
		case "layout":
			switch attr.Value {
			case "bazel":
				defaultHeaders.layout = bazel
			case "flat":
				defaultHeaders.layout = flat
			case "subdirs":
				defaultHeaders.layout = subdirs
			default:
				defaultHeaders.layout = flat
			}
		case "header":
			spltres := strings.Split(attr.Value, "=")
			if spltres[len(spltres)-1] != "" {
				defaultHeaders.emplace(attr.Key, attr.Value)
			} else {
				LOG("HTTP header error")
			}
		case "url":
			// TODO
		default:
			LOG("HTTP attribute '%s' not known!", attr.Key)
		}
	}

	transport := &http.Transport{
		// Connection pooling settings
		MaxIdleConns:        100,              // Total idle connections across all hosts
		MaxIdleConnsPerHost: 50,               // Idle connections per host
		MaxConnsPerHost:     100,              // Max concurrent connections per host
		IdleConnTimeout:     90 * time.Second, // How long to keep idle connections

		// Keep-alive settings
		DisableKeepAlives: false, // CRITICAL: Enable keep-alive

		// TCP settings
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second, // Connection timeout
			KeepAlive: 30 * time.Second, // TCP keep-alive
		}).DialContext,

		// HTTP/2 support
		ForceAttemptHTTP2: true, // Use HTTP/2 if available

		// Don't disable compression unless needed
		DisableCompression: false,
	}

	if url.User != nil {
		defaultHeaders.bearerToken = url.User.String()
	}

	httpclient := http.Client{Transport: transport, Timeout: defaultHeaders.connectionTimeout}
	return &HttpStorageBackend{url: *url, client: &httpclient,
		bearer: defaultHeaders.bearerToken, layout: defaultHeaders.layout}
}

func GetHttpBackend(url *urlib.URL, attributes []Attribute) *HttpStorageBackend {
	httpOnce.Do(func() {
		httpBackend = NewHTTPBackend(url, attributes)
	})
	return httpBackend
}

// URL format: http://HOST[:PORT][/PATH]
func getUrl(u *urlib.URL) string {
	if u.Host == "" {
		panic("user provided url is empty!")
	}
	u2 := *u
	u2.RawQuery = ""
	u2.Fragment = ""
	return u2.String()
}

func (h *HttpStorageBackend) getEntryPath(key []byte) string {
	urlPath := getUrl(&h.url)
	switch h.layout {
	case bazel:
		// Mimic hex representation of a SHA256 hash value.
		const sha256HexSize = 64
		hexDigits := hex.EncodeToString(key)

		// Ensure hexDigits has the expected size
		hexDigits += hex.EncodeToString(make([]byte, 12)) // need 24 zeros
		if len(hexDigits) != sha256HexSize {
			panic("This should not happen!")
		}

		return fmt.Sprintf("%s/ac/%s", urlPath, hexDigits)

	case flat:
		hexDigit, err := formatDigest(key)
		if err != nil {
			return ""
		}
		return fmt.Sprintf("%s/%s", urlPath, hexDigit)

	case subdirs:
		keyStr, err := formatDigest(key)
		if err != nil {
			return ""
		}
		digits := 2
		if len(keyStr) <= digits {
			panic("keyStr length is insufficient for subdirectory layout")
		}
		return fmt.Sprintf("%s/%s/%s", urlPath, keyStr[:digits], keyStr[digits:])

	default:
		panic("unknown layout")
	}
}

func (h *httpHeaders) emplace(key string, value string) {
	h.headers[key] = value
}

func (h *HttpStorageBackend) ResolveProtocolCode(code int) StatusCode {
	if code < 100 {
		return LOCAL_ERR
	} else if code == 404 {
		return NO_FILE
	} else if code == 408 {
		return TIMEOUT
	} else if code < 200 {
		return SIGWAIT
	} else if code < 300 {
		return SUCCESS
	} else if code < 400 {
		return REDIRECT
	} else {
		return ERROR
	}
}

// Remove deletes the specified key from the HTTP storage backend.
// It sends an HTTP request to remove the resource associated with the given key.
//
// The errors returned are of type BackendFailure. The Status Code of the http error
// can be translated by the ResolveProtocolCode available in the backend.
//
// key: a byte slice representing the key of the resource to be removed.
//
// Returns:
// - bool: true if the resource was successfully removed; false otherwise.
// - error: an error object if the operation failed due to network issues, server errors, or other issues.
func (h *HttpStorageBackend) Remove(key []byte) (bool, error) {
	urlPath := getUrl(&h.url)
	keyPath := h.getEntryPath(key)
	req, err := http.NewRequest("DELETE", keyPath, bytes.NewReader(key))
	if err != nil {
		return false, &BackendFailure{
			Message: fmt.Sprintf("Failed to create request for %s", urlPath),
			Code:    0}
	}

	if h.bearer != "" {
		encodedCredentials := base64.StdEncoding.EncodeToString([]byte(h.bearer))
		req.Header.Add("Authorization", "Basic "+encodedCredentials)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return false, &BackendFailure{
			Message: fmt.Sprintf("HTTP request failed: %v", err),
			Code:    resp.StatusCode}
	}
	defer resp.Body.Close()

	// Check if the status code indicates a failure
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, &BackendFailure{
			Message: fmt.Sprintf("Failed to delete %s from HTTP storage (%s)", key, resp.Status),
			Code:    resp.StatusCode}
	}
	return true, nil
}

// Get retrieves the value associated with the specified key from the HTTP storage backend.
// It returns the value as a string and an error if the operation fails.
//
// The errors returned are of type BackendFailure. The Status Code of the http error
// can be translated by the ResolveProtocolCode available in the backend.
//
// key: The byte slice representing the key to retrieve.
//
// Returns:
// - string: The value associated with the key.
// - error: An error if the retrieval fails.
func (h *HttpStorageBackend) Get(key []byte) (io.ReadCloser, int64, error) {
	keyPath := h.getEntryPath(key)
	req, err := http.NewRequest("GET", keyPath, nil)
	if err != nil {
		return nil, 0, &BackendFailure{
			Message: fmt.Sprintf("Failed to delete %s from HTTP storage", key),
			Code:    req.Response.StatusCode}
	}

	if h.bearer != "" {
		encodedCredentials := base64.StdEncoding.EncodeToString([]byte(h.bearer))
		req.Header.Add("Authorization", "Basic "+encodedCredentials)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, 0, &BackendFailure{
			Message: fmt.Sprintf("Failed to get %s from HTTP storage!", key),
			Code:    http.StatusInternalServerError}
	}

	return resp.Body, resp.ContentLength, nil
}

// Put stores data associated with the specified key in the HTTP storage backend.
// It sends an HTTP request to create or update the resource.
//
// The errors returned are of type BackendFailure. The Status Code of the http error
// can be translated by the ResolveProtocolCode available in the backend.
//
// key: a byte slice representing the key under which the data will be stored.
// data: a byte slice containing the data to be stored.
// onlyIfMissing: a boolean flag indicating whether the operation should only succeed if the key does not already exist.
//   - If true, the operation will fail if the key already exists.
//   - If false, the operation will overwrite existing data for the key.
//
// Returns:
// - bool: true if the data was successfully stored; false if the data was not stored (e.g., because the key exists and `onlyIfMissing` is true).
// - error: an error object if the operation failed due to network issues, server errors, or other reasons.
func (h *HttpStorageBackend) Put(key []byte, data []byte, onlyIfMissing bool) (bool, error) {
	urlPath := getUrl(&h.url)
	keyPath := h.getEntryPath(key)

	if onlyIfMissing {
		req, err := http.NewRequest("HEAD", keyPath, nil)
		if err != nil {
			return false, &BackendFailure{
				Message: fmt.Sprintf("Failed to create request for %s", urlPath),
				Code:    0}
		}

		if h.bearer != "" {
			encodedCredentials := base64.StdEncoding.EncodeToString([]byte(h.bearer))
			req.Header.Add("Authorization", "Basic "+encodedCredentials)
		}

		resp, err := h.client.Do(req)
		if err != nil {
			return false, &BackendFailure{
				Message: fmt.Sprintf("Failed to fetch %s from HTTP", urlPath),
				Code:    resp.StatusCode}
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return false, nil // file was found, no need to put again
		}
	}

	reader := bytes.NewReader(data)
	req, err := http.NewRequest("PUT", keyPath, reader)
	if err != nil {
		return false, &BackendFailure{
			Message: fmt.Sprintf("Failed to create put request for %s", urlPath),
			Code:    0}
	}

	if h.bearer != "" {
		encodedCredentials := base64.StdEncoding.EncodeToString([]byte(h.bearer))
		req.Header.Add("Authorization", "Basic "+encodedCredentials)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return false, &BackendFailure{
			Message: fmt.Sprintf("Failed to put %s to http storage (Server Error 500)", key),
			Code:    http.StatusInternalServerError}
	} else if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, &BackendFailure{
			Message: fmt.Sprintf("Failed to put %s to http storage (%s)", key, resp.Status),
			Code:    resp.StatusCode}
	}

	return true, nil
}
