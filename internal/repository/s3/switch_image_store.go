package s3

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// SwitchImageStore is the S3-backed repository.SwitchImageStore.
type SwitchImageStore struct {
	client  s3API
	presign s3PresignAPI
	bucket  string
}

var _ repository.SwitchImageStore = (*SwitchImageStore)(nil)

// NewSwitchImageStore returns a SwitchImageStore backed by client.
func NewSwitchImageStore(client *s3.Client, presign *s3.PresignClient, bucket string) *SwitchImageStore {
	return &SwitchImageStore{
		client:  client,
		presign: presign,
		bucket:  bucket,
	}
}

// PresignGet implements repository.SwitchImageStore.
func (s *SwitchImageStore) PresignGet(ctx context.Context, key repository.SwitchImageKey) (string, error) {
	req, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(string(key)),
	})
	if err != nil {
		return "", fmt.Errorf("presigning GET s3://%s/%s: %w", s.bucket, key, err)
	}

	return req.URL, nil
}

// PresignPut implements repository.SwitchImageStore.
func (s *SwitchImageStore) PresignPut(ctx context.Context, key repository.SwitchImageKey, contentType string) (string, error) {
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

// Delete implements repository.SwitchImageStore.
func (s *SwitchImageStore) Delete(ctx context.Context, key repository.SwitchImageKey) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(string(key)),
	})
	if err != nil {
		return fmt.Errorf("deleting s3://%s/%s: %w", s.bucket, key, err)
	}

	return nil
}

// BestEffortDelete implements repository.SwitchImageStore.
func (s *SwitchImageStore) BestEffortDelete(ctx context.Context, keys []repository.SwitchImageKey) {
	var wg sync.WaitGroup
	for _, key := range keys {
		wg.Add(1)
		go func(key repository.SwitchImageKey) {
			defer wg.Done()

			if err := s.Delete(ctx, key); err != nil {
				log.FromContext(ctx).Warn("deleting switch image object", log.Error, err)
			}
		}(key)
	}
	wg.Wait()
}
