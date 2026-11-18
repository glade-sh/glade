package dml

import (
	"math/big"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/glade-sh/glade/internal/storage"
)

var summaryCandidateBuilds atomic.Uint64

// BeginSummaryHold freezes the roll-up values of the parents these child
// records belong to, before the statement changes any child. Reads of those
// roll-ups return the frozen values until ReleaseSummaryHold. Records may lack
// the foreign key (a delete names only an Id), so the stored copies count too.
func (e *Engine) BeginSummaryHold(records []storage.Record) {
	if e == nil || e.Org == nil || e.summaryHold != nil || len(records) == 0 {
		return
	}
	// Most statements touch an object with no roll-up over it. Decide that per
	// object name, once, before collecting any records.
	var relationsByObject map[string][]summaryRelation
	last, lastRelations := "", []summaryRelation(nil)
	for i, record := range records {
		if i == 0 || record.Object != last {
			canonical, ok := storage.ResolveObjectName(*e.Org, record.Object)
			if !ok {
				canonical = record.Object
			}
			last, lastRelations = record.Object, e.summaryRelationsForChild(canonical)
			if len(lastRelations) > 0 {
				if relationsByObject == nil {
					relationsByObject = make(map[string][]summaryRelation)
				}
				relationsByObject[canonical] = lastRelations
			}
		}
	}
	if len(relationsByObject) == 0 {
		return
	}
	var hold *storage.SummaryHold
	var held []heldSummaryRelation
	for child, relations := range relationsByObject {
		var childRecords []storage.Record
		for _, record := range records {
			canonical, ok := storage.ResolveObjectName(*e.Org, record.Object)
			if !ok {
				canonical = record.Object
			}
			if canonical != child {
				continue
			}
			childRecords = append(childRecords, record)
			if record.ID != "" {
				if _, stored, ok := storage.LookupRecordByID(e.Org.Objects[child].Records, record.ID); ok {
					childRecords = append(childRecords, stored)
				}
			}
		}
		for _, relation := range relations {
			parentObject, ok := e.Org.Objects[relation.parentObject]
			if !ok {
				continue
			}
			parentIDs := canonicalSummaryParentIDs(parentObject.Records, summaryParentIDs(childRecords, relation.fkFieldName))
			values, ok := e.evaluateSummaryFieldBatch(relation, parentIDs)
			if !ok {
				continue
			}
			if hold == nil {
				hold = &storage.SummaryHold{Values: make(map[string]storage.Value), Live: make(map[string]storage.Value)}
			}
			held = append(held, heldSummaryRelation{relation: relation, parentIDs: parentIDs})
			for id, value := range values {
				key := summaryHoldKey(relation.parentObject, id, relation.parentField)
				hold.Live[key] = value
				// An enclosing statement's contribution has not landed yet:
				// this statement starts from the enclosing statement's value.
				hold.Values[key] = e.outerHeldValue(key, value)
			}
		}
	}
	if hold != nil {
		e.summaryHold = hold
		e.summaryHoldRelations = held
		e.Org.SummaryHolds = append(e.Org.SummaryHolds, hold)
	}
}

// SuspendSummaryHold takes the hold out of effect without ending it, and
// ResumeSummaryHold puts it back.
func (e *Engine) SuspendSummaryHold() {
	if e == nil || e.summaryHold == nil || e.summaryHoldSuspended || e.Org == nil {
		return
	}
	for i, hold := range e.Org.SummaryHolds {
		if hold == e.summaryHold {
			e.Org.SummaryHolds = append(e.Org.SummaryHolds[:i:i], e.Org.SummaryHolds[i+1:]...)
			e.summaryHoldSuspended = true
			return
		}
	}
}

func (e *Engine) ResumeSummaryHold() {
	if e == nil || e.summaryHold == nil || !e.summaryHoldSuspended || e.Org == nil {
		return
	}
	e.Org.SummaryHolds = append(e.Org.SummaryHolds, e.summaryHold)
	e.summaryHoldSuspended = false
}

