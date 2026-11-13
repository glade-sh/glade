package storage

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

//go:embed seeddata/permission_inventory.json
var standardPermissionInventoryJSON []byte

type capturedPermissionInventory struct {
	Licenses []struct {
		Kind, Name, Key string
	} `json:"licenses"`
	PermissionSets []struct {
		Name, Label, Type, Namespace, Profile string
		UserType                              string
		Custom, APIOnly, ModifyAll, ViewAll   bool
		License                               int
	} `json:"permissionSets"`
}

var loadStandardPermissionInventory = sync.OnceValue(func() capturedPermissionInventory {
	var inventory capturedPermissionInventory
	if err := json.Unmarshal(standardPermissionInventoryJSON, &inventory); err != nil {
		panic("invalid embedded permission inventory: " + err.Error())
	}
	return inventory
})

// seededInventoryID names a row of the captured standard inventory. The body
// "0j0000G" sorts after counter-style fixture IDs (body 0000...) and before
// runtime IDs (body offset 36^11, first digit 1, uppercase base36), so the
// unordered Id order matches native: standard rows precede rows a test
// creates. The lowercase j never occurs in a generated ID, so it cannot
// collide with either.
func seededInventoryID(prefix string, n int) ID {
	return ID(fmt.Sprintf("%s0j0000G%05d", prefix, n))
}

