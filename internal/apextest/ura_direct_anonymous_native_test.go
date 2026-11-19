package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestURADirectAnonymousNativeConformance(t *testing.T) {
	const dir = "testdata/conformance/ura_direct_anonymous"
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
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
	if len(rows) != 3 {
		t.Fatalf("native rows=%d, want 2", len(rows)-1)
	}
	source, err := os.ReadFile(filepath.Join(dir, "anonymous.apex"))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0], func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"localprobe","sourceApiVersion":%q}`, row[0]))
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			org.APIVersion = row[0]
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			_, err = runner.execute(program)
			if err == nil {
				t.Fatal("missing assertion transport")
			}
			_, payload, ok := strings.Cut(err.Error(), "URAANON|")
			if !ok {
				t.Fatalf("missing observation: %v", err)
			}
			var got, want any
			if err := json.Unmarshal([]byte(payload), &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(row[3]), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("native=%v Glade=%v", want, got)
			}
		})
	}
}
