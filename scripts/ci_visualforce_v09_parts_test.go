package scripts

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// V09 is one top-level test per row range so the Visualforce shard planner can
// spread its rows over several shards. This pins the part list, the duration
// table entries, and that the real planner puts each control suite on its own shard
// and drops no test.
func TestCIVisualforceV09PartsAreListedAndSpreadAcrossShards(t *testing.T) {
	source, err := os.ReadFile("../internal/visualforce/v09_conformance_test.go")
	if err != nil {
		t.Fatal(err)
	}
	var parts []string
	for _, match := range regexp.MustCompile(`(?m)^func (TestV09Salesforce(?:Conformance|Controls)\w*)\(t \*testing\.T\)`).FindAllStringSubmatch(string(source), -1) {
		parts = append(parts, match[1])
	}
	var want []string
	for part := 1; part <= 4; part++ {
		want = append(want, fmt.Sprintf("TestV09SalesforceConformancePart%dof4", part))
	}
	want = append(want, "TestV09SalesforceControlsWidget", "TestV09SalesforceControlsReview", "TestV09SalesforceControlsDenied")
	if !reflect.DeepEqual(parts, want) {
		t.Fatalf("V09 parts = %v, want %v", parts, want)
	}

	data, err := os.ReadFile("../internal/visualforce/testdata/shard_durations.json")
	if err != nil {
		t.Fatal(err)
	}
	var table struct {
		Tests []struct {
			Name     string `json:"name"`
			Duration int64  `json:"durationMillis"`
		} `json:"tests"`
	}
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	var names []string
	listed := map[string]bool{}
	var events strings.Builder
	for _, test := range table.Tests {
		names = append(names, test.Name)
		listed[test.Name] = true
		fmt.Fprintf(&events, "{\"Action\":\"run\",\"Package\":%q,\"Test\":%q}\n", visualforceFixturePackage, test.Name)
		fmt.Fprintf(&events, "{\"Action\":\"pass\",\"Package\":%q,\"Test\":%q,\"Elapsed\":%g}\n", visualforceFixturePackage, test.Name, float64(test.Duration)/1000)
	}
	fmt.Fprintf(&events, "{\"Action\":\"pass\",\"Package\":%q,\"Elapsed\":1}\n", visualforceFixturePackage)
	sort.Strings(names)
	for _, name := range want {
		if !listed[name] {
			t.Fatalf("duration table does not list %s", name)
		}
	}
	if listed["TestV09SalesforceConformance"] {
		t.Fatal("duration table still lists the unsplit TestV09SalesforceConformance")
	}

	root := t.TempDir()
	planner := filepath.Join(root, "cishard")
	if out, err := exec.Command(realGoCommand(t), "build", "-o", planner, "./internal/cishard").CombinedOutput(); err != nil {
		t.Fatalf("build real planner: %v\n%s", err, out)
	}
	eventsPath, history, discovery := filepath.Join(root, "events.json"), filepath.Join(root, "history.json"), filepath.Join(root, "discovery.txt")
	if err := os.WriteFile(eventsPath, []byte(events.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(discovery, []byte(strings.Join(names, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("bash", "ci-go-test.sh", "visualforce-history-bootstrap", history, eventsPath).CombinedOutput(); err != nil {
		t.Fatalf("bootstrap history: %v\n%s", err, out)
	}
	out, err := exec.Command(planner, "--package", visualforceFixturePackage, "--shards", "4", "--tests", discovery, "--history", history).Output()
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	var plan struct {
		HistoryUsed bool `json:"historyUsed"`
		Shards      []struct {
			Tests []string `json:"tests"`
		} `json:"shards"`
	}
	if err := json.Unmarshal(out, &plan); err != nil || !plan.HistoryUsed || len(plan.Shards) != 4 {
		t.Fatalf("plan = %s, err = %v", out, err)
	}
	var union []string
	partShard := map[int]string{}
	for index, shard := range plan.Shards {
		union = append(union, shard.Tests...)
		for _, name := range shard.Tests {
			if strings.HasPrefix(name, "TestV09SalesforceControls") {
				if other, shared := partShard[index]; shared {
					t.Fatalf("shard %d holds both %s and %s", index, other, name)
				}
				partShard[index] = name
			}
		}
	}
	sort.Strings(union)
	if !reflect.DeepEqual(union, names) {
		t.Fatalf("four-shard union dropped or added tests")
	}
	if len(partShard) != 3 {
		t.Fatalf("V09 tests control suites occupy %d shards, want 3: %v", len(partShard), partShard)
	}
}
