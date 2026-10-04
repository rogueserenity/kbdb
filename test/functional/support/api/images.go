package api

import "fmt"

// TestImageBytes is the payload every spec PUTs to a presigned upload URL.
// The URL is signed for the size_bytes declared when it was minted, so
// specs declare len(TestImageBytes) (ImageUploadBody does) and PUT exactly
// these bytes.
var TestImageBytes = []byte("fake-image-bytes-for-testing")

// ImageUploadBody is an ImageUploadRequest JSON body for contentType, sized
// for TestImageBytes.
func ImageUploadBody(contentType string) string {
	return fmt.Sprintf(`{"content_type":%q,"size_bytes":%d}`, contentType, len(TestImageBytes))
}
