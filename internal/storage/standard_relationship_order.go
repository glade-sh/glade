package storage

// applyStandardRelationshipOrder sets the captured relationship order on the
// listed standard reference fields. Fields outside the native table keep no
// order, matching describe's null.
func applyStandardRelationshipOrder(definition *ObjectDefinition) {
	if definition == nil {
		return
	}
	orders, ok := standardRelationshipOrders[definition.APIName]
	if !ok {
		return
	}
	for name, order := range orders {
		field, exists := definition.Fields[name]
		if !exists || field.Type != FieldReference || field.RelationshipOrder != nil && *field.RelationshipOrder == order {
			continue
		}
		field.RelationshipOrder = IntFlag(order)
		definition.Fields[name] = field
	}
}

// standardRelationshipOrderNeedsRepair reports a persisted standard definition
// written before the relationship orders were captured.
func standardRelationshipOrderNeedsRepair(definition ObjectDefinition) bool {
	orders, ok := standardRelationshipOrders[definition.APIName]
	if !ok {
		return false
	}
	for name, order := range orders {
		field, exists := definition.Fields[name]
		if exists && field.Type == FieldReference && (field.RelationshipOrder == nil || *field.RelationshipOrder != order) {
			return true
		}
	}
	return false
}
