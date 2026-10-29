package apextest

import (
	"encoding/csv"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestTypeNameResolutionPinsNativeConformance(t *testing.T) {
	const fixture = "../../testdata/conformance/type_name_resolution_pins"
	deferred := namespaceAbsenceDeferrals(t, fixture)
	rows, err := readTypeNamePinRowsForApextest(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	cases, err := readTypeNamePinCasesForApextest(filepath.Join(fixture, "cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}

	indexes := map[string]struct {
		index typesys.Index
		org   *storage.OrgState
	}{}
	for _, row := range rows[1:] {
		if len(row) != 5 || (row[1] != "anonymous" && row[1] != "named_class") {
			t.Fatalf("invalid native row: %v", row)
		}
		caseInfo, ok := cases[row[2]]
		if !ok {
			t.Fatalf("native row names unknown case %q", row[2])
		}
		projectCase := ""
		if caseInfo.kind == "named_compile" {
			projectCase = caseInfo.source
		}
		cacheKey := row[0] + "/" + projectCase
		state, ok := indexes[cacheKey]
		if !ok {
			root := copyTypeNamePinProject(t, fixture, row[0], projectCase)
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatalf("native source did not index at API %s: %+v", row[0], index.Diagnostics)
			}
			if caseInfo.kind != "named_compile" {
				analysis := sema.Analyze(index)
				if analysis.HasErrors() {
					t.Fatalf("native source did not compile at API %s: %+v", row[0], analysis.Diagnostics)
				}
			}
			org := orgFromIndex(index)
			org.APIVersion = row[0]
			state = struct {
				index typesys.Index
				org   *storage.OrgState
			}{index: index, org: &org}
			indexes[cacheKey] = state
		}
		source, err := os.ReadFile(filepath.Join(fixture, caseInfo.source))
		if err != nil {
			t.Fatal(err)
		}
		wantCompiled := row[3] == "true" || deferred[row[2]]
		if row[3] != "true" && row[3] != "false" {
			t.Fatalf("invalid compile result: %v", row)
		}
		t.Run(row[0]+"/"+row[2], func(t *testing.T) {
			var analysis sema.Result
			if caseInfo.kind == "named_compile" {
				analysis = sema.Analyze(state.index)
			} else {
				analysis = sema.AnalyzeAnonymous(state.index, string(source), row[0])
			}
			gotCompiled := !analysis.HasErrors()
			if gotCompiled != wantCompiled {
				t.Fatalf("native compiled=%t observation=%q; Glade compiled=%t diagnostics=%+v", wantCompiled, row[4], gotCompiled, analysis.Diagnostics)
			}
			if !wantCompiled || cases[row[2]].kind != "runtime" {
				return
			}
			program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			runner := newConformanceRunner(t, state.index, conformanceRunnerOptions{LinkProject: true, Org: state.org})
			result, err := runner.execute(program)
			if err != nil {
				t.Fatal(err)
			}
			got := typeNamePinObservation(result.Debug)
			if got != row[4] {
				t.Fatalf("native=%q; Glade=%q", row[4], got)
			}
		})
	}
}

func readTypeNamePinRowsForApextest(path string) ([][]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	return reader.ReadAll()
}

func readTypeNamePinCasesForApextest(path string) (map[string]struct{ kind, source string }, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	cases := make(map[string]struct{ kind, source string }, len(rows)-1)
	for _, row := range rows[1:] {
		if len(row) != 3 {
			return nil, os.ErrInvalid
		}
		cases[row[0]] = struct{ kind, source string }{kind: row[1], source: row[2]}
	}
	return cases, nil
}

func copyTypeNamePinProject(t *testing.T, fixture, api, extraSource string) string {
	t.Helper()
	projectRoot := filepath.Join(fixture, "project")
	root := t.TempDir()
	err := filepath.WalkDir(projectRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(projectRoot, path)
		if err != nil {
			return err
		}
		writeFile(t, filepath.Join(root, relative), strings.ReplaceAll(string(body), "62.0", api))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if extraSource != "" {
		sourcePath := filepath.Join(fixture, extraSource)
		body, err := os.ReadFile(sourcePath)
		if err != nil {
			t.Fatal(err)
		}
		classPath := filepath.Join(root, "force-app", "main", "default", "classes", filepath.Base(sourcePath))
		writeFile(t, classPath, strings.ReplaceAll(string(body), "62.0", api))
		meta, err := os.ReadFile(sourcePath + "-meta.xml")
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, classPath+"-meta.xml", strings.ReplaceAll(string(meta), "62.0", api))
	}
	return root
}

func typeNamePinObservation(lines []string) string {
	for _, line := range lines {
		if at := strings.Index(line, "RESULT|"); at >= 0 {
			return line[at+len("RESULT|"):]
		}
	}
	return ""
}
