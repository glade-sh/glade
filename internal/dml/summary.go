package dml

import (
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func (e *Engine) recalculateSummaryFieldsForChildren(childObjectName string, childRecords ...storage.Record) {
	if e == nil || e.Org == nil || len(childRecords) == 0 {
		return
	}
	canonicalChild, ok := storage.ResolveObjectName(*e.Org, childObjectName)
	if !ok {
		canonicalChild = childObjectName
	}
	relations := e.summaryRelationsForChild(canonicalChild)
	if len(relations) == 0 {
		return
	}
	updatedParents := make(map[string][]storage.Record)
	for _, relation := range relations {
		parentObjectName := relation.parentObject
		parentObject, ok := e.Org.Objects[parentObjectName]
		if !ok {
			continue
		}
		changed := false
		parentIDs := summaryParentIDs(childRecords, relation.fkFieldName)
		parentIDs = canonicalSummaryParentIDs(parentObject.Records, parentIDs)
		summaryValues, ok := e.evaluateSummaryFieldBatch(relation, parentIDs)
		if !ok {
			continue
		}
		for parentID := range parentIDs {
			storedParentID, parentRecord, ok := storage.LookupRecordByID(parentObject.Records, parentID)
			if !ok || parentRecord.System.IsDeleted {
				continue
			}
			value := summaryValues[parentID]
			oldValue, ok := parentRecord.Fields[relation.parentField]
			if !ok {
				oldValue = storage.NullValue()
			}
			if storageValuesEqual(relation.field, oldValue, value) {
				continue
			}
			if _, cloned := storage.EnsureMutableObjectRecords(e.Org, parentObjectName); cloned {
				parentObject = e.Org.Objects[parentObjectName]
				storedParentID, parentRecord, _ = storage.LookupRecordByID(parentObject.Records, parentID)
			}
			beforeParent := parentRecord.Clone()
			if parentRecord.Fields == nil {
				parentRecord.Fields = make(map[string]storage.Value)
			}
			parentRecord.Fields[relation.parentField] = value
			parentObject.Records[storedParentID] = parentRecord
			e.recordSummaryUpdate(parentObjectName, beforeParent, parentRecord)
			updatedParents[parentObjectName] = append(updatedParents[parentObjectName], parentRecord)
			changed = true
		}
		if changed {
			e.Org.Objects[parentObjectName] = parentObject
		}
	}
	for parentObjectName, records := range updatedParents {
		if strings.EqualFold(parentObjectName, canonicalChild) {
			continue
		}
		e.recalculateSummaryFieldsForChildren(parentObjectName, records...)
	}
}

func (e *Engine) summaryRelationsForChild(childObjectName string) []summaryRelation {
	if e == nil || e.Org == nil || strings.TrimSpace(childObjectName) == "" {
		return nil
	}
	if e.SummaryByChild == nil {
		e.SummaryByChild = NewSummaryRelationCache()
	}
	if relations, ok := e.SummaryByChild.load(childObjectName); ok {
		return relations
	}
	relations := make([]summaryRelation, 0)
	for parentObjectName, parentObject := range e.Org.Objects {
		for parentFieldName, field := range parentObject.Definition.Fields {
			if field.Type != storage.FieldSummary {
				continue
			}
			operation := strings.ToLower(strings.TrimSpace(field.SummaryOperation))
			summaryChild, _ := splitSummaryQualifiedField(field.SummarizedField)
			fkChild, fkField := splitSummaryQualifiedField(field.SummaryForeignKey)
			if summaryChild == "" && operation == "count" {
				summaryChild = fkChild
			}
			if summaryChild == "" || fkChild == "" || fkField == "" || !strings.EqualFold(summaryChild, fkChild) {
				continue
			}
			resolvedSummaryChild, ok := storage.ResolveObjectName(*e.Org, summaryChild)
			if !ok {
				resolvedSummaryChild = summaryChild
			}
			if !strings.EqualFold(resolvedSummaryChild, childObjectName) {
				continue
			}
			childObject := e.Org.Objects[resolvedSummaryChild]
			fkFieldName, ok := storage.ResolveFieldName(childObject.Definition, e.Org.Namespace, fkField)
			if !ok {
				continue
			}
			relations = append(relations, summaryRelation{
				parentObject: parentObjectName,
				parentField:  parentFieldName,
				field:        field,
				fkFieldName:  fkFieldName,
			})
		}
	}
	e.SummaryByChild.store(childObjectName, relations)
	return relations
}

func (e *Engine) evaluateSummaryFieldBatch(relation summaryRelation, parentIDs map[storage.ID]bool) (map[storage.ID]storage.Value, bool) {
	if e == nil || e.Org == nil || len(parentIDs) == 0 {
		return nil, false
	}
	childObject, childField := splitSummaryQualifiedField(relation.field.SummarizedField)
	fkObject, fkField := splitSummaryQualifiedField(relation.field.SummaryForeignKey)
	operation := strings.ToLower(strings.TrimSpace(relation.field.SummaryOperation))
	if childObject == "" && operation == "count" {
		childObject = fkObject
	}
	if childObject == "" || fkObject == "" || fkField == "" || !strings.EqualFold(childObject, fkObject) {
		return nil, false
	}
	canonicalChild, ok := storage.ResolveObjectName(*e.Org, childObject)
	if !ok {
		return nil, false
	}
	childState := e.Org.Objects[canonicalChild]
	childFieldName := ""
	if childField != "" {
		childFieldName, ok = storage.ResolveFieldName(childState.Definition, e.Org.Namespace, childField)
		if !ok {
			return nil, false
		}
	}
	fkFieldName, ok := storage.ResolveFieldName(childState.Definition, e.Org.Namespace, fkField)
	if !ok {
		return nil, false
	}
	if relation.fkFieldName != "" {
		fkFieldName = relation.fkFieldName
	}

	acc := make(map[storage.ID]summaryAccumulator, len(parentIDs))
	for parentID := range parentIDs {
		acc[parentID] = summaryAccumulator{}
	}
	for _, child := range childState.Records {
		if child.System.IsDeleted {
			continue
		}
		parentID := idFromStorageValue(child.Fields[fkFieldName])
		if parentState, ok := e.Org.Objects[relation.parentObject]; ok {
			if storedParentID, _, ok := storage.LookupRecordByID(parentState.Records, parentID); ok {
				parentID = storedParentID
			}
		}
		if parentID == "" || !parentIDs[parentID] {
			continue
		}
		if !summaryFiltersMatch(e.Org, childState.Definition, child, relation.field.SummaryFilterItems) {
			continue
		}
		if operation == "count" && childFieldName == "" {
			current := acc[parentID]
			current.count++
			acc[parentID] = current
			continue
		}
		value, ok := summaryRecordFieldValue(e.Org, childState.Definition, child, childFieldName)
		if !ok {
			continue
		}
		current := acc[parentID]
		if !current.add(operation, value) {
			continue
		}
		acc[parentID] = current
	}

	values := make(map[storage.ID]storage.Value, len(parentIDs))
	for parentID := range parentIDs {
		value, ok := acc[parentID].value(operation)
		if !ok {
			return nil, false
		}
		values[parentID] = value
	}
	return values, true
}

func (e *Engine) recordSummaryUpdate(objectName string, before, after storage.Record) {
	if e == nil || before.ID == "" || after.ID == "" {
		return
	}
	if e.IsolationJournal != nil {
		e.IsolationJournal.RecordUpdate(objectName, after.ID, before)
	}
	key := strings.ToLower(objectName) + "\x00" + strings.ToLower(string(after.ID))
	if e.summaryUpdates == nil {
		e.summaryUpdates = make(map[string]SummaryUpdate)
	}
	update, exists := e.summaryUpdates[key]
	if !exists {
		e.summaryOrder = append(e.summaryOrder, key)
		update = SummaryUpdate{Object: objectName, Before: before.Clone()}
	}
	update.After = after.Clone()
	e.summaryUpdates[key] = update
}

func summaryParentIDs(records []storage.Record, fkFieldName string) map[storage.ID]bool {
	ids := make(map[storage.ID]bool)
	for _, record := range records {
		if record.Fields == nil {
			continue
		}
		id := idFromStorageValue(record.Fields[fkFieldName])
		if id != "" {
			ids[id] = true
		}
	}
	return ids
}

func canonicalSummaryParentIDs(records map[storage.ID]storage.Record, ids map[storage.ID]bool) map[storage.ID]bool {
	if len(ids) == 0 || len(records) == 0 {
		return ids
	}
	canonical := make(map[storage.ID]bool, len(ids))
	for id := range ids {
		if storedID, _, ok := storage.LookupRecordByID(records, id); ok {
			canonical[storedID] = true
			continue
		}
		canonical[id] = true
	}
	return canonical
}

func (e *Engine) evaluateSummaryField(parent storage.Record, field storage.Field) (storage.Value, bool) {
	childObject, childField := splitSummaryQualifiedField(field.SummarizedField)
	fkObject, fkField := splitSummaryQualifiedField(field.SummaryForeignKey)
	operation := strings.ToLower(strings.TrimSpace(field.SummaryOperation))
	if childObject == "" && operation == "count" {
		childObject = fkObject
	}
	if e == nil || e.Org == nil || parent.ID == "" || childObject == "" || fkObject == "" || fkField == "" || !strings.EqualFold(childObject, fkObject) {
		return storage.Value{}, false
	}
	canonicalChild, ok := storage.ResolveObjectName(*e.Org, childObject)
	if !ok {
		return storage.Value{}, false
	}
	childState := e.Org.Objects[canonicalChild]
	childFieldName := ""
	if childField != "" {
		var ok bool
		childFieldName, ok = storage.ResolveFieldName(childState.Definition, e.Org.Namespace, childField)
		if !ok {
			return storage.Value{}, false
		}
	}
	fkFieldName, ok := storage.ResolveFieldName(childState.Definition, e.Org.Namespace, fkField)
	if !ok {
		return storage.Value{}, false
	}
	acc := summaryAccumulator{}
	for _, child := range childState.Records {
		if child.System.IsDeleted || !storage.IDsEqual(idFromStorageValue(child.Fields[fkFieldName]), parent.ID) {
			continue
		}
		if !summaryFiltersMatch(e.Org, childState.Definition, child, field.SummaryFilterItems) {
			continue
		}
		if operation == "count" && childFieldName == "" {
			acc.count++
			continue
		}
		value, ok := summaryRecordFieldValue(e.Org, childState.Definition, child, childFieldName)
		if !ok {
			continue
		}
		acc.add(operation, value)
	}
	return acc.value(operation)
}

func EvaluateRecordSummaryValueInOrg(field storage.Field, org *storage.OrgState, definition storage.ObjectDefinition, record storage.Record) (storage.Value, bool) {
	if org == nil {
		return storage.Value{}, false
	}
	if record.Object == "" {
		record.Object = definition.APIName
	}
	engine := NewEngine(org)
	return engine.evaluateSummaryField(record, field)
}

func splitSummaryQualifiedField(name string) (string, string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ""
	}
	parts := strings.Split(name, ".")
	if len(parts) < 2 {
		return "", name
	}
	return strings.Join(parts[:len(parts)-1], "."), parts[len(parts)-1]
}

