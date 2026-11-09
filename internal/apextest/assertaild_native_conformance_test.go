package apextest

import "testing"

// Families captured in one scratch org for the corpus assertion long tail, tail D.
// Each family has an org.tsv from native Salesforce (anonymous and isTest).

func TestCMDTDescribeFlagsNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "cmdt_describe_flags", "CmdtDescProbeTest", "62.0", "67.0", "68.0")
}

func TestListSettingRunAsStandardUserNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "list_setting_runas_standard_user", "ListRunAsProbeTest", "62.0", "67.0", "68.0")
}

func TestJSONUserProfileAccessNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "json_user_profile_access", "JsonUserProbeTest", "62.0", "67.0")
}

func TestDMLNotAllowedSetupObjectsNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "dml_not_allowed_setup_objects", "DmlNotAllowedProbeTest", "62.0", "66.0", "67.0", "68.0")
}

func TestUserProfilePermissionSetAssignmentNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "user_profile_permission_set_assignment", "UserPsaProbeTest", "62.0", "67.0")
}

func TestFlowEmailLogActivityNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "flow_email_log_activity", "FlowEmailLogProbeTest", "62.0", "67.0", "68.0")
}

func TestRenderTemplateWhatIDNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "render_template_what_id", "RenderWhatProbeTest", "62.0", "67.0", "68.0")
}

func TestRecordTypeIDNullInsertNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "record_type_id_null_insert", "RtNullProbeTest", "62.0", "66.0", "67.0", "68.0")
}

func TestRecordTypeIDNullInsertDefaultNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "record_type_id_null_insert_default", "RtNullProbeTest", "62.0", "67.0", "68.0")
}

func TestGlobalDescribeChangeEventKeysNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "global_describe_changeevent_keys", "ChangeEventKeysProbeTest", "62.0", "67.0", "68.0")
}

func TestHierarchySettingRunAsHandlesNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "hierarchy_setting_runas_handles", "HhProbeTest", "62.0", "67.0", "68.0")
}

func TestHierarchySettingRunAsSecEnforcedNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "hierarchy_setting_runas_secenforced", "HhProbeTest", "62.0")
}

// The full change event key set of the global describe, API 62 to 68, with
// no Change Data Capture setup. The anonymous rows carry every sorted
// key~name~label; the named-test rows carry a digest of the same list.
func TestGlobalDescribeChangeEventFullNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "global_describe_changeevent_full", "ChangeEventFullProbeTest", "62.0", "63.0", "64.0", "65.0", "66.0", "67.0", "68.0")
}
