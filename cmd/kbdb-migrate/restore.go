package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/rogueserenity/kbdb/internal/handlers/api"
)

// Run executes the restore.
//
// Restore is resume-safe at the granularity of id-map.json entries: a re-run
// skips any entity or image already recorded there. The one unavoidable gap is
// a crash in the window between a successful create/upload and the id-map save
// that records it — a re-run then repeats that single operation. For the
// single-slot image endpoints (switch, kit, avatar) the repeat is harmless
// (they replace, not append); for a top-level create it would produce one
// duplicate item, consistent with restore's always-create-new contract.
func (c *RestoreCmd) Run(ctx context.Context) error {
	token, err := resolveToken(c.Token, c.Issuer)
	if err != nil {
		return err
	}
	client, err := newAPIClient(c.BaseURL, token)
	if err != nil {
		return err
	}

	if _, err := os.Stat(filepath.Join(c.In, "manifest.json")); err != nil {
		return fmt.Errorf("%s does not look like a dump directory (no manifest.json): %w", c.In, err)
	}

	m, err := loadOrNewIDMap(c.In, client.subject)
	if err != nil {
		return err
	}
	m.RestoredAt = time.Now()

	if err := restoreKeyboards(ctx, client, c.In, m); err != nil {
		return err
	}
	if err := restoreSwitches(ctx, client, c.In, m); err != nil {
		return err
	}
	if err := restoreKeycapSets(ctx, client, c.In, m); err != nil {
		return err
	}
	if err := restoreProfile(ctx, client, c.In); err != nil {
		return err
	}
	if err := restoreBuilds(ctx, client, c.In, m); err != nil {
		return err
	}

	fmt.Printf("restore complete; id map written to %s\n", m.path)
	return nil
}

// itemDirs returns the sorted immediate subdirectories of dumpDir/sub (one per
// dumped item). Missing parent means "nothing dumped for this entity".
func itemDirs(dumpDir, sub string) ([]string, error) {
	parent := filepath.Join(dumpDir, sub)
	entries, err := os.ReadDir(parent)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", parent, err)
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	sort.Strings(dirs)
	return dirs, nil
}

// ---- keyboards ----

func restoreKeyboards(ctx context.Context, client *apiClient, dumpDir string, m *idMap) error {
	oldIDs, err := itemDirs(dumpDir, "keyboards")
	if err != nil {
		return err
	}
	for _, oldID := range oldIDs {
		if _, done := m.Keyboards[oldID]; done {
			continue
		}
		itemDir := filepath.Join(dumpDir, "keyboards", oldID)

		var full api.Keyboard
		if err := readJSONFile(filepath.Join(itemDir, "item.json"), &full); err != nil {
			return err
		}
		input := api.KeyboardInput{
			Brand:      full.Brand,
			Design:     full.Design,
			Layout:     full.Layout,
			Name:       full.Name,
			Notes:      full.Notes,
			Pcb:        full.Pcb,
			Purchase:   full.Purchase,
			Size:       full.Size,
			Visibility: full.Visibility,
		}
		var created api.Keyboard
		if err := client.doJSON(ctx, http.MethodPost, client.userPath("keyboards"), input, &created); err != nil {
			return fmt.Errorf("creating keyboard (was %s): %w", oldID, err)
		}
		mapped := &mappedEntity{NewID: created.Id, Images: map[string]string{}}
		m.Keyboards[oldID] = mapped
		if err := m.save(); err != nil {
			return err
		}

		if err := restoreArrayImages(ctx, client, itemDir,
			client.userPath("keyboards/"+created.Id+"/images"),
			client.userPath("keyboards/"+created.Id),
			mapped); err != nil {
			return fmt.Errorf("keyboard %s images: %w", created.Id, err)
		}
		if err := m.save(); err != nil {
			return err
		}
	}
	return nil
}

// ---- switches ----

