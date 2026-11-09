package storage

// WithFormulaContext returns a value copy with query-scoped actor data. The
// unexported fields are excluded from JSON, gob and persistent org snapshots.
func (o OrgState) WithFormulaContext(user ID, fields map[string]Value) OrgState {
	o.formulaUserID = user
	o.formulaPermissionFields = fields
	return o
}

// FormulaContext is only used while evaluating formulas in that query copy.
func (o OrgState) FormulaContext() (ID, map[string]Value) {
	return o.formulaUserID, o.formulaPermissionFields
}
