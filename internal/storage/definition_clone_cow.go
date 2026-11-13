package storage

// recordTypeIDFieldNeedsWrite reports whether EnsureRecordTypeIDField would
// mutate definition: it writes the policy snapshot into Metadata and adds the
// RecordTypeId field. The seeding step uses it to decide between sharing the
// definition's field map and metadata and privatizing them first. The
// equivalence test in definition_clone_cow_test.go keeps it in step with the
// helper.
func recordTypeIDFieldNeedsWrite(definition ObjectDefinition) bool {
	if len(definition.RecordTypes) == 0 || !StandardObjectRecordTypeFieldEnabled(definition) {
		return false
	}
	if standardFieldPolicyFor(definition.APIName) != nil || standardAPI68CapturedNewObject(definition.APIName) {
		if _, recorded := definition.Metadata[standardConfiguredPolicyFields]; !recorded {
			return true
		}
	}
	_, exists := ResolveFieldName(definition, "", "RecordTypeId")
	return !exists
}

// ensureObjectIsComplete reports whether ensureObject would leave object as it
// is: identity set, every wanted field present under its exact name (and
// Required where asked) and every reference relationship recorded. A case
// insensitive field match is left to the write path, which resolves it.
func ensureObjectIsComplete(object ObjectState, prefix string, fields map[string]Field) bool {
	definition := object.Definition
	if definition.APIName == "" || (definition.KeyPrefix == "" && prefix != "") || definition.Fields == nil || object.Records == nil {
		return false
	}
	for fieldName, field := range fields {
		existing, ok := definition.Fields[fieldName]
		if !ok || (field.Required && !existing.Required) {
			return false
		}
		if field.Type == FieldReference && field.RelationshipName != "" && len(field.ReferenceTo) > 0 && !hasParentRelationship(definition.Relations, field.RelationshipName) {
			return false
		}
	}
	return true
}
