package apextest

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// The observation and versions come from the anonymous native rows.
func TestDescribeDefaultValueAnonymousNative(t *testing.T) {
	const fixture = "testdata/conformance/describe_default_value"
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
	index := loadTestIndex(t, filepath.Join(fixture, "project"))
	selected := 0
	for _, row := range rows[1:] {
		if len(row) != 4 || row[1] != "anonymous" || row[2] != "null and non-null default overloads" {
			continue
		}
		selected++
		api, expected := row[0], row[3]
		t.Run(api, func(t *testing.T) {
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			source := fmt.Sprintf("System.assertEquals('%s', DefaultProbe%s.run());", expected, strings.TrimSuffix(api, ".0"))
			program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := runner.newMachine().Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
	if selected != 2 {
		t.Fatalf("expected two native rows, got %d", selected)
	}
}
