package storage

import (
	"reflect"
	"strings"

	"github.com/glade-sh/glade/internal/project"
)

// The profile inventory was captured in plain Developer orgs, with the
// Communities setting alone and with the feature plus setting. Other shapes
// retain their existing bootstrap records.
func capturedProfileInventoryShape(features []string) bool {
	developer, communities, networks := false, false, false
	for _, feature := range features {
		switch canonicalFeatureName(feature) {
		case project.StandardFieldEditionDeveloper:
			developer = true
		case project.PlainDeveloperObjectShape:
		case "Communities":
			communities = true
		case "NetworksEnabled":
			networks = true
		default:
			return false
		}
	}
	return developer && (!communities || networks)
}

// An untouched bootstrap record has a reserved synthetic identity and exactly
// the fields the seed created. A project-modified record is never migrated.
func untouchedBootstrapProfile(id ID, record Record) bool {
	original := bootstrapProfileSeedFields(id)
	return original != nil && reflect.DeepEqual(record.Fields, original)
}

// reconcileBootstrapProfileLicenses applies captured inventory links to an
// already-seeded org when its project shape becomes known. A record must still
// match its complete original seed; imported and edited profiles keep their
// own links. Missing license data leaves the record untouched until seeding.
func reconcileBootstrapProfileLicenses(org *OrgState) {
	if org == nil || !capturedProfileInventoryShape(org.Features) {
		return
	}
	profiles, profilesOK := org.Objects["Profile"]
	licenses, licensesOK := org.Objects["UserLicense"]
	if !profilesOK || !licensesOK {
		return
	}
	inventory := loadStandardPermissionInventory()
	licenseIDs := make(map[string]ID)
	for id, record := range licenses.Records {
		key := strings.ToLower(record.Fields["LicenseDefinitionKey"].String)
		if key != "" {
			if prior := licenseIDs[key]; prior == "" || id < prior {
				licenseIDs[key] = id
			}
		}
	}
	expected := make(map[string]ID)
	for _, seed := range inventory.PermissionSets {
		if seed.Profile == "" || seed.License < 0 || seed.License >= len(inventory.Licenses) {
			continue
		}
		license := inventory.Licenses[seed.License]
		if license.Kind == "UserLicense" {
			if id := licenseIDs[strings.ToLower(license.Key)]; id != "" {
				expected[seed.Profile] = id
			}
		}
	}
	changed := false
	for id, profile := range profiles.Records {
		license, ok := expected[profile.Fields["Name"].String]
		if !ok || IDsEqual(profile.Fields["UserLicenseId"].ID, license) || !untouchedBootstrapProfile(id, profile) {
			continue
		}
		mutable, _ := EnsureMutableObjectRecords(org, "Profile")
		profile = mutable.Records[id].Clone()
		profile.Fields["UserLicenseId"] = IDValue(license)
		mutable.Records[id] = profile
		changed = true
	}
	if changed && len(org.Objects["Profile"].Definition.Indexes) != 0 {
		RebuildObjectIndexes(org, "Profile")
	}
}

func pruneUncapturedBootstrapProfiles(org *OrgState) {
	if org == nil || !capturedProfileInventoryShape(org.Features) {
		return
	}
	captured := map[string]bool{}
	for _, set := range loadStandardPermissionInventory().PermissionSets {
		if set.Profile != "" {
			captured[set.Profile] = true
		}
	}
	state, ok := org.Objects["Profile"]
	if !ok {
		return
	}
	changed := false
	for id, profile := range state.Records {
		if captured[profile.Fields["Name"].String] || !untouchedBootstrapProfile(id, profile) {
			continue
		}
		referenced := false
		for _, object := range org.Objects {
			for _, record := range object.Records {
				for _, value := range record.Fields {
					if value.Kind == ValueID && IDsEqual(value.ID, id) {
						referenced = true
						break
					}
				}
				if referenced {
					break
				}
			}
			if referenced {
				break
			}
		}
		if !referenced {
			mutable, _ := EnsureMutableObjectRecords(org, "Profile")
			delete(mutable.Records, id)
			changed = true
		}
	}
	if changed && len(org.Objects["Profile"].Definition.Indexes) != 0 {
		RebuildObjectIndexes(org, "Profile")
	}
}
