package soql

import (
	"encoding/csv"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Declared-dependency binding for the namespaced_standard_field_soql rows.
// Native accepts pkgone__ParentRouteValue__c on Contact only where the
// package is installed and has the field. A project that declares pkgone as a
// dependency with no source stands for the installed org: Glade cannot see
// the package's fields, so it tolerates any pkgone__X__c (the rows
// N02/N06/N07/N09/N14/N15 for an absent same-namespace field stay a parked
// gap) and rejects every other namespace, as native does.
//
// Boundary: a project that declares nothing keeps the open tolerance, so an
// undeclared corpus with managed-package fields and no source keeps working.

type declaredBinding struct {
	name       string
	narrow     bool
	sourceless []string
	// withField puts the package field into the Contact definition, as a
	// declared dependency with source does.
	withField bool
}

func namespacedStandardFieldRows(t *testing.T) (map[string]string, [][]string) {
	t.Helper()
	raw, err := os.ReadFile(namespacedStandardFieldDir + "cases.json")
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(namespacedStandardFieldDir + "org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1
	var rows [][]string
	for first := true; ; first = false {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if !first {
			rows = append(rows, record)
		}
	}
	return cases, rows
}

func TestNamespacedStandardFieldDeclaredDependencies(t *testing.T) {
	cases, rows := namespacedStandardFieldRows(t)
	// Rows the binding cannot settle for each variant (still the parked gap).
	sameNamespaceAbsent := map[string]bool{
		"N02_soql_bogus_same_ns": true, "N06_where_bogus_same_ns": true, "N07_order_bogus_same_ns": true,
		"N09_dynamic_bogus_same_ns": true, "N14_account_bogus_same_ns": true, "N15_soql_installed_field_rel": true,
	}
	bindings := []declaredBinding{
		{name: "declared-sourceless", narrow: true, sourceless: []string{"pkgone"}},
		{name: "declared-with-source", narrow: true, withField: true},
	}
	checked := map[string]int{}
	for _, binding := range bindings {
		for _, record := range rows {
			api, label, id := record[0], record[1], record[3]
			if label != "package-installed" || !strings.HasPrefix(id, "N") {
				continue
			}
			query := namespacedStandardFieldQuery.FindString(cases[id])
			// N04 is the bare-name field-resolver rule outside this change.
			if query == "" || id == "N04_soql_unnamespaced_pkgname" {
				continue
			}
			if binding.name == "declared-sourceless" && sameNamespaceAbsent[id] {
				continue
			}
			var observed struct {
				Compiled  *bool  `json:"compiled"`
				Exception string `json:"exceptionMessage"`
			}
			if err := json.Unmarshal([]byte(record[4]), &observed); err != nil {
				t.Fatal(err)
			}
			nativeAccepted := observed.Compiled != nil && *observed.Compiled && strings.HasSuffix(observed.Exception, ": ok")
			t.Run(binding.name+"/"+api+"/"+id, func(t *testing.T) {
				org := contactOrg(binding.withField)
				org.Metadata.PackageFieldsNarrow = binding.narrow
				org.Metadata.SourcelessPackageNamespaces = binding.sourceless
				// Static query text and Database.query text reach the same
				// ParseAndExecute; the dynamic rows (N08-N10) cover that route.
				_, err := ParseAndExecute(org, query)
				if accepted := err == nil; accepted != nativeAccepted {
					t.Fatalf("native accepted=%t, got %t (%v) for %q", nativeAccepted, accepted, err, query)
				}
			})
			checked[binding.name]++
		}
		if checked[binding.name] < 8 {
			t.Fatalf("%s: expected the captured query rows, checked %d", binding.name, checked[binding.name])
		}
	}
}

func TestNamespacedStandardFieldUndeclaredKeepsOpenTolerance(t *testing.T) {
	for _, query := range []string{
		"SELECT pkgnine__ParentRouteValue__c FROM Contact LIMIT 1",
		"SELECT pkgone__ParentRouteValue__c FROM Contact WHERE pkgone__ParentRouteValue__c = 'x'",
	} {
		org := contactOrg(false)
		if _, err := ParseAndExecute(org, query); err != nil {
			t.Fatalf("undeclared project must keep the open tolerance: %v for %q", err, query)
		}
	}
	// Declared, but only pkgone: another namespace is rejected, pkgone stays open.
	org := contactOrg(false)
	org.Metadata.PackageFieldsNarrow = true
	org.Metadata.SourcelessPackageNamespaces = []string{"PkgOne"}
	if _, err := ParseAndExecute(org, "SELECT pkgone__ParentRouteValue__c FROM Contact LIMIT 1"); err != nil {
		t.Fatalf("declared namespace (case-insensitive) rejected: %v", err)
	}
	if _, err := ParseAndExecute(org, "SELECT pkgnine__ParentRouteValue__c FROM Contact LIMIT 1"); err == nil {
		t.Fatal("undeclared namespace accepted in a project that declares dependencies")
	}
}

func contactOrg(withField bool) storage.OrgState {
	org := storage.NewOrgState()
	definition, _ := storage.StandardObjectDefinition("Contact")
	definition.Fields = copyFields(definition.Fields)
	if withField {
		definition.Fields["pkgone__ParentRouteValue__c"] = storage.Field{APIName: "pkgone__ParentRouteValue__c", Type: storage.FieldString}
	}
	org.Objects["Contact"] = storage.ObjectState{Definition: definition, Records: map[storage.ID]storage.Record{
		"003000000000001": {ID: "003000000000001", Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("x")}},
	}}
	return org
}
