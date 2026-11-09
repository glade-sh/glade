package vm

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// projectReportPermissionFingerprint is deliberately limited to captured
// effective sets with no unobserved permission overlays.
func (vm *VM) projectReportPermissionFingerprint(user Value) (string, bool) {
	if vm.Org == nil || vm.currentUserProfileName(user) != "Standard User" {
		return "", false
	}
	sets := vm.assignedPermissionSetIDs(vm.currentUserID())
	profileID := stringField(user, "ProfileId")
	profileSet := vm.profileOwnedPermissionSetID(profileID)
	for _, field := range []string{"Permissions", "PermissionSets"} {
		if _, v, ok := objectFieldValue(user, field); ok && v.Kind != ValueNull {
			return "", false
		}
	}
	seed, ok := storage.SeededProfilePermissions("Standard User")
	if !ok {
		return "", false
	}
	if vm.projectReportDefaultStandardProfile(seed, profileID, profileSet, sets) {
		value := reportCapturedPermissionFingerprints()["plain"]
		return value, value != ""
	}
	if vm.projectReportHasPermissionOverlays(seed, profileID, profileSet, sets) {
		return "", false
	}
	if profileSet != "" {
		sets = append(sets, profileSet)
	}
	flags := map[string]struct{}{}
	for _, id := range sets {
		_, record, found := storage.LookupRecordByID(vm.Org.Objects["PermissionSet"].Records, storage.ID(id))
		if !found {
			return "", false
		}
		for name, value := range record.Fields {
			if strings.EqualFold(name, "Permissions") && value.Kind != storage.ValueNull {
				return "", false
			}
			if strings.HasPrefix(strings.ToLower(name), "permissions") && value.Kind == storage.ValueBoolean && value.Boolean {
				flags[strings.ToLower(name)] = struct{}{}
			}
		}
	}
	names := make([]string, 0, len(flags))
	for name := range flags {
		names = append(names, name)
	}
	sort.Strings(names)
	sum := sha256.Sum256([]byte(strings.Join(names, "\n")))
	return hex.EncodeToString(sum[:]), true
}

// The native permission snapshots capture a plain field/object-permission
// configuration. A local overlay is outside that evidence even when the
// boolean system-permission fingerprint happens to be identical. The rows the
// deterministic seed gives the profile are not an overlay; anything else on
// the profile, its set or an assigned set is.
func (vm *VM) projectReportHasPermissionOverlays(seed storage.ProfilePermissionSeed, profileID, profileSet string, assignments []string) bool {
	if !vm.projectReportProfileRowsMatchSeed(seed, profileID, profileSet) {
		return true
	}
	assigned := func(value storage.Value) bool {
		id := storage.ID(storageValueIDText(value))
		for _, assignment := range assignments {
			if !storage.IDsEqual(storage.ID(assignment), storage.ID(profileSet)) && storage.IDsEqual(id, storage.ID(assignment)) {
				return true
			}
		}
		return false
	}
	for _, object := range []string{"FieldPermissions", "ObjectPermissions"} {
		for _, permission := range vm.Org.Objects[object].Records {
			if permission.System.IsDeleted {
				continue
			}
			if parent, ok := permission.GetField("ParentId"); ok && assigned(parent) {
				return true
			}
		}
	}
	return false
}

// The runtime's deterministic Standard User seed represents the unmodified
// built-in profile: the captured profile-owned set with its captured Account
// and Contact object and field rows. Two fresh-org captures establish that
// this plain profile has the recorded effective set. Any local grant,
// assignment or field-permission override is rejected here.
func (vm *VM) projectReportDefaultStandardProfile(seed storage.ProfilePermissionSeed, profileID, profileSet string, assignments []string) bool {
	if profileSet == "" {
		return false
	}
	for _, id := range assignments {
		if !storage.IDsEqual(storage.ID(id), storage.ID(profileSet)) {
			return false
		}
	}
	_, record, found := storage.LookupRecordByID(vm.Org.Objects["PermissionSet"].Records, storage.ID(profileSet))
	if !found || !projectReportRecordMatchesSeed(record, seed.Set) {
		return false
	}
	value, ok := record.GetField("ProfileId")
	if !ok || !storage.IDsEqual(storage.ID(storageValueIDText(value)), storage.ID(profileID)) {
		return false
	}
	return vm.projectReportProfileRowsMatchSeed(seed, profileID, profileSet)
}

