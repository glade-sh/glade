package schema

import (
	"sort"
	"strings"
)

// StandardObjectNameResolver canonicalizes a standard object API name. The
// storage package installs it; without it only objects already in the
// project schema can take a scratch default record type.
var StandardObjectNameResolver func(string) (string, bool)

// addScratchDefaultRecordTypes adds the RecordTypeId lookup only for object
// types captured with scratch objectSettings, so static references compile. The record type itself, hidden
// from the running user, is added by the runtime (apextest project setup,
// account_object_settings_record_type rows).
func addScratchDefaultRecordTypes(objects map[string]*Object, recordTypes map[string]string) {
	keys := make([]string, 0, len(recordTypes))
	for key := range recordTypes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if !strings.EqualFold(key, "Account") && !strings.EqualFold(key, "Contact") && !strings.EqualFold(key, "Opportunity") {
			continue
		}
		object := scratchRecordTypeObject(objects, key)
		if object == nil {
			continue
		}
		for _, field := range object.Fields {
			if strings.EqualFold(field.Name, "RecordTypeId") {
				goto next
			}
		}
		object.Fields = append(object.Fields, Field{Name: "RecordTypeId", Label: "Record Type ID", Type: "Lookup", ReferenceTo: []string{"RecordType"}, RelationshipName: "RecordType"})
	next:
	}
}

func scratchRecordTypeObject(objects map[string]*Object, key string) *Object {
	for name, object := range objects {
		if strings.EqualFold(name, key) {
			return object
		}
	}
	if StandardObjectNameResolver == nil {
		return nil
	}
	canonical, ok := StandardObjectNameResolver(key)
	if !ok {
		return nil
	}
	object := &Object{Name: canonical, Partial: true}
	objects[canonical] = object
	return object
}
