package apextest

import "testing"

// Rollup values read by a child trigger, per statement kind.
func TestRollupTriggerTimingNativeConformance(t *testing.T) {
	gapFamily{dir: "testdata/conformance/rollup_trigger_timing", adminFrom: []string{"GapAccess"}}.replay(t)
}

// Depth-16 trigger recursion: where the failure surfaces and who can catch it.
func TestTriggerDepthUnwindNativeConformance(t *testing.T) {
	gapFamily{dir: "testdata/conformance/trigger_depth_unwind", adminFrom: []string{"GapAccess"}}.replay(t)
}

// Default order without ORDER BY: only rows that repeat natively are exact.
func TestDefaultOrderRestNativeConformance(t *testing.T) {
	gapFamily{dir: "testdata/conformance/default_order_rest", adminFrom: []string{"GapAccess"}, residual: defaultOrderResiduals, unordered: map[string]bool{
		"o3_62_read_default": true, "o3_62_read_default2": true, "o3_62_name_filter": true,
		"o3_67_read_default": true, "o3_67_read_default2": true, "o3_67_name_filter": true,
	}, limited: map[string]int{"o3_62_limited": 5, "o3_67_limited": 5}}.replay(t)
}

func TestSOQLCollationPunctuationNativeConformance(t *testing.T) {
	gapFamily{dir: "testdata/conformance/soql_collation_punctuation", adminFrom: []string{"GapAccess"}}.replay(t)
}

func TestHiddenRecordUpdateNativeConformance(t *testing.T) {
	gapFamily{dir: "testdata/conformance/hidden_record_update", adminFrom: []string{"HidAccess"}, residual: hiddenRecordResiduals, apis: []string{"62.0", "66.0", "67.0"}}.replay(t)
}

// GROUP BY group order repeats across orgs, APIs and runs (Bravo, Delta,
// Charlie, Alpha for Alpha..Delta) but the capture holds one four-value
// sample, so no rule is fitted: Glade returns ascending group values. Hash
// orders of the group strings and first-appearance orders were tried and do
// not produce it. The Account and global describe key sets differ in
// membership (59 and 1312 native keys at 62.0; 1425 at 67.0), which this
// project cannot reproduce.
var defaultOrderResiduals = func() map[string]string {
	const group = "native GROUP BY order is stable but its rule is not derivable from one four-value sample"
	const members = "native key set membership differs from Glade's schema"
	residual := map[string]string{}
	for _, api := range []string{"62", "67"} {
		for _, id := range []string{"group_agg", "group_plain", "group_sum", "group_filtered"} {
			residual["o3_"+api+"_"+id+"/*"] = group
		}
		for _, id := range []string{"fields_account", "global"} {
			residual["o3_"+api+"_"+id+"/*"] = members
		}
	}
	return residual
}()

// The native rows read a record that already existed in the org
// (SeeAllData=true) and report INSUFFICIENT_ACCESS_OR_READONLY for default-mode
// update, as a DmlException or a failed SaveResult, at 62.0, 66.0 and 67.0,
// in a default-sharing and in a without-sharing class. Glade's existing
// sharing_access_modes d2_upd rows (the record is created inside the test, same
// object shape and user) report INSUFFICIENT_ACCESS_ON_CROSS_REFERENCE_ENTITY,
// and Glade raises a SecurityException at 67.0. The two native shapes differ by
// SeeAllData and by when the record was created; the local @IsTest route has
// no way to hold a pre-existing record, so the replay runs the in-test shape
// and stays unmatched. Not fixed: it needs a native row with the record created
// in the test and SeeAllData=false to tell the two causes apart.
var hiddenRecordResiduals = func() map[string]string {
	const reason = "native SeeAllData row (record exists before the test) vs existing in-test-created rows; see the ledger comment"
	residual := map[string]string{}
	for _, api := range []string{"62", "66", "67"} {
		residual["h4t"+api+"_share/*"] = reason
		residual["h4t"+api+"_system/*"] = reason
	}
	return residual
}()

// Handler-class depth shapes, the nested roll-up read, custom field order and
// the first non-groupable GROUP BY field, from the review round captures.
func TestReviewRound1NativeConformance(t *testing.T) {
	gapFamily{dir: "testdata/conformance/review_round1", adminFrom: []string{"GapAccess"}}.replay(t)
}

// GROUP BY groupability by field type, the uncatchable non-groupable
// exception, and static-list aliasing through Map.put.
func TestReviewRound2NativeConformance(t *testing.T) {
	gapFamily{dir: "testdata/conformance/review_round2", adminFrom: []string{"GapAccess"}, residual: map[string]string{"x_alias/*": "a List read from a static field (or returned from a static method) is copied when it is put into a Map; a local List is aliased. Native aliases both. Cause not traced in the VM; the same shape explains rollup-summary rollanon"}}.replay(t)
}
