package s3

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/repository"
)

type s3API interface {
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

type s3PresignAPI interface {
	PresignGetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
	PresignPutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

// KeycapKitImageStore is the S3-backed repository.KeycapKitImageStore.
type KeycapKitImageStore struct {
	client  s3API
	presign s3PresignAPI
	bucket  string
}

var _ repository.KeycapKitImageStore = (*KeycapKitImageStore)(nil)

// NewKeycapKitImageStore returns a KeycapKitImageStore backed by client.
func NewKeycapKitImageStore(client *s3.Client, presign *s3.PresignClient, bucket string) *KeycapKitImageStore {
	return &KeycapKitImageStore{
		client:  client,
		presign: presign,
		bucket:  bucket,
	}
}

// PresignGet implements repository.KeycapKitImageStore.
func (s *KeycapKitImageStore) PresignGet(ctx context.Context, key repository.KeycapKitImageKey) (string, error) {
	req, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(string(key)),
	})
	if err != nil {
		return "", fmt.Errorf("presigning GET s3://%s/%s: %w", s.bucket, key, err)
	}

	return req.URL, nil
}

// PresignPut implements repository.KeycapKitImageStore.
func (s *KeycapKitImageStore) PresignPut(ctx context.Context, key repository.KeycapKitImageKey, contentType string) (string, error) {
	req, err := s.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(string(key)),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("presigning PUT s3://%s/%s (content-type %q): %w", s.bucket, key, contentType, err)
	}

	return req.URL, nil
}

// Delete implements repository.KeycapKitImageStore.
func (s *KeycapKitImageStore) Delete(ctx context.Context, key repository.KeycapKitImageKey) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(string(key)),
	})
	if err != nil {
		return fmt.Errorf("deleting s3://%s/%s: %w", s.bucket, key, err)
	}

	return nil
}

// BestEffortDelete implements repository.KeycapKitImageStore.
func (s *KeycapKitImageStore) BestEffortDelete(ctx context.Context, keys []repository.KeycapKitImageKey) {
	for _, key := range keys {
		if err := s.Delete(ctx, key); err != nil {
			log.FromContext(ctx).Warn("deleting keycap kit image object", log.Error, err)
		}
	}
}