func summaryFiltersMatch(org *storage.OrgState, definition storage.ObjectDefinition, record storage.Record, filters []storage.SummaryFilterItem) bool {
	for _, filter := range filters {
		_, fieldName := splitSummaryQualifiedField(filter.Field)
		if fieldName == "" {
			fieldName = filter.Field
		}
		canonical, ok := storage.ResolveFieldName(definition, org.Namespace, fieldName)
		if !ok {
			return false
		}
		value, ok := summaryRecordFieldValue(org, definition, record, canonical)
		if !ok {
			value = storage.NullValue()
		}
		if !summaryFilterMatches(value, filter) {
			return false
		}
	}
	return true
}

func summaryRecordFieldValue(org *storage.OrgState, definition storage.ObjectDefinition, record storage.Record, fieldName string) (storage.Value, bool) {
	field, ok := definition.Fields[fieldName]
	if ok && strings.TrimSpace(field.Formula) != "" {
		value, _, ok := EvaluateRecordFormulaValueInOrg(field.Formula, field, org, definition, record)
		return value, ok
	}
	if value, ok := record.Fields[fieldName]; ok {
		return value, true
	}
	return storage.Value{}, false
}

func summaryFilterMatches(value storage.Value, filter storage.SummaryFilterItem) bool {
	switch strings.ToLower(strings.TrimSpace(filter.Operation)) {
	case "", "equals":
		return summaryValueMatchesText(value, filter.Value)
	default:
		return false
	}
}

