package backend

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

// setMetadata sets an object's metadata.
// run this using a go coroutine to set the meta data of the object in the background
func setMetadata(w *storage.Writer, bucket, object string) error {
	// bucket := "bucket-name"
	// object := "object-name"
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
	fmt.Fprintf(w, "Updated custom metadata for object %v in bucket %v.\n", object, bucket)
	return nil
}

type GCSAttributes struct {
	CredentialsFile string
	ProjectID       string
	Endpoint        string
	Timeout         time.Duration
	StorageClass    string
	Location        string
}

func newGCSAttributes() *GCSAttributes {
	return &GCSAttributes{
		StorageClass: "STANDARD",
		Timeout:      30 * time.Second,
	}
}

type GCSStorageBackend struct {
	client       *storage.Client
	bucketName   string
	storageClass string
	location     string
	timeout      time.Duration
}

func (attrs *GCSAttributes) getCredentialsOption() (option.ClientOption, error) {
	if attrs.CredentialsFile != "" {
		return option.WithCredentialsFile(attrs.CredentialsFile), nil
	}
	return option.WithCredentialsFile("/home/rocky/.config/gcloud/application_default_credentials.json"), nil
}

func CreateGCSBackend(bucketName string, attributes []Attribute) *GCSStorageBackend {
	// something of form gs://my_bucket_name
	defaultAttrs := newGCSAttributes()

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
				fmt.Printf("Unknown storage class: %s\n", attr.Value)
			}
		case "location":
			defaultAttrs.Location = attr.Value
		default:
			fmt.Printf("Unknown attribute: %s\n", attr.Key)
		}
	}

	// Setup credentials options
	credsOption, err := defaultAttrs.getCredentialsOption()
	if err != nil {
		log.Printf("Failed to setup credentials: %v", err)
		return nil
	}

	// Create GCS client with context and options
	ctx := context.Background()
	client, err := storage.NewClient(ctx, credsOption)
	if err != nil {
		log.Printf("Error creating GCS client: %v", err)
		return nil
	}

	return &GCSStorageBackend{
		client:       client,
		bucketName:   bucketName,
		storageClass: defaultAttrs.StorageClass,
		location:     defaultAttrs.Location,
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

func (h *GCSStorageBackend) Get(key []byte) ([]byte, error) {
	objectName, err := formatDigest(key)
	if err != nil {
		return []byte{}, &BackendFailure{
			Message: fmt.Sprintf("Local error %s: %v", objectName, err.Error()),
			Code:    404,
		}
	}
	ctx := context.Background()

	objHandle := h.client.Bucket(h.bucketName).Object(objectName)

	reader, err := objHandle.NewReader(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return []byte{}, &BackendFailure{
				Message: fmt.Sprintf("Object %s not found in bucket %s", objectName, h.bucketName),
				Code:    404,
			}
		}
		return []byte{}, &BackendFailure{
			Message: fmt.Sprintf("Failed to get object %s: %v", objectName, err),
			Code:    500,
		}
	}
	defer reader.Close()

	body, err := io.ReadAll(reader)
	if err != nil {
		return []byte{}, &BackendFailure{
			Message: fmt.Sprintf("Failed to read object %s: %v", objectName, err),
			Code:    500,
		}
	}

	// remember to update custom time
	go setMetadata(objHandle.NewWriter(ctx), h.bucketName, objectName)
	return body, nil
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
			Code:    404,
		}
	}
	ctx := context.Background()
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