// LandSummaryHold adds the stored children's own contribution to the held COUNT
// and SUM values, for a group of an upsert whose change has landed before the
// next group's triggers run. A MIN or MAX hold drops to the live value.
func (e *Engine) LandSummaryHold(records []storage.Record) {
	if e == nil || e.Org == nil || e.summaryHold == nil {
		return
	}
	for _, record := range records {
		child, ok := storage.ResolveObjectName(*e.Org, record.Object)
		if !ok {
			child = record.Object
		}
		state := e.Org.Objects[child]
		_, stored, found := storage.LookupRecordByID(state.Records, record.ID)
		if !found || stored.System.IsDeleted {
			continue
		}
		for _, relation := range e.summaryRelationsForChild(child) {
			parentIDs := canonicalSummaryParentIDs(e.Org.Objects[relation.parentObject].Records, summaryParentIDs([]storage.Record{stored}, relation.fkFieldName))
			if !summaryFiltersMatch(e.Org, state.Definition, stored, relation.field.SummaryFilterItems) {
				continue
			}
			op := strings.ToLower(strings.TrimSpace(relation.field.SummaryOperation))
			var contribution storage.Value
			switch op {
			case "count":
				contribution = storage.IntegerValue(1)
			case "sum":
				_, fieldName := splitSummaryQualifiedField(relation.field.SummarizedField)
				resolved, ok := storage.ResolveFieldName(state.Definition, e.Org.Namespace, fieldName)
				if !ok {
					continue
				}
				value, ok := summaryRecordFieldValue(e.Org, state.Definition, stored, resolved)
				if !ok {
					continue
				}
				contribution = value
			case "max", "min":
				// A landed insert can only raise a MAX or lower a MIN
				// (review_round6 m_none: the held MAX stays 10 when the
				// inserted 7 lands).
				_, fieldName := splitSummaryQualifiedField(relation.field.SummarizedField)
				resolved, ok := storage.ResolveFieldName(state.Definition, e.Org.Namespace, fieldName)
				value, vok := summaryRecordFieldValue(e.Org, state.Definition, stored, resolved)
				for id := range parentIDs {
					key := summaryHoldKey(relation.parentObject, id, relation.parentField)
					current, in := e.summaryHold.Values[key]
					if !in {
						continue
					}
					c, ok1 := summaryNumericRat(current)
					v, ok2 := summaryNumericRat(value)
					if !ok || !vok || !ok1 || !ok2 || current.Kind == storage.ValueNull {
						e.summaryHold.Drop(key)
						continue
					}
					if (op == "max" && v.Cmp(c) > 0) || (op == "min" && v.Cmp(c) < 0) {
						e.summaryHold.Values[key] = value
					}
				}
				continue
			default:
				for id := range parentIDs {
					e.summaryHold.Drop(summaryHoldKey(relation.parentObject, id, relation.parentField))
				}
				continue
			}
			for id := range parentIDs {
				key := summaryHoldKey(relation.parentObject, id, relation.parentField)
				current, in := e.summaryHold.Values[key]
				if !in {
					continue
				}
				if next, good := addSummaryDelta(current, storage.IntegerValue(0), contribution); good {
					e.summaryHold.Values[key] = next
				} else {
					e.summaryHold.Drop(key)
				}
			}
		}
	}
}

// ReleaseSummaryHold ends the hold BeginSummaryHold started. It is safe to call
// again, and after a rollback replaced the org.
func (e *Engine) ReleaseSummaryHold() {
	if e == nil || e.summaryHold == nil {
		return
	}
	if e.Org != nil {
		e.carrySummaryDeltaOut()
		for i, hold := range e.Org.SummaryHolds {
			if hold == e.summaryHold {
				e.Org.SummaryHolds = append(e.Org.SummaryHolds[:i:i], e.Org.SummaryHolds[i+1:]...)
				break
			}
		}
	}
	e.summaryRendered = nil
	e.summaryRenderedOrder = nil
	e.summaryHold = nil
	e.summaryHoldRelations = nil
	e.summaryHoldSuspended = false
}

