package repository

import (
	"context"
	"errors"
	"fmt"

	kbdbctx "github.com/rogueserenity/kbdb/internal/ctx"
)

// BuildSwitchEntry references one of the caller's Switch resources used in
// a Build, with the count installed.
type BuildSwitchEntry struct {
	Switch string `dynamodbav:"switch" json:"switch"`
	Count  int    `dynamodbav:"count" json:"count"`
}

// BuildKeycapKitEntry references one kit (by KeycapKit.KitID) within one of
// the caller's KeycapSet resources used in a Build.
type BuildKeycapKitEntry struct {
	KeycapSet string `dynamodbav:"keycap_set" json:"keycap_set"`
	Kit       string `dynamodbav:"kit" json:"kit"`
}

// BuildStabs describes the stabilizers used in a Build. Name and MountType
// are open vocabulary, validated against lookup categories at request time
// (see internal/lookup.ValidateBuild), not closed Go enums.
type BuildStabs struct {
	Name      *string  `dynamodbav:"name,omitempty" json:"name,omitempty"`
	MountType *string  `dynamodbav:"mount_type,omitempty" json:"mount_type,omitempty"`
	Price     *float64 `dynamodbav:"price,omitempty" json:"price,omitempty"`
}

// BuildCaseMountType describes a Build's case mounting style. Type and
// Durometer are open vocabulary, validated against lookup categories at
// request time (see internal/lookup.ValidateBuild), not closed Go enums.
type BuildCaseMountType struct {
	Type      *string `dynamodbav:"type,omitempty" json:"type,omitempty"`
	Durometer *string `dynamodbav:"durometer,omitempty" json:"durometer,omitempty"`
}

// BuildImage is one entry in a Build's growable Images array. ImageID is
// server-generated and unique within its parent build, unlike KeycapKit's
// single optional image per kit.
type BuildImage struct {
	ImageID string        `dynamodbav:"image_id" json:"image_id"`
	Path    BuildImageKey `dynamodbav:"path" json:"-"`
}

// Build is a record of an actual keyboard build a user has assembled, in a
// user's collection or shared with the caller. UserID is the DynamoDB
// partition key (the owner's Cognito subject); ID is the sort key. Only
// Keyboard and Visibility are required, per api/openapi.yaml's BuildInput
// schema.
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
	// Version guards image sub-mutations against a lost update when two
	// concurrent calls read-modify-write this item's Images slice - not
	// exposed via the API, purely a repository-internal CAS mechanism.
	Version int `dynamodbav:"version" json:"-"`
}

// BuildRepository provides access to builds.
type BuildRepository interface {
	// List returns up to limit builds owned by ownerID whose Visibility is
	// in visibilities, ordered by ID. cursor, if non-empty, resumes from a
	// previous call's returned cursor; the returned cursor is empty when
	// there are no more pages.
	List(ctx context.Context, ownerID string, visibilities []Visibility, limit int, cursor string) (builds []Build, nextCursor string, err error)

	// Get returns the build owned by ownerID with the given id, or
	// ErrNotFound if it doesn't exist. Get doesn't take a visibility
	// argument: unlike List, it fetches by exact key regardless of
	// visibility - the caller (a handler) checks the returned item's
	// Visibility via internal/authz.CanReadVisibility.
	Get(ctx context.Context, ownerID, id string) (*Build, error)

	// Create stores b (UserID is set from ctx, b.ID must already be set).
	// Returns ErrAlreadyExists on an ID collision.
	Create(ctx context.Context, b Build) (*Build, error)

	// Update replaces the build with b.ID (UserID is set from ctx, b.ID
	// must already be set to the build being updated), preserving Images.
	// Returns ErrNotFound if no build with that ID exists for the owner, or
	// ErrMutationConflict if concurrent writers exhaust the retry budget.
	Update(ctx context.Context, b Build) (*Build, error)

	// Delete removes the caller's build with the given id and returns the
	// BuildImageKey of every image it had, so callers can clean up the
	// corresponding objects in a BuildImageStore. Returns ErrNotFound if id
	// doesn't exist.
	Delete(ctx context.Context, id string) ([]BuildImageKey, error)

	// AddImage appends image to the build's Images (image.ImageID must
	// already be set) and returns the stored image, matching Create's
	// shape for every other entity. Returns ErrNotFound if the parent
	// build doesn't exist, or ErrMutationConflict if concurrent writers
	// exhaust the retry budget.
	AddImage(ctx context.Context, buildID string, image BuildImage) (*BuildImage, error)

	// DeleteImage removes the image matching imageID from buildID's
	// Images and returns the image key that was removed, or nil if it was
	// already absent. Idempotent: an imageID not present in the build is
	// not an error, and returns (nil, nil). Returns ErrNotFound if
	// buildID doesn't exist for the owner, or ErrMutationConflict if
	// concurrent writers exhaust the retry budget.
	DeleteImage(ctx context.Context, buildID, imageID string) (*BuildImageKey, error)
}

