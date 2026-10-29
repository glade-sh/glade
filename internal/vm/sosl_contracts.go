package vm

import (
	"errors"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/sosl"
	"github.com/glade-sh/glade/internal/storage"
)

// Metadata errors are checked before candidate filtering, including empty
// results. SOSL shares SELECT's field and relationship resolver.
func (vm *VM) validateSOSLSelections(query sosl.Query, original *sosl.Query) error {
	metadataVM := vm.soslMetadataVM(query)
	if query.Limit.HasValue && query.Limit.Value > 2000 {
		return newExceptionError("QueryException", "maximum limit of search results is 2000")
	}
	for returningIndex, spec := range query.Returning {
		if spec.Limit.HasValue {
			if spec.Limit.Value < 0 {
				return newExceptionError("QueryException", "Limit must be a non-negative value")
			}
			if spec.Limit.Value > 2000 {
				return newExceptionError("QueryException", "Maximum entity limit is 2000")
			}
		}
		if spec.Offset.HasValue && (spec.Offset.Value < 0 && !spec.EmptyNegativeOffset || spec.Offset.Value > 2000) {
			return newExceptionError("SearchException", "SOSL offset should be between 0 and 2000")
		}
		fields := make([]string, 0, len(spec.Fields))
		seen := map[string]bool{}
		for _, field := range spec.Fields {
			key := strings.ToLower(field.Field)
			if field.Func == "" {
				if seen[key] {
					return newExceptionError("QueryException", "duplicate field selected: "+field.Field)
				}
				seen[key] = true
			}
			fields = append(fields, field.Field)
		}
		object, ok := metadataVM.resolveObjectName(spec.Object)
		if !ok {
			return newExceptionError("QueryException", soslMissingObjectMessage(spec.Object))
		}
		selection := soql.Query{Object: object, Fields: fields, Where: soslSelectionCondition(spec.Where)}
		for _, order := range spec.OrderBy {
			selection.Order = append(selection.Order, soql.OrderSpec{Field: order.Field, Desc: order.Desc, Nulls: order.Nulls})
		}
		if err := soql.ValidateReferences(*metadataVM.Org, metadataVM.Org.Objects[object].Definition, selection); err != nil {
			message := err.Error()
			var queryErr *soql.QueryError
			if errors.As(err, &queryErr) {
				message = queryErr.Message
			}
			return newExceptionError("QueryException", message)
		}
		var check func(*soql.Condition, *sosl.Condition) error
		check = func(condition *soql.Condition, source *sosl.Condition) error {
			if condition == nil {
				return nil
			}
			for i := range condition.And {
				var childSource *sosl.Condition
				if source != nil && i < len(source.And) {
					childSource = &source.And[i]
				}
				if err := check(&condition.And[i], childSource); err != nil {
					return err
				}
			}
			for i := range condition.Or {
				var childSource *sosl.Condition
				if source != nil && i < len(source.Or) {
					childSource = &source.Or[i]
				}
				if err := check(&condition.Or[i], childSource); err != nil {
					return err
				}
			}
			if condition.Op == "IN" || condition.Op == "NOT IN" {
				definition := metadataVM.Org.Objects[object].Definition
				if name, ok := storage.ResolveFieldName(definition, metadataVM.Org.Namespace, condition.Field); ok {
					if field := definition.Fields[name]; field.Filterable != nil && !*field.Filterable {
						return nil // No native rows for unfilterable-field IN precedence.
					}
				}
				// Only numeric list literals have native type-rejection rows.
				// Preserve the existing treatment of other list member types.
				literal := *condition
				literal.Values = nil
				if source != nil {
					// Read literals before expansion: a null or collection bind
					// can change the number and positions of expanded members.
					for _, value := range soslSelectionCondition(source).Values {
						if value.Kind == storage.ValueInteger || value.Kind == storage.ValueDecimal {
							literal.Values = append(literal.Values, value)
						}
					}
				}
				if len(literal.Values) == 0 {
					return nil
				}
				return metadataVM.validateSOQLSelectionLiteral(object, &literal)
			}
			return metadataVM.validateSOQLSelectionLiteralSource(object, condition, source != nil && source.Bind == "")
		}
		var source *sosl.Condition
		if original != nil && returningIndex < len(original.Returning) {
			source = original.Returning[returningIndex].Where
		}
		if err := check(selection.Where, source); err != nil {
			return err
		}
	}
	return nil
}

