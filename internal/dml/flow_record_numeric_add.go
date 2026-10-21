package dml

import (
	"math/big"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func (e *Engine) flowRecordNumericArithmetic(assignment storage.FlowAssignment, operator string, frame *flowFrame) (bool, error) {
	if (operator != "add" && operator != "subtract") || e == nil || e.Org == nil {
		return false, nil
	}
	name, fieldName, dotted := strings.Cut(strings.TrimSpace(assignment.Target), ".")
	variable, declared := frame.variables[flowFrameKey(name)]
	key := flowFrameKey(name)
	output, lookupTarget := frame.lookupOutputs[key]
	triggerRecord := strings.EqualFold(name, "$Record")
	if !dotted || strings.Contains(fieldName, ".") || (!triggerRecord && !lookupTarget && (!declared || variable.IsCollection || !strings.EqualFold(variable.DataType, "SObject"))) {
		return false, nil
	}
	definition := output.definition
	if triggerRecord {
		record, present := frame.records[key]
		if !present {
			return false, nil
		}
		objectName, modeled := storage.ResolveObjectName(*e.Org, record.Object)
		if !modeled {
			return false, nil
		}
		definition = e.Org.Objects[objectName].Definition
	} else if !lookupTarget {
		objectName, exists := storage.ResolveObjectName(*e.Org, variable.ObjectType)
		if !exists {
			return false, nil
		}
		definition = e.Org.Objects[objectName].Definition
	}
	field, exists := storage.ResolveFieldName(definition, e.Org.Namespace, fieldName)
	if !exists || (definition.Fields[field].Type != storage.FieldDecimal && definition.Fields[field].Type != storage.FieldInteger) {
		return false, nil
	}
	left, leftOK := e.flowFrameReferenceValue(assignment.Target, frame)
	right, rightOK := e.flowFrameValue(assignment, frame)
	if !leftOK || !rightOK || (left.Kind != storage.ValueNull && left.Kind != storage.ValueInteger && left.Kind != storage.ValueDecimal) || (right.Kind != storage.ValueInteger && right.Kind != storage.ValueDecimal) {
		return false, nil
	}
	if left.Kind == storage.ValueNull {
		left = storage.IntegerValue(0)
	}
	symbol := "+"
	if operator == "subtract" {
		symbol = "-"
	}
	sum, ok := preciseFormulaNumberBinary(symbol, formulaStorageValue(left), formulaStorageValue(right))
	if !ok {
		return true, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow assignment %s has unsupported numeric operands", assignment.Name)
	}
	number, ok := formulaNumberRat(sum)
	if !ok {
		return true, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow assignment %s has unsupported numeric result", assignment.Name)
	}
	record := frame.records[key]
	if lookupTarget {
		record = output.record.Clone()
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	if definition.Fields[field].Type == storage.FieldInteger {
		if number.Denom().Cmp(big.NewInt(1)) != 0 {
			return true, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow assignment %s has unsupported integer result", assignment.Name)
		}
		record.Fields[field] = storage.IntegerValue(number.Num().Int64())
	} else {
		record.Fields[field] = storage.DecimalValue(number.FloatString(definition.Fields[field].Scale))
	}
	delete(record.ExplicitNulls, field)
	frame.records[key] = record
	if lookupTarget {
		output.record = record
		frame.lookupOutputs[key] = output
	}
	delete(frame.nullRecordDefaults, key)
	frame.assigned[flowFrameKey(assignment.Target)] = true
	return true, nil
}
