package storage

import (
	"bufio"
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// Native rows: unknown_sobject_field/org.tsv, orgs case-user-surface and
// webstore-surface (named route, API 62.0 and 67.0). Every field name that a
// captured field read compiled for is part of the closed constructor surface;
// the Zed rows are rejected. The gated names below compiled in no captured
// read of the surface orgs but belong to a captured configuration (the
// standard_describe_profiles P4 and P11 deltas) or to the documented field
// list, so they stay accepted: rejecting a real field is worse than missing a
// reject.
const unknownFieldRowsPath = "../apextest/testdata/conformance/unknown_sobject_field/org.tsv"

var caseUserSurfaceGatedFields = map[string][]string{
	"Case": {
		"HasCommentsUnreadByOwner", "HasSelfServiceComments", "IsSelfServiceClosed", "IsVisibleInSelfService",
		"Language", "RecordTypeId", "SourceId", "ConnectionReceivedId", "ConnectionSentId",
		// Relationship names of the accepted reference fields.
		"ConnectionReceived", "ConnectionSent",
	},
	"User": {
		"IsPortalSelfRegistered", "IsPortalEnabled", "PortalRole", "IsPartner", "UserPermissionsAvantgoUser",
		"UserPreferencesHideBrowseProductRedirectConfirmation",
		"UserPreferencesHideOnlineSalesAppTabVisibilityRequirementsModal",
		"UserPreferencesHideOnlineSalesAppWelcomeMat",
		// Present at 67.0 only; the closed surface is the union over the APIs.
		"HasUserVerifiedEmail", "HasUserVerifiedPhone",
		"UserPreferencesDismissPersonalSpaceLegalMessage", "UserPreferencesDismissRecordChannelsSuggestedObjects",
		"UserPreferencesDismissReuseUpdateReminderModal", "UserPreferencesEnableLwrLexPilot",
		"UserPreferencesHideManagedEcaMobilePubModal",
		// Documented fields of org settings the capture org lacked (name
		// settings, divisions, Live Agent); no row, kept accepted.
		"MiddleName", "Suffix", "DefaultDivision", "UserPermissionsLiveAgentUser",
	},
}

// surfaceRowFields returns object -> accepted field names from the surface rows.
func surfaceRowFields(t *testing.T) (accepted, rejected map[string]map[string]bool) {
	t.Helper()
	file, err := os.Open(unknownFieldRowsPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	accepted = map[string]map[string]bool{}
	rejected = map[string]map[string]bool{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		cols := strings.SplitN(scanner.Text(), "\t", 5)
		if len(cols) != 5 || (cols[1] != "case-user-surface" && cols[1] != "webstore-surface") {
			continue
		}
		parts := strings.SplitN(cols[3], "_", 3)
		if len(parts) != 3 || (parts[0] != "S" && parts[0] != "SL" && parts[0] != "SU") {
			continue
		}
		field := strings.SplitN(parts[2], "_", 2)[0]
		if parts[0] != "S" {
			field = strings.TrimSuffix(strings.TrimSuffix(parts[2], "_lower"), "_upper")
		} else {
			field = strings.TrimSuffix(parts[2], "_read")
		}
		object := map[string]string{"case": "Case", "user": "User", "webstore": "WebStore"}[parts[1]]
		var obs struct {
			Compiled bool `json:"compiled"`
		}
		if err := json.Unmarshal([]byte(cols[4]), &obs); err != nil {
			t.Fatal(err)
		}
		bucket := rejected
		if obs.Compiled {
			bucket = accepted
		}
		if bucket[object] == nil {
			bucket[object] = map[string]bool{}
		}
		if parts[0] == "S" || obs.Compiled {
			bucket[object][field] = true
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return accepted, rejected
}

func TestStandardCaseUserSurfaceClosed(t *testing.T) {
	accepted, rejected := surfaceRowFields(t)
	features := []string{"MultiCurrency", "Entitlements", "PersonAccounts", "StateAndCountryPicklist", "OrderManagement"}
	for _, object := range []string{"Case", "User", "WebStore"} {
		def := ObjectDefinition{APIName: object, Fields: map[string]Field{}}
		EnsureStandardObjectFieldsForFeatures(&def, features)
		if len(accepted[object]) < 20 {
			t.Fatalf("%s: only %d accepted surface rows", object, len(accepted[object]))
		}
		var names []string
		for name := range accepted[object] {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			for _, variant := range []string{name, strings.ToLower(name), strings.ToUpper(name)} {
				if !StandardObjectConstructorFieldAllowedForFeatures(object, variant, features, def) {
					t.Errorf("%s.%s: captured field rejected", object, variant)
				}
			}
		}
		for _, name := range caseUserSurfaceGatedFields[object] {
			// NATIVE Case.Language, ConnectionReceivedId and ConnectionSentId compile in no captured
			// configuration: standard_case_field_features_*/org.tsv (none, LiveAgent, Division,
			// Knowledge, Knowledge with languages, service bundle) at 62.0 and 67.0. They stay open
			// only for a project with no scratch definition.
			unsuppliedCapturedNames := object == "Case" && (name == "Language" || strings.HasPrefix(name, "Connection"))
			for _, fs := range [][]string{nil, features} {
				d := ObjectDefinition{APIName: object, Fields: map[string]Field{}}
				EnsureStandardObjectFieldsForFeatures(&d, fs)
				if allowed := StandardObjectConstructorFieldAllowedForFeatures(object, name, fs, d); allowed == unsuppliedCapturedNames {
					t.Errorf("%s.%s: allowed=%t with features %v", object, name, allowed, fs)
				}
			}
			if unsuppliedCapturedNames {
				unspecified := []string{project.UnspecifiedStandardFieldShape}
				d := ObjectDefinition{APIName: object, Fields: map[string]Field{}}
				EnsureStandardObjectFieldsForFeatures(&d, unspecified)
				if !StandardObjectConstructorFieldAllowedForFeatures(object, name, unspecified, d) {
					t.Errorf("%s.%s: rejected for a project with no scratch definition", object, name)
				}
			}
		}
		for name := range rejected[object] {
			if name == "Zed" {
				if StandardObjectConstructorFieldAllowedForFeatures(object, name, features, def) {
					t.Errorf("%s.Zed: native rejects it, policy allows it", object)
				}
			}
		}
		for _, name := range []string{"NoSuchField", "Zed"} {
			if StandardObjectConstructorFieldAllowedForFeatures(object, name, features, def) {
				t.Errorf("%s.%s: unknown field allowed", object, name)
			}
		}
		// Custom fields stay governed by project metadata.
		if !StandardObjectConstructorFieldAllowedForFeatures(object, "Mine__c", features, def) {
			t.Errorf("%s: custom field no longer open to project metadata", object)
		}
	}
}
