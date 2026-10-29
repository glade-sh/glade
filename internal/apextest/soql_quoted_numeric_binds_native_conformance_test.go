package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// The review follow-up captures mixed literals/binds and the native distinction
// between inline expression binds and dynamic variable references.
func TestSOQLQuotedNumericBindsNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/soql_quoted_numeric_binds"
	data, err := os.ReadFile(filepath.Join(fixture, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		ID     string `json:"id"`
		Kind   string `json:"kind"`
		Source string `json:"source"`
	}
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	byID := map[string]int{}
	for i, c := range cases {
		byID[c.ID] = i
	}
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1+2*len(cases) {
		t.Fatalf("native rows=%d, cases=%d", len(rows)-1, len(cases))
	}
	for _, row := range rows[1:] {
		if len(row) != 4 {
			t.Fatalf("invalid native row: %v", row)
		}
		i, ok := byID[row[2]]
		if !ok {
			t.Fatalf("unknown native case %q", row[2])
		}
		c := cases[i]
		var want struct {
			Compiled bool   `json:"compiled"`
			Message  string `json:"message"`
			Observed any    `json:"observed"`
			Terminal string `json:"terminal"`
		}
		if err = json.Unmarshal([]byte(row[3]), &want); err != nil {
			t.Fatal(err)
		}
		t.Run(row[0]+"/"+c.ID, func(t *testing.T) {
			if c.Kind == "compile" {
				check := func(diagnostics []diagnostic.Diagnostic) {
					t.Helper()
					rejected, matched := false, false
					for _, d := range diagnostics {
						rejected = rejected || d.Severity == diagnostic.Error
						matched = matched || strings.Contains(d.Message, want.Message) || strings.Contains(d.NativeMessage, want.Message)
					}
					if rejected == want.Compiled {
						t.Errorf("native compiled=%v; diagnostics=%v", want.Compiled, diagnostics)
					}
					// Compare the recorded final diagnostic, not compiler caret layout.
					if !want.Compiled && !matched {
						t.Errorf("missing native diagnostic %q: %v", want.Message, diagnostics)
					}
				}
				for _, phase := range []string{"cold", "warm"} {
					t.Run(phase, func(t *testing.T) { check(sema.AnalyzeAnonymous(typesys.Index{}, c.Source, row[0]).Diagnostics) })
				}
				t.Run("named", func(t *testing.T) {
					root := t.TempDir()
					writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+row[0]+`"}`)
					base := filepath.Join(root, "force-app/main/default/classes/QuotedBindTest")
					writeFile(t, base+".cls", "@IsTest public class QuotedBindTest { @IsTest static void probe() { "+c.Source+" } }")
					writeFile(t, base+".cls-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+row[0]+`</apiVersion><status>Active</status></ApexClass>`)
					index := loadTestIndex(t, root)
					a := sema.Analyze(index)
					check(append(index.Diagnostics, a.Diagnostics...))
				})
				return
			}
			program, err := vm.CompileAnonymousWithOptions(c.Source, vm.CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			for _, phase := range []string{"cold", "warm"} {
				t.Run(phase, func(t *testing.T) {
					machine := vm.New(nil)
					org := storage.NewOrgState()
					org.APIVersion = row[0]
					definition, ok := storage.StandardObjectDefinition("Account")
					if !ok {
						t.Fatal("missing standard Account")
					}
					org.Objects["Account"] = storage.ObjectState{Definition: definition}
					machine.SetOrg(&org)
					result, err := machine.Execute(program)
					if want.Terminal != "" {
						if err == nil || !strings.Contains(err.Error(), want.Terminal) {
							t.Fatalf("native terminal=%q; err=%v debug=%v", want.Terminal, err, result.Debug)
						}
						if len(result.Debug) != 0 {
							t.Fatalf("native terminal failure was caught: %v", result.Debug)
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					if len(result.Debug) != 1 {
						t.Fatalf("observations=%v", result.Debug)
					}
					_, payload, ok := strings.Cut(result.Debug[0], "OBS|")
					if !ok {
						t.Fatal(result.Debug)
					}
					var got any
					if err = json.Unmarshal([]byte(payload), &got); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(got, want.Observed) {
						t.Errorf("native=%v; Glade=%v", want.Observed, got)
					}
				})
			}
		})
	}
}
