package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestValidationRuleFormulaNativeConformanceAnonymousAPIRoutes(t *testing.T) {
	const fixture = "testdata/conformance/validation_rule_formula_semantics"
	read := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	run := read("RUN-FACTS.txt")
	probe := read("probe.apex")
	if !strings.Contains("\n"+run, "\napis=62.0,67.0\n") ||
		!strings.Contains("\n"+run, "\ncontext=anonymous\n") ||
		!strings.Contains("\n"+run, "\nrows_per_api=28\n") ||
		!strings.Contains("\n"+run, "\nnative_complete=true\n") ||
		!strings.Contains("\n"+run, "\ndeployment_success=true\n") {
		t.Fatal("native capture is incomplete")
	}
	if !strings.Contains(run, fmt.Sprintf("probe_source_sha256=%x\n", sha256.Sum256([]byte(probe)))) {
		t.Fatal("native capture is detached from the Apex probe")
	}
	sourceHashes := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSuffix(run, "\n"), "\n") {
		if !strings.HasPrefix(line, "source_sha256:") {
			continue
		}
		path, hash, ok := strings.Cut(strings.TrimPrefix(line, "source_sha256:"), "=")
		if !ok || path == "" || hash == "" || sourceHashes[path] != "" {
			t.Fatalf("invalid or repeated source hash: %q", line)
		}
		sourceHashes[path] = hash
	}
	if len(sourceHashes) == 0 {
		t.Fatal("native capture has no source hashes")
	}
	if err := filepath.WalkDir(filepath.Join(fixture, "project"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(fixture, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		want, ok := sourceHashes[relative]
		if !ok {
			return fmt.Errorf("native capture has no hash for %s", relative)
		}
		actual := fmt.Sprintf("%x", sha256.Sum256([]byte(read(relative))))
		if actual != want {
			return fmt.Errorf("native capture is detached from %s", relative)
		}
		delete(sourceHashes, relative)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(sourceHashes) != 0 {
		t.Fatalf("native capture references missing source %q", sourceHashes)
	}

	type caseRow struct{ id, shape string }
	var cases []caseRow
	caseLines := strings.Split(strings.TrimSuffix(read("cases.tsv"), "\n"), "\n")
	if len(caseLines) != 29 || caseLines[0] != "id\tshape\toperation\tinput" {
		t.Fatalf("case table has %d lines or an unexpected header", len(caseLines))
	}
	caseIDs := make(map[string]bool)
	for _, line := range caseLines[1:] {
		columns := strings.Split(line, "\t")
		if len(columns) != 4 || columns[0] == "" || columns[1] == "" || caseIDs[columns[0]] {
			t.Fatalf("invalid or repeated case row: %q", line)
		}
		caseIDs[columns[0]] = true
		cases = append(cases, caseRow{id: columns[0], shape: columns[1]})
	}

	type oracleRow struct{ status, code string }
	oracle := make(map[string]map[string]oracleRow)
	orgLines := strings.Split(strings.TrimSuffix(read("org.tsv"), "\n"), "\n")
	if len(orgLines) != 57 || orgLines[0] != "api\tid\tstatus\tcode" {
		t.Fatalf("native oracle has %d lines or an unexpected header", len(orgLines))
	}
	for _, line := range orgLines[1:] {
		columns := strings.Split(line, "\t")
		if len(columns) != 4 || !caseIDs[columns[1]] {
			t.Fatalf("invalid native oracle row: %q", line)
		}
		if oracle[columns[0]] == nil {
			oracle[columns[0]] = make(map[string]oracleRow)
		}
		if _, exists := oracle[columns[0]][columns[1]]; exists {
			t.Fatalf("duplicate native oracle row: %q", line)
		}
		oracle[columns[0]][columns[1]] = oracleRow{status: columns[2], code: columns[3]}
	}

	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			if len(oracle[api]) != len(cases) {
				t.Fatalf("native rows=%d; cases=%d", len(oracle[api]), len(cases))
			}
			index := loadTestIndex(t, filepath.Join(fixture, "project"))
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			if analysis := sema.Analyze(index); analysis.HasErrors() {
				t.Fatal(analysis.Diagnostics)
			}
			if analysis := sema.AnalyzeAnonymous(index, probe, api); analysis.HasErrors() {
				t.Fatal(analysis.Diagnostics)
			}
			program, err := vm.CompileAnonymousWithOptions(probe, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			result, err := runner.execute(program)
			if err != nil {
				t.Fatal(err)
			}
			var payload string
			for _, line := range result.Debug {
				if strings.HasPrefix(line, "VALIDATION_ROWS|") {
					if payload != "" {
						t.Fatal("duplicate native result transport")
					}
					payload = strings.TrimPrefix(line, "VALIDATION_ROWS|")
				}
			}
			if payload == "" {
				t.Fatalf("missing result transport: %v", result.Debug)
			}
			var observed map[string]struct {
				Success bool   `json:"success"`
				Code    string `json:"code"`
			}
			if err := json.Unmarshal([]byte(payload), &observed); err != nil || len(observed) != len(cases) {
				t.Fatalf("invalid result payload: %s (%v)", payload, err)
			}
			counts := newConformanceCountLog(t, "ValidationRuleFormula", api)
			for _, row := range cases {
				row := row
				counts.run(t, "anonymous", "anonymous", row.shape, func(t *testing.T) {
					want, ok := oracle[api][row.id]
					got, observedRow := observed[row.id]
					if !ok || !observedRow {
						t.Fatalf("missing oracle or Glade row %s", row.id)
					}
					wantSuccess := want.status == "success"
					if got.Success != wantSuccess || got.Code != want.code {
						t.Fatalf("%s native=%s/%s Glade=%t/%s", row.id, want.status, want.code, got.Success, got.Code)
					}
				})
			}
		})
	}
}
