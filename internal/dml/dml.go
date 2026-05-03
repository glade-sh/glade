package dml

import (
	"fmt"

	"github.com/open-aer/oaer/internal/storage"
)

type Engine struct {
	Org *storage.OrgState
	IDs storage.IDGenerator
}

type Result struct {
	ID      storage.ID `json:"id,omitempty"`
	Success bool       `json:"success"`
	Error   string     `json:"error,omitempty"`
}

func NewEngine(org *storage.OrgState) Engine {
	prefixes := make(map[string]string, len(org.Objects))
	for name, object := range org.Objects {
		if object.Definition.KeyPrefix != "" {
			prefixes[name] = object.Definition.KeyPrefix
		}
	}
	ids := storage.NewIDGenerator(prefixes)
	ids.Sequences = copySequences(org.IDSequences)
	return Engine{Org: org, IDs: ids}
}

func (e *Engine) Insert(records []storage.Record) []Result {
	results := make([]Result, len(records))
	for i, record := range records {
		id, err := e.insertOne(record)
		if err != nil {
			results[i] = Result{ID: record.ID, Success: false, Error: err.Error()}
			continue
		}
		results[i] = Result{ID: id, Success: true}
	}
	e.Org.IDSequences = copySequences(e.IDs.Sequences)
	return results
}

func (e *Engine) Update(records []storage.Record) []Result {
	results := make([]Result, len(records))
	for i, record := range records {
		if err := e.updateOne(record); err != nil {
			results[i] = Result{ID: record.ID, Success: false, Error: err.Error()}
			continue
		}
		results[i] = Result{ID: record.ID, Success: true}
	}
	return results
}

func (e *Engine) Delete(records []storage.Record) []Result {
	results := make([]Result, len(records))
	for i, record := range records {
		if err := e.deleteOne(record); err != nil {
			results[i] = Result{ID: record.ID, Success: false, Error: err.Error()}
			continue
		}
		results[i] = Result{ID: record.ID, Success: true}
	}
	return results
}

func (e *Engine) Upsert(records []storage.Record) []Result {
	results := make([]Result, len(records))
	for i, record := range records {
		if record.ID == "" {
			id, err := e.insertOne(record)
			if err != nil {
				results[i] = Result{ID: record.ID, Success: false, Error: err.Error()}
				continue
			}
			results[i] = Result{ID: id, Success: true}
			continue
		}
		if err := e.updateOne(record); err != nil {
			results[i] = Result{ID: record.ID, Success: false, Error: err.Error()}
			continue
		}
		results[i] = Result{ID: record.ID, Success: true}
	}
	e.Org.IDSequences = copySequences(e.IDs.Sequences)
	return results
}

func (e *Engine) Undelete(records []storage.Record) []Result {
	results := make([]Result, len(records))
	for i, record := range records {
		if record.ID == "" {
			results[i] = Result{Success: false, Error: "dml: undelete requires id"}
			continue
		}
		if _, _, err := e.object(record.Object); err != nil {
			results[i] = Result{ID: record.ID, Success: false, Error: err.Error()}
			continue
		}
		results[i] = Result{ID: record.ID, Success: true}
	}
	return results
}

func (e *Engine) WithTransaction(fn func(*Engine) error) error {
	before := e.Org.Clone()
	if err := fn(e); err != nil {
		*e.Org = before
		e.IDs.Sequences = copySequences(before.IDSequences)
		return err
	}
	return nil
}

func (e *Engine) insertOne(record storage.Record) (storage.ID, error) {
	object, objectName, err := e.object(record.Object)
	if err != nil {
		return "", err
	}
	record, err = canonicalizeRecord(e.Org.Namespace, object.Definition, objectName, record)
	if err != nil {
		return "", err
	}
	if err := validateFields(object.Definition, e.Org.Namespace, record); err != nil {
		return "", err
	}
	if err := validateRequired(object.Definition, record); err != nil {
		return "", err
	}
	if record.ID == "" {
		id, err := e.IDs.Next(objectName)
		if err != nil {
			return "", err
		}
		record.ID = id
	}
	if err := storage.ValidateID(record.ID); err != nil {
		return "", err
	}
	if _, exists := object.Records[record.ID]; exists {
		return "", fmt.Errorf("dml: duplicate id %s", record.ID)
	}
	if object.Records == nil {
		object.Records = make(map[storage.ID]storage.Record)
	}
	object.Records[record.ID] = record.Clone()
	e.Org.Objects[objectName] = object
	return record.ID, nil
}

