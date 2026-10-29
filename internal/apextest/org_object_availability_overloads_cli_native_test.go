package apextest_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/gladecli"
	"github.com/glade-sh/glade/internal/testreport"
)

// Replay the captured public overloads when a single project changes its org
// shape. Cold, result-cache and warm-runtime executions must all agree.
func TestOrgObjectAvailabilityOverloadsNativeProjectRoutes(t *testing.T) {
	apextest.InvalidateRuntimeCaches()
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	fixture := "testdata/conformance/org_object_availability_routes"
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
		t.Fatal("incomplete native observations")
	}
	full, err := os.Open(filepath.Join(fixture, "full-org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer full.Close()
	fr := csv.NewReader(full)
	fr.Comma, fr.LazyQuotes = '\t', true
	fullRows, err := fr.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(fixture, "project")
			err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				// Copy only the owned source, never local Salesforce auth/config.
				if strings.HasPrefix(entry.Name(), ".") {
					if entry.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
				if entry.IsDir() {
					return nil
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				dest := filepath.Join(root, rel)
				if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
					return err
				}
				return os.WriteFile(dest, []byte(strings.ReplaceAll(string(body), "62.0", api)), 0644)
			})
			if err != nil {
				t.Fatal(err)
			}
			observations := rows
			if api == "67.0" {
				observations = fullRows
				body, err := os.ReadFile(filepath.Join(fixture, "full-probe.cls"))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "force-app/main/default/classes/ObjectAvailabilityRoutes.cls"), body, 0644); err != nil {
					t.Fatal(err)
				}
			}
			for _, phase := range []string{"plain", "quotes_on", "order_management", "plain"} {
				var observation string
				for _, row := range observations[1:] {
					if row[0] == api && row[1] == phase && row[2] == "isTest" {
						observation = row[3]
					}
				}
				if observation == "" {
					t.Fatal("missing native row")
				}
				source := "@IsTest private class ObjectAvailabilityRoutesTest { @IsTest static void capture() { Map<String,Object> expected=(Map<String,Object>)JSON.deserializeUntyped('" + observation + "'); System.assertEquals(expected,ObjectAvailabilityRoutes.observe()); } }"
				if err := os.WriteFile(filepath.Join(root, "force-app/main/default/classes/ObjectAvailabilityRoutesTest.cls"), []byte(source), 0644); err != nil {
					t.Fatal(err)
				}
				shape, err := os.ReadFile(filepath.Join(fixture, "shapes", phase+".json"))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(filepath.Join(root, "config"), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "config/project-scratch-def.json"), shape, 0644); err != nil {
					t.Fatal(err)
				}
				for _, cache := range []string{"cold", "cache", "warm"} {
					args := []string{"test", "--project", root, "--no-serve", "--read-only", "--no-progress", "--json", "--parallelism", "1", "--no-parallel-methods"}
					if cache == "cold" {
						args = append(args, "--no-cache")
					}
					var out, errs bytes.Buffer
					code := gladecli.Run(context.Background(), args, &out, &errs)
					var envelope struct {
						Data testreport.Run `json:"data"`
					}
					if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
						t.Fatalf("%s/%s: %v: %s", phase, cache, err, out.String())
					}
					summary := envelope.Data.Summary()
					if code != 0 || summary.Total != 1 || summary.Passed != 1 {
						t.Fatalf("%s/%s: code=%d summary=%+v stdout=%s stderr=%s", phase, cache, code, summary, out.String(), errs.String())
					}
				}
			}
		})
	}
}
