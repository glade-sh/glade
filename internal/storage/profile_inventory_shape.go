package storage

import (
	"reflect"

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
		}
	}
}
