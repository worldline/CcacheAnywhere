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
		fmt.Println(logMessage)
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
	base32Part := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(data[base16Bytes:])

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
				fmt.Printf("Unknown layout: %s\n", attr.Value)
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

func (h *HttpStorageBackend) Remove(key []byte) (bool, error) {
	urlPath := getUrl(&h.url)
	keyPath := h.getEntryPath(key)
	req, err := http.NewRequest("DELETE", keyPath, bytes.NewReader(key))
	if err != nil {
		return false, err
	}

	if h.bearer != "" {
		encodedCredentials := base64.StdEncoding.EncodeToString([]byte(h.bearer))
		req.Header.Add("Authorization", "Basic "+encodedCredentials)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		log.Printf("Failed to delete %s from HTTP storage %v", urlPath, err)
		return false, &BackendFailure{Message: err.Error(), Code: http.StatusInternalServerError}
	}
	defer resp.Body.Close()

	// Check if the status code indicates a failure
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("Failed to delete %s from HTTP storage: status code: %d", key, resp.StatusCode)
		return false, &BackendFailure{Message: "failed to delete", Code: resp.StatusCode}
	}
	return true, nil
}

func (h *HttpStorageBackend) Get(key []byte) (string, error) {
	keyPath := h.getEntryPath(key)
	req, err := http.NewRequest("GET", keyPath, nil)
	if err != nil {
		return err.Error(), err
	}

	if h.bearer != "" {
		encodedCredentials := base64.StdEncoding.EncodeToString([]byte(h.bearer))
		req.Header.Add("Authorization", "Basic "+encodedCredentials)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		fmt.Printf("Failed to perform GET %s from http storage!\n", key)
		return err.Error(), err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return string(body), err
}

func (h *HttpStorageBackend) Put(key []byte, data []byte, onlyIfMissing bool) (bool, error) {
	keyPath := h.getEntryPath(key)
	fmt.Printf("Key: %s \nData: %s Bool: %v\n", keyPath, data, onlyIfMissing)
	if onlyIfMissing {
		res, err := h.client.Head(keyPath)
		if err != nil {
			fmt.Printf("Failed to check for %s in http storage: %s\n", key, err.Error())
			return false, err
		}

		if res.StatusCode >= 200 && res.StatusCode < 300 {
			fmt.Printf("Found entry %s already within http storage: status code: %s",
				key,
				res.Status)
		}
		return false, err
	}

	// contentType := "application/octet-stream"
	reader := bytes.NewReader(data)
	req, err := http.NewRequest("PUT", keyPath, reader)
	if err != nil {
		log.Fatal(err)
		return false, err
	}

	if h.bearer != "" {
		encodedCredentials := base64.StdEncoding.EncodeToString([]byte(h.bearer))
		req.Header.Add("Authorization", "Basic "+encodedCredentials)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		log.Fatal(err)
		return false, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fmt.Printf("Failed to put %s to http storage: status code: %s",
			key,
			resp.Status)
		return false, err
	}

	return true, nil
}
