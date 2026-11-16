package apextest

import (
	"regexp"
	"testing"
)

// The second API 68 capture pass ran in a scratch org created with the
// PlatformEncryption and ApexIntegrationTests features. Glade models those
// features only when the project scratch definition lists them; the default
// org keeps the feature-off behavior the first pass captured.
func TestAPI68NativeSecondPassConformance(t *testing.T) {
	api68ReplayFamilies(t, "testdata/conformance/api68_native_second", api68SecondPassGaps, api68SecondPassPartialGaps)
}

var api68SecondPassGaps = map[string]string{}

// The shared fixture count carries the capture org's history (earlier
// integration runs committed their @BeforeClass record), and the capture shows
// the first method seeing the @BeforeClass static while Glade runs @BeforeClass
// as a setup whose data, not statics, reach the methods.
var api68IntegrationHistorySegment = regexp.MustCompile(`COUNT=\d+;STATIC=[A-Za-z-]+`)

const api68IntegrationHistoryReason = "fixture COUNT is org history and the @BeforeClass static is shared with the first method only"

var api68SecondPassPartialGaps = map[string]api68PartialGap{
	"integration-shield/INTEGRATION-RUN-68": {api68IntegrationHistorySegment, api68IntegrationHistoryReason},
	"integration-shield/INTEGRATION-RUN-67": {api68IntegrationHistorySegment, api68IntegrationHistoryReason},
}

// api68SecondPassNotReplayed lists the second-pass rows Glade cannot replay.
// The data is checked in so the rows stay reviewable; each needs a source the
// capture could not provide or a feature Glade does not model.
var api68SecondPassNotReplayed = map[string]string{
	"elastic-enabled/ELASTIC-ENABLED-SETTING":                "enableAsyncApexElasticExecutions is an Apex settings flag Glade does not read; the settings file was not captured",
	"elastic-enabled/ELASTIC-ENABLED-LIMITS":                 "OrgLimits keys and daily usage are org history; Glade models 19 keys sized from transaction caps and no elastic keys",
	"elastic-enabled/ELASTIC-ENABLED-ENQ":                    "the CapElasticProbe class source was not captured; daily usage is org history",
	"elastic-enabled/ELASTIC-ENABLED-READ":                   "the CapElasticProbe class source was not captured; daily usage is org history",
	"elastic-enabled/ELASTIC-OVERRIDE250-SETTING":            "asyncApexExecutionsOverride is an Apex settings value Glade does not read",
	"elastic-enabled/ELASTIC-OVERRIDE250-LIMITS":             "OrgLimits keys and daily usage are org history; the elastic ceiling min(2 x override, licensed) is not modeled",
	"elastic-enabled/ELASTIC-RESTORE":                        "Apex settings deploy is not modeled",
	"managed-namespace/Q4-PACKAGED-68":                       "calls an installed first-generation managed class whose source is not visible or captured",
	"managed-namespace/Q4-PACKAGED-67":                       "calls an installed first-generation managed class whose source is not visible or captured",
	"managed-namespace/Q4-SUBSCRIBER-68":                     "needs the installed package field gladetest1__Age__c and an Account row from a prior row; the package metadata was not captured",
	"managed-namespace/K3-PACKAGED-68":                       "calls an installed first-generation managed class whose source is not visible or captured",
	"managed-namespace/K3-PACKAGED-67":                       "calls an installed first-generation managed class whose source is not visible or captured",
	"managed-namespace/NAMESPACE-INSTALL-BETA-WITH-AGE":      "package install is not a Glade route",
	"managed-namespace/NAMESPACE-INSTALL-BETA-NO-AGE":        "package install is not a Glade route",
	"managed-namespace/NAMESPACE-INSTALL-BETA-JSON-WITH-AGE": "package install is not a Glade route",
}

// The inventory test keeps the exported rows consistent: every case has a
// native row, and every row is either replayed above or listed here.
func TestAPI68NativeSecondPassInventory(t *testing.T) {
	previous := api68NativeRoot
	api68NativeRoot = "testdata/conformance/api68_native_second"
	defer func() { api68NativeRoot = previous }()
	seen := map[string]bool{}
	for _, family := range []string{"elastic-enabled", "managed-namespace"} {
		for _, row := range api68NativeFamilyCases(t, family) {
			key := family + "/" + row.id
			if _, listed := api68SecondPassNotReplayed[key]; !listed {
				t.Errorf("%s is neither replayed nor listed as not replayed", key)
			}
			seen[key] = true
		}
	}
	for key := range api68SecondPassNotReplayed {
		if !seen[key] {
			t.Errorf("not-replayed list names unknown row %s", key)
		}
	}
}
