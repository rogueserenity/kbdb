package schema

// BuildSwitchEntry references one of the caller's Switch resources used in
// a Build, with the count installed.
type BuildSwitchEntry struct {
	Switch string `json:"switch" jsonschema:"the id of a Switch resource in the caller's collection"`
	Count  int    `json:"count" jsonschema:"how many of this switch are installed"`
}

// BuildKeycapKitEntry references one kit within one of the caller's
// KeycapSet resources used in a Build.
type BuildKeycapKitEntry struct {
	KeycapSet string `json:"keycap_set" jsonschema:"the id of a KeycapSet resource in the caller's collection"`
	Kit       string `json:"kit" jsonschema:"the kit_id of one of that keycap set's kits"`
}

// BuildStabs describes the stabilizers used in a Build. Name and MountType
// are open vocabulary, validated against lookup categories at request time.
type BuildStabs struct {
	Name      *string  `json:"name,omitempty" jsonschema:"the stabilizer's name; must be an approved build_stabilizer lookup value"`
	MountType *string  `json:"mount_type,omitempty" jsonschema:"how the stabilizer is mounted; must be an approved build_stabilizer_mount_type lookup value"`
	Price     *float64 `json:"price,omitempty" jsonschema:"price paid"`
}

// BuildCaseMountType describes a Build's case mounting style. Type and
// Durometer are open vocabulary, validated against lookup categories at
// request time.
type BuildCaseMountType struct {
	Type      *string `json:"type,omitempty" jsonschema:"the mount style; must be an approved build_case_mount_type lookup value"`
	Durometer *string `json:"durometer,omitempty" jsonschema:"gasket/o-ring hardness, only meaningful for mount types that support it; must be an approved build_durometer lookup value"`
}

// BuildInput is the writable half of a build, shared by create_build and a
// future update_build. It has no Images field - a build's images are
// managed entirely through their own tools, never carried in a build write.
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

// Build is the full build shape. HasImages reports whether any images are
// on file, without minting URLs every read would then have to carry - a
// future get_build_image_url tool fetches one on demand instead. This is a
// deliberate divergence from REST's inline Images array: a presigned URL
// handed back inline would be short-lived and an agent may hold this result
// across turns, long after the URL has expired. Mirrors schema.KeycapKit's
// HasImage design.
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

// CreateBuildInput is the create_build tool input.
type CreateBuildInput struct {
	BuildInput
}

// CreateBuildOutput is the create_build tool output.
type CreateBuildOutput struct {
	Build Build `json:"build" jsonschema:"the created build, including its server-generated id"`
}
