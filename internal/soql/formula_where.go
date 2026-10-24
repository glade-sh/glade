package soql

import (
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/storage"
)

const FormulaWhereVersionMessage = "FORMULA() in a SOQL WHERE clause is a Beta feature available in Winter '27 and later, and only in Developer Edition, scratch, and sandbox orgs."

// FormulaExpression is the two-field arithmetic accepted in a WHERE predicate.
// It deliberately does not expose the general formula engine's functions.
type FormulaExpression struct {
	Left, Operator, Right string
}

var formulaWherePattern = regexp.MustCompile(`^\s*([a-zA-Z_][a-zA-Z0-9_]*(?:\.[a-zA-Z_][a-zA-Z0-9_]*)*)\s*([+-])\s*([a-zA-Z_][a-zA-Z0-9_]*(?:\.[a-zA-Z_][a-zA-Z0-9_]*)*)\s*$`)

func parseWhereFormula(field string) (*FormulaExpression, error) {
	if !strings.HasPrefix(field, "FORMULA(") {
		return nil, nil
	}
	text := field[len("FORMULA('") : len(field)-2]
	parts := formulaWherePattern.FindStringSubmatch(text)
	if len(parts) != 4 {
		return nil, queryError("Invalid formula: expected two field identifiers separated by + or -", "invalid WHERE formula")
	}
	return &FormulaExpression{Left: parts[1], Operator: parts[2], Right: parts[3]}, nil
}

func conditionHasFormula(condition *Condition) bool {
	if condition == nil {
		return false
	}
	if condition.Formula != nil {
		return true
	}
	for _, child := range condition.And {
		if conditionHasFormula(&child) {
			return true
		}
	}
	for _, child := range condition.Or {
		if conditionHasFormula(&child) {
			return true
		}
	}
	if condition.Subquery != nil {
		return queryHasFormula(*condition.Subquery)
	}
	return false
}

func queryHasFormula(query Query) bool {
	if conditionHasFormula(query.Where) || conditionHasFormula(query.Having) {
		return true
	}
	for _, child := range query.ChildQueries {
		if queryHasFormula(child.Query) {
			return true
		}
	}
	return false
}

// ValidateFormulaVersion keeps the source API outside the parsed-query cache.
func ValidateFormulaVersion(query Query, apiVersion string) error {
	if apexversion.Before(apiVersion, 68) && queryHasFormula(query) {
		return queryError(FormulaWhereVersionMessage, FormulaWhereVersionMessage)
	}
	return nil
}

func formulaResultType(left, right storage.FieldType, operator string) (storage.FieldType, bool) {
	numeric := func(kind storage.FieldType) bool {
		return kind == storage.FieldInteger || kind == storage.FieldDecimal
	}
	if numeric(left) && numeric(right) {
		return storage.FieldDecimal, true
	}
	if (left == storage.FieldDate || left == storage.FieldDateTime) && left == right && operator == "-" {
		return storage.FieldDecimal, true
	}
	if left == storage.FieldDate && numeric(right) {
		return storage.FieldDate, true
	}
	return "", false
}

func validateWhereFormula(org storage.OrgState, definition storage.ObjectDefinition, condition Condition, mode string) error {
	expr := condition.Formula
	var operands [2]storage.FieldType
	for i, name := range []string{expr.Left, expr.Right} {
		if err := validateFieldReference(org, definition, name, mode); err != nil {
			return err
		}
		fields, ok := fieldDefinitionsForReference(org, definition, name)
		if !ok || len(fields) != 1 {
			return queryError("Invalid formula: no such field: "+name, "unresolved WHERE formula operand")
		}
		operands[i] = fields[0].Type
	}
	kind, ok := formulaResultType(operands[0], operands[1], expr.Operator)
	if !ok {
		return queryError("Invalid formula expression: incompatible operand types", "invalid WHERE formula operand types")
	}
	value := condition.Value
	if value.Kind == storage.ValueID && strings.HasPrefix(string(value.ID), ":") {
		return nil
	}
	if value.Kind == storage.ValueNull {
		return nil
	}
	if kind == storage.FieldDate && value.Kind == storage.ValueDate {
		return nil
	}
	if kind == storage.FieldDecimal && (value.Kind == storage.ValueInteger || value.Kind == storage.ValueDecimal) {
		return nil
	}
	return queryError("Formula result type is incompatible with the literal value type", "invalid WHERE formula comparison type")
}

func whereFormulaValue(org storage.OrgState, definition storage.ObjectDefinition, record storage.Record, expr *FormulaExpression) storage.Value {
	left, leftOK := recordValue(org, definition, record, expr.Left)
	right, rightOK := recordValue(org, definition, record, expr.Right)
	if !leftOK {
		left = storage.NullValue()
	}
	if !rightOK {
		right = storage.NullValue()
	}
	if left.Kind == storage.ValueDate || left.Kind == storage.ValueDateTime {
		var start time.Time
		if left.Kind == storage.ValueDate {
			start, _ = parseISODate(left.String)
		} else {
			start, _ = parseISODateTime(left.String)
		}
		if right.Kind == left.Kind {
			var end time.Time
			if right.Kind == storage.ValueDate {
				end, _ = parseISODate(right.String)
			} else {
				end, _ = parseISODateTime(right.String)
			}
			seconds := new(big.Rat).SetInt64(start.Unix() - end.Unix())
			seconds.Add(seconds, new(big.Rat).SetFrac64(int64(start.Nanosecond()-end.Nanosecond()), int64(time.Second)))
			return storage.DecimalValue(decimalString(seconds.Quo(seconds, big.NewRat(86400, 1))))
		}
		if right.Kind == storage.ValueNull {
			right = storage.IntegerValue(0)
		}
		days, ok := numericValue(right)
		if !ok {
			return storage.NullValue()
		}
		if expr.Operator == "-" {
			days.Neg(days)
		}
		offset, _ := days.Float64()
		return storage.DateValue(start.Add(time.Duration(offset * float64(24*time.Hour))).Format("2006-01-02"))
	}
	if left.Kind == storage.ValueNull {
		left = storage.IntegerValue(0)
	}
	if right.Kind == storage.ValueNull {
		right = storage.IntegerValue(0)
	}
	a, b, ok := numericValues(left, right)
	if !ok {
		return storage.NullValue()
	}
	if expr.Operator == "+" {
		a.Add(a, b)
	} else {
		a.Sub(a, b)
	}
	return storage.DecimalValue(decimalString(a))
}
