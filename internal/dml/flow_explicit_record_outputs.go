package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func flowExplicitIDDeleteCriteria(del storage.FlowRecordDelete, item storage.WorkflowCriteriaItem, frame *flowFrame) bool {
	variable, declared := frame.variables[flowFrameKey(item.SourceField)]
	return len(del.Criteria) == 1 && strings.EqualFold(item.Field, "Id") && strings.EqualFold(item.Operation, "equals") && declared && !variable.IsCollection && strings.EqualFold(variable.DataType, "String")
}

func (e *Engine) applyFlowExplicitLookupOutputs(lookup storage.FlowRecordLookup, records []storage.Record, definition storage.ObjectDefinition, frame *flowFrame) {
	matched := len(records) > 0
	key := flowFrameKey(lookup.OutputReference)
	if variable, declared := frame.variables[key]; key != "" && declared && strings.EqualFold(variable.DataType, "SObject") {
		if !matched {
			if lookup.AssignNullValuesIfNoRecordsFound {
				delete(frame.records, key)
				delete(frame.collections, key)
				frame.scalars[key] = storage.NullValue()
			}
		} else {
			delete(frame.scalars, key)
			projected := make([]storage.Record, 0, len(records))
			for _, record := range records {
				projected = append(projected, flowExplicitLookupRecord(record, lookup.QueriedFields, definition, e.Org.Namespace))
			}
			if variable.IsCollection {
				frame.collections[key] = projected
			} else {
				frame.records[key] = projected[0]
				delete(frame.nullRecordDefaults, key)
			}
		}
	}
	for _, output := range lookup.OutputAssignments {
		key := flowFrameKey(output.Target)
		variable, declared := frame.variables[key]
		if !declared || variable.IsCollection || flowVariableRecordCollection(variable) {
			continue
		}
		if !matched {
			if lookup.AssignNullValuesIfNoRecordsFound {
				frame.scalars[key] = storage.NullValue()
			}
			continue
		}
		field, exists := storage.ResolveFieldName(definition, e.Org.Namespace, output.SourceField)
		if !exists {
			continue
		}
		value, present := sourceRecordFieldValue(records[0], field)
		if !present {
			value = storage.NullValue()
		}
		frame.scalars[key] = value.Clone()
		frame.assigned[key] = true
	}
}

func flowExplicitLookupRecord(source storage.Record, fields []string, definition storage.ObjectDefinition, namespace string) storage.Record {
	if len(fields) == 0 {
		return source.Clone()
	}
	projected := source.Clone()
	projected.Fields = make(map[string]storage.Value)
	projected.ExplicitNulls = make(map[string]bool)
	for _, name := range fields {
		field, ok := storage.ResolveFieldName(definition, namespace, strings.TrimSpace(name))
		if !ok || strings.EqualFold(field, "Id") {
			continue
		}
		value, present := sourceRecordFieldValue(source, field)
		if !present || value.Kind == storage.ValueNull {
			projected.ExplicitNulls[field] = true
			continue
		}
		projected.Fields[field] = value.Clone()
	}
	return projected
}

// flowSubflowCarryRecordInput passes a record variable, a Get Records output or
// a record collection from the calling flow into a subflow input variable. The
// subflow then reads fields such as {!input.Lookup__c} from the real record
// instead of from an empty scalar.
func flowSubflowCarryRecordInput(subflowFrame *flowFrame, target, reference string, frame *flowFrame) bool {
	if frame == nil || subflowFrame == nil {
		return false
	}
	source := flowFrameKey(reference)
	if source == "" {
		return false
	}
	if _, scalar := frame.scalars[source]; scalar {
		return false
	}
	key := flowFrameKey(target)
	if record, ok := frame.records[source]; ok {
		subflowFrame.records[key] = record.Clone()
		if input, found := frame.subflowInputs[source]; found {
			subflowFrame.subflowInputs[key] = input
		}
		return true
	}
	if output, ok := frame.lookupOutputs[source]; ok {
		subflowFrame.records[key] = output.record.Clone()
		return true
	}
	if output, ok := frame.lookupCollections[source]; ok {
		output.records = cloneFlowRecords(output.records)
		subflowFrame.lookupCollections[key] = output
		return true
	}
	if records, ok := frame.collections[source]; ok {
		subflowFrame.collections[key] = cloneFlowRecords(records)
		return true
	}
	return false
}
