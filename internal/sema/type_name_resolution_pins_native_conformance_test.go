package sema

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestDottedCallTypeNamesNativeConformance(t *testing.T) {
	const fixture = "../../testdata/conformance/type_name_resolution_pins"
	deferred := namespaceAbsenceDeferrals(t, fixture)
	rows, err := readTypeNamePinRows(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	cases, err := readTypeNamePinCases(filepath.Join(fixture, "cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}

	for _, row := range rows {
		caseInfo, ok := cases[row[2]]
		if !ok || caseInfo.kind != "named_compile" {
			continue
		}
		api := row[0]
		compiled := false
		if err := parseNativeCompileResult(row[3], &compiled); err != nil {
			t.Fatalf("invalid native row %v: %v", row, err)
		}
		t.Run(api+"/"+row[2], func(t *testing.T) {
			index := buildTypeNamePinIndex(t, fixture, api, caseInfo.source)
			if index.HasErrors() {
				t.Fatalf("invalid synthetic source: %+v", index.Diagnostics)
			}
			result := Analyze(index)
			gotCompiled := !result.HasErrors()
			if gotCompiled != (compiled || deferred[row[2]]) {
				t.Fatalf("native compiled=%t observation=%q; Glade compiled=%t diagnostics=%+v", compiled, row[4], gotCompiled, result.Diagnostics)
			}
		})
	}
}

func readTypeNamePinRows(path string) ([][]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	return reader.ReadAll()
}

func readTypeNamePinCases(path string) (map[string]struct{ kind, source string }, error) {
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

func parseNativeCompileResult(value string, compiled *bool) error {
	if value == "true" {
		*compiled = true
		return nil
	}
	if value == "false" {
		*compiled = false
		return nil
	}
	return os.ErrInvalid
}

func buildTypeNamePinIndex(t *testing.T, fixture, api, compileSource string) typesys.Index {
	t.Helper()
	root := t.TempDir()
	classDir := filepath.Join(fixture, "project", "force-app", "main", "default", "classes")
	entries, err := os.ReadDir(classDir)
	if err != nil {
		t.Fatal(err)
	}
	var apexFiles []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".cls" {
			continue
		}
		source, err := os.ReadFile(filepath.Join(classDir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, entry.Name())
		writeSemaFile(t, path, string(source))
		meta, err := os.ReadFile(filepath.Join(classDir, entry.Name()+"-meta.xml"))
		if err != nil {
			t.Fatal(err)
		}
		writeSemaFile(t, path+"-meta.xml", strings.ReplaceAll(string(meta), "62.0", api))
		apexFiles = append(apexFiles, path)
	}
	compileSourcePath := filepath.Join(fixture, compileSource)
	compileBody, err := os.ReadFile(compileSourcePath)
	if err != nil {
		t.Fatal(err)
	}
	compileName := filepath.Base(compileSourcePath)
	compilePath := filepath.Join(root, compileName)
	writeSemaFile(t, compilePath, string(compileBody))
	compileMeta, err := os.ReadFile(compileSourcePath + "-meta.xml")
	if err != nil {
		t.Fatal(err)
	}
	writeSemaFile(t, compilePath+"-meta.xml", strings.ReplaceAll(string(compileMeta), "62.0", api))
	apexFiles = append(apexFiles, compilePath)
	return typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: apexFiles}, schema.Schema{})
}
