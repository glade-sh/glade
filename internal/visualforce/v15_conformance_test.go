package visualforce

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

type v15Diagnostic struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type v15Case struct {
	ID       string `json:"id"`
	Group    string `json:"group"`
	Kind     string `json:"kind"`
	Category string `json:"category"`
	Click    bool   `json:"click"`
	Inputs   map[string]struct {
		Name     string `json:"name"`
		Markup   string `json:"markup"`
		Metadata string `json:"metadata"`
	} `json:"inputs"`
	Expected        map[string]string        `json:"expected"`
	Diagnostics     map[string]v15Diagnostic `json:"diagnostics"`
	Owner           string                   `json:"remaining_owner,omitempty"`
	Reason          string                   `json:"remaining_reason,omitempty"`
	StructureOwner  string                   `json:"remaining_structure_owner,omitempty"`
	StructureReason string                   `json:"remaining_structure_reason,omitempty"`
}

type v15Table struct {
	Support map[string]map[string]string `json:"support"`
	Getters []struct {
		Name       string `json:"name"`
		ReturnType string `json:"return_type"`
		Body       string `json:"body"`
	} `json:"runtime_getters"`
	Cases []v15Case `json:"cases"`
}

// TestV15SalesforceConformance exports all 203 compile/metadata and 73 rendered
// rows at API 59/67 from the native presentation capture, including its final
// DOM capture. It uses the captured source names, metadata,
// controller bodies and owned resources, with no Salesforce calls. Chromium measures
// product-rendered pages with the same observer fields as the native capture.
// All row checks use exact Go string equality, including diagnostic type/text.
//
// GLADE_V15_CAPTURE=1 reports mismatches without failing on the unchanged base.
// GLADE_V15_REPORT selects the optional per-row TSV output path.
// Chromium observes computed CSS, image/canvas state, executed JavaScript and
// post-click DOM. Full rows retain every native field. Structural counts remain
// separate, so they never inflate exact matches. Every Asset rendering-owned fact is
// asserted; only the two image rows' hosted resource URL differences may be carried.
func TestV15SalesforceConformance(t *testing.T) {
	capture := os.Getenv("GLADE_V15_CAPTURE") != ""
	data, err := os.ReadFile("testdata/v15_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var table v15Table
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	versions := []string{"59.0", "67.0"}
	counts, seen := map[string]int{}, map[string]bool{}
	for _, c := range table.Cases {
		if c.ID == "" || seen[c.ID] || (c.Kind != "compile" && c.Kind != "runtime") {
			t.Fatalf("invalid or duplicate Asset rendering case %q (%s)", c.ID, c.Kind)
		}
		seen[c.ID] = true
		counts[c.Kind]++
		if c.Owner == "Asset rendering" || c.Owner == "Component structure" || c.StructureOwner == "Asset rendering" || c.StructureOwner == "Component structure" {
			t.Fatalf("%s: Asset rendering-owned facts must be asserted; Component structure cannot own new carried rows", c.ID)
		}
		if (c.Owner == "") != (c.Reason == "") || (c.StructureOwner == "") != (c.StructureReason == "") {
			t.Fatalf("%s: every carried fact requires both owner and reason", c.ID)
		}
		for _, api := range versions {
			input := c.Inputs[api]
			if input.Name == "" || input.Markup == "" || input.Metadata == "" {
				t.Fatalf("%s: missing native input at API %s", c.ID, api)
			}
			if c.Kind == "compile" {
				d, ok := c.Diagnostics[api]
				answer := c.Expected[api]
				if !ok || (answer != "COMPILE_OK" && answer != "COMPILE_ERROR") ||
					(answer == "COMPILE_ERROR" && (d.Type == "" || d.Message == "")) ||
					(answer == "COMPILE_OK" && d != (v15Diagnostic{})) {
					t.Fatalf("%s: missing/inconsistent native answer or diagnostic at API %s", c.ID, api)
				}
			} else {
				var native map[string]any
				answer := c.Expected[api]
				if !strings.HasPrefix(answer, "DOM|") || json.Unmarshal([]byte(strings.TrimPrefix(answer, "DOM|")), &native) != nil || native == nil {
					t.Fatalf("%s: missing native DOM answer at API %s", c.ID, api)
				}
				// Validate canonical serialization once; no native field is dropped
				// or normalized differently for the full-row comparison.
				if "DOM|"+v15JSON(t, native) != answer {
					t.Fatalf("%s: native DOM serialization changed at API %s", c.ID, api)
				}
			}
		}
	}
	if len(table.Cases) != 276 || counts["compile"] != 203 || counts["runtime"] != 73 || len(table.Getters) != 8 {
		t.Fatalf("Asset rendering requires 276 cases (203 compile/metadata + 73 DOM) and eight getters; got %d, %v, %d", len(table.Cases), counts, len(table.Getters))
	}
	var report strings.Builder
	tsv := csv.NewWriter(&report)
	tsv.Comma = '\t'
	write := func(fields ...string) {
		if err := tsv.Write(fields); err != nil {
			t.Fatal(err)
		}
	}
	write("api", "id", "status", "actual", "expected", "group", "kind", "diagnostic_type", "diagnostic", "expected_diagnostic_type", "expected_diagnostic", "acceptance_status", "diagnostic_status", "dom_structure_status", "actual_dom_structure", "expected_dom_structure", "owner", "reason", "unobserved", "structure_owner", "structure_reason")
	matches, total, compileMatches, acceptanceMatches, diagnosticMatches, domMatches, structureMatches := 0, 0, 0, 0, 0, 0, 0
	for _, api := range versions {
		controllers := v15Controllers(t, table, api)
		browserRows := v15BrowserObservations(t, table, api, controllers)
		apiMatches, apiCompile, apiAcceptance, apiDiagnostic, apiDOM, apiStructure := 0, 0, 0, 0, 0, 0
		for _, c := range table.Cases {
			root, _, _, compileErr := v15LoadCase(t, table, c, api)
			got := "COMPILE_OK"
			diagnostic := v15Diagnostic{}
			if compileErr != nil {
				got = "COMPILE_ERROR"
				diagnostic = v15Diagnostic{Type: "Error", Message: compileErr.Error()}
			}
			structureStatus, actualStructure, expectedStructure, unobserved := "NOT_APPLICABLE", "", "", ""
			var native map[string]any
			if c.Kind == "runtime" {
				if err := json.Unmarshal([]byte(strings.TrimPrefix(c.Expected[api], "DOM|")), &native); err != nil {
					t.Fatal(err)
				}
				expectedStructure = v15JSON(t, v15Structure(native, c.Click))
				structureStatus = "MISMATCH"
				if compileErr == nil {
					observed := browserRows[c.ID]
					got = "DOM|" + v15JSON(t, observed)
					actualStructure = v15JSON(t, v15Structure(observed, c.Click))
				}
				if actualStructure == expectedStructure {
					structureStatus = "MATCH"
					structureMatches++
					apiStructure++
				}
			}
			want := c.Expected[api]
			acceptanceMatch, diagnosticMatch := got == want, true
			acceptanceStatus, diagnosticStatus := "MISMATCH", "NOT_APPLICABLE"
			if acceptanceMatch {
				acceptanceStatus = "MATCH"
				if c.Kind == "compile" {
					acceptanceMatches++
					apiAcceptance++
				}
			}
			if c.Kind == "compile" {
				diagnosticMatch = diagnostic == c.Diagnostics[api]
				diagnosticStatus = "MISMATCH"
				if diagnosticMatch {
					diagnosticStatus = "MATCH"
					diagnosticMatches++
					apiDiagnostic++
				}
			}
			status, owner, reason := "MISMATCH", "Asset rendering", ""
			total++
			if acceptanceMatch && diagnosticMatch {
				status, owner = "MATCH", ""
				matches++
				apiMatches++
				if c.Kind == "compile" {
					compileMatches++
					apiCompile++
				} else {
					domMatches++
					apiDOM++
				}
			} else if !acceptanceMatch {
				if c.Kind == "compile" || !strings.HasPrefix(got, "DOM|") {
					reason = fmt.Sprintf("local %s; native %s", got, strings.SplitN(want, "|", 2)[0])
				} else {
					var observed map[string]any
					if err := json.Unmarshal([]byte(strings.TrimPrefix(got, "DOM|")), &observed); err != nil {
						t.Fatal(err)
					}
					reason = v15FirstDifference(t, "$", observed, native)
				}
			} else {
				reason = "exact native/local diagnostic type or text differs"
			}
			if unobserved != "" {
				reason += "; unobserved locally: " + unobserved
			}
			if c.Kind == "runtime" && structureStatus == "MISMATCH" && actualStructure != "" {
				var observedTree, nativeTree any
				if err := json.Unmarshal([]byte(actualStructure), &observedTree); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal([]byte(expectedStructure), &nativeTree); err != nil {
					t.Fatal(err)
				}
				reason += "; structural DOM: " + v15FirstDifference(t, "$", observedTree, nativeTree)
			}
			// Keep comparisons exact; replace the temporary root only in the report.
			diagnosticText := strings.ReplaceAll(diagnostic.Message, root, "<fixture>")
			if status != "MATCH" && c.Owner != "" {
				owner = c.Owner
				reason = c.Reason + "; observed: " + reason
			}
			write(api, c.ID, status, got, want, c.Group, c.Kind, diagnostic.Type, diagnosticText, c.Diagnostics[api].Type, c.Diagnostics[api].Message, acceptanceStatus, diagnosticStatus, structureStatus, actualStructure, expectedStructure, owner, reason, unobserved, c.StructureOwner, c.StructureReason)
			if !capture {
				if status != "MATCH" {
					if c.Owner == "" {
						t.Errorf("API %s %s expected <%s> actual <%s>; diagnostic expected <%#v> actual <%#v>; %s", api, c.ID, want, got, c.Diagnostics[api], diagnostic, reason)
					} else {
						t.Logf("API %s %s remaining row owned by %s: %s; expected <%s> actual <%s>; diagnostic expected <%#v> actual <%#v>", api, c.ID, c.Owner, reason, want, got, c.Diagnostics[api], diagnostic)
					}
				} else if c.Owner != "" {
					t.Errorf("API %s %s now matches native exactly; remove its stale remaining-row annotation", api, c.ID)
				}
				if c.Kind == "runtime" {
					if structureStatus != "MATCH" && c.StructureOwner == "" {
						t.Errorf("API %s %s structural DOM expected <%s> actual <%s>", api, c.ID, expectedStructure, actualStructure)
					} else if structureStatus != "MATCH" {
						t.Logf("API %s %s remaining structural DOM owned by %s: %s; expected <%s> actual <%s>", api, c.ID, c.StructureOwner, c.StructureReason, expectedStructure, actualStructure)
					} else if c.StructureOwner != "" {
						t.Errorf("API %s %s structural DOM now matches native; remove its stale remaining-structure annotation", api, c.ID)
					}
				}
			}
		}
		write("API_TOTAL", api, fmt.Sprintf("%d/276", apiMatches))
		write("COMPILE_API_TOTAL", api, fmt.Sprintf("%d/203", apiCompile))
		write("COMPILE_ACCEPTANCE_API_TOTAL", api, fmt.Sprintf("%d/203", apiAcceptance))
		write("DIAGNOSTIC_API_TOTAL", api, fmt.Sprintf("%d/203", apiDiagnostic))
		write("DOM_API_TOTAL", api, fmt.Sprintf("%d/73", apiDOM))
		write("DOM_STRUCTURE_API_TOTAL", api, fmt.Sprintf("%d/73", apiStructure))
		t.Logf("Asset rendering API %s exact matches %d/276; compile %d/203 (acceptance %d, diagnostics %d); full DOM %d/73; structural DOM %d/73", api, apiMatches, apiCompile, apiAcceptance, apiDiagnostic, apiDOM, apiStructure)
	}
	write("TOTAL", fmt.Sprintf("%d/%d", matches, total))
	write("COMPILE_TOTAL", fmt.Sprintf("%d/406", compileMatches))
	write("COMPILE_ACCEPTANCE_TOTAL", fmt.Sprintf("%d/406", acceptanceMatches))
	write("DIAGNOSTIC_TOTAL", fmt.Sprintf("%d/406", diagnosticMatches))
	write("DOM_TOTAL", fmt.Sprintf("%d/146", domMatches))
	write("DOM_STRUCTURE_TOTAL", fmt.Sprintf("%d/146", structureMatches))
	t.Logf("Asset rendering exact matches %d/%d; full DOM %d/146; structural DOM %d/146 (structural matches do not enter the exact total)", matches, total, domMatches, structureMatches)
	tsv.Flush()
	if err := tsv.Error(); err != nil {
		t.Fatal(err)
	}
	if path := os.Getenv("GLADE_V15_REPORT"); path != "" {
		if err := os.WriteFile(path, []byte(report.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func v15Controllers(t *testing.T, table v15Table, api string) []vm.Class {
	t.Helper()
	name := "FamilyV15Controller" + strings.TrimSuffix(api, ".0")
	class := vm.Class{Name: name, APIVersion: api, Access: "public", Fields: map[string]vm.Field{}, Methods: map[string]vm.Method{}}
	for _, getter := range table.Getters {
		program, err := vm.CompileAnonymousWithOptions(getter.Body, vm.CompileOptions{APIVersion: api})
		if err != nil {
			t.Fatalf("compile captured getter %s API %s: %v", getter.Name, api, err)
		}
		method := vm.Method{Name: name + "." + getter.Name, ClassName: name, ReturnType: getter.ReturnType, Access: "public", APIVersion: api, Program: program}
		class.Methods[strings.ToLower(getter.Name)] = method
		property := strings.TrimPrefix(getter.Name, "get")
		class.Fields[strings.ToLower(property)] = vm.Field{Name: property, Type: getter.ReturnType, Access: "public", Getter: &method}
	}
	// These are the two nested types in the captured controller. Point's public
	// properties and constructor body are identical to the owned source fixture.
	constructor, err := vm.CompileAnonymousWithOptions("name=n;amount=a;", vm.CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	pointName := name + ".Point"
	point := vm.Class{Name: pointName, APIVersion: api, Access: "public", Fields: map[string]vm.Field{
		"name": {Name: "name", Type: "String", Access: "public"}, "amount": {Name: "amount", Type: "Decimal", Access: "public"},
	}, Constructors: []vm.Method{{Name: pointName + ".<init>", ClassName: pointName, ReturnType: "void", Access: "public", APIVersion: api, IsConstructor: true, Params: []vm.Param{{Name: "n", Type: "String"}, {Name: "a", Type: "Decimal"}}, Program: constructor}}}
	return []vm.Class{point, {Name: name + ".V15Exception", SuperClass: "Exception", Access: "public", APIVersion: api}, class}
}

// Only this separate structural column drops browser-only fields. The primary
// comparison retains every native field and cannot pass from this projection.
func v15Structure(observed map[string]any, click bool) any {
	if click {
		observed, _ = observed["before"].(map[string]any)
	}
	if _, ok := observed["ownedException"]; ok {
		return observed
	}
	var projectTree func(any) any
	projectTree = func(value any) any {
		node, ok := value.(map[string]any)
		if !ok {
			return value
		}
		projected := map[string]any{}
		for key, value := range node {
			if key == "css" || key == "image" || key == "canvas" {
				continue
			}
			if key == "children" {
				children := []any{}
				for _, child := range value.([]any) {
					children = append(children, projectTree(child))
				}
				projected[key] = children
			} else {
				projected[key] = value
			}
		}
		return projected
	}
	return projectTree(observed["tree"])
}

func v15JSON(t *testing.T, value any) string {
	t.Helper()
	var out strings.Builder
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		t.Fatal(err)
	}
	// The captured Python TSV uses ensure_ascii=True. Match that transport
	// encoding exactly, including NBSP in pageBlock chrome and surrogate pairs.
	var ascii strings.Builder
	for _, r := range strings.TrimSuffix(out.String(), "\n") {
		if r <= 127 {
			ascii.WriteRune(r)
		} else if r <= 0xffff {
			fmt.Fprintf(&ascii, `\u%04x`, r)
		} else {
			r -= 0x10000
			fmt.Fprintf(&ascii, `\u%04x\u%04x`, 0xd800+(r>>10), 0xdc00+(r&0x3ff))
		}
	}
	return ascii.String()
}

func v15FirstDifference(t *testing.T, path string, actual, expected any) string {
	t.Helper()
	if v15JSON(t, actual) == v15JSON(t, expected) {
		return ""
	}
	if want, ok := expected.(map[string]any); ok {
		if got, ok := actual.(map[string]any); ok {
			keys := make([]string, 0, len(want)+len(got))
			for key := range want {
				keys = append(keys, key)
			}
			for key := range got {
				if _, exists := want[key]; !exists {
					keys = append(keys, key)
				}
			}
			sort.Strings(keys)
			for _, key := range keys {
				_, gotExists := got[key]
				_, wantExists := want[key]
				if gotExists != wantExists {
					return fmt.Sprintf("%s.%s field presence differs (local %t, native %t)", path, key, gotExists, wantExists)
				}
				if difference := v15FirstDifference(t, path+"."+key, got[key], want[key]); difference != "" {
					return difference
				}
			}
		}
	}
	if want, ok := expected.([]any); ok {
		if got, ok := actual.([]any); ok {
			if len(got) != len(want) {
				return fmt.Sprintf("%s length differs (local %d, native %d)", path, len(got), len(want))
			}
			for i := range want {
				if difference := v15FirstDifference(t, fmt.Sprintf("%s[%d]", path, i), got[i], want[i]); difference != "" {
					return difference
				}
			}
		}
	}
	// Escape embedded whitespace so every remaining cause stays on one line.
	return fmt.Sprintf("%s local %q; native %q", path, v15JSON(t, actual), v15JSON(t, expected))
}
