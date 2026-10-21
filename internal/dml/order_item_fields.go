package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func (e *Engine) applyOrderItemDerivedFields(record *storage.Record) {
	if e == nil || e.Org == nil || record == nil {
		return
	}
	orderItem := strings.EqualFold(record.Object, "OrderItem")
	if !orderItem && !strings.EqualFold(record.Object, "OpportunityLineItem") {
		return
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	if product, present := record.GetField("Product2Id"); !present || product.Kind == storage.ValueNull {
		entryID := idFromStorageValue(record.Fields["PricebookEntryId"])
		if _, entry, found := storage.LookupRecordByID(e.Org.Objects["PricebookEntry"].Records, entryID); found {
			if product, present := entry.GetField("Product2Id"); present {
				record.Fields["Product2Id"] = product.Clone()
			}
		}
	}
	if !orderItem {
		return
	}
	field := storage.Field{APIName: "TotalPrice", Type: storage.FieldCalculated, DisplayType: "CURRENCY", Formula: "Quantity * UnitPrice", Scale: 2, ScaleSpecified: true}
	definition := e.Org.Objects["OrderItem"].Definition
	if value, _, resolved := EvaluateRecordFormulaValueInOrg(field.Formula, field, e.Org, definition, *record); resolved {
		record.Fields["TotalPrice"] = value
	}
}
