package apextest

import (
	"encoding/csv"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestIteratorMutationErrorNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "iterator_end_boundary")
	source, err := os.ReadFile(filepath.Join(dir, "mutation-after-remove.apex"))
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(dir, "mutation-errors.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || strings.Join(rows[0], ",") != "source_api,type,message" {
		t.Fatal("incomplete native mutation matrix")
	}
	for _, row := range rows[1:] {
		t.Run(row[0], func(t *testing.T) {
			program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			_, err = vm.Execute(program, nil)
			var failure *vm.RuntimeError
			if !errors.As(err, &failure) || failure.Type != row[1] || failure.Message != row[2] {
				t.Fatalf("native=%s: %s; Glade=%v", row[1], row[2], err)
			}
		})
	}
}
