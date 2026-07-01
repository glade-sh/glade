package dbmanager

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func (m Manager) CreateRecord(objectName string, payload MutationPayload) MutationResult {
	record, definition, result := m.sparseRecord(objectName, "", payload)
	if !result.Success {
		return result
	}
	engine := dml.NewEngine(m.Org)
	results := engine.Insert([]storage.Record{record})
	return m.mutationResult(definition, firstDMLResult(results), true)
}

func (m Manager) UpdateRecord(objectName, id string, payload MutationPayload) MutationResult {
	record, definition, result := m.sparseRecord(objectName, id, payload)
	if !result.Success {
		return result
	}
	engine := dml.NewEngine(m.Org)
	results := engine.Update([]storage.Record{record})
	return m.mutationResult(definition, firstDMLResult(results), false)
}

func (m Manager) DeleteRecord(objectName, id string) MutationResult {
	resolved, object, ok := m.object(objectName)
	if !ok {
		return failedMutation("INVALID_TYPE", fmt.Sprintf("unknown object %s", objectName), nil)
	}
	engine := dml.NewEngine(m.Org)
	results := engine.Delete([]storage.Record{{Object: resolved, ID: storage.ID(id)}})
	return m.mutationResult(object.Definition, firstDMLResult(results), false)
}

func (m Manager) UndeleteRecord(objectName, id string) MutationResult {
	resolved, object, ok := m.object(objectName)
	if !ok {
		return failedMutation("INVALID_TYPE", fmt.Sprintf("unknown object %s", objectName), nil)
	}
	engine := dml.NewEngine(m.Org)
	results := engine.Undelete([]storage.Record{{Object: resolved, ID: storage.ID(id)}})
	return m.mutationResult(object.Definition, firstDMLResult(results), false)
}

func (m Manager) sparseRecord(objectName, id string, payload MutationPayload) (storage.Record, storage.ObjectDefinition, MutationResult) {
	resolved, object, ok := m.object(objectName)
	if !ok {
		return storage.Record{}, storage.ObjectDefinition{}, failedMutation("INVALID_TYPE", fmt.Sprintf("unknown object %s", objectName), nil)
	}
	record := storage.Record{
		Object:        resolved,
		ID:            storage.ID(id),
		Fields:        make(map[string]storage.Value),
		ExplicitNulls: make(map[string]bool),
	}
	for rawName, input := range payload.Fields {
		fieldName, ok := storage.ResolveFieldName(object.Definition, m.Org.Namespace, rawName)
		if !ok {
			return storage.Record{}, object.Definition, failedMutation("INVALID_FIELD", fmt.Sprintf("unknown field %s", rawName), []string{rawName})
		}
		if strings.EqualFold(fieldName, "Id") {
			return storage.Record{}, object.Definition, failedMutation("INVALID_FIELD_FOR_INSERT_UPDATE", "Id is read-only", []string{fieldName})
		}
		field := object.Definition.Fields[fieldName]
		value, explicitNull, err := FieldInputToStorageValue(field, input)
		if err != nil {
			return storage.Record{}, object.Definition, failedMutation("INVALID_FIELD", fmt.Sprintf("%s: %s", fieldName, err.Error()), []string{fieldName})
		}
		if explicitNull {
			record.ExplicitNulls[fieldName] = true
			continue
		}
		record.Fields[fieldName] = value
	}
	if len(record.ExplicitNulls) == 0 {
		record.ExplicitNulls = nil
	}
	return record, object.Definition, MutationResult{Success: true}
}

func (m Manager) mutationResult(definition storage.ObjectDefinition, result dml.Result, created bool) MutationResult {
	if !result.Success {
		return MutationResult{
			Success:    false,
			ID:         string(result.ID),
			StatusCode: result.StatusCode,
			Message:    result.Error,
			Fields:     result.Fields,
		}
	}
	out := MutationResult{Success: true, ID: string(result.ID), Created: created || result.Created}
	if result.ID != "" {
		if row, err := m.RecordDetail(definition.APIName, string(result.ID)); err == nil {
			out.Record = row
		}
	}
	return out
}

func firstDMLResult(results []dml.Result) dml.Result {
	if len(results) == 0 {
		return dml.Result{Success: false, Error: "dml returned no result", StatusCode: "UNKNOWN_EXCEPTION"}
	}
	return results[0]
}

func failedMutation(statusCode, message string, fields []string) MutationResult {
	return MutationResult{Success: false, StatusCode: statusCode, Message: message, Fields: fields}
}
