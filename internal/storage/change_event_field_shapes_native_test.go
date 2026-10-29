package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChangeEventNameFieldShapesNativeAPIRoutes(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("../apextest/testdata/conformance/change_event_describe/followups", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
		cells := strings.SplitN(line, "\t", 5)
		if len(cells) != 5 {
			t.Fatal("invalid native observation")
		}
		fieldName, shape := strings.CutPrefix(cells[3], "shape.")
		if !shape || cells[1] != "division-on" {
			continue
		}
		var want struct {
			Type     string `json:"type"`
			Length   int    `json:"length"`
			Nillable bool   `json:"nillable"`
			Label    string `json:"label"`
		}
		if err := json.Unmarshal([]byte(cells[4]), &want); err != nil {
			t.Fatal(err)
		}
		seen[cells[0]+"/"+cells[2]]++
		t.Run(cells[0]+"/"+cells[2]+"/"+fieldName, func(t *testing.T) {
			for _, route := range []string{"catalog_base", "installed_base"} {
				t.Run(route, func(t *testing.T) {
					source, ok := StandardObjectDefinition("Contact")
					if !ok {
						t.Fatal("missing source definition")
					}
					before, err := json.Marshal(source.Fields[fieldName])
					if err != nil {
						t.Fatal(err)
					}
					org := OrgState{APIVersion: cells[0], Objects: map[string]ObjectState{}}
					if route == "installed_base" {
						EnsureStandardObject(&org, "Account")
					}
					if !EnsureChangeEventObject(&org, "AccountChangeEvent") {
						t.Fatal("event not installed")
					}
					field := org.Objects["AccountChangeEvent"].Definition.Fields[fieldName]
					if field.DisplayType != want.Type || field.Length != want.Length || field.Label != want.Label || field.Nillable == nil || *field.Nillable != want.Nillable {
						t.Errorf("AccountChangeEvent.%s: Glade type=%s length=%d label=%q nillable=%v; native=%s", fieldName, field.DisplayType, field.Length, field.Label, field.Nillable, cells[4])
					}
					// Overriding an event shape must not alter cached source flags.
					source, _ = StandardObjectDefinition("Contact")
					after, err := json.Marshal(source.Fields[fieldName])
					if err != nil || string(before) != string(after) {
						t.Fatalf("event shape changed Contact.%s: %v", fieldName, err)
					}
				})
			}
		})
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, route := range []string{"administrator.anonymous", "administrator.istest", "no-field-access.anonymous", "no-field-access.istest"} {
			if seen[api+"/"+route] != 3 {
				t.Errorf("missing native shapes for %s/%s", api, route)
			}
		}
	}
}

func changeEventFollowupFeatureRows(t *testing.T) [][]string {
	t.Helper()
	raw, err := os.ReadFile("../apextest/testdata/conformance/change_event_describe/followups/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	var rows [][]string
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
		cells := strings.SplitN(line, "\t", 5)
		if len(cells) != 5 {
			t.Fatal("invalid native observation")
		}
		if cells[1] == "features-on" && strings.HasPrefix(cells[3], "features.") {
			rows = append(rows, cells)
		}
	}
	return rows
}

func applyChangeEventFollowupNativeExclusions(t *testing.T, api string, fields map[string]bool) {
	t.Helper()
	pair := map[string]map[string]bool{}
	for _, row := range changeEventFollowupFeatureRows(t) {
		if row[0] == api && row[2] == "administrator.anonymous" {
			var presence map[string]bool
			if err := json.Unmarshal([]byte(row[4]), &presence); err != nil {
				t.Fatal(err)
			}
			pair[row[3]] = presence
		}
	}
	if len(pair) != 2 {
		t.Fatal("missing feature-enabled native base/event pair")
	}
	for field, present := range pair["features.Account"] {
		if present && !pair["features.AccountChangeEvent"][field] {
			delete(fields, strings.ToLower(field))
		}
	}
}

func TestChangeEventEnabledFeatureFieldsNativeAPIRoutes(t *testing.T) {
	seen := map[string]bool{}
	for _, row := range changeEventFollowupFeatureRows(t) {
		if row[3] != "features.AccountChangeEvent" {
			continue
		}
		seen[row[0]+"/"+row[2]] = true
		t.Run(row[0]+"/"+row[2], func(t *testing.T) {
			var want map[string]bool
			if err := json.Unmarshal([]byte(row[4]), &want); err != nil {
				t.Fatal(err)
			}
			org := OrgState{APIVersion: row[0], Objects: map[string]ObjectState{}}
			// The features-on capture provisions Division and a Business Account
			// record type (followups/cases.json and org.tsv:107-108,139-140,
			// 171-172). Supply that captured base schema before deriving its event.
			// Division provisioning is outside this event-schema test's scope.
			org.Features = []string{"Division", "Communities"}
			org.Objects["Account"] = ObjectState{Definition: ObjectDefinition{
				APIName: "Account",
				Fields: map[string]Field{
					"Division": {APIName: "Division", Type: FieldPicklist},
				},
				RecordTypes: []RecordTypeInfo{{DeveloperName: "Business", Name: "Business", Active: true}},
			}}
			if !EnsureChangeEventObject(&org, "AccountChangeEvent") {
				t.Fatal("event not installed")
			}
			for field, present := range want {
				_, exists := org.Objects["AccountChangeEvent"].Definition.Fields[field]
				if exists != present {
					t.Errorf("AccountChangeEvent.%s present=%v; native=%v", field, exists, present)
				}
			}
		})
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, route := range []string{"administrator.anonymous", "administrator.istest", "no-field-access.anonymous", "no-field-access.istest"} {
			if !seen[api+"/"+route] {
				t.Errorf("missing feature-enabled native row for %s/%s", api, route)
			}
		}
	}
}

// Native features-off event rows are followups/org.tsv:99,101,103. They
// distinguish a bare catalog from the explicitly provisioned schema above.
func TestChangeEventDisabledFeatureFieldsNativeCatalogAndInstalledRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, route := range []string{"catalog-base", "installed-base"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				org := OrgState{APIVersion: api, Objects: map[string]ObjectState{}}
				if route == "installed-base" {
					EnsureStandardObject(&org, "Account")
				}
				if !EnsureChangeEventObject(&org, "AccountChangeEvent") {
					t.Fatal("event not installed")
				}
				for _, field := range []string{"Division", "RecordTypeId", "IsCustomerPortal"} {
					if _, exists := org.Objects["AccountChangeEvent"].Definition.Fields[field]; exists {
						t.Errorf("bare AccountChangeEvent exposes native-absent %s", field)
					}
				}
			})
		}
	}
}
