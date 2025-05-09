// TODO a lot of stuff
package backend

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"cloud.google.com/go/storage"
)

func (s *GcsStorageBackend) Create() GcsStorageBackend {
	// TODO specify more attributes for the context
	ctx := context.Background()

	// Sets your Google Cloud Platform project ID.
	// TODO this should be communicated through attributes
	projectID := "mimetic-sunset-456212-u4"

	// Creates a client.
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Sets the name for the new bucket.
	// TODO should be handled
	bucketName := "my-new-bucket"

	// Creates a Bucket instance.
	bucket := client.Bucket(bucketName)

	// Creates the new bucket.
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()
	if err := bucket.Create(ctx, projectID, nil); err != nil {
		log.Fatalf("Failed to create bucket: %v", err)
	}

	fmt.Printf("Bucket %v created.\n", bucketName)
	return GcsStorageBackend{bucket: *bucket, projectID: projectID}
}

type GcsStorageBackend struct {
	bucket    storage.BucketHandle
	projectID string
}

func (g *GcsStorageBackend) Get(key string) (string, error) {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatal("storage.NewClient: %w", err)
		return "", err
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	rc, err := client.Bucket(g.bucket.BucketName()).Object(key).NewReader(ctx)
	if err != nil {
		log.Fatal("Object(%w).NewReader: %w", key, err)
		return "", err
	}
	defer rc.Close()

	fmt.Printf("Blob %v downloaded to local file %v\n", g.projectID, key)

	data, err := io.ReadAll(rc)
	if err != nil {
		return "Error reading from Reader", err
	}

	return fmt.Sprintf("%x", data), err
}

func (g *GcsStorageBackend) Remove(key string) (bool, error) {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatal("storage.NewClient: %w", err)
		return false, err
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	o := client.Bucket(g.bucket.BucketName()).Object(key)

	// Optional: set a generation-match precondition to avoid potential race
	// conditions and data corruptions. The request to delete the file is aborted
	// if the object's generation number does not match your precondition.
	attrs, err := o.Attrs(ctx)
	if err != nil {
		log.Fatal("object.Attrs: %w", err.Error())
		return false, err
	}
	o = o.If(storage.Conditions{GenerationMatch: attrs.Generation})

	if err := o.Delete(ctx); err != nil {
		log.Fatal("Object(%w).Delete: %w", key, err.Error())
		return false, err
	}
	fmt.Printf("Blob %v deleted.\n", key)
	return true, nil
}

func (g *GcsStorageBackend) Put(key string, data io.Reader, onlyIfMissing bool) (bool, error) {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatal("storage.NewClient: %w", err)
		return false, err
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	o := client.Bucket(g.bucket.BucketName()).Object(key)
	o = o.If(storage.Conditions{DoesNotExist: true})
	wc := o.NewWriter(ctx)

	if _, err := io.Copy(wc, data); err != nil {
		log.Fatal("io Copy %w", err.Error())
		return false, err
	}
	if err := wc.Close(); err != nil {
		log.Fatal("Writer.Close: %w", err.Error())
		return false, err
	}

	fmt.Printf("Blob %v uploaded.\n", key)
	return true, nil
}
