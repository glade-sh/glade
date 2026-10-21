package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func (e *Engine) applyOpportunityLineItemDerivedFields(record *storage.Record, input, prior storage.Record) {
	if e == nil || e.Org == nil || record == nil || !strings.EqualFold(record.Object, "OpportunityLineItem") {
		return
	}
	unit, unitPresent := input.GetField("UnitPrice")
	total, totalPresent := input.GetField("TotalPrice")
	unitPresent = unitPresent && unit.Kind != storage.ValueNull
	totalPresent = totalPresent && total.Kind != storage.ValueNull
	definition := e.Org.Objects["OpportunityLineItem"].Definition
	if unitPresent && totalPresent {
		if prior.ID == "" {
			return
		}
		priorUnit, _ := prior.GetField("UnitPrice")
		priorTotal, _ := prior.GetField("TotalPrice")
		unitChanged := !storageValuesEqual(definition.Fields["UnitPrice"], unit, priorUnit)
		totalChanged := !storageValuesEqual(definition.Fields["TotalPrice"], total, priorTotal)
		if unitChanged == totalChanged {
			quantity, quantityPresent := input.GetField("Quantity")
			priorQuantity, _ := prior.GetField("Quantity")
			if unitChanged || !quantityPresent || storageValuesEqual(definition.Fields["Quantity"], quantity, priorQuantity) {
				return
			}
			totalPresent = true
		} else {
			totalPresent = totalChanged
		}
	} else if !unitPresent && !totalPresent {
		quantity, quantityPresent := input.GetField("Quantity")
		if !quantityPresent || quantity.Kind == storage.ValueNull {
			return
		}
		total, totalPresent = record.GetField("TotalPrice")
		if !totalPresent || total.Kind == storage.ValueNull {
			return
		}
	}
	name, formula := "TotalPrice", "Quantity * UnitPrice"
	if totalPresent {
		name, formula = "UnitPrice", "TotalPrice / Quantity"
	}
	field := storage.Field{APIName: name, Type: storage.FieldCalculated, DisplayType: "CURRENCY", Formula: formula, Scale: 2, ScaleSpecified: true}
	if value, _, resolved := EvaluateRecordFormulaValueInOrg(formula, field, e.Org, definition, *record); resolved {
		if record.Fields == nil {
			record.Fields = make(map[string]storage.Value)
		}
		record.Fields[name] = value
	}
}
