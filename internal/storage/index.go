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
		rebuildObjectIndexes(org, objectName, object)
	}
}

func RebuildObjectIndexes(org *OrgState, objectName string) {
	if org == nil || objectName == "" {
		return
	}
	object, ok := org.Objects[objectName]
	if !ok {
		return
	}
	rebuildObjectIndexes(org, objectName, object)
}

func rebuildObjectIndexes(org *OrgState, objectName string, object ObjectState) {
	if len(object.Definition.Indexes) == 0 {
		object.Indexes = nil
		org.Objects[objectName] = object
		return
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

func LookupIndex(object ObjectState, field string, value Value) ([]ID, bool) {
	for _, index := range object.Indexes {
		if len(index.Definition.Fields) != 1 || !strings.EqualFold(index.Definition.Fields[0], field) {
			continue
		}
		key := indexValueKey(value)
		if strings.EqualFold(field, "Id") {
			key = indexIDValueKey(value)
		}
		ids := append([]ID(nil), index.Entries[key]...)
		return ids, true
	}
	return nil, false
}

func indexRecordKey(record Record, definition IndexDefinition) (string, bool) {
	values := make([]Value, 0, len(definition.Fields))
	for _, field := range definition.Fields {
		if field == "Id" {
			values = append(values, indexIDValue(IDValue(record.ID)))
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

func indexIDValueKey(value Value) string {
	return indexValuesKey([]Value{indexIDValue(value)})
}

func indexIDValue(value Value) Value {
	switch value.Kind {
	case ValueID:
		value.ID = ID(normalizeIDIndexText(string(value.ID)))
	case ValueString:
		if isIndexableIDText(value.String) {
			return IDValue(ID(normalizeIDIndexText(value.String)))
		}
	}
	return value
}

func normalizeIDIndexText(text string) string {
	if isIndexableIDText(text) {
		return strings.ToLower(text[:15])
	}
	return text
}

func isIndexableIDText(text string) bool {
	return len(text) == 15 || len(text) == 18
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
