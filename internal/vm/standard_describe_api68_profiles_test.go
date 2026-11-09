package vm

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// Compare the changed native rows for multiple edition and feature profiles.
func TestStandardAPI68ProfileRouteDeltas(t *testing.T) {
	cases := []struct {
		name     string
		features []string
	}{
		{"P0", []string{project.StandardFieldEditionDeveloper}},
		{"P1", []string{project.StandardFieldEditionDeveloper, "MultiCurrency"}},
		{"P2", []string{project.StandardFieldEditionDeveloper, "PersonAccounts"}},
		{"P3", []string{project.StandardFieldEditionDeveloper, "StateAndCountryPicklist"}},
		{"P4", []string{project.StandardFieldEditionDeveloper, "Communities"}},
		{"P5", []string{project.StandardFieldEditionDeveloper, "Quotes"}},
		{"P6", []string{project.StandardFieldEditionDeveloper, "ProductsAndSchedules"}},
		{"P7", []string{project.StandardFieldEditionDeveloper, "Entitlements"}},
		{"P8", []string{project.StandardFieldEditionDeveloper, "OrderManagement"}},
		{"P9", []string{project.StandardFieldEditionDeveloper, "ContactsToMultipleAccounts"}},
		{"P10", []string{project.StandardFieldEditionDeveloper, "B2BCommerce", "Communities", "CommerceEnabled"}},
		{"P11", []string{project.StandardFieldEditionEnterprise}},
		{"PU", []string{project.StandardFieldEditionDeveloper, "Communities", "Entitlements", "MultiCurrency", "OrderManagement", "PersonAccounts", "ProductsAndSchedules", "SalesUser", "ServiceCloud", "StateAndCountryPicklist", "Quotes"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file, err := os.Open(filepath.Join("../apextest/testdata/conformance/standard_describe_api68_drift", tc.name+"-routes", "org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			reader := csv.NewReader(file)
			reader.Comma = '\t'
			rows, err := reader.ReadAll()
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			storage.ApplyProjectOrgShape(&org, tc.features)
			for _, row := range rows[1:] {
				if len(row) != 10 {
					t.Fatalf("invalid native row %v", row)
				}
				storage.EnsureStandardObject(&org, row[3])
			}
			machine := New(nil)
			machine.SetOrg(&org)
			for _, row := range rows[1:] {
				if row[1] != "default" || row[0] != "68.0" || row[5] != "true" {
					continue
				}
				machine.currentMethod = Method{APIVersion: row[0], SourceContextBound: true}
				machine.currentClass = ""
				if row[2] == "named" {
					machine.currentClass = "CapturedNamedClass"
				}
				fields := fieldMapGet(t, machine, fieldMapWrapper(t, machine, row[3]))
				_, present := fields.Map[mapKey(String(strings.ToLower(row[4])))]
				if present != (row[6] == "true") {
					t.Errorf("%s %s.%s present=%t, native=%s", row[2], row[3], row[4], present, row[6])
				}
			}
		})
	}
}
