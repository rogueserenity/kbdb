package schema

// BuildSwitchEntry is one entry in BuildInput.Switches.
type BuildSwitchEntry struct {
	Switch string `json:"switch" jsonschema:"the id of a Switch resource in the caller's collection"`
	Count  int    `json:"count" jsonschema:"how many of this switch are installed"`
}

// BuildKeycapKitEntry is one entry in BuildInput.KeycapKits.
type BuildKeycapKitEntry struct {
	KeycapSet string `json:"keycap_set" jsonschema:"the id of a KeycapSet resource in the caller's collection"`
	Kit       string `json:"kit" jsonschema:"the kit_id of one of that keycap set's kits"`
}

// BuildStabs is BuildInput.Stabs.
type BuildStabs struct {
	Name      *string  `json:"name,omitempty" jsonschema:"the stabilizer's name; must be an approved build_stabilizer lookup value"`
	MountType *string  `json:"mount_type,omitempty" jsonschema:"how the stabilizer is mounted; must be an approved build_stabilizer_mount_type lookup value"`
	Price     *float64 `json:"price,omitempty" jsonschema:"price paid"`
}

// BuildCaseMountType is BuildInput.CaseMountType.
type BuildCaseMountType struct {
	Type      *string `json:"type,omitempty" jsonschema:"the mount style; must be an approved build_case_mount_type lookup value"`
	Durometer *string `json:"durometer,omitempty" jsonschema:"gasket/o-ring hardness, only meaningful for mount types that support it; must be an approved build_durometer lookup value"`
}

// BuildInput has no Images field - a build's images are managed entirely
// through their own tools, never carried in a build write.
type BuildInput struct {
	Keyboard      string                `json:"keyboard" jsonschema:"the id of a Keyboard resource in the caller's collection"`
	Plate         *string               `json:"plate,omitempty" jsonschema:"which of the keyboard's design.plates options is installed"`
	CaseMountType *BuildCaseMountType   `json:"case_mount_type,omitempty" jsonschema:"the case's mounting style"`
	Stabs         *BuildStabs           `json:"stabs,omitempty" jsonschema:"the stabilizers used"`
	Foam          *bool                 `json:"foam,omitempty" jsonschema:"whether the build has case foam"`
	Switches      []BuildSwitchEntry    `json:"switches,omitempty" jsonschema:"the switches used and how many of each"`
	KeycapKits    []BuildKeycapKitEntry `json:"keycap_kits,omitempty" jsonschema:"the keycap kits used"`
	BuildDate     *string               `json:"build_date,omitempty" jsonschema:"when the build was assembled (YYYY-MM-DD)"`
	Notes         *string               `json:"notes,omitempty" jsonschema:"free-form notes"`
	Visibility    string                `json:"visibility" jsonschema:"who can read this build; one of \"public\", \"authenticated\", \"private\""`
}

// Build reports HasImages rather than a presigned URL, unlike REST's
// inline Images array, to avoid handing back a URL that may have expired
// by the time an agent acts on a held result; a future
// get_build_image_url tool fetches one on demand.
type Build struct {
	ID            string                `json:"id" jsonschema:"the build's unique id"`
	Keyboard      string                `json:"keyboard" jsonschema:"the id of the Keyboard resource this build is based on"`
	Plate         *string               `json:"plate,omitempty" jsonschema:"which of the keyboard's design.plates options is installed"`
	CaseMountType *BuildCaseMountType   `json:"case_mount_type,omitempty" jsonschema:"the case's mounting style"`
	Stabs         *BuildStabs           `json:"stabs,omitempty" jsonschema:"the stabilizers used"`
	Foam          *bool                 `json:"foam,omitempty" jsonschema:"whether the build has case foam"`
	Switches      []BuildSwitchEntry    `json:"switches,omitempty" jsonschema:"the switches used and how many of each"`
	KeycapKits    []BuildKeycapKitEntry `json:"keycap_kits,omitempty" jsonschema:"the keycap kits used"`
	BuildDate     *string               `json:"build_date,omitempty" jsonschema:"when the build was assembled (YYYY-MM-DD)"`
	Notes         *string               `json:"notes,omitempty" jsonschema:"free-form notes"`
	Visibility    string                `json:"visibility" jsonschema:"who can read this build; one of \"public\", \"authenticated\", \"private\""`
	HasImages     bool                  `json:"has_images" jsonschema:"whether this build has any images on file"`
}

