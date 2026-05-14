package storage

import (
	"encoding/json"
	"strings"
)

func RebuildIndexes(org *OrgState) {
	if org == nil {
		return
	}
	for objectName, object := range org.Objects {
		if len(object.Definition.Indexes) == 0 {
			object.Indexes = nil
			org.Objects[objectName] = object
			continue
		}
		object.Indexes = make(map[string]IndexSet, len(object.Definition.Indexes))
		for _, definition := range object.Definition.Indexes {
			index := IndexSet{Definition: definition, Entries: make(map[string][]ID)}
			for id, record := range object.Records {
				key, ok := indexRecordKey(record, definition)
				if !ok {
					continue
				}
				index.Entries[key] = append(index.Entries[key], id)
			}
			object.Indexes[definition.Name] = index
		}
		org.Objects[objectName] = object
	}
}

func LookupIndex(object ObjectState, field string, value Value) ([]ID, bool) {
	for _, index := range object.Indexes {
		if len(index.Definition.Fields) != 1 || !strings.EqualFold(index.Definition.Fields[0], field) {
			continue
		}
		ids := append([]ID(nil), index.Entries[indexValueKey(value)]...)
		return ids, true
	}
	return nil, false
}

func indexRecordKey(record Record, definition IndexDefinition) (string, bool) {
	values := make([]Value, 0, len(definition.Fields))
	for _, field := range definition.Fields {
		if field == "Id" {
			values = append(values, IDValue(record.ID))
			continue
		}
		value, ok := record.GetField(field)
		if !ok {
			return "", false
		}
		values = append(values, value)
	}
	return indexValuesKey(values), true
}

func indexValueKey(value Value) string {
	return indexValuesKey([]Value{value})
}

func indexValuesKey(values []Value) string {
	for i := range values {
		if values[i].Kind == ValueString {
			values[i].String = strings.ToLower(values[i].String)
		}
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	return string(raw)
}
