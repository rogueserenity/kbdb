package main

import (
	"fmt"
	"time"
)

// Config is populated from environment variables via Kong. There are no CLI
// flags for this service — it's a Lambda entrypoint, not a CLI tool — but
// Kong's struct-tag env binding, defaults, and required-field validation are
// useful regardless of whether flag parsing is ever exercised.
type Config struct {
	OIDCIssuerURL string `env:"OIDC_ISSUER_URL" required:""`
	OIDCAudience  string `env:"OIDC_AUDIENCE" required:""`
	// The IdP's browser-SDK public token, rendered into the GET /authorize
	// consent page - see internal/consent.
	IDPConsentPublicToken string `env:"IDP_CONSENT_PUBLIC_TOKEN" required:""`
	// Comma-separated origins GET /logout may redirect back to - see
	// internal/consent.
	LogoutReturnOrigins      string `env:"LOGOUT_RETURN_ORIGINS" required:""`
	ImagesBucketName         string `env:"IMAGES_BUCKET_NAME" required:""`
	SwitchTableName          string `env:"SWITCH_TABLE_NAME" required:""`
	KeyboardTableName        string `env:"KEYBOARD_TABLE_NAME" required:""`
	KeycapSetTableName       string `env:"KEYCAP_SET_TABLE_NAME" required:""`
	BuildTableName           string `env:"BUILD_TABLE_NAME" required:""`
	ProfileTableName         string `env:"PROFILE_TABLE_NAME" required:""`
	ProfileUsernameTableName string `env:"PROFILE_USERNAME_TABLE_NAME" required:""`

	// ImageGetPresignBucket/ImageGetPresignMinTTL together control caching
	// of presigned GET image URLs: two requests for the same object within
	// the same bucket window get a byte-identical URL, letting browser/CDN
	// HTTP caching dedupe the underlying S3 GETs instead of re-fetching on
	// every page load. Presigned PUT URLs are unaffected and keep the AWS
	// SDK's default expiry (15m) - see internal/repository/s3.
	ImageGetPresignBucket time.Duration `env:"IMAGE_GET_PRESIGN_BUCKET" default:"24h" help:"Cache lifetime for presigned GET image URLs: requests for the same object within the same bucket window get an identical URL."`
	ImageGetPresignMinTTL time.Duration `env:"IMAGE_GET_PRESIGN_MIN_TTL" default:"3h" help:"Floor on a presigned GET URL's remaining validity; if less than this remains in the current bucket window, signing rolls forward to the next one."`

	// Empty in real deployments; set locally to point at LocalStack.
	DynamoDBEndpointURL string `env:"DYNAMODB_ENDPOINT_URL"`

	// Empty in real deployments; set locally to point at LocalStack.
	S3EndpointURL string `env:"S3_ENDPOINT_URL"`
}

// Validate is called by kong.Parse after the struct is populated. It
// enforces the invariants ImageGetPresignBucket/ImageGetPresignMinTTL rely
// on: MinTTL must leave room within a bucket window (otherwise every
// request would roll forward, defeating the cache), and Bucket must evenly
// divide 24h so windows land on the same wall-clock boundaries every day
// rather than drifting.
func (c Config) Validate() error {
	if c.ImageGetPresignMinTTL >= c.ImageGetPresignBucket {
		return fmt.Errorf("IMAGE_GET_PRESIGN_MIN_TTL (%s) must be less than IMAGE_GET_PRESIGN_BUCKET (%s)",
			c.ImageGetPresignMinTTL, c.ImageGetPresignBucket)
	}

	if c.ImageGetPresignBucket <= 0 || (24*time.Hour)%c.ImageGetPresignBucket != 0 {
		return fmt.Errorf("IMAGE_GET_PRESIGN_BUCKET (%s) must evenly divide 24h", c.ImageGetPresignBucket)
	}

	return nil
}
