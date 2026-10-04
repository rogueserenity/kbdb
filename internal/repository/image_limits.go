package repository

// MaxImageSizeBytes caps a single uploaded image at 5 MB. Every presigned
// PUT is signed for the declared size, so S3 rejects a body of any other
// length; api/openapi.yaml's ImageUploadRequest.size_bytes carries the
// same bound for REST.
const MaxImageSizeBytes = 5 * 1024 * 1024

// MaxImagesPerItem caps the images on a keyboard or build. Switches, keycap
// kits and profiles have a single image slot and aren't affected.
const MaxImagesPerItem = 10
