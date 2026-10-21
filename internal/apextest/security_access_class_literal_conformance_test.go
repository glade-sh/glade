package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// These six native positive controls are separate from the main A28 export.
// Their methods execute inside anonymous-declared classes, not named @IsTest
// classes. The old empty COMPILE_ERROR payloads did not establish rejection.
func TestSecurityAccessClassLiteralAnonymousConformance(t *testing.T) {
	var data struct {
		APIVersions []string
		Route       string
		Metadata    map[string]string
		Cases       []struct {
			ID, Code, CaseSHA256, Anonymous, AnonymousSHA256, Expected string
			Native                                                     map[string]struct {
				Compiled, Success bool
				ExceptionMessage  string
			}
		}
	}
	raw, err := os.ReadFile("testdata/conformance/security_access_class_literals.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,68.0" || data.Route != "anonymous-declared-class" || len(data.Cases) != 6 {
		t.Fatal("invalid anonymous class-literal capture scope")
	}
	for i, row := range data.Cases {
		if row.ID != fmt.Sprintf("NC%03d", i+7) || row.Expected != "true" || fmt.Sprintf("%x", sha256.Sum256([]byte(row.Code))) != row.CaseSHA256 || fmt.Sprintf("%x", sha256.Sum256([]byte(row.Anonymous))) != row.AnonymousSHA256 || !strings.Contains(row.Anonymous, row.Code) || len(row.Native) != 2 {
			t.Fatalf("invalid source binding for %s", row.ID)
		}
		for _, api := range data.APIVersions {
			native := row.Native[api]
			marker := "FamilyProbeTransportException: GLADE_FAMILY_ROWS|[\"P|" + row.ID + "|true\"]"
			if !native.Compiled || native.Success || native.ExceptionMessage != marker {
				t.Fatalf("native positive observation missing for %s at %s", row.ID, api)
			}
		}
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			for _, row := range data.Cases {
				t.Run(row.ID, func(t *testing.T) {
					root := t.TempDir()
					for path, source := range data.Metadata {
						writeFile(t, filepath.Join(root, path), source)
					}
					p, err := project.Load(root)
					if err != nil {
						t.Fatal(err)
					}
					p.SourceAPIVersion = api
					schema, err := gladeschema.LoadProject(p)
					if err != nil {
						t.Fatal(err)
					}
					helperPath := filepath.Join(root, "P.cls")
					writeFile(t, helperPath, `public class P {
 public void out(String id,Object value){
  System.assertEquals('true',String.valueOf(value),id);
 }
}`)
					p.ApexFiles = []string{helperPath}
					// Match exec's declaration extraction and request runtime path. Keep the
					// captured snippet bytes; mask only the declarations linked into the index.
					masked := []byte(row.Code)
					declarations := 0
					for _, decl := range apexast.NewParser().ParseSource("AnonymousClassLiteral.cls", row.Code).Declarations {
						if decl.Kind != apexast.DeclarationClass {
							continue
						}
						start, end := decl.Range.Start.Offset, decl.Range.End.Offset
						if start < 0 || end <= start || end > len(row.Code) {
							t.Fatal("invalid declaration source range")
						}
						path := filepath.Join(root, decl.Name+".cls")
						writeFile(t, path, row.Code[start:end])
						p.ApexFiles = append(p.ApexFiles, path)
						declarations++
						for i := start; i < end; i++ {
							if masked[i] != '\n' && masked[i] != '\r' {
								masked[i] = ' '
							}
						}
					}
					if declarations != 1 {
						t.Fatalf("expected one captured class, got %d", declarations)
					}
					index := sema.WithAnonymousDeclarationContext(typesys.Build(p, schema))
					if index.HasErrors() {
						t.Fatalf("anonymous parser: %v", index.Diagnostics)
					}
					if analysis := sema.AnalyzeAnonymousDeclarations(index); analysis.HasErrors() {
						t.Fatalf("anonymous declaration semantics: %v", analysis.Diagnostics)
					}
					source := "P pq=new P();\n" + string(masked)
					if analysis := sema.AnalyzeAnonymous(index, source, api); analysis.HasErrors() {
						t.Fatalf("anonymous semantics: %v", analysis.Diagnostics)
					}
					runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
					if err != nil {
						t.Fatal(err)
					}
					org := orgFromIndex(index)
					org.APIVersion = api
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
					program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := runner.execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
			t.Log("six native anonymous-class controls; no API67 or native named qualification")
		})
	}
}
