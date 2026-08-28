package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/rogueserenity/kbdb/internal/handlers/api"
)

// verifyStatus is the per-item outcome.
type verifyStatus string

const (
	statusOK                 verifyStatus = "OK"
	statusMissing            verifyStatus = "MISSING"
	statusFieldMismatch      verifyStatus = "FIELD_MISMATCH"
	statusImageCountMismatch verifyStatus = "IMAGE_COUNT_MISMATCH"
	statusImageHashMismatch  verifyStatus = "IMAGE_HASH_MISMATCH"
)

// verifyResult is one line of the report.
type verifyResult struct {
	entity string
	oldID  string
	newID  string
	status verifyStatus
	detail string
}

// Run executes verify and prints a per-item report, exiting non-zero if
// anything is not OK.
func (c *VerifyCmd) Run(ctx context.Context) error {
	token, err := resolveToken(c.Token, c.Issuer)
	if err != nil {
		return err
	}
	client, err := newAPIClient(c.BaseURL, token)
	if err != nil {
		return err
	}

	var m idMap
	if err := readJSONFile(filepath.Join(c.In, "id-map.json"), &m); err != nil {
		return fmt.Errorf("verify needs the id-map.json a restore wrote: %w", err)
	}
	m.ensureMaps()

	var results []verifyResult
	results = append(results, verifyKeyboards(ctx, client, c.In, &m)...)
	results = append(results, verifySwitches(ctx, client, c.In, &m)...)
	results = append(results, verifyKeycapSets(ctx, client, c.In, &m)...)
	results = append(results, verifyBuilds(ctx, client, c.In, &m)...)
	results = append(results, verifyProfile(ctx, client, c.In)...)

	bad := 0
	for _, r := range results {
		line := fmt.Sprintf("%-11s %-10s %s -> %s", r.status, r.entity, r.oldID, r.newID)
		if r.detail != "" {
			line += "  (" + r.detail + ")"
		}
		fmt.Println(line)
		if r.status != statusOK {
			bad++
		}
	}
	fmt.Printf("\n%d items checked, %d not OK\n", len(results), bad)
	if bad > 0 {
		return fmt.Errorf("%d item(s) failed verification", bad)
	}
	return nil
}

// jsonScalars strips server-owned / derived keys and reduces the rest to a
// comparable map, so verify compares what a user actually supplied. Nested
// objects are kept; arrays of images and resolved refs are dropped.
func jsonScalars(raw []byte, dropKeys ...string) (map[string]any, error) {
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("parsing item for comparison: %w", err)
	}
	for _, k := range append([]string{"id", "user_id"}, dropKeys...) {
		delete(v, k)
	}
	return v, nil
}

func compareScalars(dumpRaw, liveRaw []byte, dropKeys ...string) (bool, string) {
	a, err := jsonScalars(dumpRaw, dropKeys...)
	if err != nil {
		return false, err.Error()
	}
	b, err := jsonScalars(liveRaw, dropKeys...)
	if err != nil {
		return false, err.Error()
	}
	if reflect.DeepEqual(a, b) {
		return true, ""
	}
	return false, "scalar fields differ from the dump"
}

// ---- per-entity verify ----

func verifyKeyboards(ctx context.Context, client *apiClient, dumpDir string, m *idMap) []verifyResult {
	var out []verifyResult
	for oldID, mapped := range m.Keyboards {
		r := verifyResult{entity: "keyboard", oldID: oldID, newID: mapped.NewID}
		itemDir := filepath.Join(dumpDir, "keyboards", oldID)
		dumpBody, err := os.ReadFile(filepath.Join(itemDir, "item.json")) //nolint:gosec // dump dir.
		if err != nil {
			r.status, r.detail = statusMissing, "dump item.json unreadable: "+err.Error()
			out = append(out, r)
			continue
		}
		liveBody, err := client.getRaw(ctx, client.userPath("keyboards/"+mapped.NewID))
		if err != nil {
			r.status, r.detail = statusMissing, err.Error()
			out = append(out, r)
			continue
		}
		if ok, detail := compareScalars(dumpBody, liveBody, "images"); !ok {
			r.status, r.detail = statusFieldMismatch, detail
			out = append(out, r)
			continue
		}
		if st, detail := verifyArrayImages(ctx, itemDir, liveBody, mapped); st != statusOK {
			r.status, r.detail = st, detail
			out = append(out, r)
			continue
		}
		r.status = statusOK
		out = append(out, r)
	}
	return out
}

