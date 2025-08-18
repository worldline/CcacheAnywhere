package backend

import (
	"context"
	"errors"
	"fmt"
	urlib "net/url"
	"runtime"
	"strings"
	"time"

	"ccache-backend-client/internal/constants"
	//lint:ignore ST1001 do want nice LOG operations
	. "ccache-backend-client/internal/logger"
	"ccache-backend-client/internal/tlv"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

type GCSAttributes struct {
	CredentialsFile string
	ProjectID       string
	Endpoint        string
	Timeout         time.Duration
	StorageClass    string
	Location        string
}

type GCSStorageBackend struct {
	client       *storage.Client
	bucketName   string
	storageClass string
	location     string
	timeout      time.Duration
}

func NewGCSAttributes() *GCSAttributes {
	return &GCSAttributes{
		StorageClass: "STANDARD",
		Timeout:      30 * time.Second,
	}
}

// setMetadata sets an object's metadata.
// run this using a go coroutine to set the meta data of the object in the background
func setMetadata(bucket, object string) error {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("storage.NewClient: %w", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()
	o := client.Bucket(bucket).Object(object)

	attrs, err := o.Attrs(ctx)
	if err != nil {
		return fmt.Errorf("object.Attrs: %w", err)
	}
	o = o.If(storage.Conditions{MetagenerationMatch: attrs.Metageneration})

	// Update the object to set the metadata.
	objectAttrsToUpdate := storage.ObjectAttrsToUpdate{
		CustomTime: time.Now(),
	}
	if _, err := o.Update(ctx, objectAttrsToUpdate); err != nil {
		return fmt.Errorf("ObjectHandle(%q).Update: %w", object, err)
	}
	LOG("Updated custom metadata for object %v in bucket %v.\n", object, bucket)
	return nil
}

// getCredentialsOption returns a Google Cloud client option configured with the appropriate credentials file.
// It provides flexibility by using a user-specified credentials file if set, or defaults based on the operating system.
func (attrs *GCSAttributes) getCredentialsOption() (option.ClientOption, error) {
	if attrs.CredentialsFile != "" {
		return option.WithCredentialsFile(attrs.CredentialsFile), nil
	}
	if runtime.GOOS == "windows" {
		return option.WithCredentialsFile("%APPDATA%\\gcloud\application_default_credentials.json"), nil
	}
	return option.WithCredentialsFile("$HOME/.config/gcloud/application_default_credentials.json"), nil
}

func NewGCSBackend(url *urlib.URL, attributes []Attribute) *GCSStorageBackend {
	// something of form gs://my_bucket_name
	defaultAttrs := NewGCSAttributes()

	for _, attr := range attributes {
		switch attr.Key {
		case "credentials-file":
			// Path to the JSON credentials file
			defaultAttrs.CredentialsFile = attr.Value
		case "project-id":
			defaultAttrs.ProjectID = attr.Value
		case "endpoint":
			// Optional custom endpoint URL
			defaultAttrs.Endpoint = attr.Value
		case "timeout":
			defaultAttrs.Timeout = parseTimeoutAttribute(attr.Value)
		case "storage-class":
			switch attr.Value {
			case "STANDARD", "NEARLINE", "COLDLINE", "ARCHIVE":
				defaultAttrs.StorageClass = attr.Value
			default:
				defaultAttrs.StorageClass = "STANDARD"
				LOG("Unknown storage class: %s - defaulting to Standard", attr.Value)
			}
		case "location":
			defaultAttrs.Location = attr.Value
		default:
			LOG("Unknown attribute: %s", attr.Key)
		}
	}

	// Setup credentials options
	credsOption, err := defaultAttrs.getCredentialsOption()
	if err != nil {
		LOG("Failed to setup credentials: %v", err)
		return nil
	}

	// Create GCS client with context and options
	ctx := context.Background()
	client, err := storage.NewClient(ctx, credsOption)
	if err != nil {
		LOG("Error creating GCS client: %v", err)
		return nil
	}

	location := url.Path
	if strings.HasPrefix(location, "/") {
		location = location[1:] + "/"
	}

	return &GCSStorageBackend{
		bucketName:   url.Host,
		client:       client,
		location:     location,
		storageClass: defaultAttrs.StorageClass,
		timeout:      defaultAttrs.Timeout,
	}
}

func (h *GCSStorageBackend) ResolveProtocolCode(code int) StatusCode {
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

func (h *GCSStorageBackend) Get(key []byte, serializer *tlv.Serializer) error {
	objectName, err := formatDigest(key)
	if err != nil {
		return &BackendFailure{
			Message: fmt.Sprintf("Local error %s: %v", objectName, err.Error()),
			Code:    404,
		}
	}
	objectName = h.location + objectName

	objHandle := h.client.Bucket(h.bucketName).Object(objectName)

	ctx := context.Background()
	reader, err := objHandle.NewReader(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return &BackendFailure{
				Message: fmt.Sprintf("Object %s not found in bucket %s", objectName, h.bucketName),
				Code:    404,
			}
		}
		return &BackendFailure{
			Message: fmt.Sprintf("Failed to get object %s: %v", objectName, err),
			Code:    500,
		}
	}
	defer reader.Close()

	// remember to update custom time
	go setMetadata(h.bucketName, objectName)
	return serializer.AddFieldFromReader(constants.TypeValue, reader, reader.Attrs.Size)
}

func (h *GCSStorageBackend) Remove(key []byte) (bool, error) {
	objectName, err := formatDigest(key)
	if err != nil {
		return false, &BackendFailure{
			Message: fmt.Sprintf("Local error %s: %v", objectName, err.Error()),
			Code:    404,
		}
	}
	ctx := context.Background()
	objectName = h.location + objectName

	objHandle := h.client.Bucket(h.bucketName).Object(objectName)

	err = objHandle.Delete(ctx)
	if err != nil {
		if err == storage.ErrObjectNotExist {
			return false, &BackendFailure{
				Message: fmt.Sprintf("Object %s does not exist in bucket %s", objectName, h.bucketName),
				Code:    404,
			}
		}

		return false, &BackendFailure{
			Message: fmt.Sprintf("Failed to delete object %s: %v", objectName, err),
			Code:    500,
		}
	}

	return true, nil
}

func (h *GCSStorageBackend) Put(key []byte, data []byte, onlyIfMissing bool) (bool, error) {
	objectName, err := formatDigest(key)
	if err != nil {
		return false, &BackendFailure{
			Message: fmt.Sprintf("Local error %s: %v", objectName, err.Error()),
			Code:    500,
		}
	}
	ctx := context.Background()
	objectName = h.location + objectName
	objHandle := h.client.Bucket(h.bucketName).Object(objectName)

	if onlyIfMissing {
		_, err := objHandle.Attrs(ctx)
		if err == nil {
			return false, nil
		}
		if err != storage.ErrObjectNotExist {
			return false, &BackendFailure{
				Message: fmt.Sprintf("Failed to check existence of object %s: %v", objectName, err),
				Code:    500,
			}
		}
	}

	wc := objHandle.NewWriter(ctx)
	wc.StorageClass = h.storageClass
	// this is necessary for enabling LRU in Object Lifecycle Management
	wc.ObjectAttrs.CustomTime = time.Now()

	_, err = wc.Write(data)
	if err != nil {
		return false, &BackendFailure{
			Message: fmt.Sprintf("Failed to write object %s: %v", objectName, err),
			Code:    500,
		}
	}
	if err := wc.Close(); err != nil {
		return false, &BackendFailure{
			Message: fmt.Sprintf("Failed to close writer for object %s: %v", objectName, err),
			Code:    500,
		}
	}

	return true, nil
}
