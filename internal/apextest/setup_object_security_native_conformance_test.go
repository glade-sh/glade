package apextest

import (
	"encoding/csv"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// SECURITY-RUN.txt records native rejection of this legacy clause at API 67.
// Replay the exact API 62 source, including runAs and both permission outcomes.
func TestSetupObjectSecurityEnforcedAPI62NativeProjectRoute(t *testing.T) {
	dir := "testdata/conformance/setup_object_read"
	file, err := os.Open(filepath.Join(dir, "security-enforced-api62.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.LazyQuotes = '\t', true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 2 || strings.Join(rows[0], ",") != "source_api,route,observation" {
		t.Fatalf("native security matrix: %v rows=%v", err, rows)
	}
	row := rows[1]
	if row[0] != "62.0" || row[1] != "isTest" {
		t.Fatalf("unexpected native route: %v", row[:2])
	}
	t.Run(row[0]+"/"+row[1]+"/withSharing/SECURITY_ENFORCED", func(t *testing.T) {
		root := t.TempDir()
		project := filepath.Join(dir, "project")
		err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			name, err := filepath.Rel(project, path)
			if err != nil {
				return err
			}
			writeFile(t, filepath.Join(root, name), string(body))
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		source, err := os.ReadFile(filepath.Join(dir, "security-enforced-api62.cls.txt"))
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/MeadowSetupProbe.cls"), string(source))
		// The legacy observation preceded enabling Communities in the owned org.
		writeFile(t, filepath.Join(root, "glade.yml"), "org:\n  features: []\n")
		index := loadTestIndex(t, root)
		analysis := sema.Analyze(index)
		if index.HasErrors() || analysis.HasErrors() {
			t.Fatalf("native source compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
		}
		run := Run(index, Options{SelectedClasses: []string{"MeadowSetupProbeTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
		if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
			t.Fatalf("expected one observation: %+v", run.Summary())
		}
		result := run.Suites[0].Cases[0]
		if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
			t.Fatalf("missing assertion transport: %+v", result.Problem)
		}
		_, observed, found := strings.Cut(result.Problem.Message, "OBS|")
		var got, want map[string]any
		if !found || json.Unmarshal([]byte(observed), &got) != nil || json.Unmarshal([]byte(row[2]), &want) != nil {
			t.Fatalf("invalid observation: %q", observed)
		}
		for _, key := range []string{"Minimum Access - Salesforce.roles.security", "Minimum Access - Salesforce.roles.user"} {
			// Compare denial only; exception identity is a separate compatibility axis.
			for _, values := range []map[string]any{got, want} {
				value := values[key]
				if value != "System.QueryException" && value != "System.SecurityException" {
					t.Fatalf("%s: expected access denial, got %v", key, value)
				}
				values[key] = "denied"
			}
		}
		for key, expected := range want {
			if !reflect.DeepEqual(got[key], expected) {
				t.Errorf("%s: native=%v; Glade=%v", key, expected, got[key])
			}
		}
		if len(got) != len(want) {
			t.Errorf("observation keys: native=%d; Glade=%d", len(want), len(got))
		}
	})
}
