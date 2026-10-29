package apextest_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/gladecli"
	"github.com/glade-sh/glade/internal/testreport"
)

// The CLI reuses one project while its explicit scratch shape changes. Each
// assertion comes from the native anonymous/@IsTest pairs in the TSV.
func TestOrgObjectAvailabilityNativeProjectRoutes(t *testing.T) {
	apextest.InvalidateRuntimeCaches()
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	fixture := "testdata/conformance/org_object_availability"
	writeTestFile := func(t *testing.T, path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	decodeTestRunJSON := func(data []byte) (testreport.Run, error) {
		var envelope struct {
			Data testreport.Run `json:"data"`
		}
		err := json.Unmarshal(data, &envelope)
		return envelope.Data, err
	}
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma, r.LazyQuotes = '\t', true
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 13 {
		t.Fatalf("native row count = %d", len(rows)-1)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			for _, phase := range []string{"plain", "quotes_on", "order_management", "plain"} {
				var observed map[string]any
				for _, row := range rows[1:] {
					if row[0] == api && row[1] == phase && row[2] == "isTest" {
						if err := json.Unmarshal([]byte(row[3]), &observed); err != nil {
							t.Fatal(err)
						}
					}
				}
				if observed == nil {
					t.Fatalf("missing native row: %s/%s", api, phase)
				}
				var assertions strings.Builder
				for _, object := range []string{"Network", "WebStore", "Quote", "QuoteLineItem", "Account", "AbsentObject__c"} {
					value, ok := observed[object+".describe"].(bool)
					if !ok {
						t.Fatalf("missing native describe: %s", object)
					}
					if value {
						assertions.WriteString("System.assertEquals(true,")
					} else {
						assertions.WriteString("System.assertEquals(false,")
					}
					assertions.WriteString("g.containsKey('" + object + "'),'" + object + "');")
				}
				writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/ProbeTest.cls"), "@IsTest private class ProbeTest { @IsTest static void repro() { Map<String,Schema.SObjectType> g=Schema.getGlobalDescribe();"+assertions.String()+" } }")
				writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/ProbeTest.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
				shape, err := os.ReadFile(filepath.Join(fixture, "shapes", phase+".json"))
				if err != nil {
					t.Fatal(err)
				}
				writeTestFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(shape))
				for _, cache := range []string{"cold", "cache", "warm"} {
					args := []string{"test", "--project", root, "--no-serve", "--read-only", "--no-progress", "--json", "--parallelism", "1", "--no-parallel-methods"}
					if cache == "cold" {
						args = append(args, "--no-cache")
					}
					var out, errs bytes.Buffer
					code := gladecli.Run(context.Background(), args, &out, &errs)
					run, err := decodeTestRunJSON(out.Bytes())
					if err != nil {
						t.Fatalf("%s/%s: %v: %s", phase, cache, err, out.String())
					}
					summary := run.Summary()
					if code != 0 || summary.Passed != 1 || summary.Total != 1 {
						t.Fatalf("%s/%s: code=%d summary=%+v stdout=%s stderr=%s", phase, cache, code, summary, out.String(), errs.String())
					}
				}
			}
		})
	}
}
