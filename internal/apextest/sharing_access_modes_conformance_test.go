package apextest

import (
	"encoding/csv"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Native sharing_access_* rows: a Standard User (or Minimum Access user) in
// System.runAs runs explicit USER_MODE DML on readable objects without the
// operation's grant, DML on records hidden by sharing, async callbacks with
// each sharing keyword, runAs governor counting, object-denial query shapes,
// stripInaccessible, mode conflicts and the user-mode DML exception type by
// source API. Each org.tsv row is one @IsTest method at one class API; rows
// that Glade does not yet match are listed in sharingAccessCarried with their
// reason and are not asserted.
func TestSharingAccessModesNativeConformance(t *testing.T) {
	runSharingAccessFamily(t, "sharing_access_modes", "SamProbe", 588)
}

func TestSharingAccessGateNativeConformance(t *testing.T) {
	runSharingAccessFamily(t, "sharing_access_gate", "SamGate", 125)
}

func TestSharingAccessExtraNativeConformance(t *testing.T) {
	runSharingAccessFamily(t, "sharing_access_extra", "SamExtra", 32)
}

func TestSharingAccessFollowNativeConformance(t *testing.T) {
	runSharingAccessFamily(t, "sharing_access_follow", "SamFollow", 108)
}

// sharingAccessCarried maps family/api/method (api "*" for every API) to the
// reason a native row is not asserted.
var sharingAccessCarried = map[string]string{
	"sharing_access_follow/*/f_undelete_sh_user": "undelete of a record deleted by another user: recycle-bin access is not modeled (native INSUFFICIENT_ACCESS_OR_READONLY)",
}

func sharingAccessCarry(family, api, method string) (string, bool) {
	for _, key := range []string{family + "/" + api + "/" + method, family + "/*/" + method} {
		if reason, ok := sharingAccessCarried[key]; ok {
			return reason, true
		}
	}
	return "", false
}

func runSharingAccessFamily(t *testing.T, name, prefix string, count int) {
	t.Helper()
	family := filepath.Join("testdata/conformance", name)
	raw, err := os.Open(filepath.Join(family, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	reader := csv.NewReader(raw)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	reader.FieldsPerRecord = 3
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1+count || strings.Join(rows[0], ",") != "api,method,observation" {
		t.Fatalf("incomplete native rows: %d", len(rows))
	}
	root := t.TempDir()
	project := filepath.Join(family, "project")
	err = filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(project, path)
		if err != nil {
			return err
		}
		writeFile(t, filepath.Join(root, rel), string(body))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	index := loadTestIndex(t, root)
	for i, row := range rows[1:] {
		api, method, want := row[0], row[1], row[2]
		line := i + 2
		t.Run(api+"/"+method, func(t *testing.T) {
			if reason, ok := sharingAccessCarry(name, api, method); ok {
				t.Logf("CARRIED %s/org.tsv:%d %s: %s; native=%q", name, line, method, reason, want)
				return
			}
			var got string
			if file, ok := strings.CutPrefix(method, "anonymous:"); ok {
				source, err := os.ReadFile(filepath.Join(family, "anonymous", file+".apex"))
				if err != nil {
					t.Fatal(err)
				}
				got = "compiled"
				for _, d := range sema.AnalyzeAnonymous(index, string(source), api).Diagnostics {
					if d.Severity != "error" {
						continue
					}
					message := d.NativeMessage
					if message == "" {
						message = d.Message
					}
					position := ""
					if d.Range != nil {
						position = fmt.Sprintf("%d:%d", d.Range.Start.Line, d.Range.Start.Column)
					}
					got = "compile " + position + " " + message
					break
				}
			} else {
				class := prefix + strings.TrimSuffix(api, ".0")
				run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1, LimitMode: vm.LimitModeStrict})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
					t.Fatalf("missing runner observation %+v", run.Summary())
				}
				problem := run.Suites[0].Cases[0].Problem
				if _, after, ok := strings.Cut(problem.Message, " => "); ok {
					got = strings.TrimSpace(after)
				} else {
					message := problem.Message
					if problem.Type != "" && !strings.HasPrefix(message, problem.Type) {
						message = problem.Type + ": " + message
					}
					got = "RAW " + strings.TrimSpace(message)
				}
			}
			if got != strings.TrimSpace(want) {
				t.Fatalf("NATIVE %s %s/org.tsv:%d istest %s\nnative=%q\nglade =%q", method, name, line, api, want, got)
			}
		})
	}
}

