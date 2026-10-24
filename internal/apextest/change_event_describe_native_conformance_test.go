package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

const changeEventDescribeFixture = "testdata/conformance/change_event_describe"

type changeEventDescribeRow struct {
	Name     string   `json:"name"`
	Fields   []string `json:"fields"`
	Children []string `json:"children"`
}

func readChangeEventDescribeRows(t *testing.T, api string) map[string]changeEventDescribeRow {
	t.Helper()
	dir := filepath.Join(changeEventDescribeFixture, "native", "api"+strings.TrimSuffix(api, ".0"), "anonymous")
	receipt, err := os.ReadFile(filepath.Join(dir, "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"api=" + api + "\n", "context=anonymous\n", "rows=4\n", "native_complete=true\n"} {
		if !strings.Contains(string(receipt), required) {
			t.Fatalf("incomplete native capture: %s", required)
		}
	}
	captured := filepath.Join(changeEventDescribeFixture, "project", "force-app")
	if err := filepath.WalkDir(captured, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		name, err := filepath.Rel(filepath.Join(changeEventDescribeFixture, "project"), path)
		if err != nil {
			return err
		}
		if !strings.Contains(string(receipt), fmt.Sprintf("source_sha256:%s=%x\n", filepath.ToSlash(name), sha256.Sum256(body))) {
			t.Errorf("native source changed: %s", name)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 5 || lines[0] != "id\troute\tobservation" {
		t.Fatal("incomplete native observations")
	}
	rows := map[string]changeEventDescribeRow{}
	for _, line := range lines[1:] {
		cells := strings.SplitN(line, "\t", 3)
		var row changeEventDescribeRow
		if len(cells) != 3 || cells[1] != "anonymous" || json.Unmarshal([]byte(cells[2]), &row) != nil || row.Name == "" || len(row.Fields) == 0 {
			t.Fatalf("invalid native row: %.80s", line)
		}
		rows[row.Name] = row
	}
	for _, name := range []string{"AccountChangeEvent", "Parcel__ChangeEvent", "Account", "Parcel__c"} {
		if _, ok := rows[name]; !ok {
			t.Fatalf("missing native row %s", name)
		}
	}
	return rows
}

var changeEventDescribeLine = regexp.MustCompile(`^CAP (\S+) (fields|eventOrParcelChildren)\((\d+)\)=(.*)$`)

func observeChangeEventDescribe(t *testing.T, api string) map[string]changeEventDescribeRow {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(changeEventDescribeFixture, "project")
	if err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		name, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(body), "68.0", api))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	index := loadTestIndex(t, root)
	if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native project compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
	if err != nil {
		t.Fatal(err)
	}
	org := orgFromIndex(index)
	org.APIVersion = api
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
	machine := runner.newMachine()
	// Glade materializes a change event when a trigger subscribes to it.
	for _, event := range []string{"AccountChangeEvent", "Parcel__ChangeEvent"} {
		if err := machine.RegisterTrigger(vm.Trigger{Name: event + "Observer", Object: event, Timing: "after", Operation: "insert", APIVersion: api}); err != nil {
			t.Fatal(err)
		}
	}
	body, err := os.ReadFile(filepath.Join(changeEventDescribeFixture, "anonymous", "describe.apex"))
	if err != nil {
		t.Fatal(err)
	}
	program, err := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	result, err := machine.Execute(program)
	if err != nil {
		t.Fatalf("native describe completed; Glade=%v", err)
	}
	got := map[string]changeEventDescribeRow{}
	for _, line := range result.Debug {
		match := changeEventDescribeLine.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		row := got[match[1]]
		row.Name = match[1]
		items := []string{}
		for _, item := range strings.Split(match[4], ",") {
			if item != "" {
				items = append(items, item)
			}
		}
		if match[2] == "fields" {
			row.Fields = items
		} else {
			row.Children = items
		}
		got[match[1]] = row
	}
	return got
}

func changeEventDescribeSet(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}

func changeEventDescribeDiff(t *testing.T, label string, got, want map[string]bool) {
	t.Helper()
	var extra, missing []string
	for value := range got {
		if !want[value] {
			extra = append(extra, value)
		}
	}
	for value := range want {
		if !got[value] {
			missing = append(missing, value)
		}
	}
	sort.Strings(extra)
	sort.Strings(missing)
	if len(extra) != 0 || len(missing) != 0 {
		t.Errorf("%s: Glade extra=%v missing=%v", label, extra, missing)
	}
}

// Native rows: a change event carries its base describe fields, minus the
// fields the event row lacks, plus the fields only the event row has. Event
// objects have no child relationships, and a parent lists a standard event's
// lookup with a null relationship name but never a custom event's lookup.
func TestChangeEventDescribeNativeConformanceAPIRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			native := readChangeEventDescribeRows(t, api)
			glade := observeChangeEventDescribe(t, api)
			for _, name := range []string{"AccountChangeEvent", "Parcel__ChangeEvent", "Account", "Parcel__c"} {
				if len(glade[name].Fields) == 0 {
					t.Fatalf("Glade describe did not report %s", name)
				}
			}
			changeEventDescribeDiff(t, "Parcel__ChangeEvent fields", changeEventDescribeSet(glade["Parcel__ChangeEvent"].Fields), changeEventDescribeSet(native["Parcel__ChangeEvent"].Fields))

			nativeBase := changeEventDescribeSet(native["Account"].Fields)
			nativeEvent := changeEventDescribeSet(native["AccountChangeEvent"].Fields)
			want := changeEventDescribeSet(glade["Account"].Fields)
			for field := range nativeBase {
				if !nativeEvent[field] {
					delete(want, field)
				}
			}
			for field := range nativeEvent {
				if !nativeBase[field] {
					want[field] = true
				}
			}
			applyChangeEventFollowupNativeExclusions(t, api, want)
			changeEventDescribeDiff(t, "AccountChangeEvent fields (Glade Account under the native event rule)", changeEventDescribeSet(glade["AccountChangeEvent"].Fields), want)

			installed := map[string]bool{"AccountChangeEvent": true, "Parcel__ChangeEvent": true, "Parcel__c": true}
			for _, name := range []string{"AccountChangeEvent", "Parcel__ChangeEvent", "Account", "Parcel__c"} {
				restrict := func(children []string) map[string]bool {
					out := map[string]bool{}
					for _, child := range children {
						object, _, _ := strings.Cut(child, ".")
						if installed[object] {
							out[child] = true
						}
					}
					return out
				}
				changeEventDescribeDiff(t, name+" children of installed objects", restrict(glade[name].Children), restrict(native[name].Children))
			}
		})
	}
}
