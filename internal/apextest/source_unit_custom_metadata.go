package apextest

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func hasSourceUnitMetadataCollisions(org storage.OrgState) bool {
	for _, object := range org.Objects {
		if !storage.IsCustomMetadataDefinition(object.Definition) {
			continue
		}
		units := make(map[string]string)
		for _, record := range object.Records {
			if record.MetadataSourceUnit == "" {
				continue
			}
			identity := customMetadataRecordIdentity(record)
			if prior, found := units[identity]; found && prior != record.MetadataSourceUnit {
				return true
			}
			units[identity] = record.MetadataSourceUnit
		}
	}
	return false
}

// Resolve colliding installed-metadata identities before setup and test journals
// are created. Keep the full captured project and the immutable seed unchanged.
// Unscoped metadata and identities without a local variant retain their behavior.
func customMetadataForSourceUnit(org storage.OrgState, sourceUnit string) storage.OrgState {
	if !strings.HasPrefix(sourceUnit, "sfdx:") {
		return org
	}
	var objects map[string]storage.ObjectState
	for name, object := range org.Objects {
		if !storage.IsCustomMetadataDefinition(object.Definition) {
			continue
		}
		local := make(map[string]bool)
		for _, record := range object.Records {
			if record.MetadataSourceUnit == sourceUnit {
				local[customMetadataRecordIdentity(record)] = true
			}
		}
		var records map[storage.ID]storage.Record
		for id, record := range object.Records {
			if record.MetadataSourceUnit != "" && record.MetadataSourceUnit != sourceUnit && local[customMetadataRecordIdentity(record)] {
				if records == nil {
					records = make(map[storage.ID]storage.Record, len(object.Records))
					for retainedID, retained := range object.Records {
						records[retainedID] = retained
					}
				}
				delete(records, id)
			}
		}
		if records != nil {
			if objects == nil {
				objects = make(map[string]storage.ObjectState, len(org.Objects))
				for objectName, state := range org.Objects {
					objects[objectName] = state
				}
			}
			object.Records, object.Indexes = records, nil
			object.RecordsShared, object.IndexesShared = false, false
			objects[name] = object
		}
	}
	if objects != nil {
		org.Objects = objects
	}
	return org
}

func customMetadataRecordIdentity(record storage.Record) string {
	developer, _ := record.GetField("DeveloperName")
	namespace, _ := record.GetField("NamespacePrefix")
	return strings.ToLower(namespace.String + "." + developer.String)
}
