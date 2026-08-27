package s3

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/rogueserenity/kbdb/internal/repository"
)

// ProfileImageStore is the S3-backed repository.ProfileImageStore.
type ProfileImageStore struct {
	client  s3API
	presign s3PresignAPI
	bucket  string
}

var _ repository.ProfileImageStore = (*ProfileImageStore)(nil)

// NewProfileImageStore returns a ProfileImageStore backed by client.
func NewProfileImageStore(client *s3.Client, presign *s3.PresignClient, bucket string) *ProfileImageStore {
	return &ProfileImageStore{
		client:  client,
		presign: presign,
		bucket:  bucket,
	}
}

// PresignGet implements repository.ProfileImageStore.
func (s *ProfileImageStore) PresignGet(ctx context.Context, key repository.ProfileImageKey) (string, error) {
	req, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(string(key)),
	})
	if err != nil {
		return "", fmt.Errorf("presigning GET s3://%s/%s: %w", s.bucket, key, err)
	}

	return req.URL, nil
}

// PresignPut implements repository.ProfileImageStore.
func (s *ProfileImageStore) PresignPut(ctx context.Context, key repository.ProfileImageKey, contentType string) (string, error) {
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

// Delete implements repository.ProfileImageStore.
func (s *ProfileImageStore) Delete(ctx context.Context, key repository.ProfileImageKey) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(string(key)),
	})
	if err != nil {
		return fmt.Errorf("deleting s3://%s/%s: %w", s.bucket, key, err)
	}

	return nil
}