func summaryValueMatchesText(value storage.Value, text string) bool {
	text = strings.TrimSpace(text)
	switch value.Kind {
	case storage.ValueBoolean:
		return strings.EqualFold(strconv.FormatBool(value.Boolean), text)
	case storage.ValueString:
		return strings.EqualFold(value.String, text)
	case storage.ValueID:
		return strings.EqualFold(string(value.ID), text)
	case storage.ValueInteger:
		parsed, err := strconv.ParseInt(text, 10, 64)
		return err == nil && value.Integer == parsed
	case storage.ValueDecimal:
		return strings.TrimRight(strings.TrimRight(value.Decimal, "0"), ".") == strings.TrimRight(strings.TrimRight(text, "0"), ".")
	case storage.ValueNull:
		return strings.EqualFold(text, "null") || text == ""
	default:
		return false
	}
}

type summaryAccumulator struct {
	count int64
	sum   float64
	has   bool
	min   storage.Value
	max   storage.Value
}

func (a *summaryAccumulator) add(operation string, value storage.Value) bool {
	switch operation {
	case "count":
		a.count++
		return true
	case "", "sum":
		number, ok := summaryNumericValue(value)
		if !ok {
			return false
		}
		a.count++
		a.sum += number
		return true
	case "max", "min":
		if _, ok := summaryComparableValue(value); !ok {
			return false
		}
		a.count++
		if !a.has {
			a.has = true
			a.min = value
			a.max = value
			return true
		}
		if cmp, ok := compareSummaryValues(value, a.min); ok && cmp < 0 {
			a.min = value
		}
		if cmp, ok := compareSummaryValues(value, a.max); ok && cmp > 0 {
			a.max = value
		}
		return true
	default:
		return false
	}
}

