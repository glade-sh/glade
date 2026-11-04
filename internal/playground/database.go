package playground

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/glade-sh/glade/internal/storage"
)

const maxDatabaseBrowserRows = 200

func databaseSnapshot(org storage.OrgState) DatabaseSnapshot {
	names := make([]string, 0, len(org.Objects))
	for name := range org.Objects {
		names = append(names, name)
	}
	sort.Strings(names)

	objects := make([]DatabaseObject, 0, len(names))
	for _, name := range names {
		object := org.Objects[name]
		objects = append(objects, databaseObject(name, object))
	}
	return DatabaseSnapshot{Objects: objects}
}

func databaseObject(name string, object storage.ObjectState) DatabaseObject {
	columns := databaseColumns(object)
	rows := make([]DatabaseRow, 0, min(len(object.Records), maxDatabaseBrowserRows))
	ids := make([]string, 0, len(object.Records))
	for id := range object.Records {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	for _, id := range ids {
		if len(rows) >= maxDatabaseBrowserRows {
			break
		}
		record := object.Records[storage.ID(id)]
		rows = append(rows, databaseRow(record, columns))
	}
	return DatabaseObject{
		Name:        name,
		Label:       object.Definition.Label,
		Kind:        databaseObjectKind(object.Definition),
		KeyPrefix:   object.Definition.KeyPrefix,
		Columns:     columns,
		RecordCount: len(object.Records),
		Rows:        rows,
	}
}

func databaseObjectKind(definition storage.ObjectDefinition) string {
	name := definition.APIName
	if storage.IsCustomMetadataDefinition(definition) {
		return "custom_metadata"
	}
	if storage.IsCustomSettingDefinition(definition) {
		return "custom_setting"
	}
	if isSystemRecordObject(name) {
		return "system"
	}
	if isCustomObjectName(name) {
		return "custom"
	}
	return "standard"
}

func isCustomObjectName(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	for _, suffix := range []string{"__c", "__e", "__b"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

func isSystemRecordObject(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "apexclass", "apexcomponent", "apexpage", "apextrigger",
		"customapplication", "customnotificationtype",
		"fieldpermissions", "objectpermissions",
		"permissionset", "permissionsetassignment", "permissionsetgroup",
		"profile", "recordtype", "staticresource", "tabdefinition",
		"user", "userrole":
		return true
	default:
		return false
	}
}

func databaseColumns(object storage.ObjectState) []string {
	seen := map[string]bool{"Id": true}
	columns := []string{"Id"}
	if object.Definition.Fields != nil {
		names := make([]string, 0, len(object.Definition.Fields))
		for name := range object.Definition.Fields {
			if name != "Id" {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		for _, name := range names {
			if !seen[name] {
				seen[name] = true
				columns = append(columns, name)
			}
		}
	}
	observed := make([]string, 0)
	for _, record := range object.Records {
		for name := range record.Fields {
			if !seen[name] {
				seen[name] = true
				observed = append(observed, name)
			}
		}
	}
	sort.Strings(observed)
	columns = append(columns, observed...)
	for _, name := range []string{"CreatedDate", "LastModifiedDate", "OwnerId", "IsDeleted"} {
		if !seen[name] {
			columns = append(columns, name)
		}
	}
	return columns
}

func databaseRow(record storage.Record, columns []string) DatabaseRow {
	fields := make(map[string]any, len(columns))
	fields["Id"] = string(record.ID)
	for name, value := range record.Fields {
		fields[name] = databaseValue(value)
	}
	if record.System.CreatedDate != "" {
		fields["CreatedDate"] = record.System.CreatedDate
	}
	if record.System.LastModifiedDate != "" {
		fields["LastModifiedDate"] = record.System.LastModifiedDate
	}
	if record.System.OwnerID != "" {
		fields["OwnerId"] = string(record.System.OwnerID)
	}
	if record.System.IsDeleted {
		fields["IsDeleted"] = true
	}
	return DatabaseRow{ID: string(record.ID), Fields: fields}
}

func databaseValue(value storage.Value) any {
	switch value.Kind {
	case storage.ValueNull:
		return nil
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime, storage.ValueBlob:
		return value.String
	case storage.ValueInteger:
		return value.Integer
	case storage.ValueBoolean:
		return value.Boolean
	case storage.ValueDecimal:
		return value.Decimal
	case storage.ValueID:
		return string(value.ID)
	case storage.ValueList:
		items := make([]any, 0, len(value.List))
		for _, item := range value.List {
			items = append(items, databaseValue(item))
		}
		return items
	default:
		return value.String
	}
}

// publicDatabaseSnapshot bounds the input before constructing or encoding a
// browser snapshot. The conservative estimate includes every record, even
// those beyond the browser row limit, to bound sorting and copying work too.
func (r *Runner) publicDatabaseSnapshot() (DatabaseSnapshot, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.lastOrg != nil {
		return boundedDatabaseSnapshot(publicDatabaseOrg(*r.lastOrg))
	}
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	return boundedDatabaseSnapshot(publicDatabaseOrg(r.store.org))
}

// publicDatabaseOrg drops standard objects that hold no records. A run loads
// the whole standard catalog (thousands of objects and columns), which would
// exceed the public response budget before any visitor data is counted. Custom
// and system objects, and every object with records, remain visible.
func publicDatabaseOrg(org storage.OrgState) storage.OrgState {
	objects := make(map[string]storage.ObjectState, len(org.Objects))
	for name, object := range org.Objects {
		if len(object.Records) == 0 && databaseObjectKind(object.Definition) == "standard" {
			continue
		}
		objects[name] = object
	}
	org.Objects = objects
	return org
}

func boundedDatabaseSnapshot(org storage.OrgState) (DatabaseSnapshot, bool) {
	budget := maxPublicDatabaseBytes
	consume := func(n int) bool {
		if n < 0 || n > budget {
			return false
		}
		budget -= n
		return true
	}
	// Count the actual JSON string escape cost before materializing any rows.
	stringFits := func(s string) bool {
		if !consume(2) || len(s) > budget {
			return false
		}
		for len(s) > 0 {
			r, n := utf8.DecodeRuneInString(s)
			cost := n
			switch {
			case r == '"' || r == '\\' || r == '\b' || r == '\f' || r == '\n' || r == '\r' || r == '\t':
				cost = 2
			case r < 0x20 || r == '<' || r == '>' || r == '&' || r == '\u2028' || r == '\u2029' || (r == utf8.RuneError && n == 1):
				cost = 6
			}
			if !consume(cost) {
				return false
			}
			s = s[n:]
		}
		return true
	}
	var valueFits func(storage.Value, int) bool
	valueFits = func(v storage.Value, depth int) bool {
		if depth > 64 || !consume(32) {
			return false
		}
		switch v.Kind {
		case storage.ValueString, storage.ValueDate, storage.ValueDateTime, storage.ValueBlob:
			return stringFits(v.String)
		case storage.ValueDecimal:
			return stringFits(v.Decimal)
		case storage.ValueID:
			return stringFits(string(v.ID))
		case storage.ValueList:
			for _, child := range v.List {
				if !valueFits(child, depth+1) {
					return false
				}
			}
		case storage.ValueNull, storage.ValueInteger, storage.ValueBoolean:
			// These are encoded as bounded JSON scalars.
		default:
			return stringFits(v.String)
		}
		return true
	}
	if !consume(32) {
		return DatabaseSnapshot{}, false
	}
	for name, object := range org.Objects {
		if !consume(256) || !stringFits(name) || !stringFits(object.Definition.APIName) || !stringFits(object.Definition.Label) || !stringFits(object.Definition.KeyPrefix) {
			return DatabaseSnapshot{}, false
		}
		for field := range object.Definition.Fields {
			if !consume(8) || !stringFits(field) {
				return DatabaseSnapshot{}, false
			}
		}
		for id, record := range object.Records {
			if !consume(96) || !stringFits(string(id)) || !stringFits(string(record.ID)) || !stringFits(string(record.ID)) || !stringFits(record.System.CreatedDate) || !stringFits(record.System.LastModifiedDate) || !stringFits(string(record.System.OwnerID)) {
				return DatabaseSnapshot{}, false
			}
			for field, value := range record.Fields {
				// A field name may appear both as a column and in the row.
				if !consume(16) || !stringFits(field) || !stringFits(field) || !valueFits(value, 0) {
					return DatabaseSnapshot{}, false
				}
			}
		}
	}
	return databaseSnapshot(org), true
}
