package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// orgSettingsSourceFacets are the native facets Glade is held to. DML facets
// are not listed: they record the native exception for an empty insert, which
// depends on each object's required fields, not on availability.
func orgSettingsSourceFacets(observed map[string]any) []string {
	var facets []string
	for key := range observed {
		if strings.HasSuffix(key, ".describe") || strings.HasSuffix(key, ".type") || strings.HasSuffix(key, ".query") {
			facets = append(facets, key)
		}
	}
	return facets
}

// Each phase deploys cumulative settings files to a fresh Developer scratch
// shape (phases.tsv). The Glade project carries the same settings files as
// source, with no settings in the scratch definition.
func TestOrgSettingsSourceNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "org_settings_source")
	phases := map[string][2]string{}
	phaseData, err := os.ReadFile(filepath.Join(dir, "phases.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(phaseData)), "\n")[1:] {
		fields := strings.Split(line, "\t")
		for len(fields) < 3 {
			fields = append(fields, "")
		}
		phases[fields[0]] = [2]string{fields[1], fields[2]}
	}
	data, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	probe, err := os.ReadFile(filepath.Join(dir, "probe.apex"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 33 || lines[0] != "source_api\tphase\troute\tobservation" {
		t.Fatal("incomplete native settings observations")
	}
	for _, line := range lines[1:] {
		row := strings.SplitN(line, "\t", 4)
		if len(row) != 4 || (row[0] != "62.0" && row[0] != "67.0") || row[2] != "anonymous" {
			t.Fatalf("invalid row %q", line)
		}
		phase, ok := phases[row[1]]
		if !ok {
			t.Fatalf("unknown phase %s", row[1])
		}
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), strings.ReplaceAll(readTestFile(t, filepath.Join(dir, "project", "sfdx-project.json")), "62.0", row[0]))
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), readTestFile(t, filepath.Join(dir, "shapes", phase[0]+".json")))
			if err := os.MkdirAll(filepath.Join(root, "force-app/main/default/classes"), 0o755); err != nil {
				t.Fatal(err)
			}
			for _, setting := range strings.Split(phase[1], ",") {
				if setting == "" {
					continue
				}
				files, err := filepath.Glob(filepath.Join(dir, "settings", setting, "*.settings-meta.xml"))
				if err != nil || len(files) == 0 {
					t.Fatalf("missing settings %s", setting)
				}
				for _, file := range files {
					writeFile(t, filepath.Join(root, "force-app/main/default/settings", filepath.Base(file)), readTestFile(t, file))
				}
			}
			index := loadTestIndex(t, root)
			if analysis := sema.AnalyzeAnonymous(index, string(probe), row[0]); analysis.HasErrors() {
				t.Fatal(analysis.Diagnostics)
			}
			program, err := vm.CompileAnonymousWithOptions(string(probe), vm.CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			org.APIVersion = row[0]
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
			result, err := runner.execute(program)
			if err != nil {
				t.Fatal(err)
			}
			const marker = "OBSERVED|capture|"
			var payload string
			for _, message := range result.Debug {
				if strings.HasPrefix(message, marker) {
					payload = strings.TrimPrefix(message, marker)
				}
			}
			var got, want map[string]any
			if json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(row[3]), &want) != nil {
				t.Fatalf("invalid observation: %s", payload)
			}
			for _, facet := range orgSettingsSourceFacets(want) {
				if !reflect.DeepEqual(got[facet], want[facet]) {
					t.Errorf("%s: native=%v; Glade=%v", facet, want[facet], got[facet])
				}
			}
		})
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
