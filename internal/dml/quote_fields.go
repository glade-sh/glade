package dml

import (
	"math/big"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// Quote and QuoteLineItem totals follow the native quote_totals rows.
// Line Subtotal and TotalPrice use the formula engine, as OrderItem does.
// Quote Subtotal, TotalPrice and LineItemCount flow through the summary
// relation pipeline (summaryRelationsForChild). GrandTotal and Discount
// derive from those stored values and are refreshed on every Quote write.

func quoteRollupField(objectName, fieldName string) bool {
	if !strings.EqualFold(objectName, "Quote") {
		return false
	}
	switch strings.ToLower(fieldName) {
	case "subtotal", "totalprice", "lineitemcount":
		return true
	}
	return false
}

func (e *Engine) applyQuoteLineItemDerivedFields(record *storage.Record) {
	if e == nil || e.Org == nil || record == nil || !strings.EqualFold(record.Object, "QuoteLineItem") {
		return
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	if list, present := record.GetField("ListPrice"); !present || list.Kind == storage.ValueNull {
		entryID := idFromStorageValue(record.Fields["PricebookEntryId"])
		if _, entry, found := storage.LookupRecordByID(e.Org.Objects["PricebookEntry"].Records, entryID); found {
			if price, present := entry.GetField("UnitPrice"); present && price.Kind != storage.ValueNull {
				record.Fields["ListPrice"] = price.Clone()
			}
		}
	}
	definition := e.Org.Objects["QuoteLineItem"].Definition
	for _, derived := range []struct{ name, formula string }{
		{"Subtotal", "Quantity * UnitPrice"},
		{"TotalPrice", "Quantity * UnitPrice * (1 - BLANKVALUE(Discount, 0) / 100)"},
	} {
		field := storage.Field{APIName: derived.name, Type: storage.FieldCalculated, DisplayType: "CURRENCY", Formula: derived.formula, Scale: 2, ScaleSpecified: true}
		if value, _, resolved := EvaluateRecordFormulaValueInOrgWithOptions(derived.formula, field, e.Org, definition, *record, e.formulaEvaluationOptions()); resolved {
			record.Fields[derived.name] = value
		}
	}
}

func quoteDecimal(record *storage.Record, name string) *big.Rat {
	value, present := record.GetField(name)
	if !present {
		return new(big.Rat)
	}
	number, ok := summaryNumericRat(value)
	if !ok {
		return new(big.Rat)
	}
	return number
}

// quoteFixed renders number at scale places, rounding half away from zero.
func quoteFixed(number *big.Rat, scale int) string {
	factor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(scale)), nil)
	scaled := new(big.Rat).Mul(number, new(big.Rat).SetInt(factor))
	half := big.NewRat(1, 2)
	if scaled.Sign() < 0 {
		scaled.Sub(scaled, half)
	} else {
		scaled.Add(scaled, half)
	}
	integer := new(big.Int).Quo(scaled.Num(), scaled.Denom())
	text := new(big.Int).Abs(integer).String()
	for len(text) <= scale {
		text = "0" + text
	}
	text = text[:len(text)-scale] + "." + text[len(text)-scale:]
	if integer.Sign() < 0 {
		text = "-" + text
	}
	return text
}

func (e *Engine) applyQuoteDerivedFields(record *storage.Record) {
	if e == nil || e.Org == nil || record == nil || !strings.EqualFold(record.Object, "Quote") {
		return
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	subtotal := quoteDecimal(record, "Subtotal")
	total := quoteDecimal(record, "TotalPrice")
	record.Fields["Subtotal"] = storage.DecimalValue(quoteFixed(subtotal, 2))
	record.Fields["TotalPrice"] = storage.DecimalValue(quoteFixed(total, 2))
	if _, present := record.GetField("LineItemCount"); !present {
		record.Fields["LineItemCount"] = storage.IntegerValue(0)
	}
	grand := new(big.Rat).Add(total, quoteDecimal(record, "Tax"))
	grand.Add(grand, quoteDecimal(record, "ShippingHandling"))
	record.Fields["GrandTotal"] = storage.DecimalValue(quoteFixed(grand, 2))
	discount := "0.00"
	if subtotal.Sign() != 0 && subtotal.Cmp(total) != 0 {
		percent := new(big.Rat).Sub(subtotal, total)
		percent.Quo(percent, subtotal)
		percent.Mul(percent, big.NewRat(100, 1))
		discount = quoteFixed(percent, 8)
	}
	record.Fields["Discount"] = storage.DecimalValue(discount)
}
