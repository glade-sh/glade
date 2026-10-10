package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type dateTimeOracleRow struct {
	ID, Code, Expected string
	RemainingReason    string `json:"remainingReason,omitempty"`
}
type dateTimeOracleVersion struct {
	APIVersion                 string `json:"apiVersion"`
	Locale, Timezone, Language string
	Cases                      []dateTimeOracleRow
	Controls                   []dateTimeOracleRow `json:"controls,omitempty"`
}
type dateTimeObservation struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// This owned export has no dependency on glade-tools, credentials, or Salesforce.
// The optional capture directory records an actual baseline, including mismatches;
// without it this same full-table adapter enforces conformance. Named observations
// leave via a tagged assertion because the runner deliberately does not expose
// System.debug output; the tag/type guard distinguishes it from a real failure.
func TestDateTimeOrgConformance(t *testing.T) {
	var data struct {
		Versions []dateTimeOracleVersion `json:"versions"`
	}
	raw, err := os.ReadFile("testdata/date_time_oracle.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Versions) != 2 {
		t.Fatalf("API versions=%d, want floor and ceiling", len(data.Versions))
	}
	versions := map[string]bool{"62.0": false, "67.0": false}
	for _, version := range data.Versions {
		if seen, expected := versions[version.APIVersion]; !expected || seen {
			t.Fatalf("unexpected or duplicate API version %q", version.APIVersion)
		}
		versions[version.APIVersion] = true
	}
	captureDir := os.Getenv("GLADE_DATE_TIME_CAPTURE_DIR")
	for _, v := range data.Versions {
		t.Run(v.APIVersion, func(t *testing.T) {
			if len(v.Cases) != 450 {
				t.Fatalf("rows=%d, want450", len(v.Cases))
			}
			primaryIDs := make(map[string]bool, len(v.Cases))
			for _, row := range v.Cases {
				primaryIDs[row.ID] = true
			}
			rows := append(append([]dateTimeOracleRow{}, v.Cases...), v.Controls...)
			seen := make(map[string]bool, len(rows))
			enforced := 0
			for _, row := range rows {
				if row.ID == "" || seen[row.ID] {
					t.Fatalf("empty or duplicate row ID %q", row.ID)
				}
				seen[row.ID] = true
				if primaryIDs[row.ID] && row.RemainingReason == "" {
					enforced++
				}
			}
			anonymous, named := map[string]dateTimeObservation{}, map[string]dateTimeObservation{}
			actor := fmt.Sprintf("new User(Username='a12-owned@example.invalid',LastName='A12',Alias='a12',Email='a12@example.invalid',EmailEncodingKey='UTF-8',ProfileId=UserInfo.getProfileId(),LocaleSidKey=%s,TimeZoneSidKey=%s,LanguageLocaleKey=%s)", conformanceApexString(v.Locale), conformanceApexString(v.Timezone), conformanceApexString(v.Language))
			body := func(row dateTimeOracleRow) string {
				code := row.Code
				if !strings.Contains(code, ";") {
					code = "r=" + code + ";"
				}
				rawNullCheck := ""
				if row.Expected == "null" {
					// String.valueOf would also render a String containing "null".
					// Native-null rows must first assert the raw Object is null.
					rawNullCheck = "System.assert(r == null," + conformanceApexString(row.ID+" expected raw null actual <") + "+String.valueOf(r)+'>'); "
				}
				return "String observed; try { Object r; " + code + " " + rawNullCheck + "observed=''+String.valueOf(r); } catch(Exception e) { observed='EXC|'+e.getTypeName()+'|'+e.getMessage(); } "
			}
			classRoot := t.TempDir()
			classPath := filepath.Join(classRoot, "DateTimeOracle.cls")
			var class strings.Builder
			class.WriteString("@IsTest private class DateTimeOracle {\n")
			anonymousRunner := newConformanceRunner(t, typesys.Index{}, conformanceRunnerOptions{})
			for _, row := range rows {
				source := body(row) + "System.debug('A12OBS|'+observed+'|A12END');"
				analysis := sema.AnalyzeAnonymous(typesys.Index{}, source, v.APIVersion)
				program, compileErr := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: v.APIVersion})
				if analysis.HasErrors() {
					anonymous[row.ID] = dateTimeObservation{"COMPILE_ERROR", fmt.Sprint(analysis.Diagnostics)}
				} else if compileErr != nil {
					anonymous[row.ID] = dateTimeObservation{"COMPILE_ERROR", compileErr.Error()}
				} else {
					result, runErr := anonymousRunner.execute(program, func(machine *vm.VM) {
						machine.SetCurrentUser(storage.Record{ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{"LocaleSidKey": storage.StringValue(v.Locale), "TimeZoneSidKey": storage.StringValue(v.Timezone), "LanguageLocaleKey": storage.StringValue(v.Language)}})
					})
					found := false
					for _, line := range result.Debug {
						if value, ok := dateTimeTaggedObservation(line); ok {
							if found {
								t.Fatalf("duplicate anonymous observation%s", row.ID)
							}
							anonymous[row.ID] = dateTimeObservation{"OBSERVED", value}
							found = true
						}
					}
					if !found {
						value := "no observation"
						if runErr != nil {
							value = runErr.Error()
						}
						anonymous[row.ID] = dateTimeObservation{"UNCAUGHT", value}
					}
				}
				namedBody := "System.runAs(" + actor + ") { " + body(row) + "System.assert(false,'A12OBS|'+observed+'|A12END'); }"
				if strings.HasPrefix(row.Expected, "COMPILE_ERROR") {
					// Rejected source gets its own class; it cannot invalidate other rows.
					root := t.TempDir()
					path := filepath.Join(root, "DateTimeRejected.cls")
					writeFile(t, path, "@IsTest private class DateTimeRejected { @IsTest static void observed() {"+namedBody+"} }")
					writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+v.APIVersion+"</apiVersion></ApexClass>")
					index := typesys.Build(project.Project{Root: root, ApexFiles: []string{path}}, gladeschema.Schema{})
					if index.HasErrors() {
						named[row.ID] = dateTimeObservation{"COMPILE_ERROR", fmt.Sprint(index.Diagnostics)}
					} else if a := sema.Analyze(index); a.HasErrors() {
						named[row.ID] = dateTimeObservation{"COMPILE_ERROR", fmt.Sprint(a.Diagnostics)}
					} else {
						run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
						for _, suite := range run.Suites {
							for _, r := range suite.Cases {
								named[row.ID] = dateTimeNamedObservation(r)
							}
						}
						if _, ok := named[row.ID]; !ok {
							named[row.ID] = dateTimeObservation{"MISSING", "named runner produced no case"}
						}
					}
				} else {
					fmt.Fprintf(&class, "@IsTest static void observed%s() { %s }\n", row.ID, namedBody)
				}
			}
			class.WriteString("}\n")
			writeFile(t, classPath, class.String())
			writeFile(t, classPath+"-meta.xml", "<ApexClass><apiVersion>"+v.APIVersion+"</apiVersion></ApexClass>")
			index := typesys.Build(project.Project{Root: classRoot, ApexFiles: []string{classPath}}, gladeschema.Schema{})
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
			for _, suite := range run.Suites {
				for _, r := range suite.Cases {
					id := strings.TrimPrefix(r.MethodName, "observed")
					if _, exists := named[id]; exists {
						t.Fatalf("duplicate named row%s", id)
					}
					named[id] = dateTimeNamedObservation(r)
				}
			}
			for route, observations := range map[string]map[string]dateTimeObservation{"anonymous": anonymous, "named": named} {
				matches := 0
				enforcedMatches := 0
				categoryMatches, categoryTotal := 0, 0
				controlMatches := 0
				var tsv strings.Builder
				for _, row := range rows {
					got, exists := observations[row.ID]
					if !exists {
						t.Fatalf("%s missing row%s", route, row.ID)
					}
					value := got.Value
					if got.Kind != "OBSERVED" {
						value = got.Kind + "\t" + value
					}
					fmt.Fprintf(&tsv, "%s\t%s\n", row.ID, strings.ReplaceAll(value, "\n", "\\n"))
					// Go string equality preserves case and length; Apex assertEquals
					// would use the looser == path. Compile rejections compare by kind.
					if row.RemainingReason == "" && strings.HasPrefix(row.Expected, "COMPILE_ERROR") {
						categoryTotal++
						if got.Kind == "COMPILE_ERROR" {
							categoryMatches++
						}
					}
					match := got.Kind == "OBSERVED" && got.Value == row.Expected || got.Kind == "COMPILE_ERROR" && strings.HasPrefix(row.Expected, "COMPILE_ERROR")
					if match {
						if row.RemainingReason == "" {
							enforcedMatches++
						}
						if primaryIDs[row.ID] {
							matches++
						} else {
							controlMatches++
						}
						if captureDir == "" && row.RemainingReason != "" {
							t.Errorf("%s %s now matches; remove stale remainingReason: %s", route, row.ID, row.RemainingReason)
						}
					} else if captureDir == "" && row.RemainingReason == "" {
						t.Errorf("%s%s got%s:%q want%q", route, row.ID, got.Kind, got.Value, row.Expected)
					}
				}
				if captureDir == "" {
					logRoute := route
					if route == "named" {
						logRoute = "@IsTest"
					}
					t.Logf("date and time API %s %s exact %d/%d; category-only %d; carried 0; partial 0; legacy 0", v.APIVersion, logRoute, enforcedMatches-categoryMatches, enforced+len(v.Controls)-categoryTotal, categoryTotal)
				}
				t.Logf("API%s %s %d/450 matches, %d enforced; controls %d/%d (%s/%s/%s)", v.APIVersion, route, matches, enforced, controlMatches, len(v.Controls), v.Locale, v.Timezone, v.Language)
				if captureDir != "" {
					dir := filepath.Join(captureDir, "api"+strings.TrimSuffix(v.APIVersion, ".0"))
					if err = os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
					if err = os.WriteFile(filepath.Join(dir, route+".tsv"), []byte(tsv.String()), 0644); err != nil {
						t.Fatal(err)
					}
					report := struct {
						APIVersion, Route, Locale, Timezone, Language string
						Cases, Matches                                int
						Enforced, ControlCases, ControlMatches        int
						Observations                                  map[string]dateTimeObservation
					}{v.APIVersion, route, v.Locale, v.Timezone, v.Language, len(v.Cases), matches, enforced, len(v.Controls), controlMatches, observations}
					raw, marshalErr := json.MarshalIndent(report, "", "  ")
					if marshalErr != nil {
						t.Fatal(marshalErr)
					}
					if err = os.WriteFile(filepath.Join(dir, route+".json"), append(raw, '\n'), 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
}
func dateTimeTaggedObservation(message string) (string, bool) {
	start := strings.Index(message, "A12OBS|")
	if start < 0 {
		return "", false
	}
	start += len("A12OBS|")
	end := strings.LastIndex(message, "|A12END")
	if end < start {
		return "", false
	}
	return message[start:end], true
}
func dateTimeNamedObservation(result testreport.Case) dateTimeObservation {
	if result.Status == testreport.StatusCompileError {
		return dateTimeObservation{"COMPILE_ERROR", fmt.Sprint(result.Problem)}
	}
	if result.Problem != nil {
		if result.Problem.Type == "System.AssertException" || result.Problem.Type == "AssertException" || result.Problem.Type == "AssertionException" {
			if value, ok := dateTimeTaggedObservation(result.Problem.Message); ok {
				return dateTimeObservation{"OBSERVED", value}
			}
		}
		return dateTimeObservation{"UNCAUGHT", result.Problem.Type + "|" + result.Problem.Message}
	}
	return dateTimeObservation{"MISSING", fmt.Sprintf("runner status%s without observation", result.Status)}
}