func restoreSwitches(ctx context.Context, client *apiClient, dumpDir string, m *idMap) error {
	oldIDs, err := itemDirs(dumpDir, "switches")
	if err != nil {
		return err
	}
	for _, oldID := range oldIDs {
		if _, done := m.Switches[oldID]; done {
			continue
		}
		itemDir := filepath.Join(dumpDir, "switches", oldID)

		var full api.Switch
		if err := readJSONFile(filepath.Join(itemDir, "item.json"), &full); err != nil {
			return err
		}
		input := api.SwitchInput{
			Brand:        full.Brand,
			FactoryLubed: full.FactoryLubed,
			Force:        full.Force,
			Manufacturer: full.Manufacturer,
			Material:     full.Material,
			Name:         full.Name,
			Notes:        full.Notes,
			Pins:         full.Pins,
			Purchase:     full.Purchase,
			Spring:       full.Spring,
			Type:         full.Type,
			Visibility:   full.Visibility,
		}
		var created api.Switch
		if err := client.doJSON(ctx, http.MethodPost, client.userPath("switches"), input, &created); err != nil {
			return fmt.Errorf("creating switch (was %s): %w", oldID, err)
		}
		m.Switches[oldID] = &mappedEntity{NewID: created.Id}
		if err := m.save(); err != nil {
			return err
		}

		if err := restoreSwitchImage(ctx, client, itemDir, created.Id); err != nil {
			return fmt.Errorf("switch %s image: %w", created.Id, err)
		}
	}
	return nil
}

// ---- keycap sets ----

func restoreKeycapSets(ctx context.Context, client *apiClient, dumpDir string, m *idMap) error {
	oldIDs, err := itemDirs(dumpDir, "keycap-sets")
	if err != nil {
		return err
	}
	for _, oldSetID := range oldIDs {
		setDir := filepath.Join(dumpDir, "keycap-sets", oldSetID)

		var full api.KeycapSet
		if err := readJSONFile(filepath.Join(setDir, "item.json"), &full); err != nil {
			return err
		}

		// Resume-aware: create the set only if it isn't mapped yet, then
		// (re)walk kits, skipping any already mapped. A crash between "set
		// created" and "all kits created" must still finish the kits.
		mapped := m.KeycapSets[oldSetID]
		if mapped == nil {
			input := api.KeycapSetInput{
				Brand:      full.Brand,
				Material:   full.Material,
				Name:       full.Name,
				Notes:      full.Notes,
				Profile:    full.Profile,
				Visibility: full.Visibility,
			}
			var createdSet api.KeycapSet
			if err := client.doJSON(ctx, http.MethodPost, client.userPath("keycap-sets"), input, &createdSet); err != nil {
				return fmt.Errorf("creating keycap set (was %s): %w", oldSetID, err)
			}
			mapped = &mappedKeycaps{NewID: createdSet.Id, Kits: map[string]string{}}
			m.KeycapSets[oldSetID] = mapped
			if err := m.save(); err != nil {
				return err
			}
		}

		var primaryKitID string
		if full.PrimaryKitId != nil {
			primaryKitID = *full.PrimaryKitId
		}
		var kits []api.KeycapKit
		if full.Kits != nil {
			kits = *full.Kits
		}
		for _, kit := range kits {
			if _, done := mapped.Kits[kit.KitId]; done {
				continue
			}
			kitInput := api.KeycapKitInput{
				Name:     kit.Name,
				Purchase: kit.Purchase,
			}
			if kit.KitId == primaryKitID && primaryKitID != "" {
				t := true
				kitInput.Primary = &t
			}
			var createdKit api.KeycapKit
			if err := client.doJSON(ctx, http.MethodPost,
				client.userPath("keycap-sets/"+mapped.NewID+"/kits"), kitInput, &createdKit); err != nil {
				return fmt.Errorf("creating kit %q in keycap set %s (was %s): %w", kit.Name, mapped.NewID, oldSetID, err)
			}
			mapped.Kits[kit.KitId] = createdKit.KitId
			if err := m.save(); err != nil {
				return err
			}

			kitDir := filepath.Join(setDir, "kits", kit.KitId)
			if err := restoreKitImage(ctx, client, kitDir, mapped.NewID, createdKit.KitId); err != nil {
				return fmt.Errorf("keycap set %s kit %s image: %w", mapped.NewID, createdKit.KitId, err)
			}
		}
	}
	return nil
}

// ---- profile ----

