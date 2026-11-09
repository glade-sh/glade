package apextest

import "testing"

// Families captured in one scratch org for the corpus assertion long tail
// (second pass). Each family has an org.tsv from native Salesforce
// (anonymous and isTest).

func TestSOQLParentTextIDBindNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "soql_parent_text_id_bind", "TextIdBindProbeTest", "62.0", "67.0")
}

func TestIDLetterStringsNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "id_letter_strings", "IdLetterProbeTest", "62.0", "67.0")
}

func TestPermsetAssignmentInvalidIDNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "permset_assignment_invalid_id", "PsaInvalidProbeTest", "62.0", "67.0")
}

func TestSetupEntityQueriesNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "setup_entity_queries", "SetupEntityProbeTest", "62.0", "67.0")
}

func TestLoadDataTriggerChunksNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "load_data_trigger_chunks", "LoadDataChunkProbeTest", "62.0", "67.0")
}

func TestSchemaTokenHashNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "schema_token_hash", "SchemaTokenHashProbeTest", "62.0", "67.0")
}
