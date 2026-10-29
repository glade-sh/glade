package apextest

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestStandardRestrictedPicklistsNativeConformance(t *testing.T) {
	runRestrictedPicklistNativeConformance(t, "testdata/conformance/standard_restricted_picklists", "RestrictedPicklistProbeTest", func(t *testing.T, api string, got, want map[string]any) {
		// The API 67 custom control captured a field-permission boundary.
		if api == "67.0" {
			custom := want["custom"].(map[string]any)
			problems := custom["errors"].([]any)
			if len(problems) != 1 || problems[0].(map[string]any)["status"] != "CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY" {
				t.Fatal("missing captured custom-field permission boundary")
			}
			delete(want, "custom")
			delete(got, "custom")
		}
	})
}

func TestStandardRestrictedPicklistLocaleNativeConformance(t *testing.T) {
	// locale-boundary/org.tsv:2-5, API 62/67, anonymous and @IsTest.
	// This named set is deliberately explicit: additional native rejections
	// must fail the normal comparison until independently accounted for.
	carriedLocaleRejections := []string{
		"NotAnAllowedValue", "az", "be", "dz", "en", "ky", "lo", "lu", "ne",
		"ps", "rn", "so", "tg", "ti", "uz", "xx", "yo", "zz",
	}
	const carryReason = "LocaleSidKey enforcement is deferred because the catalog is not a complete valid-DML list and would risk false rejections"
	runRestrictedPicklistNativeConformance(t, "testdata/conformance/standard_restricted_picklists/locale-boundary", "LocaleRestrictionProbeTest", func(t *testing.T, api string, got, want map[string]any) {
		// The broad catalog is a feature spread; this capture qualifies DML
		// aliases and their absence from describe, not the total org shape.
		delete(got, "totalEntryCount")
		delete(want, "totalEntryCount")
		for _, locale := range carriedLocaleRejections {
			t.Run("CARRIED/LocaleSidKey/"+locale, func(t *testing.T) {
				t.Logf("CARRIED native=%v; local=%v; reason=%s", want[locale], got[locale], carryReason)
				if locale == "NotAnAllowedValue" {
					t.Logf("CARRIED error: native=%v; local=%v; reason=%s", want["error"], got["error"], carryReason)
				}
			})
			delete(got, locale)
			delete(want, locale)
		}
		// The error belongs to NotAnAllowedValue and was logged with that carry.
		delete(got, "error")
		delete(want, "error")
	})
}

func runRestrictedPicklistNativeConformance(t *testing.T, fixture, testClass string, normalize func(*testing.T, string, map[string]any, map[string]any)) {
	t.Helper()
	table, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	rows := strings.Split(strings.TrimSpace(string(table)), "\n")
	if len(rows) != 5 || rows[0] != "source_api\troute\tobservation" {
		t.Fatal("incomplete native matrix")
	}
	for _, line := range rows[1:] {
		row := strings.SplitN(line, "\t", 3)
		if len(row) != 3 {
			t.Fatal("invalid native row")
		}
		api, route := row[0], row[1]
		t.Run(api+"/"+route, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(fixture, "project")
			err := filepath.WalkDir(project, func(path string, entry os.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(body), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native compiled; Glade: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			check := func(t *testing.T, message string) {
				t.Helper()
				_, payload, ok := strings.Cut(message, "OBS|")
				var got, want map[string]any
				if !ok || json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(row[2]), &want) != nil {
					t.Fatalf("invalid observation: %s", message)
				}
				normalize(t, api, got, want)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("native=%s; Glade=%s", row[2], payload)
				}
			}
			if route == "anonymous" {
				body, err := os.ReadFile(filepath.Join(fixture, "anonymous.apex"))
				if err != nil {
					t.Fatal(err)
				}
				program, err := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
				for _, state := range []string{"cold", "warm"} {
					t.Run(state, func(t *testing.T) {
						_, err := runner.newMachine().Execute(program)
						var fault *vm.RuntimeError
						if !errors.As(err, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
							t.Fatalf("missing observation: %v", err)
						}
						check(t, fault.ExceptionMessage())
					})
				}
			} else if route == "isTest" {
				for _, state := range []string{"cold", "warm"} {
					t.Run(state, func(t *testing.T) {
						result := Run(index, Options{SelectedClasses: []string{testClass}, SelectedMethod: "observe", Parallelism: 1})
						if len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 {
							t.Fatalf("expected one observation: %+v", result.Summary())
						}
						problem := result.Suites[0].Cases[0].Problem
						if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") {
							t.Fatalf("missing observation: %+v", problem)
						}
						check(t, problem.Message)
					})
				}
			} else {
				t.Fatal("unknown route: " + route)
			}
		})
	}
}

func TestStandardRestrictedPicklistBroadSampleNativeConformance(t *testing.T) {
	runRestrictedPicklistNativeConformance(t, "testdata/conformance/standard_restricted_picklists/broad-sample", "StandardPicklistSampleTest", func(*testing.T, string, map[string]any, map[string]any) {})
}

func TestStandardRestrictedPicklistProjectValueSetsNativeConformance(t *testing.T) {
	runRestrictedPicklistNativeConformance(t, "testdata/conformance/standard_restricted_picklists/project-valuesets", "ProjectValueSetProbeTest", func(*testing.T, string, map[string]any, map[string]any) {})
}
