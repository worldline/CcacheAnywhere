package backend

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

func getUrlPath(url string) string {
	return url
}

func getPartialUrl() string {
	return "Partial"
}

func getUrl() string {
	return "gs://kls.net"
}

func formatDigest(key string) string {
	return "string"
}

func parseTimeoutAttribute(value string) int {
	// rework this later
	var timeout int
	fmt.Sscanf(value, "%d", &timeout)
	return timeout
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
	connectionTimeout int
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
	bearer  string
	urlPath string
	client  *http.Client
	layout  Layout
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
func (s *HttpStorageBackend) Create(url string, attributes []Attribute) HttpStorageBackend {
	s.urlPath = getUrlPath(url) // TODO create URL object

	defaultHeaders := newHttpHeaders()
	for _, attr := range attributes {
		switch attr.Key {
		case "bearer-token":
			defaultHeaders.bearerToken = attr.Value
		case "connect-timeout":
			defaultHeaders.connectionTimeout = parseTimeoutAttribute(attr.Value)
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
		default:
			log.Fatal("Attribute not known!")
		}
	}
	httpclient := http.Client{Timeout: time.Duration(defaultHeaders.connectionTimeout)}
	return HttpStorageBackend{client: &httpclient, bearer: defaultHeaders.bearerToken, layout: defaultHeaders.layout}
}

func (h *HttpStorageBackend) GetEntryPath(key string) string {
	switch h.layout {
	case bazel:
		// Mimic hex representation of a SHA256 hash value.
		const sha256HexSize = 64
		hexDigits := formatDigest(key)

		// Ensure hexDigits has the expected size
		if len(hexDigits) < sha256HexSize {
			hexDigits += string(make([]byte, sha256HexSize-len(hexDigits)))
		}

		logMessage := fmt.Sprintf("Translated key %s to Bazel layout ac/%s", formatDigest(key), hexDigits)
		fmt.Println(logMessage) // Replace LOG() with simple printing for this example
		return fmt.Sprintf("%s/ac/%s", h.urlPath, hexDigits)

	case flat:
		return fmt.Sprintf("%s%s", h.urlPath, formatDigest(key))

	case subdirs:
		keyStr := formatDigest(key)
		digits := 2
		if len(keyStr) <= digits {
			panic("keyStr length is insufficient for subdirectory layout")
		}
		return fmt.Sprintf("%s%s/%s", h.urlPath, keyStr[:digits], keyStr[digits:])

	default:
		panic("unknown layout")
	}
}

func (h *HttpStorageBackend) Remove(key string) (bool, error) {
	urlPath := h.GetEntryPath(key)
	req, err := http.NewRequest("DELETE", urlPath, nil)
	if err != nil {
		return false, err
	}

	resp, err := h.client.Do(req)
	if err != nil {
		log.Printf("Failed to delete %s from HTTP storage %v", urlPath, err)
		return false, &BackendFailure{Message: err.Error(), Code: http.StatusInternalServerError}
	}
	defer resp.Body.Close()

	// Check if the status code indicates a failure
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("Failed to delete %s from HTTP storage: status code: %d", urlPath, resp.StatusCode)
		return false, &BackendFailure{Message: "failed to delete", Code: resp.StatusCode}
	}
	return true, nil
}

func (h *HttpStorageBackend) Get(key string) (string, error) {
	urlPath := h.GetEntryPath(key)
	req, err := http.NewRequest("GET", urlPath, nil)
	if err != nil {
		return err.Error(), err
	}

	resp, err := h.client.Do(req)
	if err != nil {
		fmt.Printf("Failed to perform GET %s from http storage!", urlPath)
		return err.Error(), err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return string(body), err
}

func (h *HttpStorageBackend) Put(key string, data io.Reader, onlyIfMissing bool) (bool, error) {
	urlPath := h.GetEntryPath(key)

	if onlyIfMissing {
		res, err := h.client.Head(urlPath)

		if err != nil {
			fmt.Printf("Failed to check for %s in http storage: %s", urlPath, err.Error())
			return false, err
		}

		if res.StatusCode >= 200 && res.StatusCode < 300 {
			fmt.Printf("Found entry %s already within http storage: status code: %s",
				urlPath,
				res.Status)
		}
		return false, err
	}

	// contentType := "application/octet-stream"
	req, err := http.NewRequest("PUT", urlPath, data)
	if err != nil {
		log.Fatal(err)
		return false, err
	}

	resp, err := h.client.Do(req)
	if err != nil {
		log.Fatal(err)
		return false, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fmt.Printf("Failed to put %s to http storage: status code: %s",
			urlPath,
			resp.Status)
		return false, err
	}

	return true, nil
}