// CreateBuildInput is the create_build tool's input.
type CreateBuildInput struct {
	BuildInput
}

// CreateBuildOutput is the create_build tool's output.
type CreateBuildOutput struct {
	Build Build `json:"build" jsonschema:"the created build, including its server-generated id"`
}

// UpdateBuildInput is the update_build tool's input.
type UpdateBuildInput struct {
	BuildInput
	BuildID string `json:"build_id" jsonschema:"the build's unique id"`
}

// UpdateBuildOutput is the update_build tool's output.
type UpdateBuildOutput struct {
	Build Build `json:"build" jsonschema:"the updated build"`
}

// DeleteBuildInput is the delete_build tool's input.
type DeleteBuildInput struct {
	BuildID string `json:"build_id" jsonschema:"the id of the build to delete"`
}

// DeleteBuildOutput is the delete_build tool's output. Deleting is
// idempotent, so there is no payload.
type DeleteBuildOutput struct{}

// GetBuildInput is the get_build tool's input.
type GetBuildInput struct {
	BuildID string `json:"build_id" jsonschema:"the build's unique id"`
	UserID  string `json:"user_id,omitempty" jsonschema:"whose collection to read from; omit for your own"`
}

// GetBuildOutput is the get_build tool's output.
type GetBuildOutput struct {
	Build Build `json:"build" jsonschema:"the requested build"`
}

// ListBuildsInput is the list_builds tool's input.
type ListBuildsInput struct {
	UserID string `json:"user_id,omitempty" jsonschema:"whose collection to list; omit for your own"`
	Limit  int    `json:"limit,omitempty" jsonschema:"maximum number of builds to return (1-100, default 20)"`
	Cursor string `json:"cursor,omitempty" jsonschema:"resume from a previous call's next_cursor"`
}

// ListBuildsOutput is the list_builds tool's output.
type ListBuildsOutput struct {
	Builds     []BuildSummary `json:"builds" jsonschema:"the builds in this page"`
	NextCursor string         `json:"next_cursor,omitempty" jsonschema:"pass as cursor to fetch the next page; empty when there are no more"`
}

// BuildSummary is the reduced build shape list_builds returns.
type BuildSummary struct {
	ID        string                `json:"id" jsonschema:"the build's unique id"`
	BuildDate *string               `json:"build_date,omitempty" jsonschema:"when the build was assembled (YYYY-MM-DD)"`
	HasImage  bool                  `json:"has_image" jsonschema:"whether this build has any images on file"`
	Keyboard  *BuildSummaryKeyboard `json:"keyboard,omitempty" jsonschema:"the build's keyboard, denormalized for display; omitted if the referenced keyboard no longer exists"`
}

// BuildSummaryKeyboard is BuildSummary's denormalized keyboard reference.
type BuildSummaryKeyboard struct {
	Brand string `json:"brand" jsonschema:"the referenced keyboard's brand"`
	Name  string `json:"name" jsonschema:"the referenced keyboard's name"`
}

// AddBuildImageInput is the add_build_image tool's input. It doesn't carry
// the image bytes themselves - see UploadURL on the output.
type AddBuildImageInput struct {
	BuildID     string `json:"build_id" jsonschema:"the id of the build to add an image to"`
	ContentType string `json:"content_type" jsonschema:"the image's MIME type; must be an approved image_content_type lookup value"`
}

// AddBuildImageOutput is the add_build_image tool's output. UploadURL is a
// presigned S3 PUT URL - the caller uploads the image bytes directly to it,
// matching REST's AddBuildImage; the tool call itself never carries image
// bytes.
type AddBuildImageOutput struct {
	ImageID   string `json:"image_id" jsonschema:"the newly-created image's id"`
	UploadURL string `json:"upload_url" jsonschema:"a freshly-minted, short-lived presigned URL to PUT the image bytes to directly, using the requested content_type as the Content-Type header; do not cache or persist it, it expires within minutes"`
}

// DeleteBuildImageInput is the delete_build_image tool's input.
type DeleteBuildImageInput struct {
	BuildID string `json:"build_id" jsonschema:"the id of the build the image belongs to"`
	ImageID string `json:"image_id" jsonschema:"the id of the image to remove, as returned by add_build_image"`
}

// DeleteBuildImageOutput is the delete_build_image tool's output. Deleting
// is idempotent, so there is no payload.
type DeleteBuildImageOutput struct{}
