package apextest

import (
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

type permsetMutingPartialFailure struct {
	ComponentType string `json:"componentType"`
	FullName      string `json:"fullName"`
	Problem       string `json:"problem"`
}

func readPermsetMutingPartialTSV(t *testing.T, path string) []map[string]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]string
	for _, row := range rows[1:] {
		record := map[string]string{}
		for i, name := range rows[0] {
			record[name] = row[i]
		}
		out = append(out, record)
	}
	return out
}

// permsetMutingPartialAnyObject lists the fixtures whose muting set has a rule
// violation on two objects. Native reports one error for the set, and which
// object it names changed between identical deployments (laneC1 X10b runs 2
// and 3), so any one of the two violated rules matches.
var permsetMutingPartialAnyObject = map[string][]string{
	"C10":  {"To mute Edit, first mute Delete and Modify All Records.", "To mute Delete, first mute Modify All Records."},
	"X10b": {"To mute Edit, first mute Delete and Modify All Records.", "To mute Delete, first mute Modify All Records."},
	"X10c": {"To mute Edit, first mute Delete and Modify All Records.", "To mute Delete, first mute Modify All Records."},
}

// TestPermsetMutingPartialDeployRejections replays the native component errors
// for object muting with partly muted dependencies (Dot k7a3f9e1b442 org.tsv,
// fixtures C01-C11) and the laneC1 follow-up deploys (laneC1/deploy.tsv): one
// error per muting set, the View All missing-label text, and the group errors.
func TestPermsetMutingPartialDeployRejections(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "permset_muting_partial")
	type observation struct {
		fixture, api, root string
		failures           []permsetMutingPartialFailure
	}
	var observations []observation
	for _, row := range readPermsetMutingPartialTSV(t, filepath.Join(dir, "org.tsv")) {
		var cell struct {
			Value []permsetMutingPartialFailure `json:"value"`
		}
		if err := json.Unmarshal([]byte(row["componentFailures"]), &cell); err != nil {
			t.Fatal(err)
		}
		observations = append(observations, observation{row["fixtureId"], row["apiVersion"], filepath.Join(dir, "source", row["fixtureId"]), cell.Value})
	}
	for _, row := range readPermsetMutingPartialTSV(t, filepath.Join(dir, "laneC1", "deploy.tsv")) {
		var failures []permsetMutingPartialFailure
		if err := json.Unmarshal([]byte(row["componentFailures"]), &failures); err != nil {
			t.Fatal(err)
		}
		observations = append(observations, observation{row["fixtureId"] + "/run" + row["run"], row["apiVersion"], filepath.Join(dir, "laneC1", "source", row["fixtureId"]), failures})
	}
	if len(observations) != 36 {
		t.Fatalf("native rows=%d, want 22 Dot and 14 laneC1", len(observations))
	}
	for _, obs := range observations {
		t.Run(obs.fixture, func(t *testing.T) {
			var want []string
			for _, failure := range obs.failures {
				want = append(want, failure.Problem)
			}
			var got []string
			for _, d := range loadTestIndex(t, obs.root).Diagnostics {
				if d.Code == "GLADEPERMMUTE001" {
					got = append(got, d.Message)
				}
			}
			shape, _, _ := strings.Cut(obs.fixture, "_")
			if allowed, ok := permsetMutingPartialAnyObject[shape]; ok && len(got) == len(want) && len(got) > 0 {
				matched := false
				for _, message := range allowed {
					matched = matched || got[0] == message
				}
				if !matched || !reflect.DeepEqual(got[1:], want[1:]) {
					t.Fatalf("native=%q (first error one of %q); Glade=%q", want, allowed, got)
				}
				return
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%q; Glade=%q", want, got)
			}
		})
	}
}

var permsetMutingPartialPayload = regexp.MustCompile(`MP_CAPTURE_V1:([A-Za-z0-9+/=]+)`)

// TestPermsetMutingPartialAcceptedDescribe runs the native probe of the
// accepted fixtures 8 (Edit, Delete and Modify All muted) and 9 (Create
// muted) as a Standard User holding only the group, and compares
// isUpdateable and isDeletable with runtime.tsv.
func TestPermsetMutingPartialAcceptedDescribe(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "permset_muting_partial")
	rows := readPermsetMutingPartialTSV(t, filepath.Join(dir, "runtime.tsv"))
	if len(rows) != 4 {
		t.Fatalf("runtime rows=%d, want 4", len(rows))
	}
	for _, row := range rows {
		t.Run(row["fixtureId"], func(t *testing.T) {
			index := loadTestIndex(t, filepath.Join(dir, "source", row["fixtureId"]))
			if index.HasErrors() {
				t.Fatalf("native accepted this metadata; Glade: %v", index.Diagnostics)
			}
			run := Run(index, Options{SelectedClasses: []string{"MP" + row["fixtureId"] + "Probe"}, NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("expected one assertion transport: %+v", run.Summary())
			}
			match := permsetMutingPartialPayload.FindStringSubmatch(run.Suites[0].Cases[0].Problem.Message)
			if match == nil {
				t.Fatalf("no payload: %s", run.Suites[0].Cases[0].Problem.Message)
			}
			raw, err := base64.StdEncoding.DecodeString(match[1])
			if err != nil {
				t.Fatal(err)
			}
			var payload map[string]any
			if err := json.Unmarshal(raw, &payload); err != nil {
				t.Fatal(err)
			}
			for _, field := range []string{"observationComplete", "isUpdateable", "isDeletable"} {
				got, _ := json.Marshal(payload[field])
				if string(got) != row[field] {
					t.Fatalf("%s: native=%s Glade=%s (%v)", field, row[field], got, payload["exceptionMessage"])
				}
			}
		})
	}
}
