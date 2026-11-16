package apextest

import (
	"encoding/csv"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	diagnosticpkg "github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
)

// Native rows for Apex release-note gates between API 62 and 67. Each probe
// class returns one line per case and is saved at the API version the row
// names, so a version gate shows through the class. The @IsTest wrapper fails
// with the lines as its message, which is what the istest rows record.
// compile.tsv holds the deploy outcome of classes the platform accepts or
// rejects at compile time.
//
// parkedReleaseGateRows lists rows whose native answer Glade does not match
// yet. Each names the missing evidence or the reason the row stays out.
var parkedReleaseGateRows = map[string]string{
	"JxProbe62/JX11b": "a custom exception built by JSON.deserialize below API 63 reports a null message; Glade reports its default text",
	"RpProbe63/RP10":  "an explicit null put on a master-detail field is dropped before the DML layer, so the update never sees it",
	"RpProbe67/RP10":  "same as RP10 at API 63",
	"SjProbe62/SJ9":   "EUC-JP is a valid native spelling; Glade has no EUC-JP codec",
	"SjProbe67/SJ9":   "same as SJ9 at API 62",
	"OlProbe66/OL4":   "the native map holds 75 entries in a scratch Developer org; Glade lists the 17 captured for another org shape",
	"OlProbe67/OL4":   "same as OL4 at API 66",
	// Rows of the follow-up run that Glade does not match yet.
	"PjIdProbe62/PI3":    "a pause by a string that is not an Id reports Invalid ID natively; Glade reports a missing job",
	"PjIdProbe62/PI4":    "same as PI3 for resume",
	"PjIdProbe67/PI3":    "same as PI3 at API 62",
	"PjIdProbe67/PI4":    "same as PI4 at API 62",
	"SjAliasProbe62/SA2": "windows-932 is a native spelling of Windows-31J; Glade does not know the name",
	"SjAliasProbe62/SA3": "MS_Kanji is a native spelling of Shift-JIS; Glade does not know the name",
	"SjAliasProbe62/SA4": "cp932 is a third native charset; Glade has no codec for it",
	"SjAliasProbe62/SA5": "Windows-31J encodes the private-use area to the user-defined rows; Glade decodes them but encodes a question mark",
	"SjAliasProbe67/SA2": "same as SA2 at API 62",
	"SjAliasProbe67/SA3": "same as SA3 at API 62",
	"SjAliasProbe67/SA4": "same as SA4 at API 62",
	"SjAliasProbe67/SA5": "same as SA5 at API 62",
}

// parkedReleaseGateCompileRows lists compile rows Glade does not reject yet.
var parkedReleaseGateCompileRows = map[string]string{
	"NsDeep62": "Glade enforces the four-level child subquery limit when the query runs, not when a class compiles",
	"NsDeep67": "same as NsDeep62",
}

func TestAPEXReleaseGatesNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "apex_release_gates")
	rows := readReleaseGateTSV(t, filepath.Join(dir, "org.tsv"), "case,api,route,outcome")
	want := map[string]map[string][]string{}
	var tests []string
	for _, row := range rows {
		if row[2] != "istest" {
			continue
		}
		class, caseID, ok := strings.Cut(row[0], "/")
		if !ok {
			t.Fatalf("invalid case %q", row[0])
		}
		if want[class] == nil {
			want[class] = map[string][]string{}
			tests = append(tests, class+"T")
		}
		// A probe may report a case id more than once; rows keep their order.
		want[class][caseID] = append(want[class][caseID], normalizeReleaseGateLine(row[3]))
	}
	if len(tests) < 40 {
		t.Fatalf("incomplete native matrix: %d classes", len(tests))
	}
	sort.Strings(tests)
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
	index := loadTestIndex(t, root)
	if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native sources compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	result := Run(index, Options{SelectedClasses: tests, NoDiskCache: true, Parallelism: 1})
	observed := map[string]string{}
	for _, suite := range result.Suites {
		for _, c := range suite.Cases {
			if c.Problem == nil {
				t.Fatalf("%s.%s passed; the probe must fail with its lines", c.ClassName, c.MethodName)
			}
			observed[strings.TrimSuffix(c.ClassName, "T")] = c.Problem.Message
		}
	}
	for _, class := range tests {
		class = strings.TrimSuffix(class, "T")
		message, ok := observed[class]
		if !ok {
			t.Errorf("%s did not run", class)
			continue
		}
		actual := map[string][]string{}
		for _, part := range strings.Split(strings.TrimPrefix(message, "Assertion Failed: RESULT "), " ## ") {
			caseID, _, _ := strings.Cut(part, " ")
			actual[caseID] = append(actual[caseID], normalizeReleaseGateLine(part))
		}
		var ids []string
		for caseID := range want[class] {
			ids = append(ids, caseID)
		}
		sort.Strings(ids)
		for _, caseID := range ids {
			key := class + "/" + caseID
			t.Run(key, func(t *testing.T) {
				if reason, parked := parkedReleaseGateRows[key]; parked {
					t.Skip(reason)
				}
				if got, native := strings.Join(actual[caseID], "\n"), strings.Join(want[class][caseID], "\n"); got != native {
					t.Fatalf("native: %s\nGlade:  %s", native, got)
				}
			})
		}
	}
}

// Classes the platform rejects at compile time must fail in Glade's checks,
// and classes it accepts must pass them.
func TestAPEXReleaseGatesCompileConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "apex_release_gates")
	rows := readReleaseGateTSV(t, filepath.Join(dir, "compile.tsv"), "class,api,outcome")
	if len(rows) < 30 {
		t.Fatalf("incomplete native compile matrix: %d rows", len(rows))
	}
	for _, row := range rows {
		class, outcome := row[0], row[2]
		t.Run(class, func(t *testing.T) {
			if reason, parked := parkedReleaseGateCompileRows[class]; parked {
				t.Skip(reason)
			}
			root := t.TempDir()
			project := filepath.Join(dir, "project")
			err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				name, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, name), string(body))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if outcome != "OK" {
				for _, suffix := range []string{".cls", ".cls-meta.xml"} {
					body, err := os.ReadFile(filepath.Join(dir, "reject", class+suffix))
					if err != nil {
						t.Fatal(err)
					}
					writeFile(t, filepath.Join(root, "force-app", "main", "default", "classes", class+suffix), string(body))
				}
			}
			index := loadTestIndex(t, root)
			rejected := false
			for _, diagnostic := range index.Diagnostics {
				if strings.Contains(diagnostic.File, class+".cls") {
					rejected = true
				}
			}
			for _, diagnostic := range sema.Analyze(index).Diagnostics {
				if strings.Contains(diagnostic.File, class+".cls") && diagnostic.Severity == diagnosticpkg.Error {
					rejected = true
				}
			}
			if rejected != (outcome != "OK") {
				t.Fatalf("native %s; Glade rejected=%t", outcome, rejected)
			}
		})
	}
}

func readReleaseGateTSV(t *testing.T, path, header string) [][]string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) < 2 || strings.Join(rows[0], ",") != header {
		t.Fatalf("incomplete native table %s: rows=%d err=%v", path, len(rows), err)
	}
	return rows[1:]
}

func normalizeReleaseGateLine(line string) string {
	return strings.TrimSpace(strings.Join(strings.Fields(line), " "))
}