func (a summaryAccumulator) value(operation string) (storage.Value, bool) {
	switch operation {
	case "count":
		return storage.IntegerValue(a.count), true
	case "", "sum":
		return storage.DecimalValue(strconv.FormatFloat(a.sum, 'f', -1, 64)), true
	case "max":
		if !a.has {
			return storage.NullValue(), true
		}
		return a.max, true
	case "min":
		if !a.has {
			return storage.NullValue(), true
		}
		return a.min, true
	default:
		return storage.Value{}, false
	}
}

type summaryComparable struct {
	number  float64
	text    string
	numeric bool
}

func compareSummaryValues(left, right storage.Value) (int, bool) {
	leftValue, ok := summaryComparableValue(left)
	if !ok {
		return 0, false
	}
	rightValue, ok := summaryComparableValue(right)
	if !ok || leftValue.numeric != rightValue.numeric {
		return 0, false
	}
	if leftValue.numeric {
		switch {
		case leftValue.number < rightValue.number:
			return -1, true
		case leftValue.number > rightValue.number:
			return 1, true
		default:
			return 0, true
		}
	}
	return strings.Compare(leftValue.text, rightValue.text), true
}

func summaryComparableValue(value storage.Value) (summaryComparable, bool) {
	if number, ok := summaryNumericValue(value); ok {
		return summaryComparable{number: number, numeric: true}, true
	}
	switch value.Kind {
	case storage.ValueDate, storage.ValueDateTime:
		text := strings.TrimSpace(value.String)
		if text == "" {
			return summaryComparable{}, false
		}
		return summaryComparable{text: text}, true
	default:
		return summaryComparable{}, false
	}
}

func summaryNumericValue(value storage.Value) (float64, bool) {
	switch value.Kind {
	case storage.ValueInteger:
		return float64(value.Integer), true
	case storage.ValueDecimal:
		parsed, err := strconv.ParseFloat(value.Decimal, 64)
		return parsed, err == nil
	case storage.ValueString:
		parsed, err := strconv.ParseFloat(value.String, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}
