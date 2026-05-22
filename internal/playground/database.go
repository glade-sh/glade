package playground

import (
	"sort"

	"github.com/open-aer/oaer/internal/storage"
)

const maxDatabaseBrowserRows = 200

func databaseSnapshot(org storage.OrgState) DatabaseSnapshot {
	names := make([]string, 0, len(org.Objects))
	for name := range org.Objects {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		left := org.Objects[names[i]]
		right := org.Objects[names[j]]
		if len(left.Records) != len(right.Records) {
			return len(left.Records) > len(right.Records)
		}
		return names[i] < names[j]
	})

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
		KeyPrefix:   object.Definition.KeyPrefix,
		Columns:     columns,
		RecordCount: len(object.Records),
		Rows:        rows,
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
