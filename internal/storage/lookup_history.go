package storage

// RememberClearedLookup records an automatic SetNull result for one DML
// transaction. The scope itself is deliberately transient across snapshots.
func (r *Record) RememberClearedLookup(scope *LookupHistoryScope, field string, parent ID) {
	if r == nil || scope == nil {
		return
	}
	if r.clearedLookupScope != scope {
		r.clearedLookupReferences = nil
		r.clearedLookupScope = scope
	}
	if r.clearedLookupReferences == nil {
		r.clearedLookupReferences = make(map[string]ID)
	}
	r.clearedLookupReferences[field] = parent
}

// ClearedLookupForScope returns a lookup cleared earlier in the same DML
// transaction.
func (r Record) ClearedLookupForScope(scope *LookupHistoryScope, field string) (ID, bool) {
	if scope == nil || r.clearedLookupScope != scope {
		return "", false
	}
	parent, ok := r.clearedLookupReferences[field]
	return parent, ok
}

// ForgetChangedClearedLookups discards transaction history invalidated by a
// later explicit field assignment.
func (r *Record) ForgetChangedClearedLookups(input Record) {
	if r == nil || len(r.clearedLookupReferences) == 0 {
		return
	}
	retained := make(map[string]ID, len(r.clearedLookupReferences))
	for field, parent := range r.clearedLookupReferences {
		if input.HasExplicitNull(field) {
			continue
		}
		if value, supplied := input.GetField(field); supplied {
			provided := ID("")
			switch value.Kind {
			case ValueID:
				provided = value.ID
			case ValueString:
				provided = ID(value.String)
			}
			if !IDsEqual(provided, parent) {
				continue
			}
		}
		retained[field] = parent
	}
	r.clearedLookupReferences = retained
}

// ClearClearedLookupHistory removes transaction-local history from a new
// stored record.
func (r *Record) ClearClearedLookupHistory() {
	if r == nil {
		return
	}
	r.clearedLookupReferences = nil
	r.clearedLookupScope = nil
}