func restoreProfile(ctx context.Context, client *apiClient, dumpDir string) error {
	profDir := filepath.Join(dumpDir, "profile")
	itemPath := filepath.Join(profDir, "item.json")
	if _, err := os.Stat(itemPath); os.IsNotExist(err) {
		return nil
	}

	var full api.Profile
	if err := readJSONFile(itemPath, &full); err != nil {
		return err
	}
	input := api.ProfileInput{
		Bio:             full.Bio,
		DiscordUsername: full.DiscordUsername,
		Discoverable:    full.Discoverable,
		Links:           full.Links,
		Username:        full.Username,
	}

	path := "/v1/profile/" + client.subject
	err := client.doJSON(ctx, http.MethodPost, path, input, nil)
	var apiErr *apiError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusConflict {
		// Already have a profile for this subject: replace it. A
		// username-unavailable conflict is fatal and surfaces here.
		if err := client.doJSON(ctx, http.MethodPut, path, input, nil); err != nil {
			return fmt.Errorf("replacing existing profile: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("creating profile: %w", err)
	}

	return restoreAvatarImage(ctx, client, profDir, path)
}

// ---- builds ----

func restoreBuilds(ctx context.Context, client *apiClient, dumpDir string, m *idMap) error {
	oldIDs, err := itemDirs(dumpDir, "builds")
	if err != nil {
		return err
	}
	for _, oldID := range oldIDs {
		if _, done := m.Builds[oldID]; done {
			continue
		}
		itemDir := filepath.Join(dumpDir, "builds", oldID)

		var full api.Build
		if err := readJSONFile(filepath.Join(itemDir, "item.json"), &full); err != nil {
			return err
		}
		input, err := buildInputFromResolved(full, m)
		if err != nil {
			return fmt.Errorf("build %s: %w", oldID, err)
		}
		var created api.Build
		if err := client.doJSON(ctx, http.MethodPost, client.userPath("builds"), input, &created); err != nil {
			return fmt.Errorf("creating build (was %s): %w", oldID, err)
		}
		mapped := &mappedEntity{NewID: created.Id, Images: map[string]string{}}
		m.Builds[oldID] = mapped
		if err := m.save(); err != nil {
			return err
		}

		if err := restoreArrayImages(ctx, client, itemDir,
			client.userPath("builds/"+created.Id+"/images"),
			client.userPath("builds/"+created.Id),
			mapped); err != nil {
			return fmt.Errorf("build %s images: %w", created.Id, err)
		}
		if err := m.save(); err != nil {
			return err
		}
	}
	return nil
}

// buildInputFromResolved collapses a resolved Build GET body to a BuildInput,
// remapping every cross-entity reference through m. A reference with no
// mapping is a hard error — we never POST a build with a dangling ref.
func buildInputFromResolved(full api.Build, m *idMap) (api.BuildInput, error) {
	if full.Keyboard == nil {
		return api.BuildInput{}, errors.New("resolved build has no keyboard (was its keyboard deleted before the dump?)")
	}
	kb, ok := m.Keyboards[full.Keyboard.Id]
	if !ok {
		return api.BuildInput{}, fmt.Errorf("keyboard %s is not in the id map; restore keyboards first", full.Keyboard.Id)
	}

	input := api.BuildInput{
		BuildDate:     full.BuildDate,
		CaseMountType: full.CaseMountType,
		Foam:          full.Foam,
		Keyboard:      kb.NewID,
		Notes:         full.Notes,
		Plate:         full.Plate,
		Stabs:         full.Stabs,
		Visibility:    full.Visibility,
	}

	if full.Switches != nil {
		entries := make([]api.BuildSwitchEntry, 0, len(*full.Switches))
		for _, e := range *full.Switches {
			if e.Switch == nil {
				return api.BuildInput{}, errors.New("resolved build has a switch entry with no switch (deleted before the dump?)")
			}
			sw, ok := m.Switches[e.Switch.Id]
			if !ok {
				return api.BuildInput{}, fmt.Errorf("switch %s is not in the id map; restore switches first", e.Switch.Id)
			}
			entries = append(entries, api.BuildSwitchEntry{Switch: sw.NewID, Count: e.Count})
		}
		input.Switches = &entries
	}

	if full.KeycapKits != nil {
		entries := make([]api.BuildKeycapKitEntry, 0, len(*full.KeycapKits))
		for _, e := range *full.KeycapKits {
			if e.KeycapSet == nil {
				return api.BuildInput{}, errors.New("resolved build has a keycap-kit entry with no keycap set (deleted before the dump?)")
			}
			set, ok := m.KeycapSets[e.KeycapSet.Id]
			if !ok {
				return api.BuildInput{}, fmt.Errorf("keycap set %s is not in the id map; restore keycap sets first", e.KeycapSet.Id)
			}
			newKit, ok := set.Kits[e.KitId]
			if !ok {
				return api.BuildInput{}, fmt.Errorf("kit %s of keycap set %s is not in the id map", e.KitId, e.KeycapSet.Id)
			}
			entries = append(entries, api.BuildKeycapKitEntry{KeycapSet: set.NewID, Kit: newKit})
		}
		input.KeycapKits = &entries
	}

	return input, nil
}

// ---- shared image restore ----

// restoreArrayImages walks itemDir/images/images.manifest.json, re-uploading
// each image via the two-step POST-then-PUT, then re-fetching the parent to
// verify the stored bytes hash-match the dump. mapped.Images gains
// old_image_id -> new_image_id for each.
func restoreArrayImages(ctx context.Context, client *apiClient, itemDir, imagesPath, parentPath string, mapped *mappedEntity) error {
	manifestPath := filepath.Join(itemDir, "images", "images.manifest.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		return nil
	}
	var manifest []imageManifestEntry
	if err := readJSONFile(manifestPath, &manifest); err != nil {
		return err
	}

	for _, entry := range manifest {
		if _, done := mapped.Images[entry.ImageID]; done {
			continue
		}
		data, err := os.ReadFile(filepath.Join(itemDir, "images", entry.Filename)) //nolint:gosec // inside the dump dir.
		if err != nil {
			return fmt.Errorf("reading %s: %w", entry.Filename, err)
		}

		var upload api.KeyboardImageUpload // {image_id, upload_url} — same shape for builds.
		if err := client.doJSON(ctx, http.MethodPost, imagesPath,
			api.ImageUploadRequest{ContentType: entry.ContentType}, &upload); err != nil {
			return fmt.Errorf("allocating image slot: %w", err)
		}
		if err := uploadImage(ctx, upload.UploadUrl, entry.ContentType, data); err != nil {
			return err
		}
		mapped.Images[entry.ImageID] = upload.ImageId

		if err := verifyParentImageHash(ctx, client, parentPath, upload.ImageId, entry.SHA256); err != nil {
			return err
		}
	}
	return nil
}

// loadSingleImageManifest reads dir/<name>.manifest.json + the image file it
// names. found is false when there is no manifest (the entity had no image).
func loadSingleImageManifest(dir, name string) (entry imageManifestEntry, data []byte, found bool, err error) {
	manifestPath := filepath.Join(dir, name+".manifest.json")
	if _, statErr := os.Stat(manifestPath); os.IsNotExist(statErr) {
		return imageManifestEntry{}, nil, false, nil
	}
	if err := readJSONFile(manifestPath, &entry); err != nil {
		return imageManifestEntry{}, nil, false, err
	}
	data, err = os.ReadFile(filepath.Join(dir, entry.Filename)) //nolint:gosec // inside the dump dir.
	if err != nil {
		return imageManifestEntry{}, nil, false, fmt.Errorf("reading %s: %w", entry.Filename, err)
	}
	return entry, data, true, nil
}

// uploadSingleSlot POSTs an ImageUploadRequest to imagePath, expecting a
// {upload_url} response (the shape switch/kit/avatar all share), then PUTs the
// bytes.
func uploadSingleSlot(ctx context.Context, client *apiClient, imagePath string, entry imageManifestEntry, data []byte) error {
	var upload api.SwitchImageUpload
	if err := client.doJSON(ctx, http.MethodPost, imagePath,
		api.ImageUploadRequest{ContentType: entry.ContentType}, &upload); err != nil {
		return fmt.Errorf("allocating image slot: %w", err)
	}
	return uploadImage(ctx, upload.UploadUrl, entry.ContentType, data)
}

// restoreSwitchImage restores itemDir/image.<ext> onto the given switch.
func restoreSwitchImage(ctx context.Context, client *apiClient, itemDir, switchID string) error {
	entry, data, found, err := loadSingleImageManifest(itemDir, "image")
	if err != nil || !found {
		return err
	}
	if err := uploadSingleSlot(ctx, client, client.userPath("switches/"+switchID+"/image"), entry, data); err != nil {
		return err
	}
	body, err := client.getRaw(ctx, client.userPath("switches/"+switchID))
	if err != nil {
		return fmt.Errorf("re-fetching switch %s to verify image: %w", switchID, err)
	}
	return verifyURLHash(ctx, topLevelImageURL(body), entry.SHA256)
}

// restoreAvatarImage restores profDir/avatar.<ext> onto the caller's profile.
func restoreAvatarImage(ctx context.Context, client *apiClient, profDir, profilePath string) error {
	entry, data, found, err := loadSingleImageManifest(profDir, "avatar")
	if err != nil || !found {
		return err
	}
	if err := uploadSingleSlot(ctx, client, profilePath+"/image", entry, data); err != nil {
		return err
	}
	body, err := client.getRaw(ctx, profilePath)
	if err != nil {
		return fmt.Errorf("re-fetching profile to verify avatar: %w", err)
	}
	return verifyURLHash(ctx, avatarURL(body), entry.SHA256)
}

// restoreKitImage restores kitDir/image.<ext> onto one kit of a keycap set,
// verifying by locating that kit in the re-fetched set.
func restoreKitImage(ctx context.Context, client *apiClient, kitDir, setID, kitID string) error {
	entry, data, found, err := loadSingleImageManifest(kitDir, "image")
	if err != nil || !found {
		return err
	}
	if err := uploadSingleSlot(ctx, client,
		client.userPath("keycap-sets/"+setID+"/kits/"+kitID+"/image"), entry, data); err != nil {
		return err
	}
	body, err := client.getRaw(ctx, client.userPath("keycap-sets/"+setID))
	if err != nil {
		return fmt.Errorf("re-fetching keycap set %s to verify kit image: %w", setID, err)
	}
	url, err := kitImageURL(body, kitID)
	if err != nil {
		return err
	}
	return verifyURLHash(ctx, url, entry.SHA256)
}

// verifyURLHash downloads url and checks its bytes against wantSHA. An empty
// url means the image is missing after upload, which is an error.
func verifyURLHash(ctx context.Context, url, wantSHA string) error {
	if url == "" {
		return errors.New("image is not present after upload")
	}
	data, _, err := downloadImage(ctx, url)
	if err != nil {
		return fmt.Errorf("downloading restored image: %w", err)
	}
	if got := sha256Hex(data); got != wantSHA {
		return fmt.Errorf("restored image hash mismatch: want %s, got %s", wantSHA, got)
	}
	return nil
}

// topLevelImageURL reads {"image":{"url"}} from a body, "" if absent.
func topLevelImageURL(body []byte) string {
	var v struct {
		Image *struct {
			URL string `json:"url"`
		} `json:"image"`
	}
	if err := json.Unmarshal(body, &v); err != nil || v.Image == nil {
		return ""
	}
	return v.Image.URL
}

// avatarURL reads {"avatar":{"url"}} from a body, "" if absent.
func avatarURL(body []byte) string {
	var v struct {
		Avatar *struct {
			URL string `json:"url"`
		} `json:"avatar"`
	}
	if err := json.Unmarshal(body, &v); err != nil || v.Avatar == nil {
		return ""
	}
	return v.Avatar.URL
}

// kitImageURL finds kits[kit_id==kitID].image.url in a keycap set body.
func kitImageURL(body []byte, kitID string) (string, error) {
	var v struct {
		Kits []struct {
			KitID string `json:"kit_id"`
			Image *struct {
				URL string `json:"url"`
			} `json:"image"`
		} `json:"kits"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return "", fmt.Errorf("reading kits: %w", err)
	}
	for _, k := range v.Kits {
		if k.KitID == kitID {
			if k.Image == nil {
				return "", nil
			}
			return k.Image.URL, nil
		}
	}
	return "", fmt.Errorf("kit %s not found in re-fetched set", kitID)
}

// verifyParentImageHash re-fetches parentPath, finds imageID in its images[],
// downloads it, and checks the bytes against wantSHA.
func verifyParentImageHash(ctx context.Context, client *apiClient, parentPath, imageID, wantSHA string) error {
	body, err := client.getRaw(ctx, parentPath)
	if err != nil {
		return fmt.Errorf("re-fetching %s to verify image: %w", parentPath, err)
	}
	refs, err := arrayImageRefs(body)
	if err != nil {
		return err
	}
	for _, ref := range refs {
		if ref.imageID != imageID {
			continue
		}
		data, _, err := downloadImage(ctx, ref.url)
		if err != nil {
			return fmt.Errorf("downloading restored image %s: %w", imageID, err)
		}
		if got := sha256Hex(data); got != wantSHA {
			return fmt.Errorf("restored image %s hash mismatch: want %s, got %s", imageID, wantSHA, got)
		}
		return nil
	}
	return fmt.Errorf("restored image %s not found on %s after upload", imageID, parentPath)
}
