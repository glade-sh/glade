package apextest

import (
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestDescribeTextDefaultExpressionsNative(t *testing.T) {
	const fixture = "testdata/conformance/describe_text_default_expressions"
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	index := loadTestIndex(t, filepath.Join(fixture, "project"))
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			var source strings.Builder
			fmt.Fprintf(&source, "Map<String,Object> observed=(Map<String,Object>)JSON.deserializeUntyped(TextDefaultMatrix%s.run());", strings.TrimSuffix(api, ".0"))
			count := 0
			for _, row := range rows[1:] {
				if len(row) != 4 || row[0] != api || row[1] != "anonymous" {
					continue
				}
				var value struct {
					Formula string `json:"formula"`
					Value   any    `json:"value"`
				}
				if err := json.Unmarshal([]byte(row[3]), &value); err != nil {
					t.Fatal(err)
				}
				if value.Value != nil {
					t.Fatalf("unexpected non-null captured default for %s", row[2])
				}
				escaped := strings.ReplaceAll(strings.ReplaceAll(value.Formula, `\`, `\\`), `'`, `\'`)
				fmt.Fprintf(&source, "System.assertEquals(null, ((Map<String,Object>)observed.get('%s')).get('value'), '%s');", row[2], row[2])
				fmt.Fprintf(&source, "System.assertEquals('%s', ((Map<String,Object>)observed.get('%s')).get('formula'), '%s formula');", escaped, row[2], row[2])
				count++
			}
			if count != 9 {
				t.Fatalf("expected nine anonymous native rows, got %d", count)
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
}

func TestDescribeTextDefaultRejectedExpressionNative(t *testing.T) {
	const fixture = "testdata/conformance/describe_text_default_expressions"
	source, err := os.ReadFile(filepath.Join(fixture, "reject.field-meta.xml"))
	if err != nil {
		t.Fatal(err)
	}
	var field struct {
		Default string `xml:"defaultValue"`
	}
	if err := xml.Unmarshal(source, &field); err != nil {
		t.Fatal(err)
	}
	if message := dml.ValidateFormulaInstance(field.Default, nil, storage.ObjectDefinition{}, "STRING", false); message == "" {
		t.Fatal("native rejected the incompatible expression; local validation accepted it")
	}
}