func verifySwitches(ctx context.Context, client *apiClient, dumpDir string, m *idMap) []verifyResult {
	var out []verifyResult
	for oldID, mapped := range m.Switches {
		r := verifyResult{entity: "switch", oldID: oldID, newID: mapped.NewID}
		itemDir := filepath.Join(dumpDir, "switches", oldID)
		dumpBody, err := os.ReadFile(filepath.Join(itemDir, "item.json")) //nolint:gosec // dump dir.
		if err != nil {
			r.status, r.detail = statusMissing, "dump item.json unreadable: "+err.Error()
			out = append(out, r)
			continue
		}
		liveBody, err := client.getRaw(ctx, client.userPath("switches/"+mapped.NewID))
		if err != nil {
			r.status, r.detail = statusMissing, err.Error()
			out = append(out, r)
			continue
		}
		if ok, detail := compareScalars(dumpBody, liveBody, "image"); !ok {
			r.status, r.detail = statusFieldMismatch, detail
			out = append(out, r)
			continue
		}
		if st, detail := verifySingleImage(ctx, itemDir, "image", topLevelImageURL(liveBody)); st != statusOK {
			r.status, r.detail = st, detail
			out = append(out, r)
			continue
		}
		r.status = statusOK
		out = append(out, r)
	}
	return out
}

func verifyKeycapSets(ctx context.Context, client *apiClient, dumpDir string, m *idMap) []verifyResult {
	var out []verifyResult
	for oldID, mapped := range m.KeycapSets {
		r := verifyResult{entity: "keycap_set", oldID: oldID, newID: mapped.NewID}
		setDir := filepath.Join(dumpDir, "keycap-sets", oldID)
		dumpBody, err := os.ReadFile(filepath.Join(setDir, "item.json")) //nolint:gosec // dump dir.
		if err != nil {
			r.status, r.detail = statusMissing, "dump item.json unreadable: "+err.Error()
			out = append(out, r)
			continue
		}
		liveBody, err := client.getRaw(ctx, client.userPath("keycap-sets/"+mapped.NewID))
		if err != nil {
			r.status, r.detail = statusMissing, err.Error()
			out = append(out, r)
			continue
		}
		// kits[], primary_kit_id, and order_status are all
		// server-managed/derived relative to KeycapSetInput.
		if ok, detail := compareScalars(dumpBody, liveBody, "kits", "primary_kit_id", "order_status"); !ok {
			r.status, r.detail = statusFieldMismatch, detail
			out = append(out, r)
			continue
		}
		if st, detail := verifyKitImages(ctx, setDir, liveBody, mapped); st != statusOK {
			r.status, r.detail = st, detail
			out = append(out, r)
			continue
		}
		r.status = statusOK
		out = append(out, r)
	}
	return out
}

func verifyBuilds(ctx context.Context, client *apiClient, dumpDir string, m *idMap) []verifyResult {
	var out []verifyResult
	for oldID, mapped := range m.Builds {
		r := verifyResult{entity: "build", oldID: oldID, newID: mapped.NewID}
		itemDir := filepath.Join(dumpDir, "builds", oldID)
		var dumpFull api.Build
		if err := readJSONFile(filepath.Join(itemDir, "item.json"), &dumpFull); err != nil {
			r.status, r.detail = statusMissing, "dump item.json unreadable: "+err.Error()
			out = append(out, r)
			continue
		}
		liveBody, err := client.getRaw(ctx, client.userPath("builds/"+mapped.NewID))
		if err != nil {
			r.status, r.detail = statusMissing, err.Error()
			out = append(out, r)
			continue
		}
		var liveFull api.Build
		if err := json.Unmarshal(liveBody, &liveFull); err != nil {
			r.status, r.detail = statusFieldMismatch, "live build unparseable: "+err.Error()
			out = append(out, r)
			continue
		}
		if detail := compareBuildRefs(dumpFull, liveFull, m); detail != "" {
			r.status, r.detail = statusFieldMismatch, detail
			out = append(out, r)
			continue
		}
		if st, detail := verifyArrayImages(ctx, itemDir, liveBody, mapped); st != statusOK {
			r.status, r.detail = st, detail
			out = append(out, r)
			continue
		}
		r.status = statusOK
		out = append(out, r)
	}
	return out
}

