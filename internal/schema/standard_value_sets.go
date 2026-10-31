package schema

import (
	"strings"

	"github.com/glade-sh/glade/internal/project"
)

// Standard value sets can be the only metadata supplied for a standard field.
// Status bindings are covered by standard_restricted_picklists/project-valuesets;
// Industry is covered by sobject_json3_collision. Other field facets are retained.
func bindProjectStandardValueSets(p project.Project, objects map[string]*Object, valueSets map[string][]PicklistValue) {
	for _, path := range p.StandardValueSetFiles {
		name := valueSetNameFromPath(path)
		var objectName string
		fieldName := "Status"
		switch strings.ToLower(name) {
		case "industry":
			objectName, fieldName = "Account", "Industry"
		case "contractstatus":
			objectName = "Contract"
		case "orderstatus":
			objectName = "Order"
		case "taskstatus":
			objectName = "Task"
		default:
			continue
		}
		object := objects[objectName]
		if object == nil {
			object = &Object{Name: objectName, Partial: true}
			objects[objectName] = object
		}
		i := fieldIndex(object.Fields, fieldName)
		if i < 0 {
			i = len(object.Fields)
			object.Fields = append(object.Fields, Field{Name: fieldName})
		}
		field := &object.Fields[i]
		if field.Type == "" {
			field.Type = "Picklist"
		}
		field.ValueSetName = name
		key := strings.ToLower(name)
		field.StandardValueSetApplied = len(valueSets[key]) != 0
	}
}