// BuildImageKey is the object key an image is stored under in a
// BuildImageStore.
type BuildImageKey string

// NewBuildImageKey builds the deterministic object key for imageID's image
// within buildID. ownerID comes from ctx, not a parameter, so a caller
// can't build a key addressing anyone else's prefix. Unlike
// NewKeycapKitImageKey's fixed per-kit path, images are keyed by their own
// server-generated id, since a build's Images array is growable rather than
// a single optional slot.
func NewBuildImageKey(ctx context.Context, buildID, imageID string) (BuildImageKey, error) {
	ownerID, ok := kbdbctx.UserID(ctx)
	if !ok {
		return "", ErrNoUserID
	}

	return BuildImageKey(fmt.Sprintf("builds/%s/%s/images/%s", ownerID, buildID, imageID)), nil
}

// BuildImageStore stores a build image object in a private object store,
// addressed by BuildImageKey. Never called with the caller-facing presigned
// URL - that's minted fresh per request, never persisted. A new parallel
// interface to KeycapKitImageStore, not shared: a build's images are a
// growable array of server-generated ids, unlike a kit's single optional
// image. Method names are suffixed (unlike KeycapKitImageStore's bare
// PresignGet/PresignPut/Delete) because the concrete s3.ImageStore
// implements both interfaces on the same struct, and Go doesn't allow two
// methods of the same name with different signatures on one type.
type BuildImageStore interface {
	// PresignGetBuildImage returns a short-lived presigned GET URL for key.
	PresignGetBuildImage(ctx context.Context, key BuildImageKey) (url string, err error)

	// PresignPutBuildImage returns a short-lived presigned PUT URL for key,
	// locked to contentType via the Content-Type header the upload must
	// match.
	PresignPutBuildImage(ctx context.Context, key BuildImageKey, contentType string) (url string, err error)

	// DeleteBuildImage removes the object at key. Idempotent: a
	// nonexistent key is not an error, matching S3's own DeleteObject
	// semantics.
	DeleteBuildImage(ctx context.Context, key BuildImageKey) error
}

// ResolveBuildSummaryKeyboard fetches b's referenced Keyboard for a list
// summary's denormalized brand/name display, shared by both
// repoapi.BuildToAPISummary and repomcp.BuildToMCPSummary so the two
// protocols' not-found tolerance can't silently diverge. ok is false (with a
// nil Keyboard and nil error) if the keyboard can't be resolved (e.g. it was
// deleted after the build was created - builds are validated to reference
// an existing keyboard at create/update time via internal/buildrefs, so
// this should be rare in practice); callers should omit the denormalized
// fields in that case rather than failing the whole list request over one
// bad denormalization. Any other repository error is returned as-is.
func ResolveBuildSummaryKeyboard(ctx context.Context, b Build, keyboardRepo KeyboardRepository) (kb *Keyboard, ok bool, err error) {
	kb, err = keyboardRepo.Get(ctx, b.UserID, b.Keyboard)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, false, nil
		}

		return nil, false, fmt.Errorf("getting keyboard %q for build %q: %w", b.Keyboard, b.ID, err)
	}

	return kb, true, nil
}
