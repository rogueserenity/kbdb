package s3

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/rogueserenity/kbdb/internal/repository"
)

type s3API interface {
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

type s3PresignAPI interface {
	PresignGetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
	PresignPutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

// ImageStore is the S3-backed repository.KeycapKitImageStore.
type ImageStore struct {
	client  s3API
	presign s3PresignAPI
	bucket  string
}

var _ repository.KeycapKitImageStore = (*ImageStore)(nil)
var _ repository.BuildImageStore = (*ImageStore)(nil)

// NewImageStore returns a ImageStore backed by client.
func NewImageStore(client *s3.Client, presign *s3.PresignClient, bucket string) *ImageStore {
	return &ImageStore{
		client:  client,
		presign: presign,
		bucket:  bucket,
	}
}

// PresignGet implements repository.KeycapKitImageStore.
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

// PresignPut implements repository.KeycapKitImageStore.
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

// Delete implements repository.KeycapKitImageStore.
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

// PresignGetBuildImage implements repository.BuildImageStore.
func (s *ImageStore) PresignGetBuildImage(ctx context.Context, key repository.BuildImageKey) (string, error) {
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
func (s *ImageStore) PresignPutBuildImage(ctx context.Context, key repository.BuildImageKey, contentType string) (string, error) {
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
func (s *ImageStore) DeleteBuildImage(ctx context.Context, key repository.BuildImageKey) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(string(key)),
	})
	if err != nil {
		return fmt.Errorf("deleting s3://%s/%s: %w", s.bucket, key, err)
	}

	return nil
}
