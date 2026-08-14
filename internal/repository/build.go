package repository

import (
	"context"
	"fmt"

	kbdbctx "github.com/rogueserenity/kbdb/internal/ctx"
)

// BuildSwitchEntry is one entry in Build.Switches.
type BuildSwitchEntry struct {
	Switch string `dynamodbav:"switch" json:"switch"`
	Count  int    `dynamodbav:"count" json:"count"`
}

// BuildKeycapKitEntry references one kit by KeycapKit.KitID.
type BuildKeycapKitEntry struct {
	KeycapSet string `dynamodbav:"keycap_set" json:"keycap_set"`
	Kit       string `dynamodbav:"kit" json:"kit"`
}

// BuildStabs has open-vocabulary Name and MountType, validated via
// internal/lookup.ValidateBuild, not closed Go enums.
type BuildStabs struct {
	Name      *string  `dynamodbav:"name,omitempty" json:"name,omitempty"`
	MountType *string  `dynamodbav:"mount_type,omitempty" json:"mount_type,omitempty"`
	Price     *float64 `dynamodbav:"price,omitempty" json:"price,omitempty"`
}

// BuildCaseMountType has open-vocabulary Type and Durometer, validated
// via internal/lookup.ValidateBuild, not closed Go enums.
type BuildCaseMountType struct {
	Type      *string `dynamodbav:"type,omitempty" json:"type,omitempty"`
	Durometer *string `dynamodbav:"durometer,omitempty" json:"durometer,omitempty"`
}

// BuildImage is one entry in Build.Images.
type BuildImage struct {
	ImageID string        `dynamodbav:"image_id" json:"image_id"`
	Path    BuildImageKey `dynamodbav:"path" json:"-"`
}

// Build has UserID as the DynamoDB partition key and ID as the sort key.
type Build struct {
	UserID        string                `dynamodbav:"user_id" json:"-"`
	ID            string                `dynamodbav:"id" json:"id"`
	Keyboard      string                `dynamodbav:"keyboard" json:"keyboard"`
	Plate         *string               `dynamodbav:"plate,omitempty" json:"plate,omitempty"`
	CaseMountType *BuildCaseMountType   `dynamodbav:"case_mount_type,omitempty" json:"case_mount_type,omitempty"`
	Stabs         *BuildStabs           `dynamodbav:"stabs,omitempty" json:"stabs,omitempty"`
	Foam          *bool                 `dynamodbav:"foam,omitempty" json:"foam,omitempty"`
	Switches      []BuildSwitchEntry    `dynamodbav:"switches,omitempty" json:"switches,omitempty"`
	KeycapKits    []BuildKeycapKitEntry `dynamodbav:"keycap_kits,omitempty" json:"keycap_kits,omitempty"`
	BuildDate     *string               `dynamodbav:"build_date,omitempty" json:"build_date,omitempty"`
	Notes         *string               `dynamodbav:"notes,omitempty" json:"notes,omitempty"`
	Visibility    Visibility            `dynamodbav:"visibility" json:"visibility"`
	Images        []BuildImage          `dynamodbav:"images,omitempty" json:"images,omitempty"`
	// Version is a repository-internal CAS guard against lost updates on
	// concurrent Images mutations, not exposed via the API.
	Version int `dynamodbav:"version" json:"-"`
}

// BuildRepository provides access to builds.
type BuildRepository interface {
	List(ctx context.Context, ownerID string, visibilities []Visibility, limit int, cursor string) (builds []Build, nextCursor string, err error)

	// Get fetches by exact key regardless of visibility; the caller checks
	// the result via internal/authz.CanReadVisibility.
	Get(ctx context.Context, ownerID, id string) (*Build, error)

	// Create returns ErrAlreadyExists on an ID collision.
	Create(ctx context.Context, b Build) (*Build, error)

	// Update returns ErrNotFound if no build with that ID exists for the
	// owner, or ErrMutationConflict if concurrent writers exhaust the
	// retry budget.
	Update(ctx context.Context, b Build) (*Build, error)

	// Delete returns the BuildImageKey of every image the build had, so
	// callers can clean up the corresponding objects in a BuildImageStore.
	Delete(ctx context.Context, id string) ([]BuildImageKey, error)

	// AddImage returns ErrMutationConflict if concurrent writers exhaust
	// the retry budget.
	AddImage(ctx context.Context, buildID string, image BuildImage) (*BuildImage, error)

	// DeleteImage is idempotent: an imageID not present in the build
	// returns (nil, nil), not an error.
	DeleteImage(ctx context.Context, buildID, imageID string) (*BuildImageKey, error)
}

// BuildImageKey is the object key an image is stored under in a
// BuildImageStore.
type BuildImageKey string

// NewBuildImageKey takes ownerID from ctx, not a parameter, so a caller
// can't build a key addressing anyone else's prefix.
func NewBuildImageKey(ctx context.Context, buildID, imageID string) (BuildImageKey, error) {
	ownerID, ok := kbdbctx.UserID(ctx)
	if !ok {
		return "", ErrNoUserID
	}

	return BuildImageKey(fmt.Sprintf("builds/%s/%s/images/%s", ownerID, buildID, imageID)), nil
}

// BuildImageStore is a parallel interface to KeycapKitImageStore, not
// shared, since a build's images are a growable array of server-generated
// ids rather than a single optional slot. Method names are suffixed
// because the concrete s3.ImageStore implements both interfaces on one
// struct, and Go doesn't allow two methods of the same name with
// different signatures.
type BuildImageStore interface {
	PresignGetBuildImage(ctx context.Context, key BuildImageKey) (url string, err error)
	PresignPutBuildImage(ctx context.Context, key BuildImageKey, contentType string) (url string, err error)

	// DeleteBuildImage is idempotent, matching S3's own DeleteObject.
	DeleteBuildImage(ctx context.Context, key BuildImageKey) error
}
