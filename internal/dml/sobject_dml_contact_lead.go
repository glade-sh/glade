package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func normalizeNameFields(objectName string, definition storage.ObjectDefinition, record *storage.Record) {
	if record == nil {
		return
	}
	if strings.EqualFold(objectName, "Contact") || strings.EqualFold(objectName, "Lead") {
		normalizeFirstLastName(definition, record)
		return
	}
	normalizePersonAccountFields(objectName, definition, record)
}

func normalizeFirstLastName(definition storage.ObjectDefinition, record *storage.Record) {
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	if _, ok := definition.Fields["Name"]; !ok {
		return
	}
	if _, ok := record.Fields["Name"]; ok && !hasNameComponentField(record.Fields) {
		return
	}
	firstName := stringField(record.Fields, "FirstName")
	lastName := stringField(record.Fields, "LastName")
	switch {
	case firstName != "" && lastName != "":
		record.Fields["Name"] = storage.StringValue(firstName + " " + lastName)
	case lastName != "":
		record.Fields["Name"] = storage.StringValue(lastName)
	}
}

func hasNameComponentField(fields map[string]storage.Value) bool {
	_, hasFirst := fields["FirstName"]
	_, hasLast := fields["LastName"]
	return hasFirst || hasLast
}

func stringField(fields map[string]storage.Value, name string) string {
	value, ok := fields[name]
	if !ok || value.Kind != storage.ValueString {
		return ""
	}
	return strings.TrimSpace(value.String)
}