func verifyProfile(ctx context.Context, client *apiClient, dumpDir string) []verifyResult {
	profDir := filepath.Join(dumpDir, "profile")
	dumpBody, err := os.ReadFile(filepath.Join(profDir, "item.json")) //nolint:gosec // dump dir.
	if os.IsNotExist(err) {
		return nil
	}
	r := verifyResult{entity: "profile", oldID: "-", newID: client.subject}
	if err != nil {
		r.status, r.detail = statusMissing, err.Error()
		return []verifyResult{r}
	}
	liveBody, err := client.getRaw(ctx, "/v1/profile/"+client.subject)
	if err != nil {
		r.status, r.detail = statusMissing, err.Error()
		return []verifyResult{r}
	}
	if ok, detail := compareScalars(dumpBody, liveBody, "avatar"); !ok {
		r.status, r.detail = statusFieldMismatch, detail
		return []verifyResult{r}
	}
	if st, detail := verifySingleImage(ctx, profDir, "avatar", avatarURL(liveBody)); st != statusOK {
		r.status, r.detail = st, detail
		return []verifyResult{r}
	}
	r.status = statusOK
	return []verifyResult{r}
}

// compareBuildRefs checks that the live build's remapped references match what
// the id map says they should be. Returns "" on match.
func compareBuildRefs(dump, live api.Build, m *idMap) string {
	if dump.Keyboard != nil {
		want := ""
		if kb, ok := m.Keyboards[dump.Keyboard.Id]; ok {
			want = kb.NewID
		}
		got := ""
		if live.Keyboard != nil {
			got = live.Keyboard.Id
		}
		if want == "" || got != want {
			return fmt.Sprintf("keyboard ref: want %s, live has %s", want, got)
		}
	}

	dumpSwitches := derefSwitches(dump.Switches)
	liveSwitches := derefSwitches(live.Switches)
	if len(dumpSwitches) != len(liveSwitches) {
		return fmt.Sprintf("switch entry count: dump %d, live %d", len(dumpSwitches), len(liveSwitches))
	}
	for i := range dumpSwitches {
		want := ""
		if dumpSwitches[i].Switch != nil {
			if sw, ok := m.Switches[dumpSwitches[i].Switch.Id]; ok {
				want = sw.NewID
			}
		}
		got := ""
		if liveSwitches[i].Switch != nil {
			got = liveSwitches[i].Switch.Id
		}
		if want == "" || got != want {
			return fmt.Sprintf("switch[%d] ref: want %s, live has %s", i, want, got)
		}
		if dumpSwitches[i].Count != liveSwitches[i].Count {
			return fmt.Sprintf("switch[%d] count: dump %d, live %d", i, dumpSwitches[i].Count, liveSwitches[i].Count)
		}
	}

	dumpKits := derefKits(dump.KeycapKits)
	liveKits := derefKits(live.KeycapKits)
	if len(dumpKits) != len(liveKits) {
		return fmt.Sprintf("keycap kit entry count: dump %d, live %d", len(dumpKits), len(liveKits))
	}
	for i := range dumpKits {
		wantSet, wantKit := "", ""
		if dumpKits[i].KeycapSet != nil {
			if set, ok := m.KeycapSets[dumpKits[i].KeycapSet.Id]; ok {
				wantSet = set.NewID
				wantKit = set.Kits[dumpKits[i].KitId]
			}
		}
		gotSet := ""
		if liveKits[i].KeycapSet != nil {
			gotSet = liveKits[i].KeycapSet.Id
		}
		if wantSet == "" || gotSet != wantSet || wantKit == "" || liveKits[i].KitId != wantKit {
			return fmt.Sprintf("keycap_kit[%d] ref: want set %s kit %s, live has set %s kit %s", i, wantSet, wantKit, gotSet, liveKits[i].KitId)
		}
	}
	return ""
}

func derefSwitches(p *[]api.BuildSwitchEntryResolved) []api.BuildSwitchEntryResolved {
	if p == nil {
		return nil
	}
	return *p
}

