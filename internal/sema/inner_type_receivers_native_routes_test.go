package sema

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Replay the captured named sources through each body checker independently.
// checkBodyText itself invokes IR, so the text route below calls only its text
// scope, assignment, return and call checks. Anonymous syntax and runtime
// observations remain covered by TestInnerTypeReceiverNativeConformance.
func TestInnerTypeReceiverNativeIRAndTextRoutes(t *testing.T) {
	const dir = "../apextest/testdata/conformance/inner_type_receivers"
	var cases []struct {
		ID, TestClass, TestSource string
		RunnerReject              bool
		Classes, SetupClasses     map[string]string
	}
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]int, len(cases))
	setup := map[string]string{}
	for i, c := range cases {
		if _, duplicate := byID[c.ID]; c.ID == "" || duplicate {
			t.Fatalf("missing or duplicate captured case: %q", c.ID)
		}
		byID[c.ID] = i
		for name, source := range c.SetupClasses {
			setup[name] = source
		}
	}
	raw, err = os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) == 0 || strings.Join(rows[0], ",") != "id,api,route,compiled,success,outcome" {
		t.Fatalf("invalid native table: %v", err)
	}
	// Retain the deployed helper project, including foreign duplicate inner
	// names, so a reduced project cannot accidentally create a unique alias.
	projects := map[string]map[string]string{"62.0": {}, "67.0": {}}
	named := map[string][]string{}
	rowNumbers := map[string]int{}
	for i, row := range rows[1:] {
		if len(row) != 6 {
			t.Fatalf("invalid native row: %q", row)
		}
		if row[2] != "isTest" {
			continue
		}
		caseIndex, exists := byID[row[0]]
		if !exists || projects[row[1]] == nil {
			t.Fatalf("unknown captured case or API: %q", row)
		}
		key := row[0] + "/" + row[1]
		if named[key] != nil {
			t.Fatalf("duplicate named capture: %s", key)
		}
		named[key], rowNumbers[key] = row, i+2
		if row[3] == "true" {
			for name, source := range cases[caseIndex].Classes {
				projects[row[1]][name] = source
			}
		}
	}
	for _, id := range []string{
		"saveresult_reject", "describefieldresult_reject",
		"saveresult_direct_reject", "describefieldresult_direct_reject",
		"parameter_direct_reject", "duplicate_direct_reject",
		"duplicate_reject", "field_enum_direct_reject",
		"parameter_public", "saveresult_public", "describefieldresult_public",
		"duplicate_public", "field_enum_boundary",
	} {
		caseIndex, exists := byID[id]
		if !exists {
			t.Fatalf("missing captured source: %s", id)
		}
		c := cases[caseIndex]
		for _, api := range []string{"62.0", "67.0"} {
			t.Run(id+"/"+api, func(t *testing.T) {
				key := id + "/" + api
				row := named[key]
				if row == nil || row[3] != "true" && row[3] != "false" {
					t.Fatalf("missing native acceptance for %s", key)
				}
				t.Logf("native: %s/org.tsv:%d compiled=%s outcome=%q", dir, rowNumbers[key], row[3], row[5])
				files := map[string]string{}
				for name, source := range projects[api] {
					files[name] = source
				}
				if c.RunnerReject {
					for name, source := range setup {
						files[name] = source
					}
				}
				for name, source := range c.Classes {
					files[name] = source
				}
				files[c.TestClass] = c.TestSource
				root := t.TempDir()
				var paths []string
				for name, source := range files {
					path := filepath.Join(root, name+".cls")
					writeSemaFile(t, path, source)
					paths = append(paths, path)
				}
				sort.Strings(paths)
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
				if index.HasErrors() {
					t.Fatalf("captured source index: %v", index.Diagnostics)
				}
				for _, route := range []string{"IR", "text"} {
					t.Run(route, func(t *testing.T) {
						a := NewAnalyzer()
						a.prepareAnalysisContext(index, AnalyzeOptions{})
						a.prepareAnalysisModel(index)
						model := buildSemaTypeMemberView(index)
						constructability := buildConstructability(index)
						var diagnostics []diagnostic.Diagnostic
						for _, typ := range index.Types {
							source, err := os.ReadFile(typ.File)
							if err != nil {
								t.Fatal(err)
							}
							typeModel := semaModelWithCurrentType(model, typ)
							for _, member := range typ.Members {
								if member.Kind != apexast.DeclarationMethod {
									continue
								}
								body, offset, ok := semaBodyFromRange(string(source), member.BodyRange)
								if !ok {
									continue
								}
								member = semaNormalizeMemberTypes(typeModel, typ.Name, semaCloneMemberSymbol(member))
								base := semaBodyBaseScope(typ, member, typeModel)
								if route == "IR" {
									diags, compiled := a.checkBodyIRWithCompileStatus(typ, member, body, offset, string(source), base, typeModel, constructability)
									if !compiled {
										t.Fatalf("captured body did not lower: %s.%s: %v", typ.Name, member.Name, diags)
									}
									diagnostics = append(diagnostics, diags...)
								} else {
									scan := newSemaBodyExpressionScan(body)
									scopes, diags := a.collectBodyScopesWithLocalDeclMatches(typ, member, body, offset, string(source), base, typeModel, scan.localDeclMatches)
									diagnostics = append(diagnostics, diags...)
									diagnostics = append(diagnostics, a.checkBodyAssignments(typ, member, scan, offset, string(source), scopes, typeModel)...)
									diagnostics = append(diagnostics, a.checkBodyReturns(typ, member, scan, offset, string(source), scopes, typeModel)...)
									diagnostics = append(diagnostics, a.checkBodyCalls(typ, member, body, offset, string(source), scopes, typeModel)...)
								}
							}
						}
						diagnostics = NativeLifecycleDiagnostics(index, diagnostics)
						got := innerShadowNativeErrors(diagnostics)
						if row[3] == "true" {
							if len(got) != 0 {
								t.Fatalf("native accepts; got %q", got)
							}
							return
						}
						for _, native := range strings.Split(row[5], " | ") {
							if innerShadowHasNative(got, strings.TrimSpace(native)) {
								return
							}
						}
						t.Fatalf("native rejection=%q; got %q", row[5], got)
					})
				}
			})
		}
	}
}

// This is a checker-boundary regression test, not an additional native row.
// As on the release base, generic containers must reach generic assignability;
// resolving only their erased Map/List names loses the argument identities.
func TestInnerTypeReceiverGenericIdentityPreservesBase(t *testing.T) {
	model := allocationTestView(typeMembers{name: "Map"}, typeMembers{name: "List"})
	for _, pair := range [][2]string{
		{"Map<String, Outer.Inner>", "Map<Integer, Outer.Inner>"},
		{"Map<Outer.Inner, String>", "Map<Outer.Inner, Integer>"},
		{"List<Left.Inner>", "List<Right.Inner>"},
	} {
		if semaNestedShortTypeEquivalent(pair[0], pair[1], model) {
			t.Errorf("short-name comparison erased generic arguments: %s / %s", pair[0], pair[1])
		}
	}
}