type heldSummaryRelation struct {
	relation  summaryRelation
	parentIDs map[storage.ID]bool
}

// outerHeldValue is the value an enclosing statement's hold keeps for the key,
// else the live value.
func (e *Engine) outerHeldValue(key string, live storage.Value) storage.Value {
	for i := len(e.Org.SummaryHolds) - 1; i >= 0; i-- {
		if e.Org.SummaryHolds[i].Dropped[key] {
			return live
		}
		if value, ok := e.Org.SummaryHolds[i].Values[key]; ok {
			return value
		}
	}
	return live
}

// carrySummaryDeltaOut adds what this statement changed in a roll-up to the
// enclosing statements' held values (native rollup nested rows: the inner
// statement's change lands, the enclosing statement's has not yet). A roll-up
// that is not a COUNT or SUM drops its held value instead.
func (e *Engine) carrySummaryDeltaOut() {
	own := -1
	for i, hold := range e.Org.SummaryHolds {
		if hold == e.summaryHold {
			own = i
		}
	}
	if own <= 0 {
		return
	}
	for _, held := range e.summaryHoldRelations {
		now, ok := e.evaluateSummaryFieldBatch(held.relation, held.parentIDs)
		for id := range held.parentIDs {
			key := summaryHoldKey(held.relation.parentObject, id, held.relation.parentField)
			before, had := e.summaryHold.Live[key]
			// Only the nearest enclosing hold that keeps this roll-up takes the
			// change; that statement's own release carries it further out.
			for i := own - 1; i >= 0; i-- {
				outer := e.Org.SummaryHolds[i]
				current, in := outer.Values[key]
				if !in || outer.Dropped[key] {
					continue
				}
				op := strings.ToLower(strings.TrimSpace(held.relation.field.SummaryOperation))
				// MIN and MAX read the live value after a nested statement
				// (review_round3 n_sib*, n_del): the hold forgets the key and
				// stops outer holds from answering for it.
				if ok && had && outer.Upsert && (op == "max" || op == "min") && summaryValuesEqual(before, now[id]) {
					// This statement left the extreme alone: the enclosing
					// statement's held value stands (review_round6 m_nested).
					break
				}
				if !ok || !had || (op != "count" && op != "sum") {
					outer.Drop(key)
					break
				}
				if next, good := addSummaryDelta(current, before, now[id]); good {
					outer.Values[key] = next
				} else {
					outer.Drop(key)
				}
				break
			}
		}
	}
}

func addSummaryDelta(current, before, now storage.Value) (storage.Value, bool) {
	c, ok1 := summaryNumericRat(current)
	b, ok2 := summaryNumericRat(before)
	n, ok3 := summaryNumericRat(now)
	if !ok1 || !ok2 || !ok3 || current.Kind == storage.ValueNull {
		return storage.Value{}, false
	}
	sum := new(big.Rat).Add(c, new(big.Rat).Sub(n, b))
	if current.Kind == storage.ValueInteger && sum.IsInt() {
		return storage.IntegerValue(sum.Num().Int64()), true
	}
	scale := 0
	if current.Kind == storage.ValueDecimal {
		if point := strings.IndexByte(current.Decimal, '.'); point >= 0 {
			scale = len(current.Decimal) - point - 1
		}
	}
	return summaryValueAtScale(storage.DecimalValue(summaryDecimalString(sum)), scale), true
}

func summaryHoldKey(object string, id storage.ID, field string) string {
	return strings.ToLower(object) + "\x00" + strings.ToLower(string(id)) + "\x00" + strings.ToLower(field)
}

