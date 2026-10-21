package apextest

import "testing"

func TestFormulaExceptionNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "formula_exception", "FormulaFailureProbeTest")
}

func TestSettingsIDUpsertNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "settings_id_upsert", "SettingsIdentityProbeTest")
}

func TestOrderProductIdentityNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "order_product_identity", "OrderProductProbeTest")
}

func TestCatalogPrerequisitesNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "catalog_prerequisites", "CatalogPrerequisiteProbeTest")
}

func TestFlowUpdateEntryFormulaNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "flow_update_entry_formula", "UpdateEntryProbeTest")
}

func TestStandardIdentityDefaultsNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "standard_identity_defaults", "StandardIdentityProbeTest")
}

func TestFlowFaultScalarNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "flow_fault_scalar", "FaultScalarProbeTest")
}