// Shapes next to the captured sharing_access_* rows keep their earlier
// behavior: source API 54.0 user-mode field and required-field DML, API 58.0
// and 61.0 hidden-record user-mode updates and the 58.0 sharing delete text,
// explicit user-mode upserts of hidden or create-only records, COUNT() with a
// LIKE or a compound filter, a SYSTEM_MODE clause with a SYSTEM_MODE argument,
// countQuery with a SYSTEM_MODE argument, clause conflicts outside runAs,
// UPDATABLE stripping of an unreadable child and READABLE stripping outside
// runAs, Search.suggest with fixed search results, a Batchable enqueued in
// runAs and drained outside it, and an administrator's undelete. expected.tsv
// records the base results; these shapes have no native rows.
func TestSharingAccessUncapturedNeighborsKeepBase(t *testing.T) {
	neighbors := "testdata/sharing_access_neighbors"
	raw, err := os.Open(filepath.Join(neighbors, "expected.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	reader := csv.NewReader(raw)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	reader.FieldsPerRecord = 3
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 66 || strings.Join(rows[0], ",") != "api,method,observation" {
		t.Fatalf("incomplete neighbor rows: %d", len(rows))
	}
	root := t.TempDir()
	for _, source := range []struct{ from, to string }{
		{"testdata/conformance/sharing_access_modes/project", ""},
		{filepath.Join(neighbors, "classes"), "force-app/main/default/classes"},
	} {
		err := filepath.WalkDir(source.from, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(source.from, path)
			if err != nil {
				return err
			}
			writeFile(t, filepath.Join(root, source.to, rel), string(body))
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	index := loadTestIndex(t, root)
	for _, row := range rows[1:] {
		api, method, want := row[0], row[1], row[2]
		t.Run(api+"/"+method, func(t *testing.T) {
			class := "SamNeighbor" + strings.TrimSuffix(api, ".0")
			run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("missing runner observation %+v", run.Summary())
			}
			_, got, ok := strings.Cut(run.Suites[0].Cases[0].Problem.Message, " => ")
			if !ok || strings.TrimSpace(got) != want {
				t.Fatalf("base=%q\nglade=%q", want, run.Suites[0].Cases[0].Problem.Message)
			}
		})
	}
}

// Native permset_in_test rows: a permission set that a test assigns to the
// running user does not apply outside System.runAs. It applies inside a runAs
// block entered after the assignment and is gone when the block ends, whether
// the assignment came from the test method, a runAs block, @TestSetup or
// Test.startTest. A block that makes the assignment does not see it itself.
// Another user's assignment applies in that user's runAs block.
func TestPermissionSetAssignedInTestNativeConformance(t *testing.T) {
	family := filepath.Join("testdata/conformance", "permset_in_test")
	raw, err := os.Open(filepath.Join(family, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	reader := csv.NewReader(raw)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	reader.FieldsPerRecord = 3
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1+108 || strings.Join(rows[0], ",") != "api,method,observation" {
		t.Fatalf("incomplete native rows: %d", len(rows))
	}
	root := t.TempDir()
	project := filepath.Join(family, "project")
	err = filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(project, path)
		if err != nil {
			return err
		}
		writeFile(t, filepath.Join(root, rel), string(body))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	index := loadTestIndex(t, root)
	for i, row := range rows[1:] {
		api, method, want := row[0], row[1], row[2]
		line := i + 2
		t.Run(api+"/"+method, func(t *testing.T) {
			prefix, name, _ := strings.Cut(method, ".")
			class := prefix + strings.TrimSuffix(api, ".0")
			run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: name, NoDiskCache: true, Parallelism: 1, LimitMode: vm.LimitModeStrict})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("missing runner observation %+v", run.Summary())
			}
			_, got, _ := strings.Cut(run.Suites[0].Cases[0].Problem.Message, " => ")
			if strings.TrimSpace(got) != strings.TrimSpace(want) {
				t.Fatalf("NATIVE %s permset_in_test/org.tsv:%d istest %s\nnative=%q\nglade =%q", method, line, api, want, strings.TrimSpace(got))
			}
		})
	}
}
