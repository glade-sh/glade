package storage

// IsGeneratedHistoryObject distinguishes a derived history table from ordinary
// source or fixture objects whose names happen to have a history suffix.
func IsGeneratedHistoryObject(definition ObjectDefinition) bool {
	return definition.Metadata["kind"] == "generatedHistory" && definition.Metadata["parentObject"] != ""
}

// IsGeneratedShareObject distinguishes the share table generated for a custom
// object from an ordinary object.
func IsGeneratedShareObject(definition ObjectDefinition) bool {
	return definition.Metadata["kind"] == "generatedShare" && definition.Metadata["parentObject"] != ""
}
