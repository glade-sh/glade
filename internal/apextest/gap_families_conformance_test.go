package apextest

import (
	"strconv"
	"strings"
	"testing"
)

// Rollup values read by a child trigger, per statement kind.
func TestRollupTriggerTimingNativeConformance(t *testing.T) {
	gapFamily{dir: "testdata/conformance/rollup_trigger_timing", adminFrom: []string{"GapAccess"}, classAPI: map[string]string{"67.0": "62.0"}}.replay(t)
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

// GROUP BY without ORDER BY: the SF-4 samples (4 to 8 text values, several
// insertion orders, APIs 62.0 and 67.0, two repeats) show an order that is neither
// insertion nor alphabetical and that differs between anonymous Apex and the
// @IsTest route on the same API, so no portable rule exists and Glade keeps
// ascending values. The Account field map follows the captured platform order
// for the 59 captured keys, but the native org holds 59 fields (60 in the test
// route, which adds tier) where Glade's schema holds 66. The global describe key
// set differs in membership (1450 native keys, 5712 in Glade).
var defaultOrderResiduals = func() map[string]string {
	const group = "native GROUP BY order depends on the route (anonymous vs @IsTest) and is not a portable rule (SF-4 samples)"
	const members = "native key set membership differs from Glade's schema (order of the shared Account keys is fitted)"
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
// update. The SF-4 hidden_record_grid rows settle the cause: records created in
// the test method or in @TestSetup report INSUFFICIENT_ACCESS_ON_CROSS_REFERENCE_ENTITY
// in every sharing declaration at 62.0, 66.0 and 67.0 (Glade matches), and records
// that already exist under SeeAllData=true report INSUFFICIENT_ACCESS_OR_READONLY.
// The local @IsTest route holds no pre-existing record, so the replay runs the
// in-test shape and these rows stay unmatched.
var hiddenRecordResiduals = func() map[string]string {
	const reason = "native row reads a record that already exists in the org (SeeAllData); the local replay creates it in the test"
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
	gapFamily{dir: "testdata/conformance/review_round2", adminFrom: []string{"GapAccess"}}.replay(t)
}

// Nested roll-up statements and the master-detail update grid.
func TestReviewRound3NativeConformance(t *testing.T) {
	gapFamily{dir: "testdata/conformance/review_round3", adminFrom: []string{"GapAccess"}, residual: map[string]string{
		"g_amt_a/62.0": beforeGrant, "g_name_a/62.0": beforeGrant, "g_sys_a/62.0": beforeGrant, "g_usr_a/62.0": beforeGrant,
	}}.replay(t)
}

// These rows ran before the permission set was assigned: the custom field did
// not exist for the administrator. The replay always grants the Admin profile.
const beforeGrant = "native row ran before the permission set; the replay grants the Admin profile throughout"

// SF-4 rows: a mixed upsert with a child after-trigger reading the parent
// roll-up directly and through Parent__r.
func TestSF4MixedUpsertNativeConformance(t *testing.T) {
	gapFamily{dir: "testdata/conformance/sf4_mixed_upsert", adminFrom: []string{"GapAccess"}}.replay(t)
}

// Test-class update of an inserted record with its master-detail value: with an
// after-trigger, with a roll-up, with both, and plain.
func TestReviewRound4NativeConformance(t *testing.T) {
	gapFamily{dir: "testdata/conformance/review_round4", adminFrom: []string{"GapAccess"}}.replay(t)
}

// Master-detail update factors in a test class, and the in-memory Parent__r
// roll-up read.
func TestReviewRound5NativeConformance(t *testing.T) {
	gapFamily{dir: "testdata/conformance/review_round5", adminFrom: []string{"GapAccess"}}.replay(t)
}

func TestReviewRound6NativeConformance(t *testing.T) {
	gapFamily{dir: "testdata/conformance/review_round6", adminFrom: []string{"GapAccess"}, cleanBetween: true}.replay(t)
}

func TestReviewRound7NativeConformance(t *testing.T) {
	gapFamily{dir: "testdata/conformance/review_round7", adminFrom: []string{"GapAccess"}, cleanBetween: true}.replay(t)
}

func TestReviewRound8NativeConformance(t *testing.T) {
	gapFamily{dir: "testdata/conformance/review_round8", adminFrom: []string{"GapAccess"}, cleanBetween: true}.replay(t)
}

func TestReviewRound9NativeConformance(t *testing.T) {
	gapFamily{dir: "testdata/conformance/review_round9", adminFrom: []string{"GapAccess"}, cleanBetween: true}.replay(t)
}

func TestReviewRound10NativeConformance(t *testing.T) {
	gapFamily{
		dir: "testdata/conformance/review_round10", adminFrom: []string{"GapAccess"}, cleanBetween: true,
		normalize: func(id, line string) string {
			if id != "g_gd_remove" && id != "g_gd_clear" {
				return line
			}
			// The global describe counts are environment values. g_gd_remove
			// masks them relative to the first one; g_gd_clear masks the fresh
			// getGlobalDescribe().size() after the two cleared counts.
			parts := strings.Split(line, "|")
			if id == "g_gd_clear" {
				if len(parts) == 5 {
					if _, err := strconv.Atoi(parts[4]); err == nil {
						parts[4] = "N"
					}
				}
				return strings.Join(parts, "|")
			}
			if len(parts) < 3 {
				return line
			}
			first, err := strconv.Atoi(parts[2])
			if err != nil {
				return line
			}
			for i := 2; i < len(parts); i++ {
				if n, err := strconv.Atoi(parts[i]); err == nil {
					switch n {
					case first:
						parts[i] = "N"
					case first - 1:
						parts[i] = "N-1"
					}
				}
			}
			return strings.Join(parts, "|")
		},
	}.replay(t)
}

func TestReviewRound11NativeConformance(t *testing.T) {
	gapFamily{dir: "testdata/conformance/review_round11", cleanBetween: true}.replay(t)
}
