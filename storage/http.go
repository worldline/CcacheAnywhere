package backend

import (
	"bytes"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	urlib "net/url"
	"strings"
	"time"
)

// URL format: http://HOST[:PORT][/PATH]
func getUrlPath(urlString string) (string, error) {
	u, err := urlib.Parse(urlString)
	if err != nil {
		return "", err
	}
	return u.Path, nil
}

func getUrl(u *urlib.URL) string {
	if u.Host == "" {
		panic("user provided url is empty!")
	}
	var port string
	if u.Port() != "" {
		port = ":" + u.Port()
	}
	return fmt.Sprintf("%s://%s%s/", u.Scheme, u.Hostname(), port)
}

func (h *HttpStorageBackend) getEntryPath(key []byte) string {
	urlPath := getUrl(&h.url)
	switch h.layout {
	case bazel:
		// Mimic hex representation of a SHA256 hash value.
		const sha256HexSize = 64
		hexDigits := hex.EncodeToString(key)

		// Ensure hexDigits has the expected size
		if len(hexDigits) < sha256HexSize {
			hexDigits += string(make([]byte, sha256HexSize-len(hexDigits)))
		}

		logMessage := fmt.Sprintf("Translated key %s to Bazel layout ac/%s", key, hexDigits)
		log.Println(logMessage)
		return fmt.Sprintf("%s/ac/%s", urlPath, hexDigits)

	case flat:
		hexDigit, err := formatDigest(key)
		if err != nil {
			return ""
		}
		return fmt.Sprintf("%s%s", urlPath, hexDigit)

	case subdirs:
		keyStr, err := formatDigest(key)
		if err != nil {
			return ""
		}
		digits := 2
		if len(keyStr) <= digits {
			panic("keyStr length is insufficient for subdirectory layout")
		}
		return fmt.Sprintf("%s%s/%s", urlPath, keyStr[:digits], keyStr[digits:])

	default:
		panic("unknown layout")
	}
}

func formatDigest(data []byte) (string, error) {
	const base16Bytes = 2

	if len(data) < base16Bytes {
		return "", fmt.Errorf("data size must be at least %d bytes", base16Bytes)
	}

	base16Part := hex.EncodeToString(data[:base16Bytes])
	base32Part := strings.ToLower(base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString(data[base16Bytes:]))

	return base16Part + base32Part, nil
}

func parseTimeoutAttribute(value string) time.Duration {
	var timeout time.Duration
	fmt.Sscanf(value, "%d", &timeout)
	return time.Duration(timeout.Seconds())
}

type Layout int

const (
	bazel Layout = iota
	flat
	subdirs
)

type httpHeaders struct {
	headers           map[string]string
	bearerToken       string
	connectionTimeout time.Duration
	operationTimeout  time.Duration
	layout            Layout
}

func newHttpHeaders() *httpHeaders {
	return &httpHeaders{
		headers: make(map[string]string),
	}
}

func (h *httpHeaders) emplace(key string, value string) {
	h.headers[key] = value
}

type HttpStorageBackend struct {
	bearer string
	url    urlib.URL
	client *http.Client
	layout Layout
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

// RedactSecrets modifies the attributes by redacting the bearer token if it exists
func (h *HttpStorageBackend) RedactSecrets(attributes []Attribute) {
	for i, attr := range attributes {
		if attr.Key == "bearer-token" {
			attributes[i].Value = RedactedPassword
			attributes[i].RawValue = RedactedPassword
		}
	}
}

// TODO define a backendATTributes struct as argument for create
// each create deals with it as it wishes
func CreateHTTPBackend(urlString string, attributes []Attribute) *HttpStorageBackend {
	defaultHeaders := newHttpHeaders()
	for _, attr := range attributes {
		switch attr.Key {
		case "bearer-token":
			defaultHeaders.bearerToken = attr.Value
		case "connect-timeout":
			defaultHeaders.connectionTimeout = parseTimeoutAttribute(attr.Value)
		case "keep-alive":
			// TODO
		case "operation-timeout":
			defaultHeaders.operationTimeout = parseTimeoutAttribute(attr.Value)
		case "layout":
			switch attr.Value {
			case "bazel":
				defaultHeaders.layout = bazel
			case "flat":
				defaultHeaders.layout = flat
			case "subdirs":
				defaultHeaders.layout = subdirs
			default:
				log.Printf("Unknown layout: %s\n", attr.Value)
			}
		case "header":
			spltres := strings.Split(attr.Value, "=")
			if spltres[len(spltres)-1] != "" {
				defaultHeaders.emplace(attr.Key, attr.Value)
			} else {
				log.Fatal("error")
			}
		case "url":
			// TODO
		default:
			log.Println("Attribute not known!", attr.Key)
		}
	}

	url, err := urlib.Parse(urlString)
	if err != nil {
		return nil
	}
	httpclient := http.Client{Timeout: defaultHeaders.connectionTimeout}
	return &HttpStorageBackend{url: *url, client: &httpclient,
		bearer: defaultHeaders.bearerToken, layout: defaultHeaders.layout}
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
func (h *HttpStorageBackend) Get(key []byte) ([]byte, error) {
	keyPath := h.getEntryPath(key)
	req, err := http.NewRequest("GET", keyPath, nil)
	if err != nil {
		return []byte{}, &BackendFailure{
			Message: fmt.Sprintf("Failed to delete %s from HTTP storage", key),
			Code:    req.Response.StatusCode}
	}

	if h.bearer != "" {
		encodedCredentials := base64.StdEncoding.EncodeToString([]byte(h.bearer))
		req.Header.Add("Authorization", "Basic "+encodedCredentials)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return []byte{}, &BackendFailure{
			Message: fmt.Sprintf("Failed to get %s from HTTP storage!\n", key),
			Code:    http.StatusInternalServerError}
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return []byte{}, &BackendFailure{
			Message: fmt.Sprintf("Failed to get %s from HTTP storage!\n", key),
			Code:    0}
	}
	return body, nil
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