// A bare anonymous VM still has standard schema. Use definitions only, leaving
// its record store absent so this validation cannot manufacture search results.
// Attached orgs retain their own custom schema and enabled features.
func (vm *VM) soslMetadataVM(query sosl.Query) *VM {
	if vm.Org != nil {
		return vm
	}
	org := storage.NewOrgState()
	for _, spec := range query.Returning {
		if definition, ok := storage.StandardObjectDefinition(spec.Object); ok {
			org.Objects[definition.APIName] = storage.ObjectState{Definition: definition}
		}
	}
	return &VM{Org: &org}
}

func soslMissingObjectMessage(name string) string {
	return "sObject type '" + strings.ToLower(name) + "' is not supported. If you are attempting to use a custom object, be sure to append the '__c' after the entity name. Please reference your WSDL or the describe call for the appropriate names."
}

func soslAccessLevel(mode string) Value {
	level := Object("AccessLevel")
	level.Text = mode
	return level
}

func soslLiteralTermEnd(raw string) int {
	start := len(raw) - len(strings.TrimLeft(raw, " \t\r\n"))
	if len(raw)-start < 4 || !strings.EqualFold(raw[start:start+4], "FIND") {
		return 0
	}
	start += 4
	for start < len(raw) && strings.ContainsRune(" \t\r\n", rune(raw[start])) {
		start++
	}
	if start >= len(raw) || (raw[start] != '{' && raw[start] != '\'') {
		return 0
	}
	closing := byte('\'')
	if raw[start] == '{' {
		closing = '}'
	}
	for i := start + 1; i < len(raw); i++ {
		if raw[i] == '\\' && i+1 < len(raw) {
			i++
			continue
		}
		if raw[i] == closing {
			if closing == '\'' && i+1 < len(raw) && raw[i+1] == '\'' {
				i++
				continue
			}
			return i + 1
		}
	}
	return 0
}

func soslSelectionCondition(condition *sosl.Condition) *soql.Condition {
	if condition == nil {
		return nil
	}
	out := &soql.Condition{Field: condition.Field, Op: condition.Operator, Not: condition.Not}
	for i := range condition.And {
		out.And = append(out.And, *soslSelectionCondition(&condition.And[i]))
	}
	for i := range condition.Or {
		out.Or = append(out.Or, *soslSelectionCondition(&condition.Or[i]))
	}
	switch {
	case condition.ValueIsNull:
		out.Value = storage.NullValue()
	case condition.Bind != "":
		out.Value = storage.StringValue(":" + condition.Bind)
	case condition.ValueQuoted:
		out.Value = storage.StringValue(condition.Value)
	default:
		if n, err := strconv.ParseInt(condition.Value, 10, 64); err == nil {
			out.Value = storage.IntegerValue(n)
		} else if _, err := strconv.ParseFloat(condition.Value, 64); err == nil {
			out.Value = storage.DecimalValue(condition.Value)
		} else {
			out.Value = storage.StringValue(condition.Value)
		}
	}
	for i, value := range condition.Values {
		literal := storage.StringValue(value)
		if condition.UnquotedValues[i] {
			if n, err := strconv.ParseInt(value, 10, 64); err == nil {
				literal = storage.IntegerValue(n)
			} else if _, err := strconv.ParseFloat(value, 64); err == nil {
				literal = storage.DecimalValue(value)
			}
		}
		out.Values = append(out.Values, literal)
	}
	return out
}
