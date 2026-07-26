package repository

// Lookup category names, matching model/lookup_seed.json exactly. Entity
// handlers reference these instead of re-typing category name strings, so a
// typo can't silently create a mismatch between a handler's checks and the
// seeded data.
const (
	// CategoryVisibility exists only so GET /v1/lookups[/{category}] can
	// list it like any other category - Visibility is a closed OpenAPI
	// enum/Go type (see Visibility.Valid()), never runtime-checked via
	// ValidateFields the way the categories below are.
	CategoryVisibility                  = "visibility"
	CategoryKeyboardPCBAssemblyType     = "keyboard_pcb_assembly_type"
	CategoryKeyboardPCBConnectivityType = "keyboard_pcb_connectivity_type"
	CategoryKeyboardSize                = "keyboard_size"
	CategoryKeyboardLayout              = "keyboard_layout"
	CategoryKeyboardCaseMaterial        = "keyboard_case_material"
	CategoryKeyboardPlateMaterial       = "keyboard_plate_material"
	CategoryKeyboardWeightMaterial      = "keyboard_weight_material"
	CategoryKeyboardPCBFirmware         = "keyboard_pcb_firmware"
	CategorySwitchType                  = "switch_type"
	CategorySwitchMaterial              = "switch_material"
	CategorySwitchSpringMaterial        = "switch_spring_material"
	CategoryKeycapProfile               = "keycap_profile"
	CategoryKeycapMaterial              = "keycap_material"
	CategoryVendor                      = "vendor"
	CategoryOrderStatus                 = "order_status"
	CategoryBuildStabilizer             = "build_stabilizer"
	CategoryBuildStabilizerMountType    = "build_stabilizer_mount_type"
	CategoryBuildCaseMountType          = "build_case_mount_type"
	CategoryBuildDurometer              = "build_durometer"
	CategoryImageContentType            = "image_content_type"
)
