// Package s3 holds the S3-backed implementation of
// internal/repository.KeycapKitImageStore, mirroring
// internal/repository/dynamo's role for the DynamoDB-backed repositories.
package s3

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/rogueserenity/kbdb/internal/repository"
)

type ImageStore struct {
	client  *s3.Client
	presign *s3.PresignClient
	bucket  string
}

var _ repository.KeycapKitImageStore = (*ImageStore)(nil)

func NewImageStore(client *s3.Client, bucket string) *ImageStore {
	return &ImageStore{
		client:  client,
		presign: s3.NewPresignClient(client),
		bucket:  bucket,
	}
}

func (s *ImageStore) PresignGet(ctx context.Context, key repository.KeycapKitImageKey) (string, error) {
	req, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(string(key)),
	})
	if err != nil {
		return "", fmt.Errorf("presigning GET s3://%s/%s: %w", s.bucket, key, err)
	}

	return req.URL, nil
}

func (s *ImageStore) PresignPut(ctx context.Context, key repository.KeycapKitImageKey, contentType string) (string, error) {
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

func (s *ImageStore) Delete(ctx context.Context, key repository.KeycapKitImageKey) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(string(key)),
	})
	if err != nil {
		return fmt.Errorf("deleting s3://%s/%s: %w", s.bucket, key, err)
	}

	return nil
}