func derefKits(p *[]api.BuildKeycapKitEntryResolved) []api.BuildKeycapKitEntryResolved {
	if p == nil {
		return nil
	}
	return *p
}

// ---- image verification ----

// verifyArrayImages checks that the live entity has one image per dump
// manifest entry, matched by the id map, each hash-matching the dump.
func verifyArrayImages(ctx context.Context, itemDir string, liveBody []byte, mapped *mappedEntity) (verifyStatus, string) {
	manifestPath := filepath.Join(itemDir, "images", "images.manifest.json")
	var manifest []imageManifestEntry
	if _, err := os.Stat(manifestPath); err == nil {
		if err := readJSONFile(manifestPath, &manifest); err != nil {
			return statusImageHashMismatch, err.Error()
		}
	}

	liveRefs, err := arrayImageRefs(liveBody)
	if err != nil {
		return statusImageHashMismatch, err.Error()
	}
	if len(liveRefs) != len(manifest) {
		return statusImageCountMismatch, fmt.Sprintf("dump has %d image(s), live has %d", len(manifest), len(liveRefs))
	}

	liveByID := map[string]string{}
	for _, ref := range liveRefs {
		liveByID[ref.imageID] = ref.url
	}
	for _, entry := range manifest {
		newID, ok := mapped.Images[entry.ImageID]
		if !ok {
			return statusImageHashMismatch, "id map has no restored id for dumped image " + entry.ImageID
		}
		url, ok := liveByID[newID]
		if !ok {
			return statusImageHashMismatch, "restored image " + newID + " is not on the live entity"
		}
		if st, detail := checkURLHash(ctx, url, entry.SHA256); st != statusOK {
			return st, detail
		}
	}
	return statusOK, ""
}

// verifyKitImages checks each dumped kit image against the live set.
func verifyKitImages(ctx context.Context, setDir string, liveBody []byte, mapped *mappedKeycaps) (verifyStatus, string) {
	kitsDir := filepath.Join(setDir, "kits")
	entries, err := os.ReadDir(kitsDir)
	if os.IsNotExist(err) {
		return statusOK, ""
	}
	if err != nil {
		return statusImageHashMismatch, err.Error()
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		oldKitID := e.Name()
		manifestPath := filepath.Join(kitsDir, oldKitID, "image.manifest.json")
		if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
			continue
		}
		var entry imageManifestEntry
		if err := readJSONFile(manifestPath, &entry); err != nil {
			return statusImageHashMismatch, err.Error()
		}
		newKitID, ok := mapped.Kits[oldKitID]
		if !ok {
			return statusImageHashMismatch, "id map has no restored kit id for " + oldKitID
		}
		url, err := kitImageURL(liveBody, newKitID)
		if err != nil {
			return statusImageHashMismatch, err.Error()
		}
		if st, detail := checkURLHash(ctx, url, entry.SHA256); st != statusOK {
			return st, detail
		}
	}
	return statusOK, ""
}

// verifySingleImage checks a single-slot image (switch/profile) against
// liveURL. name is "image" or "avatar".
func verifySingleImage(ctx context.Context, dir, name, liveURL string) (verifyStatus, string) {
	manifestPath := filepath.Join(dir, name+".manifest.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		if liveURL != "" {
			return statusImageCountMismatch, "dump has no " + name + " but live does"
		}
		return statusOK, ""
	}
	var entry imageManifestEntry
	if err := readJSONFile(manifestPath, &entry); err != nil {
		return statusImageHashMismatch, err.Error()
	}
	if liveURL == "" {
		return statusImageCountMismatch, "dump has " + name + " but live does not"
	}
	return checkURLHash(ctx, liveURL, entry.SHA256)
}

// checkURLHash downloads url and compares the bytes' SHA-256 to wantSHA.
func checkURLHash(ctx context.Context, url, wantSHA string) (verifyStatus, string) {
	data, _, err := downloadImage(ctx, url)
	if err != nil {
		return statusImageHashMismatch, "download failed: " + err.Error()
	}
	if got := sha256Hex(data); got != wantSHA {
		return statusImageHashMismatch, fmt.Sprintf("want %s, got %s", wantSHA, got)
	}
	return statusOK, ""
}
