package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type stringFamilyCase struct {
	ID               string `json:"id"`
	Code             string `json:"code"`
	Compile          bool   `json:"compile"`
	Expected         string `json:"expected"`
	FloorExpected    string `json:"floorExpected"`
	NativeBlocker    string `json:"nativeBlocker,omitempty"`
	API68Observation *struct {
		APIVersion, Route, Category, CodeSHA256 string
		Compiled                                bool
		Success                                 *bool
	} `json:"api68Observation,omitempty"`
}

func loadStringFamilyCases(t *testing.T) []stringFamilyCase {
	t.Helper()
	raw, err := os.ReadFile("../../testdata/conformance/string/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []stringFamilyCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 300 {
		t.Fatalf("cases=%d want=300", len(cases))
	}
	blocked := 0
	for _, tc := range cases {
		if tc.NativeBlocker == "" {
			continue
		}
		if tc.ID != "R291" {
			t.Fatalf("unexpected native blocker %s", tc.ID)
		}
		observation := tc.API68Observation
		if observation == nil || observation.APIVersion != "68.0" || observation.Route != "anonymous" ||
			!observation.Compiled || observation.Success == nil || *observation.Success ||
			observation.Category != tc.Expected || observation.CodeSHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte(tc.Code))) {
			t.Fatalf("incomplete API68 native blocker observation for %s: %+v", tc.ID, observation)
		}
		blocked++
	}
	if blocked != 1 {
		t.Fatalf("native blockers=%d want=1 (R291)", blocked)
	}
	return cases
}

func stringFamilyLiteral(s string) string {
	return "'" + strings.NewReplacer("\\", "\\\\", "'", "\\'", "\n", "\\n", "\r", "\\r", "\t", "\\t").Replace(s) + "'"
}

func stringFamilyAssertion(tc stringFamilyCase, api string) string {
	assertion := "System.assertEquals(" + stringFamilyLiteral(tc.Expected) + ", familyObserved, " + stringFamilyLiteral(tc.ID) + ");\n"
	if api == "62.0" {
		assertion = "String familyExpected = " + stringFamilyLiteral(tc.FloorExpected) + "; System.assert(familyExpected.equals(familyObserved), " + stringFamilyLiteral(tc.ID) + " + ' expected <' + familyExpected + '> actual <' + familyObserved + '>');\n"
	}
	return "String familyObserved; try { Object r; " + tc.Code + " familyObserved = String.valueOf(r); } catch (Exception e) { familyObserved = 'EXC|' + e.getTypeName() + '|' + e.getMessage().split('\\n')[0]; }\n" +
		assertion
}

// API62 anonymous oracle rows are an owned native export.
// API68 anonymous capture at native-capture has
// SHA-256 5036dff7109069d85cce63deaaf1d5cf0544c9da813dc3a9100b7653bf2e8e93.
// Its 299 stable answers match the API67 oracle. The later direct capture in
// R291's api68Observation records an uncaught native system error. R291 remains
// excluded from executable parity; the request-specific error IDs stay private.
// API63-66 exercise the API67 projection without native captures.
// No Salesforce CLI, credentials or network is used.
func TestStringFamilyConformanceAnonymous(t *testing.T) {
	cases := loadStringFamilyCases(t)
	nativeCompilerRows := map[string]string{}
	for _, tc := range cases {
		if strings.HasPrefix(tc.FloorExpected, "COMPILE_ERROR") {
			nativeCompilerRows[tc.ID] = tc.FloorExpected
		}
	}
	floorMismatches := newFloorCompilerMismatches(t, "String", nativeCompilerRows, "anonymous")
	for _, api := range []string{"62.0", "63.0", "64.0", "65.0", "66.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "String", api)
			if api != "62.0" && api != "67.0" && api != "68.0" {
				t.Logf("String API %s expectations project the API 67.0 oracle; no native capture at API %s", api, api)
			}
			for _, tc := range cases {
				if tc.NativeBlocker != "" {
					t.Logf("%s excluded from executable conformance: %s", tc.ID, tc.NativeBlocker)
					if api == "68.0" {
						t.Logf("%s API68 anonymous native failure captured (%s); no local parity count", tc.ID, tc.API68Observation.Category)
					}
					continue
				}
				kind := "exact"
				if api != "62.0" {
					kind = "legacy"
				}
				if tc.Compile && api != "62.0" {
					kind = "category"
				}
				if tc.Compile && api == "62.0" {
					kind = floorMismatches.Kind(tc.ID, "anonymous")
				}
				counts.run(t, tc.ID, "anonymous", kind, func(t *testing.T) {
					if tc.Compile {
						code := strings.Split(tc.Code, "pq.out(")[0]
						result := sema.AnalyzeAnonymous(typesys.Index{}, code, api)
						if len(result.Diagnostics) == 0 {
							t.Fatalf("expected Salesforce compile rejection for %s", tc.Code)
						}
						if api == "62.0" {
							observed := conformanceCompilerText(result.Diagnostics, nil)
							floorMismatches.Check(t, tc.ID, "anonymous", tc.FloorExpected, observed)
						}
						return
					}
					program, err := vm.CompileAnonymousWithOptions(stringFamilyAssertion(tc, api), vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := vm.New(nil).Execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		})
	}
}

// The @IsTest route checks local execution against anonymous native values;
// this family has no native named-method capture.
func TestStringFamilyConformanceIsTestClass(t *testing.T) {
	cases := loadStringFamilyCases(t)
	t.Log("API68 named checks are local assertions against anonymous native values; no native named capture, R291 remains excluded")
	for _, api := range []string{"62.0", "63.0", "64.0", "65.0", "66.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "String", api)
			if api != "62.0" && api != "67.0" && api != "68.0" {
				t.Logf("String API %s expectations project the API 67.0 oracle; no native capture at API %s", api, api)
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			var source strings.Builder
			source.WriteString("@IsTest private class StringFamilyOracleTest {\n")
			methods := 0
			count := 0
			for _, tc := range cases {
				if tc.Compile || tc.NativeBlocker != "" {
					continue
				}
				if count%30 == 0 {
					if count > 0 {
						source.WriteString("}\n")
					}
					source.WriteString(fmt.Sprintf("@IsTest static void batch%d() {\n", methods))
					methods++
				}
				count++
				source.WriteString("{\n" + stringFamilyAssertion(tc, api) + "}\n")
			}
			source.WriteString("}}\n")
			if count != 293 {
				t.Fatalf("runtime cases=%d want=293", count)
			}
			path := filepath.Join(root, "force-app/main/default/classes/StringFamilyOracleTest.cls")
			writeFile(t, path, source.String())
			writeFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			kind := "exact"
			if api != "62.0" {
				kind = "legacy"
			}
			counts.TrackKind(t, "@IsTest", kind, count)
			run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
			if got := run.Summary(); got.Total != methods || got.Passed != methods {
				t.Fatalf("summary=%#v want=%d passing batches containing %d cases; first problem: %s", got, methods, count, firstRunProblem(run))
			}
		})
	}
}