// seedStandardPermissionInventory installs the captured Developer Edition
// inventory. License and profile relationships are local, not captured IDs.
func seedStandardPermissionInventory(org *OrgState) map[string]ID {
	inventory := loadStandardPermissionInventory()
	// ensureDefaultPackageLicenses installs the PermissionSetLicense object.
	// Default setup licenses and profiles keep their counter-style identities.
	ensureDefaultCommunityLicense(org)
	ensureDefaultPackageLicenses(org)
	licenseIDs := make([]ID, len(inventory.Licenses))
	usedLicenses := map[ID]bool{}
	for i, license := range inventory.Licenses {
		state := org.Objects[license.Kind]
		nameField, keyField := "Name", "LicenseDefinitionKey"
		prefix := "100"
		if license.Kind == "PermissionSetLicense" {
			nameField, keyField, prefix = "MasterLabel", "DeveloperName", "0PL"
		}
		ids := make([]ID, 0, len(state.Records))
		for id := range state.Records {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		for _, id := range ids {
			record := state.Records[id]
			if !usedLicenses[id] && strings.EqualFold(record.Fields[keyField].String, license.Key) {
				licenseIDs[i] = id
				break
			}
		}
		if licenseIDs[i] == "" {
			licenseIDs[i] = ID(seededInventoryID(prefix, i+1))
			putSeedRecord(org, license.Kind, Record{ID: licenseIDs[i], Object: license.Kind, Fields: map[string]Value{nameField: StringValue(license.Name), keyField: StringValue(license.Key)}})
		}
		usedLicenses[licenseIDs[i]] = true
	}
	profileSets := map[string]ID{}
	usedProfiles := map[ID]bool{}
	profileRecordsChanged := false
	for i, seed := range inventory.PermissionSets {
		id := ID(seededInventoryID("0PS", i+1))
		fields := map[string]Value{
			"Name": StringValue(seed.Name), "Label": StringValue(seed.Label), "Type": StringValue(seed.Type),
			"IsCustom": BooleanValue(seed.Custom), "IsOwnedByProfile": BooleanValue(seed.Profile != ""),
			"PermissionsApiUserOnly": BooleanValue(seed.APIOnly), "PermissionsModifyAllData": BooleanValue(seed.ModifyAll),
			"PermissionsViewAllData": BooleanValue(seed.ViewAll), "LicenseId": IDValue(licenseIDs[seed.License]),
			"NamespacePrefix": NullValue(), "PermissionSetGroupId": NullValue(),
		}
		if seed.Namespace != "" {
			fields["NamespacePrefix"] = StringValue(seed.Namespace)
		}
		if seed.Profile == "" {
			// An existing standard set (for example one imported from an org)
			// keeps its own identity; seeding never adds a second row.
			if existing, ok := findNamedPermissionSet(org, seed.Name, seed.Namespace); ok {
				id = existing
			}
		}
		if seed.Profile != "" {
			profiles := org.Objects["Profile"].Records
			ids := make([]ID, 0, len(profiles))
			for profileID := range profiles {
				ids = append(ids, profileID)
			}
			sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
			var profileID ID
			for _, candidate := range ids {
				if !usedProfiles[candidate] && profiles[candidate].Fields["Name"].String == seed.Profile {
					profileID = candidate
					break
				}
			}
			if profileID == "" {
				profileID = ID(seededInventoryID("00e", i+1))
				putSeedRecord(org, "Profile", Record{ID: profileID, Object: "Profile", Fields: map[string]Value{"Name": StringValue(seed.Profile), "UserLicenseId": IDValue(licenseIDs[seed.License])}})
				profileRecordsChanged = true
			} else if capturedProfileInventoryShape(org.Features) {
				profile := profiles[profileID]
				if untouchedBootstrapProfile(profileID, profile) && !IDsEqual(profile.Fields["UserLicenseId"].ID, licenseIDs[seed.License]) {
					// Repair only an untouched reserved bootstrap profile. User edits
					// and profiles outside the captured shapes keep their license.
					mutable, _ := EnsureMutableObjectRecords(org, "Profile")
					profile = mutable.Records[profileID].Clone()
					profile.Fields["UserLicenseId"] = IDValue(licenseIDs[seed.License])
					mutable.PutRecord(profileID, profile)
					profileRecordsChanged = true
				}
			}
			// Profile.UserType from the fresh-org capture (profile_usertype).
			// putSeedRecord fills absent fields only, so a profile seeded or
			// imported with its own UserType keeps it.
			if seed.UserType != "" {
				if _, exists := org.Objects["Profile"].Records[profileID].Fields["UserType"]; !exists {
					profileRecordsChanged = true
				}
				putSeedRecord(org, "Profile", Record{ID: profileID, Object: "Profile", Fields: map[string]Value{"UserType": StringValue(seed.UserType)}})
			}
			usedProfiles[profileID] = true
			fields["ProfileId"] = IDValue(profileID)
			if seed.Name == "" {
				fields["Name"], fields["Label"] = StringValue("X"+ID18(string(profileID))), StringValue(ID18(string(profileID)))
			}
			applyCapturedProfilePermissions(fields, seed.Profile)
			if seed.Profile == "System Administrator" {
				fields["PermissionsInstallPackaging"] = BooleanValue(true)
			}
			profileSets[seed.Profile] = id
		}
		putSeedRecord(org, "PermissionSet", Record{ID: id, Object: "PermissionSet", Fields: fields})
	}
	pruneUncapturedBootstrapProfiles(org)
	if profileRecordsChanged && len(org.Objects["Profile"].Definition.Indexes) != 0 {
		RebuildObjectIndexes(org, "Profile")
	}
	return profileSets
}

//go:embed seeddata/profile_field_permissions.json
var standardProfileFieldPermissionsJSON []byte

type capturedProfileFieldPermission struct {
	Profile, Object, Field string
	Read, Edit             bool
}

var loadStandardProfileFieldPermissions = sync.OnceValue(func() []capturedProfileFieldPermission {
	var rows []capturedProfileFieldPermission
	if err := json.Unmarshal(standardProfileFieldPermissionsJSON, &rows); err != nil {
		panic("invalid embedded profile field permissions: " + err.Error())
	}
	return rows
})

// seedStandardProfileFieldPermissions materializes the queried default field
// junctions for the two captured built-in profiles.
func seedStandardProfileFieldPermissions(org *OrgState, parents map[string]ID) {
	for i, row := range loadStandardProfileFieldPermissions() {
		id := ID(seededInventoryID("0FP", i+1))
		putSeedRecord(org, "FieldPermissions", Record{ID: id, Object: "FieldPermissions", Fields: map[string]Value{
			"ParentId": IDValue(parents[row.Profile]), "SobjectType": StringValue(row.Object), "Field": StringValue(row.Field),
			"PermissionsRead": BooleanValue(row.Read), "PermissionsEdit": BooleanValue(row.Edit),
		}})
	}
}

func findNamedPermissionSet(org *OrgState, name, namespace string) (ID, bool) {
	var found ID
	for id, record := range org.Objects["PermissionSet"].Records {
		if record.Fields["IsOwnedByProfile"].Boolean || record.Fields["Name"].String != name || record.Fields["NamespacePrefix"].String != namespace {
			continue
		}
		if found == "" || id < found {
			found = id
		}
	}
	return found, found != ""
}

// migrateLegacyPermissionInventory repoints rows persisted by earlier builds,
// which seeded counter-style permission set IDs, to the captured inventory.
// Only rows whose ID and identity both match the former seed are migrated.
// Every field in the org that references PermissionSet is repointed. The two
// synthetic sets (GladeBaseline, Proving_With_User_Mode_Works) have no native
// equivalent: they are dropped when nothing references them and kept otherwise,
// so assignments and setup-entity grants a user created on them survive.
func migrateLegacyPermissionInventory(org *OrgState, profileSets map[string]ID) {
	permissionSets := org.Objects["PermissionSet"]
	sets := permissionSets.Records
	remap := map[ID]ID{}
	synthetic := map[ID]bool{}
	for i := 1; i <= 11; i++ {
		legacy := ID(fmt.Sprintf("0PS%012d", i))
		record, ok := sets[legacy]
		if !ok {
			continue
		}
		name := record.Fields["Name"].String
		switch {
		case i == 1 && name == "GladeBaseline", i == 2 && name == "Proving_With_User_Mode_Works":
			synthetic[legacy] = true
		case i == 11 && name == "CommerceUser" && record.Fields["NamespacePrefix"].String == "":
			if target, ok := findNamedPermissionSet(org, "CommerceUser", "force"); ok {
				remap[legacy] = target
			}
		case i >= 3 && i <= 10 && record.Fields["IsOwnedByProfile"].Boolean:
			profile := org.Objects["Profile"].Records[record.Fields["ProfileId"].ID]
			if target, ok := profileSets[profile.Fields["Name"].String]; ok && name == "X"+string(record.Fields["ProfileId"].ID) {
				remap[legacy] = target
			}
		}
	}
	if len(remap) == 0 && len(synthetic) == 0 {
		return
	}
	references := permissionSetReferenceFields(org)
	referenced := map[ID]bool{}
	for _, reference := range references {
		for _, record := range org.Objects[reference.object].Records {
			if parent, ok := legacyPermissionSetID(record.Fields[reference.field], synthetic); ok {
				referenced[parent] = true
			}
		}
	}
	for legacy := range synthetic {
		if !referenced[legacy] {
			permissionSets.DeleteRecord(legacy)
		}
	}
	for legacy := range remap {
		permissionSets.DeleteRecord(legacy)
	}
	for _, reference := range references {
		state := org.Objects[reference.object]
		assigned := map[string]bool{}
		if reference.object == "PermissionSetAssignment" {
			for _, record := range state.Records {
				assigned[string(record.Fields["AssigneeId"].ID)+"/"+string(record.Fields["PermissionSetId"].ID)] = true
			}
		}
		for id, record := range state.Records {
			legacy, ok := legacyPermissionSetID(record.Fields[reference.field], remap)
			if !ok {
				continue
			}
			target := remap[legacy]
			if reference.object == "PermissionSetAssignment" && assigned[string(record.Fields["AssigneeId"].ID)+"/"+string(target)] {
				state.DeleteRecord(id)
				continue
			}
			record.Fields[reference.field] = IDValue(target)
			state.PutRecord(id, record)
		}
	}
	// The former seed gave the Marketing User Account row PermissionsDelete
	// false. The native profile_account_contact_rows capture says true.
	if record, ok := org.Objects["ObjectPermissions"].Records["110000000000006"]; ok {
		marketing, seeded := profileSets["Marketing User"]
		if seeded && IDsEqual(record.Fields["ParentId"].ID, marketing) && strings.EqualFold(record.Fields["SobjectType"].String, "Account") {
			if value, ok := record.Fields["PermissionsDelete"]; ok && value.Kind == ValueBoolean && !value.Boolean {
				record.Fields["PermissionsDelete"] = BooleanValue(true)
				objectPermissions := org.Objects["ObjectPermissions"]
				objectPermissions.PutRecord("110000000000006", record)
			}
		}
	}
}

type permissionSetReference struct {
	object, field string
}

// permissionSetReferenceFields lists every field of the org whose lookup
// targets PermissionSet (PermissionSetAssignment.PermissionSetId, the ParentId
// of ObjectPermissions, FieldPermissions, SetupEntityAccess and
// PermissionSetTabSetting, PermissionSetGroupComponent.PermissionSetId, ...).
func permissionSetReferenceFields(org *OrgState) []permissionSetReference {
	var references []permissionSetReference
	for objectName, state := range org.Objects {
		if len(state.Records) == 0 || strings.EqualFold(objectName, "PermissionSet") {
			continue
		}
		for fieldName, field := range state.Definition.Fields {
			if field.Type != FieldReference || !referencesObject(field.ReferenceTo, "PermissionSet") {
				continue
			}
			references = append(references, permissionSetReference{object: objectName, field: fieldName})
		}
	}
	sort.Slice(references, func(i, j int) bool {
		if references[i].object != references[j].object {
			return references[i].object < references[j].object
		}
		return references[i].field < references[j].field
	})
	return references
}

// legacyPermissionSetID returns the legacy key a reference value names, for
// 15- or 18-character forms.
func legacyPermissionSetID[T any](value Value, legacy map[ID]T) (ID, bool) {
	if value.Kind != ValueID && value.Kind != ValueString {
		return "", false
	}
	text := string(value.ID)
	if text == "" {
		text = value.String
	}
	if len(text) > 15 {
		text = text[:15]
	}
	if _, ok := legacy[ID(text)]; ok {
		return ID(text), true
	}
	return "", false
}

//go:embed seeddata/profile_object_permissions.json
var standardProfileObjectPermissionsJSON []byte

//go:embed seeddata/object_permission_requirements.json
var objectPermissionRequirementsJSON []byte

type capturedObjectPermissionRequirement struct {
	Object    string
	Method    string
	UserField string
}

var loadObjectPermissionRequirements = sync.OnceValue(func() []capturedObjectPermissionRequirement {
	var rows []capturedObjectPermissionRequirement
	if err := json.Unmarshal(objectPermissionRequirementsJSON, &rows); err != nil {
		panic("invalid embedded object permission requirements: " + err.Error())
	}
	return rows
})

// ObjectPermissionUserField returns a captured user-field prerequisite for an
// object operation, or an empty string when no prerequisite was captured.
func ObjectPermissionUserField(object, method string) string {
	for _, row := range loadObjectPermissionRequirements() {
		if strings.EqualFold(row.Object, object) && strings.EqualFold(row.Method, method) {
			return row.UserField
		}
	}
	return ""
}

type capturedProfileObjectPermission struct {
	Profile, Object string
	// Read, Create, Edit, Delete, ViewAllRecords, ModifyAllRecords.
	Flags [6]bool
}

var loadStandardProfileObjectPermissions = sync.OnceValue(func() []capturedProfileObjectPermission {
	var rows []capturedProfileObjectPermission
	if err := json.Unmarshal(standardProfileObjectPermissionsJSON, &rows); err != nil {
		panic("invalid embedded profile object permissions: " + err.Error())
	}
	return rows
})

// seedStandardProfileObjectPermissions materializes the captured Account and
// Contact rows of every profile-owned set. Rows already seeded for the same
// parent and object keep their IDs and gain any missing flag.
func seedStandardProfileObjectPermissions(org *OrgState, parents map[string]ID) {
	existing := map[string]ID{}
	for id, record := range org.Objects["ObjectPermissions"].Records {
		existing[string(record.Fields["ParentId"].ID)+"/"+record.Fields["SobjectType"].String] = id
	}
	for i, row := range loadStandardProfileObjectPermissions() {
		parent, ok := parents[row.Profile]
		if !ok {
			continue
		}
		id, found := existing[string(parent)+"/"+row.Object]
		if !found {
			id = ID(seededInventoryID("110", i+1))
		}
		putSeedRecord(org, "ObjectPermissions", Record{ID: id, Object: "ObjectPermissions", Fields: map[string]Value{
			"ParentId": IDValue(parent), "SobjectType": StringValue(row.Object),
			"PermissionsRead": BooleanValue(row.Flags[0]), "PermissionsCreate": BooleanValue(row.Flags[1]),
			"PermissionsEdit": BooleanValue(row.Flags[2]), "PermissionsDelete": BooleanValue(row.Flags[3]),
			"PermissionsViewAllRecords": BooleanValue(row.Flags[4]), "PermissionsModifyAllRecords": BooleanValue(row.Flags[5]),
		}})
	}
}

func referencesObject(targets []string, want string) bool {
	for _, target := range targets {
		if strings.EqualFold(target, want) {
			return true
		}
	}
	return false
}

// ProfilePermissionSeed is what the deterministic seed gives one standard
// profile: its profile-owned permission set and the ObjectPermissions and
// FieldPermissions rows whose parent is that set or the profile itself.
// ObjectPermissions is keyed by lower-case SobjectType, FieldPermissions by
// lower-case Field.
type ProfilePermissionSeed struct {
	Set               Record
	ObjectPermissions map[string]Record
	FieldPermissions  map[string]Record
}

var seededPlatformOrg = sync.OnceValue(func() *OrgState {
	org := NewOrgState()
	EnsureDeterministicPlatformData(&org)
	return &org
})

// SeededProfilePermissions returns the untouched seed state of the named
// standard profile, so a runtime can tell a project override from the seed.
func SeededProfilePermissions(profileName string) (ProfilePermissionSeed, bool) {
	org := seededPlatformOrg()
	var profileID ID
	for id, record := range org.Objects["Profile"].Records {
		if record.Fields["Name"].String == profileName && (profileID == "" || id < profileID) {
			profileID = id
		}
	}
	if profileID == "" {
		return ProfilePermissionSeed{}, false
	}
	var setID ID
	seed := ProfilePermissionSeed{ObjectPermissions: map[string]Record{}, FieldPermissions: map[string]Record{}}
	for id, record := range org.Objects["PermissionSet"].Records {
		if record.Fields["IsOwnedByProfile"].Boolean && IDsEqual(record.Fields["ProfileId"].ID, profileID) {
			setID, seed.Set = id, record.Clone()
			break
		}
	}
	if setID == "" {
		return ProfilePermissionSeed{}, false
	}
	owned := func(record Record) bool {
		parent, ok := record.GetField("ParentId")
		return ok && (IDsEqual(parent.ID, setID) || IDsEqual(parent.ID, profileID))
	}
	for _, record := range org.Objects["ObjectPermissions"].Records {
		if owned(record) {
			object, _ := record.GetField("SobjectType")
			seed.ObjectPermissions[strings.ToLower(object.String)] = record.Clone()
		}
	}
	for _, record := range org.Objects["FieldPermissions"].Records {
		if owned(record) {
			field, _ := record.GetField("Field")
			seed.FieldPermissions[strings.ToLower(field.String)] = record.Clone()
		}
	}
	return seed, true
}

// inventoryUserLicenseSeedID is the ID ensureInventoryUserLicense gives the
// captured UserLicense with this key when no persisted record occupies it.
func inventoryUserLicenseSeedID(key string) ID {
	for i, license := range loadStandardPermissionInventory().Licenses {
		if license.Kind == "UserLicense" && strings.EqualFold(license.Key, key) {
			return seededInventoryID("100", i+1)
		}
	}
	panic("captured permission inventory has no UserLicense " + key)
}

// ensureInventoryUserLicense returns the UserLicense with the given key,
// seeding it at the ID the captured inventory assigns to that key, so the
// inventory pass reuses the same row.
func ensureInventoryUserLicense(org *OrgState, name, key string) ID {
	EnsureStandardObject(org, "UserLicense")
	ids := make([]ID, 0, len(org.Objects["UserLicense"].Records))
	for id := range org.Objects["UserLicense"].Records {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		if strings.EqualFold(org.Objects["UserLicense"].Records[id].Fields["LicenseDefinitionKey"].String, key) {
			return id
		}
	}
	id := inventoryUserLicenseSeedID(key)
	// A persisted license with another key may occupy the inventory ID; take
	// the next free one. The inventory pass finds the license by key.
	for n := 1; ; n++ {
		if _, taken := org.Objects["UserLicense"].Records[id]; !taken {
			break
		}
		id = seededInventoryID("100", 50000+n)
	}
	putSeedRecord(org, "UserLicense", Record{ID: id, Object: "UserLicense", Fields: map[string]Value{
		"Name": StringValue(name), "LicenseDefinitionKey": StringValue(key),
	}})
	return id
}

// migrateLegacyLicenseSeeds repairs orgs persisted by builds that named the
// PID_Customer_Community_Login license "Chatter External", linked Chatter
// External User and Standard Platform User to the wrong license, and seeded a
// "Customer Community Guest User" profile the fresh org does not have
// (profile_usertype). Only rows that still carry the former seed are changed;
// the guest profile is kept while a User references it.
func migrateLegacyLicenseSeeds(org *OrgState, salesforceLicenseID, communityLoginLicenseID, chatterExternalLicenseID, platformLicenseID ID) {
	licenses := org.Objects["UserLicense"]
	if license, ok := licenses.Records[communityLoginLicenseID]; ok &&
		license.Fields["Name"].String == "Chatter External" && license.Fields["LicenseDefinitionKey"].String == "PID_Customer_Community_Login" {
		license.Fields["Name"] = StringValue("Customer Community Login")
		licenses.PutRecord(communityLoginLicenseID, license)
	}
	profiles := org.Objects["Profile"]
	for _, legacy := range []struct {
		id         ID
		name       string
		from, into ID
	}{
		{ID("00e000000000003"), "Chatter External User", communityLoginLicenseID, chatterExternalLicenseID},
		{ID("00e000000000004"), "Standard Platform User", salesforceLicenseID, platformLicenseID},
	} {
		if profile, ok := profiles.Records[legacy.id]; ok && profile.Fields["Name"].String == legacy.name && IDsEqual(profile.Fields["UserLicenseId"].ID, legacy.from) {
			profile.Fields["UserLicenseId"] = IDValue(legacy.into)
			profiles.PutRecord(legacy.id, profile)
		}
	}
	guestID := ID("00e000000000007")
	if profile, ok := profiles.Records[guestID]; ok && legacyGuestProfileUntouched(org, profile, communityLoginLicenseID) {
		permissionSets := org.Objects["PermissionSet"]
		for id, set := range permissionSets.Records {
			if IDsEqual(set.Fields["ProfileId"].ID, guestID) {
				permissionSets.DeleteRecord(id)
			}
		}
		profiles.DeleteRecord(guestID)
	}
}

// legacyGuestProfileUntouched reports whether the former seeded guest profile
// still holds exactly its seeded data and nothing else refers to it. A changed
// license or permission field, or a reference from any record other than its
// own profile-owned permission set, keeps the profile.
func legacyGuestProfileUntouched(org *OrgState, profile Record, seededLicenseID ID) bool {
	if profile.Fields["Name"].String != "Customer Community Guest User" ||
		profile.Fields["UserType"].String != "Guest" ||
		!IDsEqual(profile.Fields["UserLicenseId"].ID, seededLicenseID) {
		return false
	}
	for field, value := range profile.Fields {
		if strings.HasPrefix(field, "Permissions") && value.Boolean {
			return false
		}
	}
	for objectName, object := range org.Objects {
		for id, record := range object.Records {
			if objectName == "Profile" && IDsEqual(id, profile.ID) {
				continue
			}
			owned := objectName == "PermissionSet" && IDsEqual(record.Fields["ProfileId"].ID, profile.ID) && record.Fields["IsOwnedByProfile"].Boolean
			for field, value := range record.Fields {
				if owned && strings.HasPrefix(field, "Permissions") && value.Boolean {
					return false
				}
				if IDsEqual(value.ID, profile.ID) && !(owned && field == "ProfileId") {
					return false
				}
			}
		}
	}
	return true
}
