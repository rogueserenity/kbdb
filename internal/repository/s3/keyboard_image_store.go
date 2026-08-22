package s3

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// KeyboardImageStore is the S3-backed repository.KeyboardImageStore.
type KeyboardImageStore struct {
	client  s3API
	presign s3PresignAPI
	bucket  string
}

var _ repository.KeyboardImageStore = (*KeyboardImageStore)(nil)

// NewKeyboardImageStore returns a KeyboardImageStore backed by client.
func NewKeyboardImageStore(client *s3.Client, presign *s3.PresignClient, bucket string) *KeyboardImageStore {
	return &KeyboardImageStore{
		client:  client,
		presign: presign,
		bucket:  bucket,
	}
}

// PresignGetKeyboardImage implements repository.KeyboardImageStore.
func (s *KeyboardImageStore) PresignGetKeyboardImage(ctx context.Context, key repository.KeyboardImageKey) (string, error) {
	req, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(string(key)),
	})
	if err != nil {
		return "", fmt.Errorf("presigning GET s3://%s/%s: %w", s.bucket, key, err)
	}

	return req.URL, nil
}

// PresignPutKeyboardImage implements repository.KeyboardImageStore.
func (s *KeyboardImageStore) PresignPutKeyboardImage(ctx context.Context, key repository.KeyboardImageKey, contentType string) (string, error) {
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

// DeleteKeyboardImage implements repository.KeyboardImageStore.
func (s *KeyboardImageStore) DeleteKeyboardImage(ctx context.Context, key repository.KeyboardImageKey) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(string(key)),
	})
	if err != nil {
		return fmt.Errorf("deleting s3://%s/%s: %w", s.bucket, key, err)
	}

	return nil
}

// BestEffortDelete implements repository.KeyboardImageStore.
func (s *KeyboardImageStore) BestEffortDelete(ctx context.Context, keys []repository.KeyboardImageKey) {
	for _, key := range keys {
		if err := s.DeleteKeyboardImage(ctx, key); err != nil {
			log.FromContext(ctx).Warn("deleting keyboard image object", log.Error, err)
		}
	}
}