// projectReportProfileRowsMatchSeed reports whether the ObjectPermissions and
// FieldPermissions rows of the profile and its set are exactly the seed's:
// the same rows, with the same values, and no others. Explicit denials are
// extra rows or changed flags, so they fail the comparison too. An org without
// the platform seed (no rows at all for the profile or its set) is still the
// plain profile, as it was before the rows were seeded.
func (vm *VM) projectReportProfileRowsMatchSeed(seed storage.ProfilePermissionSeed, profileID, profileSet string) bool {
	total := 0
	owned := func(record storage.Record) bool {
		parent, ok := record.GetField("ParentId")
		if !ok {
			return false
		}
		id := storage.ID(storageValueIDText(parent))
		return storage.IDsEqual(id, storage.ID(profileID)) || (profileSet != "" && storage.IDsEqual(id, storage.ID(profileSet)))
	}
	for _, kind := range []struct {
		object, key string
		seed        map[string]storage.Record
	}{{"ObjectPermissions", "SobjectType", seed.ObjectPermissions}, {"FieldPermissions", "Field", seed.FieldPermissions}} {
		seen := 0
		for _, permission := range vm.Org.Objects[kind.object].Records {
			if permission.System.IsDeleted || !owned(permission) {
				continue
			}
			key, ok := permission.GetField(kind.key)
			if !ok || key.Kind != storage.ValueString {
				return false
			}
			want, ok := kind.seed[strings.ToLower(key.String)]
			if !ok || !projectReportRecordMatchesSeed(permission, want) {
				return false
			}
			seen++
		}
		if seen != len(kind.seed) && seen != 0 {
			return false
		}
		total += seen
	}
	return total == 0 || total == len(seed.ObjectPermissions)+len(seed.FieldPermissions)
}

func projectReportRecordMatchesSeed(record, seed storage.Record) bool {
	if len(record.Fields) != len(seed.Fields) {
		return false
	}
	for name, want := range seed.Fields {
		value, ok := record.GetField(name)
		if !ok || !storage.ValuesEqual(value, want) {
			return false
		}
	}
	return true
}

func reportCapturedPermissionFingerprints() map[string]string {
	data, err := standardReportCatalogs.ReadFile("report_catalog_templates/security-fingerprints.json")
	if err != nil {
		return nil
	}
	var captured map[string]string
	if json.Unmarshal(data, &captured) != nil {
		return nil
	}
	return captured
}

func (vm *VM) projectReportCapturedPermissionDenied(user Value) bool {
	if vm.testContext != nil {
		return false
	}
	value, ok := vm.projectReportPermissionFingerprint(user)
	if !ok {
		return false
	}
	for _, candidate := range reportCapturedPermissionFingerprints() {
		if candidate == value {
			return true
		}
	}
	return false
}

// A standard user with the captured plain profile has the same catalog in both
// isolated and SeeAllData tests. Other permissions or custom fields need their
// own native catalog before this path can expose one.
func (vm *VM) projectReportCapturedStandardCatalog(user Value) bool {
	if vm.testContext == nil {
		return false
	}
	value, ok := vm.projectReportPermissionFingerprint(user)
	if !ok || value != reportCapturedPermissionFingerprints()["plain"] {
		return false
	}
	for _, object := range vm.Org.Objects {
		for name := range object.Definition.Fields {
			if strings.HasSuffix(strings.ToLower(name), "__c") {
				return false
			}
		}
	}
	return true
}
