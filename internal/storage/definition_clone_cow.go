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
