package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type ObjectStore interface {
	Put(context.Context, string, string, []byte) error
	Get(context.Context, string) ([]byte, error)
	Delete(context.Context, string) error
	List(context.Context, string, string, int) ([]ObjectInfo, error)
}

type ObjectInfo struct {
	Key          string
	LastModified time.Time
}

type S3Config struct {
	Endpoint, AccessKey, SecretKey, Bucket, Region string
	Secure                                         bool
}

type S3 struct {
	client *minio.Client
	bucket string
}

func NewS3(cfg S3Config) (*S3, error) {
	if cfg.Endpoint == "" || cfg.AccessKey == "" || cfg.SecretKey == "" || cfg.Bucket == "" {
		return nil, fmt.Errorf("object storage configuration is incomplete")
	}
	client, err := minio.New(cfg.Endpoint, &minio.Options{Creds: credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""), Secure: cfg.Secure, Region: cfg.Region})
	if err != nil {
		return nil, err
	}
	return &S3{client: client, bucket: cfg.Bucket}, nil
}

func (store *S3) EnsureBucket(ctx context.Context, create bool) error {
	exists, err := store.client.BucketExists(ctx, store.bucket)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if !create {
		return fmt.Errorf("private object bucket %q does not exist", store.bucket)
	}
	return store.client.MakeBucket(ctx, store.bucket, minio.MakeBucketOptions{})
}

func (store *S3) Ping(ctx context.Context) error {
	exists, err := store.client.BucketExists(ctx, store.bucket)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("private object bucket %q does not exist", store.bucket)
	}
	return nil
}

func (store *S3) Put(ctx context.Context, key, contentType string, data []byte) error {
	_, err := store.client.PutObject(ctx, store.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (store *S3) Get(ctx context.Context, key string) ([]byte, error) {
	object, err := store.client.GetObject(ctx, store.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer object.Close()
	data, err := io.ReadAll(io.LimitReader(object, (10<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 10<<20 {
		return nil, fmt.Errorf("stored object exceeds size limit")
	}
	return data, nil
}

func (store *S3) Delete(ctx context.Context, key string) error {
	return store.client.RemoveObject(ctx, store.bucket, key, minio.RemoveObjectOptions{})
}

func (store *S3) List(ctx context.Context, prefix, after string, limit int) ([]ObjectInfo, error) {
	if limit <= 0 || limit > 10000 {
		limit = 1000
	}
	items := make([]ObjectInfo, 0, limit)
	for object := range store.client.ListObjects(ctx, store.bucket, minio.ListObjectsOptions{Prefix: prefix, StartAfter: after, Recursive: true}) {
		if object.Err != nil {
			return nil, object.Err
		}
		items = append(items, ObjectInfo{Key: object.Key, LastModified: object.LastModified})
		if len(items) >= limit {
			break
		}
	}
	return items, nil
}