func (e *Engine) updateOne(record storage.Record) error {
	object, objectName, err := e.object(record.Object)
	if err != nil {
		return err
	}
	record, err = canonicalizeRecord(e.Org.Namespace, object.Definition, objectName, record)
	if err != nil {
		return err
	}
	if record.ID == "" {
		return fmt.Errorf("dml: update requires id")
	}
	if err := validateFields(object.Definition, e.Org.Namespace, record); err != nil {
		return err
	}
	existing, ok := object.Records[record.ID]
	if !ok {
		return fmt.Errorf("dml: record %s does not exist", record.ID)
	}
	if existing.Fields == nil {
		existing.Fields = make(map[string]storage.Value)
	}
	if existing.ExplicitNulls == nil {
		existing.ExplicitNulls = make(map[string]bool)
	}
	for field, value := range record.Fields {
		existing.Fields[field] = value.Clone()
		delete(existing.ExplicitNulls, field)
	}
	for field, isNull := range record.ExplicitNulls {
		if isNull {
			delete(existing.Fields, field)
			existing.ExplicitNulls[field] = true
		}
	}
	object.Records[record.ID] = existing
	e.Org.Objects[objectName] = object
	return nil
}

func (e *Engine) deleteOne(record storage.Record) error {
	object, objectName, err := e.object(record.Object)
	if err != nil {
		return err
	}
	if record.ID == "" {
		return fmt.Errorf("dml: delete requires id")
	}
	if _, ok := object.Records[record.ID]; !ok {
		return fmt.Errorf("dml: record %s does not exist", record.ID)
	}
	delete(object.Records, record.ID)
	e.Org.Objects[objectName] = object
	return nil
}

func (e *Engine) object(name string) (storage.ObjectState, string, error) {
	objectName, ok := storage.ResolveObjectName(*e.Org, name)
	if !ok {
		return storage.ObjectState{}, "", fmt.Errorf("dml: unknown object %s", name)
	}
	object := e.Org.Objects[objectName]
	if object.Records == nil {
		object.Records = make(map[storage.ID]storage.Record)
	}
	return object, objectName, nil
}

func canonicalizeRecord(namespace string, definition storage.ObjectDefinition, objectName string, record storage.Record) (storage.Record, error) {
	record.Object = objectName
	fields := make(map[string]storage.Value, len(record.Fields))
	for field, value := range record.Fields {
		canonical, ok := storage.ResolveFieldName(definition, namespace, field)
		if !ok {
			fields[field] = value
			continue
		}
		if _, exists := fields[canonical]; exists && canonical != field {
			return storage.Record{}, fmt.Errorf("dml: duplicate field alias %s.%s", objectName, field)
		}
		fields[canonical] = value
	}
	record.Fields = fields
	nulls := make(map[string]bool, len(record.ExplicitNulls))
	for field, value := range record.ExplicitNulls {
		canonical, ok := storage.ResolveFieldName(definition, namespace, field)
		if !ok {
			nulls[field] = value
			continue
		}
		nulls[canonical] = value
	}
	record.ExplicitNulls = nulls
	return record, nil
}

func validateFields(definition storage.ObjectDefinition, namespace string, record storage.Record) error {
	for field := range record.Fields {
		if field == "Id" {
			continue
		}
		if _, ok := storage.ResolveFieldName(definition, namespace, field); !ok {
			return fmt.Errorf("dml: unknown field %s.%s", record.Object, field)
		}
	}
	for field := range record.ExplicitNulls {
		if _, ok := storage.ResolveFieldName(definition, namespace, field); !ok {
			return fmt.Errorf("dml: unknown field %s.%s", record.Object, field)
		}
	}
	return nil
}

func validateRequired(definition storage.ObjectDefinition, record storage.Record) error {
	for name, field := range definition.Fields {
		if !field.Required {
			continue
		}
		if _, ok := record.Fields[name]; ok {
			continue
		}
		if record.ExplicitNulls[name] {
			return fmt.Errorf("dml: required field %s.%s is null", record.Object, name)
		}
		return fmt.Errorf("dml: missing required field %s.%s", record.Object, name)
	}
	return nil
}

func copySequences(in map[string]uint64) map[string]uint64 {
	out := make(map[string]uint64, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
