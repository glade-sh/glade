package dml

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func (e *Engine) validateLookupFilters(definition storage.ObjectDefinition, record storage.Record) error {
	var names []string
	for name, field := range definition.Fields {
		if field.FilteredLookupInfo.Active && !field.FilteredLookupInfo.OptionalFilter {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		field := definition.Fields[name]
		filter := field.FilteredLookupInfo
		if !filter.Active || filter.OptionalFilter {
			continue
		}
		value, _ := record.GetField(name)
		id := idFromStorageValue(value)
		if id == "" {
			continue
		} // Required-field validation owns missing references.
		target, targetDefinition, found := formulaParentRecord(*e.Org, field, id)
		if !found {
			return dmlErrorf("FIELD_INTEGRITY_EXCEPTION", []string{name}, "invalid lookup reference: %s", name)
		}
		matches := make([]bool, len(filter.FilterItems))
		if len(matches) == 0 {
			return fmt.Errorf("dml: active lookup filter %s.%s has no criteria", definition.APIName, name)
		}
		for i, item := range filter.FilterItems {
			actual, valueField, valueDefinition, err := e.lookupFilterValue(item.Field, definition, record, targetDefinition, target)
			if err != nil {
				return err
			}
			var want formulaValue
			if item.ValueField != "" {
				want, _, _, err = e.lookupFilterValue(item.ValueField, definition, record, targetDefinition, target)
				if err != nil {
					return err
				}
			} else if item.Value != "" {
				value := item.Value
				if strings.EqualFold(valueField.APIName, "RecordTypeId") && valueField.Type == storage.FieldReference {
					// Salesforce lookup-filter RecordType literals use the
					// developer name first; retain Glade's existing label fallback.
					// Prefer the developer name so a label collision cannot
					// select a different record type.
					for _, recordType := range valueDefinition.RecordTypes {
						if recordType.DeveloperName == value && recordType.ID != "" {
							value = string(recordType.ID)
							break
						}
					}
					if value == item.Value {
						for _, recordType := range valueDefinition.RecordTypes {
							if recordType.Name == value && recordType.ID != "" {
								value = string(recordType.ID)
								break
							}
						}
					}
				}
				literal, _, ok := workflowLiteralValue(valueField, value)
				if !ok {
					return fmt.Errorf("dml: unsupported lookup filter literal for %s", item.Field)
				}
				want = formulaStorageValue(literal)
			}
			equal := false
			if actual.blank() || want.blank() {
				equal = actual.blank() && want.blank()
			} else {
				equal = compareFormulaValues(actual, want, "=")
			}
			switch strings.ToLower(item.Operation) {
			case "equals":
				matches[i] = equal
			case "notequal":
				matches[i] = !equal
			default:
				return fmt.Errorf("dml: unsupported lookup filter operation %q on %s.%s", item.Operation, definition.APIName, name)
			}
		}
		accepted, ok := evaluateLookupFilterLogic(filter.BooleanFilter, matches)
		if !ok {
			return fmt.Errorf("dml: unsupported lookup filter logic %q on %s.%s", filter.BooleanFilter, definition.APIName, name)
		}
		if !accepted {
			message := filter.ErrorMessage
			if message == "" {
				message = "Value does not exist or does not match filter criteria."
			}
			return dmlErrorf("FIELD_FILTER_VALIDATION_EXCEPTION", []string{name}, "%s", message)
		}
	}
	return nil
}

func (e *Engine) lookupFilterValue(path string, sourceDefinition storage.ObjectDefinition, source storage.Record, targetDefinition storage.ObjectDefinition, target storage.Record) (formulaValue, storage.Field, storage.ObjectDefinition, error) {
	definition, record := targetDefinition, target
	parts := strings.Split(path, ".")
	if strings.EqualFold(parts[0], "$Source") {
		definition, record = sourceDefinition, source
		parts = parts[1:]
	} else if canonical, ok := storage.ResolveObjectName(*e.Org, parts[0]); ok && canonical == targetDefinition.APIName {
		parts = parts[1:]
	}
	if len(parts) == 0 {
		return formulaValue{}, storage.Field{}, storage.ObjectDefinition{}, fmt.Errorf("dml: invalid lookup filter field %q", path)
	}
	// Validate every path segment before formula evaluation. An unknown field must
	// not become a null value that accidentally satisfies a blank criterion.
	fieldDefinition := definition
	for _, relationship := range parts[:len(parts)-1] {
		lookup, ok := relationshipLookupField(fieldDefinition, e.Org.Namespace, relationship)
		if !ok || len(lookup.ReferenceTo) != 1 {
			return formulaValue{}, storage.Field{}, storage.ObjectDefinition{}, fmt.Errorf("dml: unsupported lookup filter relationship %q", path)
		}
		object, ok := storage.ResolveObjectName(*e.Org, lookup.ReferenceTo[0])
		if !ok {
			return formulaValue{}, storage.Field{}, storage.ObjectDefinition{}, fmt.Errorf("dml: unknown lookup filter target %q", path)
		}
		fieldDefinition = e.Org.Objects[object].Definition
	}
	field, ok := storage.ResolveFieldName(fieldDefinition, e.Org.Namespace, parts[len(parts)-1])
	if !ok {
		return formulaValue{}, storage.Field{}, storage.ObjectDefinition{}, fmt.Errorf("dml: unknown lookup filter field %q", path)
	}
	parser := formulaParser{org: e.Org, definition: definition, record: record, namespace: e.Org.Namespace}
	return parser.valueForRecordField(record, strings.Join(parts, ".")), fieldDefinition.Fields[field], fieldDefinition, nil
}

func evaluateLookupFilterLogic(expression string, matches []bool) (bool, bool) {
	if strings.TrimSpace(expression) == "" {
		for _, match := range matches {
			if !match {
				return false, true
			}
		}
		return true, len(matches) > 0
	}
	var parts []string
	for _, token := range tokenizeFormula(expression) {
		switch token.typ {
		case formulaTokenEOF:
		case formulaTokenNumber:
			index, err := strconv.Atoi(token.text)
			if err != nil || index < 1 || index > len(matches) {
				return false, false
			}
			parts = append(parts, strconv.FormatBool(matches[index-1]))
		case formulaTokenIdent:
			switch strings.ToUpper(token.text) {
			case "AND":
				parts = append(parts, "&&")
			case "OR":
				parts = append(parts, "||")
			default:
				return false, false
			}
		case formulaTokenSymbol:
			if token.text != "(" && token.text != ")" {
				return false, false
			}
			parts = append(parts, token.text)
		default:
			return false, false
		}
	}
	return evaluateRecordFormula(strings.Join(parts, " "), storage.Record{})
}
