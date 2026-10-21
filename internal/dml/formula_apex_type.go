package dml

import "github.com/glade-sh/glade/internal/storage"

// FormulaInstance CONTAINS on the captured null String field and nonempty text
// literal returns null. Stored formulas, other functions and argument shapes
// retain their existing coercion rules.
func (p *formulaParser) formulaApexTypeContainsNull(args []formulaValue) bool {
	return p.formulaSurface && !p.storedFormula && len(args) == 2 &&
		args[0].kind == formulaNull && args[0].fieldType == storage.FieldString &&
		args[1].kind == formulaString && args[1].fieldType == "" && args[1].text != ""
}
