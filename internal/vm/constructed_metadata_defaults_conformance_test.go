package vm_test

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestConstructedMetadataDefaultsNativeConformance(t *testing.T) {
	const fixture = "testdata/constructed_metadata_defaults"
	t.Cleanup(apextest.InvalidateRuntimeCaches)
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
	if len(rows) != 129 || strings.Join(rows[0], "|") != "api|context|field|access|value" {
		t.Fatalf("native matrix has %d rows; want header and 128 observations", len(rows))
	}
	root, err := filepath.Abs(fixture)
	if err != nil {
		t.Fatal(err)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	imported, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	index := typesys.Build(p, imported)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			org := apextest.OrgFromIndex(index)
			machine := vm.New(nil)
			machine.SetOrg(&org)
			if err := apextest.RegisterProjectRuntime(machine, index); err != nil {
				t.Fatal(err)
			}
			source, err := os.ReadFile(filepath.Join(fixture, "invoke"+strings.TrimSuffix(api, ".0")+".apex"))
			if err != nil {
				t.Fatal(err)
			}
			program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			result, err := machine.Execute(program)
			if err != nil {
				t.Fatal(err)
			}
			got := make(map[string]string)
			for _, line := range result.Debug {
				if strings.HasPrefix(line, "ROW|") {
					parts := strings.Split(line, "|")
					if len(parts) != 5 {
						t.Fatalf("invalid observation %q", line)
					}
					key := strings.Join(parts[1:4], "/")
					if _, exists := got[key]; exists {
						t.Fatalf("duplicate observation %s", key)
					}
					got[key] = parts[4]
				}
			}
			checked := 0
			for _, row := range rows[1:] {
				if row[0] != api {
					continue
				}
				key := strings.Join(row[1:4], "/")
				t.Run(key, func(t *testing.T) {
					if value, exists := got[key]; !exists || value != row[4] {
						t.Errorf("native=%s; Glade=%s (present=%t)", row[4], value, exists)
					}
				})
				checked++
			}
			if checked != 64 || len(got) != checked {
				t.Fatalf("native=%d Glade=%d observations; want 64", checked, len(got))
			}
		})
	}
}
