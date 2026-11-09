package apextest

import (
	"encoding/csv"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const bareFieldFixture = "testdata/conformance/metadata_bare_field_relationship"

var bareFieldIDs = []string{"bare-column", "bare-standard-column", "bare-entity", "bare-groups", "static-init"}

func bareFieldExpected(t *testing.T) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(bareFieldFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 21 {
		t.Fatalf("native matrix: rows=%d err=%v", len(rows), err)
	}
	expected := map[string]string{}
	for _, row := range rows[1:] {
		expected[strings.Join(row[:3], "/")] = row[3]
	}
	return expected
}

func copyBareFieldFixture(t *testing.T, api, namespace string) string {
	t.Helper()
	root := t.TempDir()
	base := filepath.Join(bareFieldFixture, "fixture")
	err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(source)
		if strings.HasSuffix(path, ".cls-meta.xml") {
			text = fmt.Sprintf("<ApexClass><apiVersion>%s</apiVersion><status>Active</status></ApexClass>", api)
		}
		writeFile(t, filepath.Join(root, rel), text)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	ns := ""
	if namespace != "" {
		ns = fmt.Sprintf(`,"namespace":%q`, namespace)
	}
	writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]%s}`, api, ns))
	return root
}

// A metadata relationship controlled by an entity field stores the bare field
// name. Native non-namespaced orgs answer it exactly like Object.Field values.
func TestMetadataBareFieldRelationshipNativeConformance(t *testing.T) {
	expected := bareFieldExpected(t)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := copyBareFieldFixture(t, api, "")
			var assertions strings.Builder
			for _, id := range bareFieldIDs {
				want, ok := expected[api+"/istest/"+id]
				if !ok {
					t.Fatal("missing native row " + id)
				}
				if id == "bare-groups" {
					// Native map key order is hash order; compare the entries, not the order.
					fmt.Fprintf(&assertions, "System.assertEquals(JSON.deserializeUntyped('%s'),JSON.deserializeUntyped(MeadowBareProbe.observe('%s')),'%s');\n", want, id, id)
					continue
				}
				fmt.Fprintf(&assertions, "System.assertEquals('%s',MeadowBareProbe.observe('%s'),'%s');\n", want, id, id)
			}
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/MeadowBareNativeTest.cls"), "@IsTest private class MeadowBareNativeTest { @IsTest static void verifyRows(){"+assertions.String()+"}}")
			run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
			if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
				t.Fatalf("native named assertions: summary=%+v problem=%s", got, firstRunProblem(run))
			}
		})
	}
}

// The same bare controlling-field values in a namespaced owner org answer the
// field's namespace-qualified API name, as describe does.
func TestMetadataBareFieldRelationshipNamespacedNativeConformance(t *testing.T) {
	raw, err := os.ReadFile("testdata/conformance/metadata_bare_field_relationship_ns/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 21 {
		t.Fatalf("native matrix: rows=%d err=%v", len(rows), err)
	}
	expected := map[string]string{}
	for _, row := range rows[1:] {
		expected[strings.Join(row[:3], "/")] = row[3]
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := copyBareFieldFixture(t, api, "gladens004")
			if err := os.Remove(filepath.Join(root, "force-app/main/default/classes/MeadowNativeTest.cls")); err != nil {
				t.Fatal(err)
			}
			var assertions strings.Builder
			for _, id := range bareFieldIDs {
				want, ok := expected[api+"/istest/"+id]
				if !ok {
					t.Fatal("missing native row " + id)
				}
				fmt.Fprintf(&assertions, "System.assertEquals('%s',MeadowBareProbe.observe('%s'),'%s');\n", want, id, id)
			}
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/MeadowBareNativeTest.cls"), "@IsTest private class MeadowBareNativeTest { @IsTest static void verifyRows(){"+assertions.String()+"}}")
			run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
			if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
				t.Fatalf("native named assertions: summary=%+v problem=%s", got, firstRunProblem(run))
			}
		})
	}
}