// heldSummary returns the frozen value of a parent's roll-up, innermost
// statement first.
func (e *Engine) heldSummary(parent storage.Record, field storage.Field) (storage.Value, bool) {
	object, ok := storage.ResolveObjectName(*e.Org, parent.Object)
	if !ok {
		object = parent.Object
	}
	id := parent.ID
	if storedID, _, ok := storage.LookupRecordByID(e.Org.Objects[object].Records, id); ok {
		id = storedID
	}
	key := summaryHoldKey(object, id, field.APIName)
	for i := len(e.Org.SummaryHolds) - 1; i >= 0; i-- {
		if e.Org.SummaryHolds[i].Dropped[key] {
			return storage.Value{}, false
		}
		if value, ok := e.Org.SummaryHolds[i].Values[key]; ok {
			return value, true
		}
	}
	return storage.Value{}, false
}

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
			e.applyQuoteDerivedFields(&parentRecord)
			parentObject.PutRecord(storedParentID, parentRecord)
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

// A template cache is already shared only within its frozen schema generation.
// Index summary fields once for that generation, rather than scanning every
// scalar field again for each distinct child object. Unstamped host orgs remain
// mutable and retain the full scan on each uncached child lookup.
func (c *SummaryRelationCache) summaryCandidates(org *storage.OrgState) []summaryRelation {
	collect := func() []summaryRelation {
		summaryCandidateBuilds.Add(1)
		var fields []summaryRelation
		for parentName, parent := range org.Objects {
			for fieldName, field := range parent.Definition.Fields {
				if field.Type == storage.FieldSummary {
					fields = append(fields, summaryRelation{parentObject: parentName, parentField: fieldName, field: field})
				}
			}
		}
		return fields
	}
	if org.RuntimeSchemaStamp == "" {
		return collect()
	}
	c.candidatesOnce.Do(func() { c.candidates = collect() })
	return c.candidates
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
	for _, candidate := range e.SummaryByChild.summaryCandidates(e.Org) {
		parentObjectName, parentFieldName, field := candidate.parentObject, candidate.parentField, candidate.field
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
	// DuplicateRecordSet.RecordCount is a system-maintained count of manually
	// persisted items. Reuse the rollup pipeline, including deletion and rollback.
	if strings.EqualFold(childObjectName, "DuplicateRecordItem") {
		parent, parentOK := e.Org.Objects["DuplicateRecordSet"]
		field, fieldOK := parent.Definition.Fields["RecordCount"]
		if parentOK && fieldOK && field.Type != storage.FieldSummary {
			field.SummaryOperation = "count"
			field.SummaryForeignKey = "DuplicateRecordItem.DuplicateRecordSetId"
			relations = append(relations, summaryRelation{parentObject: "DuplicateRecordSet", parentField: "RecordCount", field: field, fkFieldName: "DuplicateRecordSetId"})
		}
	}
	if strings.EqualFold(childObjectName, "WorkOrderLineItem") {
		parent, parentOK := e.Org.Objects["WorkOrder"]
		field, fieldOK := parent.Definition.Fields["LineItemCount"]
		if parentOK && fieldOK && field.Type != storage.FieldSummary {
			field.SummaryOperation = "count"
			field.SummaryForeignKey = "WorkOrderLineItem.WorkOrderId"
			relations = append(relations, summaryRelation{parentObject: "WorkOrder", parentField: "LineItemCount", field: field, fkFieldName: "WorkOrderId"})
		}
	}
	// Order totals use the same child-mutation, deletion and rollback path
	// as declared rollups without changing the public describe field shape.
	if strings.EqualFold(childObjectName, "OrderItem") {
		parent, parentOK := e.Org.Objects["Order"]
		field, fieldOK := parent.Definition.Fields["TotalAmount"]
		if parentOK && fieldOK && field.Type != storage.FieldSummary {
			field.SummaryOperation = "sum"
			field.SummaryForeignKey = "OrderItem.OrderId"
			field.SummarizedField = "OrderItem.TotalPrice"
			relations = append(relations, summaryRelation{parentObject: "Order", parentField: "TotalAmount", field: field, fkFieldName: "OrderId"})
		}
	}
	if strings.EqualFold(childObjectName, "OpportunityLineItem") {
		parent, parentOK := e.Org.Objects["Opportunity"]
		field, fieldOK := parent.Definition.Fields["Amount"]
		if parentOK && fieldOK && field.Type != storage.FieldSummary {
			field.SummaryOperation = "sum"
			field.SummaryForeignKey = "OpportunityLineItem.OpportunityId"
			field.SummarizedField = "OpportunityLineItem.TotalPrice"
			relations = append(relations, summaryRelation{parentObject: "Opportunity", parentField: "Amount", field: field, fkFieldName: "OpportunityId"})
		}
	}
	if strings.EqualFold(childObjectName, "QuoteLineItem") {
		parent, parentOK := e.Org.Objects["Quote"]
		for _, rollup := range []struct{ field, operation, summarized string }{
			{"Subtotal", "sum", "QuoteLineItem.Subtotal"},
			{"TotalPrice", "sum", "QuoteLineItem.TotalPrice"},
			{"LineItemCount", "count", ""},
		} {
			field, fieldOK := parent.Definition.Fields[rollup.field]
			if !parentOK || !fieldOK || field.Type == storage.FieldSummary {
				continue
			}
			field.SummaryOperation = rollup.operation
			field.SummaryForeignKey = "QuoteLineItem.QuoteId"
			field.SummarizedField = rollup.summarized
			relations = append(relations, summaryRelation{parentObject: "Quote", parentField: rollup.field, field: field, fkFieldName: "QuoteId"})
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
	scale := summaryScale(operation, relation.field, childState.Definition, childFieldName)
	for parentID := range parentIDs {
		value, ok := acc[parentID].value(operation)
		if !ok {
			return nil, false
		}
		values[parentID] = summaryValueAtScale(value, scale)
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
	if len(e.Org.SummaryHolds) > 0 {
		if value, ok := e.heldSummary(parent, field); ok {
			return value, true
		}
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
	value, ok := acc.value(operation)
	if !ok {
		return value, ok
	}
	return summaryValueAtScale(value, summaryScale(operation, field, childState.Definition, childFieldName)), true
}

// summaryScale is the scale a sum, minimum or maximum roll-up carries: the
// roll-up field's own declared scale, else the scale of the summarized Number
// field. Native rollup-summary rolltest and rollanon (62.0 and 67.0): a
// roll-up with no declared scale over a Number(10,2) rolls up as 60.00, 100.00
// and 0.00 for an empty sum. Only Number is captured; Currency, Percent, counts
// and other shapes keep their value (scale 0 here).
func summaryScale(operation string, rollup storage.Field, definition storage.ObjectDefinition, fieldName string) int {
	switch operation {
	case "", "sum", "min", "max":
	default:
		return 0
	}
	if rollup.ScaleSpecified && rollup.Scale > 0 {
		return rollup.Scale
	}
	field, ok := definition.Fields[fieldName]
	if !ok || field.Type != storage.FieldDecimal || field.Scale <= 0 || !field.ScaleSpecified || !strings.EqualFold(strings.TrimSpace(field.DisplayType), "DOUBLE") {
		return 0
	}
	return field.Scale
}

func summaryValueAtScale(value storage.Value, scale int) storage.Value {
	if scale <= 0 {
		return value
	}
	number, ok := summaryNumericRat(value)
	if !ok {
		return value
	}
	text := summaryDecimalString(number)
	if point := strings.IndexByte(text, '.'); point >= 0 && len(text)-point-1 >= scale {
		return value
	}
	return storage.DecimalValue(number.FloatString(scale))
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
	case "notequal":
		return !summaryValueMatchesText(value, filter.Value)
	case "greaterthan", "lessthan", "greaterorequal", "lessorequal":
		// review_round3 Fc__c: a Number filter compares as numbers.
		left, ok := summaryNumericRat(value)
		right, ok2 := new(big.Rat).SetString(strings.TrimSpace(filter.Value))
		if !ok || !ok2 {
			return false
		}
		c := left.Cmp(right)
		switch strings.ToLower(strings.TrimSpace(filter.Operation)) {
		case "greaterthan":
			return c > 0
		case "lessthan":
			return c < 0
		case "greaterorequal":
			return c >= 0
		default:
			return c <= 0
		}
	default:
		return false
	}
}

func summaryValueMatchesText(value storage.Value, text string) bool {
	for _, candidate := range strings.Split(text, ",") {
		if summaryValueMatchesSingleText(value, candidate) {
			return true
		}
	}
	return false
}

func summaryValueMatchesSingleText(value storage.Value, text string) bool {
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
	sum   *big.Rat
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
		number, ok := summaryNumericRat(value)
		if !ok {
			return false
		}
		a.count++
		if a.sum == nil {
			a.sum = new(big.Rat)
		}
		a.sum.Add(a.sum, number)
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
		if a.sum == nil {
			return storage.DecimalValue("0"), true
		}
		return storage.DecimalValue(summaryDecimalString(a.sum)), true
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

func summaryNumericRat(value storage.Value) (*big.Rat, bool) {
	switch value.Kind {
	case storage.ValueInteger:
		return new(big.Rat).SetInt64(value.Integer), true
	case storage.ValueDecimal:
		parsed, ok := new(big.Rat).SetString(value.Decimal)
		return parsed, ok
	case storage.ValueString:
		parsed, ok := new(big.Rat).SetString(value.String)
		return parsed, ok
	default:
		return nil, false
	}
}

func summaryDecimalString(value *big.Rat) string {
	if value.IsInt() {
		return value.Num().String()
	}
	numerator := new(big.Int).Set(value.Num())
	denominator := new(big.Int).Set(value.Denom())
	sign := ""
	if numerator.Sign() < 0 {
		sign = "-"
		numerator.Abs(numerator)
	}
	twos := 0
	fives := 0
	two := big.NewInt(2)
	five := big.NewInt(5)
	zero := new(big.Int)
	for {
		quotient, rem := new(big.Int).QuoRem(denominator, two, new(big.Int))
		if rem.Cmp(zero) != 0 {
			break
		}
		denominator = quotient
		twos++
	}
	for {
		quotient, rem := new(big.Int).QuoRem(denominator, five, new(big.Int))
		if rem.Cmp(zero) != 0 {
			break
		}
		denominator = quotient
		fives++
	}
	if denominator.Sign() == 1 && denominator.Cmp(big.NewInt(1)) == 0 {
		scale := twos
		if fives > scale {
			scale = fives
		}
		multiplier := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(scale)), nil)
		scaled := new(big.Int).Mul(numerator, multiplier)
		scaled.Quo(scaled, value.Denom())
		text := scaled.String()
		if scale == 0 {
			return sign + text
		}
		if len(text) <= scale {
			text = strings.Repeat("0", scale-len(text)+1) + text
		}
		point := len(text) - scale
		return strings.TrimRight(strings.TrimRight(sign+text[:point]+"."+text[point:], "0"), ".")
	}
	return strings.TrimRight(strings.TrimRight(value.FloatString(18), "0"), ".")
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

// ValidateSummaryUpdate evaluates parent validation rules after its before trigger.
func (e *Engine) ValidateSummaryUpdate(update SummaryUpdate, record storage.Record) Result {
	object, ok := e.Org.Objects[update.Object]
	if !ok {
		return Result{Success: true}
	}
	if err := e.validateValidationRules(update.Object, object.Definition, record, &update.Before, false); err != nil {
		return resultFromError("", err)
	}
	return Result{Success: true}
}

// SummaryValidationRelation returns the parent and foreign key for a direct,
// unfiltered COUNT whose parent has an active validation rule. Other summary
// shapes need their own evidence for attributing parent errors to child rows.
func (e *Engine) SummaryValidationRelation(childObject string) (parent, foreignKey string) {
	relations := e.summaryRelationsForChild(childObject)
	if len(relations) != 1 {
		return "", ""
	}
	relation := relations[0]
	if relation.field.Type != storage.FieldSummary || !strings.EqualFold(relation.field.SummaryOperation, "count") || len(relation.field.SummaryFilterItems) != 0 {
		return "", ""
	}
	// A parent that is itself rolled up queues grandparent updates; those
	// chains have no native evidence.
	if len(e.summaryRelationsForChild(relation.parentObject)) != 0 {
		return "", ""
	}
	for _, rule := range e.Org.Objects[relation.parentObject].Definition.ValidationRules {
		if rule.Active {
			return relation.parentObject, relation.fkFieldName
		}
	}
	return "", ""
}

func summaryValuesEqual(a, b storage.Value) bool {
	x, ok1 := summaryNumericRat(a)
	y, ok2 := summaryNumericRat(b)
	return ok1 && ok2 && x.Cmp(y) == 0
}

// PeekSummaryUpdates returns the pending parent roll-up updates without
// taking them.
func (e *Engine) PeekSummaryUpdates() []SummaryUpdate {
	if e == nil || len(e.summaryOrder) == 0 {
		return nil
	}
	updates := make([]SummaryUpdate, 0, len(e.summaryOrder))
	for _, key := range e.summaryOrder {
		if update, ok := e.summaryUpdates[key]; ok {
			updates = append(updates, update)
		}
	}
	return updates
}

// RequeueSummaryUpdates puts parent updates back so the parent's triggers run
// again for a later group of the same statement.
func (e *Engine) RequeueSummaryUpdates(updates []SummaryUpdate) {
	if e == nil {
		return
	}
	for _, update := range updates {
		key := strings.ToLower(update.Object) + "\x00" + strings.ToLower(string(update.After.ID))
		if e.summaryUpdates == nil {
			e.summaryUpdates = make(map[string]SummaryUpdate)
		}
		if _, exists := e.summaryUpdates[key]; !exists {
			e.summaryOrder = append(e.summaryOrder, key)
		}
		e.summaryUpdates[key] = update
	}
}

// HeldSummaryDiffersFromLive reports whether the hold's values for any held
// roll-up differ from the stored value, that is, whether the statement still
// has a change to land.
func (e *Engine) HeldSummaryDiffersFromLive() bool {
	if e == nil || e.Org == nil || e.summaryHold == nil {
		return false
	}
	for _, held := range e.summaryHoldRelations {
		now, ok := e.evaluateSummaryFieldBatch(held.relation, held.parentIDs)
		if !ok {
			continue
		}
		for id := range held.parentIDs {
			key := summaryHoldKey(held.relation.parentObject, id, held.relation.parentField)
			value, in := e.summaryHold.Values[key]
			if !in || e.summaryHold.Dropped[key] {
				continue
			}
			if !summaryValuesEqual(value, now[id]) {
				return true
			}
		}
	}
	return false
}

// MarkSummaryHoldUpsert marks the hold as spanning an upsert's groups.
func (e *Engine) MarkSummaryHoldUpsert() {
	if e != nil && e.summaryHold != nil {
		e.summaryHold.Upsert = true
	}
}

// summaryParentKeys names the pending-update keys of the parents these child
// records belong to.
func (e *Engine) summaryParentKeys(records []storage.Record) map[string]bool {
	keys := make(map[string]bool)
	if e == nil || e.Org == nil {
		return keys
	}
	for _, record := range records {
		child, ok := storage.ResolveObjectName(*e.Org, record.Object)
		if !ok {
			child = record.Object
		}
		candidates := []storage.Record{record}
		if _, stored, found := storage.LookupRecordByID(e.Org.Objects[child].Records, record.ID); found {
			candidates = append(candidates, stored)
		}
		for _, relation := range e.summaryRelationsForChild(child) {
			parentIDs := canonicalSummaryParentIDs(e.Org.Objects[relation.parentObject].Records, summaryParentIDs(candidates, relation.fkFieldName))
			for id := range parentIDs {
				keys[strings.ToLower(relation.parentObject)+"\x00"+strings.ToLower(string(id))] = true
			}
		}
	}
	return keys
}

// SetSummaryTakeScope limits TakeSummaryUpdates to the parents of these child
// records; the others stay pending.
func (e *Engine) SetSummaryTakeScope(records []storage.Record) {
	if e == nil {
		return
	}
	e.summaryTakeScope = e.summaryParentKeys(records)
}

// ClearSummaryTakeScope makes TakeSummaryUpdates take every parent again.
func (e *Engine) ClearSummaryTakeScope() {
	if e != nil {
		e.summaryTakeScope = nil
	}
}

// OverlayHeldSummary gives a parent update the roll-up values its triggers read
// while a hold is in effect: the old record carries the values from before the
// statement, the new record the held (landed) values. Later groups of the same
// upsert start from what this one rendered (review_round7 b_single, b_two).
func (e *Engine) OverlayHeldSummary(update *SummaryUpdate) {
	if e == nil || update == nil || e.Org == nil || e.summaryHold == nil || e.summaryHoldSuspended {
		return
	}
	changed := false
	for _, held := range e.summaryHoldRelations {
		if !strings.EqualFold(held.relation.parentObject, update.Object) || !held.parentIDs[update.After.ID] {
			continue
		}
		key := summaryHoldKey(held.relation.parentObject, update.After.ID, held.relation.parentField)
		if live, ok := e.summaryHold.Live[key]; ok && update.Before.Fields != nil {
			if rendered, had := e.summaryRendered[strings.ToLower(update.Object)+"\x00"+strings.ToLower(string(update.After.ID))]; had {
				if value, ok := rendered.Fields[held.relation.parentField]; ok {
					live = value
				}
			}
			update.Before.Fields[held.relation.parentField] = live
			changed = true
		}
		if value, ok := e.summaryHold.Values[key]; ok && !e.summaryHold.Dropped[key] && update.After.Fields != nil {
			update.After.Fields[held.relation.parentField] = value
			changed = true
		}
	}
	if changed {
		if e.summaryRendered == nil {
			e.summaryRendered = make(map[string]storage.Record)
		}
		renderedKey := strings.ToLower(update.Object) + "\x00" + strings.ToLower(string(update.After.ID))
		if _, seen := e.summaryRendered[renderedKey]; !seen {
			e.summaryRenderedOrder = append(e.summaryRenderedOrder, renderedKey)
		}
		e.summaryRendered[renderedKey] = update.After.Clone()
	}
}

// RequeueRenderedSummaryUpdates fires a parent again for a later group when
// that group's children belong to it and its held roll-up still differs from
// the stored one.
func (e *Engine) RequeueRenderedSummaryUpdates(records []storage.Record) {
	if e == nil || e.Org == nil || e.summaryHold == nil || len(e.summaryRendered) == 0 {
		return
	}
	scope := e.summaryParentKeys(records)
	// Refire in the order the insert pass fired, not in map order.
	for _, key := range e.summaryRenderedOrder {
		rendered := e.summaryRendered[key]
		if !scope[key] || !e.heldSummaryDiffersForParent(rendered.Object, rendered.ID) {
			continue
		}
		objectName := rendered.Object
		_, stored, ok := storage.LookupRecordByID(e.Org.Objects[objectName].Records, rendered.ID)
		if !ok {
			continue
		}
		e.RequeueSummaryUpdates([]SummaryUpdate{{Object: objectName, Before: rendered.Clone(), After: stored.Clone()}})
	}
}

func (e *Engine) heldSummaryDiffersForParent(object string, id storage.ID) bool {
	for _, held := range e.summaryHoldRelations {
		if !strings.EqualFold(held.relation.parentObject, object) || !held.parentIDs[id] {
			continue
		}
		now, ok := e.evaluateSummaryFieldBatch(held.relation, map[storage.ID]bool{id: true})
		if !ok {
			continue
		}
		key := summaryHoldKey(held.relation.parentObject, id, held.relation.parentField)
		value, in := e.summaryHold.Values[key]
		if !in || e.summaryHold.Dropped[key] {
			continue
		}
		if !summaryValuesEqual(value, now[id]) {
			return true
		}
	}
	return false
}
