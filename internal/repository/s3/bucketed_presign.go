package s3

import (
	"context"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// getPresignConfig controls how presigned GET URLs are bucketed so that
// repeated requests for the same object, within the same bucket window,
// produce a byte-identical URL - letting browser/CDN HTTP caching dedupe the
// underlying S3 GETs instead of re-fetching on every page load.
//
// bucket is the nominal cache lifetime: signingTime is truncated to the
// start of the bucket window containing now, and the mint URL is otherwise
// identical for every call landing in that window. minTTL is a floor on the
// URL's remaining validity: if the current window has less than minTTL left
// before it ends, signing rolls forward to the next window instead, so a
// caller never receives a URL that's about to (or already did) expire.
//
// Zero bucket falls through to the SDK's own default behavior (signingTime
// = now, 15m expiry) - see [PresignGetKeyboardImage] and its siblings.
type getPresignConfig struct {
	bucket  time.Duration
	minTTL  time.Duration
	nowFunc func() time.Time
}

func (c getPresignConfig) now() time.Time {
	if c.nowFunc != nil {
		return c.nowFunc()
	}

	return time.Now()
}

// window returns the fixed signingTime and Expires duration to use for a
// presigned GET minted at c.now(), per the bucketing/minTTL rules above.
func (c getPresignConfig) window() (signingTime time.Time, expires time.Duration) {
	now := c.now()
	if c.bucket <= 0 {
		return now, 0
	}

	start := now.Truncate(c.bucket)
	remaining := c.bucket - now.Sub(start)
	if remaining < c.minTTL {
		remaining += c.bucket
	}

	return start, remaining
}

// presignOptionFns returns the s3.PresignOptions mutators that pin a GET
// presign to c's current bucket window.
func (c getPresignConfig) presignOptionFns() []func(*s3.PresignOptions) {
	if c.bucket <= 0 {
		return nil
	}

	signingTime, expires := c.window()

	return []func(*s3.PresignOptions){
		func(o *s3.PresignOptions) {
			o.Expires = expires
			o.Presigner = fixedTimeSigner{
				wrapped:     v4.NewSigner(),
				signingTime: signingTime,
			}
		},
	}
}

// fixedTimeSigner wraps an [s3.HTTPPresignerV4], overriding whatever
// signingTime the SDK would otherwise derive from the wall clock with a
// fixed one - this is what makes two presign calls landing in the same
// bucket window produce an identical URL.
type fixedTimeSigner struct {
	wrapped     s3.HTTPPresignerV4
	signingTime time.Time
}

func (f fixedTimeSigner) PresignHTTP(
	ctx context.Context, credentials aws.Credentials, r *http.Request,
	payloadHash string, service string, region string, _ time.Time,
	optFns ...func(*v4.SignerOptions),
) (string, http.Header, error) {
	return f.wrapped.PresignHTTP(ctx, credentials, r, payloadHash, service, region, f.signingTime, optFns...)
}
