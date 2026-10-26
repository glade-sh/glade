// Package enumvaluefixture shares owned native enum-call fixtures across entry-point tests.
package enumvaluefixture

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
)

type Case struct{ ID, Shape, Method, Route, Name, Source, Expression, Boundary string }
type Row struct {
	API        string
	Case       Case
	Compiled   bool
	Diagnostic string
	Line       int
}
type Fixture struct {
	Support map[string]string
	Cases   []Case
}

func Load(t *testing.T) (Fixture, []Row) {
	t.Helper()
	const dir = "../sema/testdata/enum_value_methods"
	data, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f Fixture
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	byID := map[string]Case{}
	for _, c := range f.Cases {
		byID[c.ID] = c
	}
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 187 {
		t.Fatalf("native rows = %d, want 186", len(records)-1)
	}
	var rows []Row
	for i, fields := range records[1:] {
		c, ok := byID[fields[1]]
		if !ok {
			t.Fatalf("missing source: %s", fields[1])
		}
		compiled, err := strconv.ParseBool(fields[2])
		if err != nil {
			t.Fatal(err)
		}
		rows = append(rows, Row{fields[0], c, compiled, fields[4], i + 2})
	}
	return f, rows
}

func WriteProject(t *testing.T, root string, f Fixture, row Row) (paths []string, target string) {
	t.Helper()
	write := func(path, source string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	config, err := json.Marshal(map[string]any{"sourceApiVersion": row.API, "packageDirectories": []map[string]any{{"path": "force-app", "default": true}}})
	if err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(root, "sfdx-project.json"), string(config))
	add := func(name, source string) string {
		path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
		write(path, source)
		write(path+"-meta.xml", "<ApexClass><apiVersion>"+row.API+"</apiVersion><status>Active</status></ApexClass>")
		paths = append(paths, path)
		return path
	}
	for _, name := range []string{"EnumMethodOuter", "EnumMethodLevel"} {
		add(name, f.Support[name])
	}
	if row.Case.Route != "anonymous" {
		target = add(row.Case.Name, row.Case.Source)
	}
	return paths, target
}

func AssertDiagnostics(t *testing.T, row Row, diagnostics []diagnostic.Diagnostic) {
	t.Helper()
	var errors []diagnostic.Diagnostic
	for _, d := range diagnostics {
		if d.Severity == diagnostic.Error {
			errors = append(errors, d)
		}
	}
	if row.Compiled {
		if len(errors) != 0 {
			t.Fatalf("org.tsv:%d accepted; diagnostics=%+v", row.Line, errors)
		}
		return
	}
	if len(errors) != 1 || errors[0].NativeMessage != row.Diagnostic {
		t.Fatalf("org.tsv:%d wants %q; diagnostics=%+v", row.Line, row.Diagnostic, errors)
	}
	if errors[0].Range == nil || errors[0].Range.Start.Line < 1 {
		t.Fatalf("missing diagnostic source location: %+v", errors[0])
	}
}
