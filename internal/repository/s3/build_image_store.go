package s3

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// BuildImageStore is the S3-backed repository.BuildImageStore.
type BuildImageStore struct {
	client  s3API
	presign s3PresignAPI
	bucket  string
}

var _ repository.BuildImageStore = (*BuildImageStore)(nil)

// NewBuildImageStore returns a BuildImageStore backed by client.
func NewBuildImageStore(client *s3.Client, presign *s3.PresignClient, bucket string) *BuildImageStore {
	return &BuildImageStore{
		client:  client,
		presign: presign,
		bucket:  bucket,
	}
}

// PresignGetBuildImage implements repository.BuildImageStore.
func (s *BuildImageStore) PresignGetBuildImage(ctx context.Context, key repository.BuildImageKey) (string, error) {
	req, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(string(key)),
	})
	if err != nil {
		return "", fmt.Errorf("presigning GET s3://%s/%s: %w", s.bucket, key, err)
	}

	return req.URL, nil
}

// PresignPutBuildImage implements repository.BuildImageStore.
func (s *BuildImageStore) PresignPutBuildImage(ctx context.Context, key repository.BuildImageKey, contentType string) (string, error) {
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

// DeleteBuildImage implements repository.BuildImageStore.
func (s *BuildImageStore) DeleteBuildImage(ctx context.Context, key repository.BuildImageKey) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(string(key)),
	})
	if err != nil {
		return fmt.Errorf("deleting s3://%s/%s: %w", s.bucket, key, err)
	}

	return nil
}

// BestEffortDelete implements repository.BuildImageStore.
func (s *BuildImageStore) BestEffortDelete(ctx context.Context, keys []repository.BuildImageKey) {
	for _, key := range keys {
		if err := s.DeleteBuildImage(ctx, key); err != nil {
			log.FromContext(ctx).Warn("deleting build image object", log.Error, err)
		}
	}
}
