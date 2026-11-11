package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestDescribeDefaultValueMatrixNative(t *testing.T) {
	const fixture = "testdata/conformance/describe_default_value"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	r.LazyQuotes = true
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	index := loadTestIndex(t, filepath.Join(fixture, "project"))
	count := 0
	for _, row := range rows[1:] {
		if len(row) != 4 || row[1] != "anonymous" || row[2] != "default metadata matrix" {
			continue
		}
		count++
		api := row[0]
		var values map[string]any
		if err := json.Unmarshal([]byte(row[3]), &values); err != nil {
			t.Fatal(err)
		}
		keys := make([]string, 0, len(values))
		for key := range values {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		t.Run(api, func(t *testing.T) {
			var source strings.Builder
			fmt.Fprintf(&source, "Map<String,Object> observed=(Map<String,Object>)JSON.deserializeUntyped(DefaultMatrix%s.run());", strings.TrimSuffix(api, ".0"))
			for _, key := range keys {
				expected := "null"
				if value, ok := values[key].(bool); ok {
					expected = fmt.Sprint(value)
				}
				fmt.Fprintf(&source, "System.assertEquals(%s, observed.get('%s'), '%s');", expected, key, key)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			program, err := vm.CompileAnonymousWithOptions(source.String(), vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := runner.newMachine().Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
	if count != 2 {
		t.Fatalf("expected two native rows, got %d", count)
	}
}
