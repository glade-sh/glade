package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

const whatIDTiebreakFixture = "testdata/conformance/whatid_targets_tiebreak"

// whatIDTiebreakRows reads org.tsv as org -> api|route|probe -> observation.
func whatIDTiebreakRows(t *testing.T) map[string]map[string]string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(whatIDTiebreakFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) != 73 || lines[0] != "org\tapi\troute\tprobe\tobservation" {
		t.Fatalf("incomplete WhatId tiebreak rows: %d", len(lines))
	}
	rows := map[string]map[string]string{}
	for _, line := range lines[1:] {
		cells := strings.SplitN(line, "\t", 5)
		if len(cells) != 5 {
			t.Fatalf("invalid row %q", line)
		}
		if rows[cells[0]] == nil {
			rows[cells[0]] = map[string]string{}
		}
		rows[cells[0]][cells[1]+"|"+cells[2]+"|"+cells[3]] = cells[4]
	}
	return rows
}

// whatIDTiebreakObservation runs the captured probe body and returns its JSON.
func whatIDTiebreakObservation(t *testing.T, api, route string) map[string]any {
	t.Helper()
	anon, err := os.ReadFile(filepath.Join(whatIDTiebreakFixture, "project", "anon.apex"))
	if err != nil {
		t.Fatal(err)
	}
	major := strings.TrimSuffix(api, ".0")
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
	writeFile(t, filepath.Join(root, "config", "project-scratch-def.json"), `{"orgName": "laneC3 plain probe", "edition": "Developer"}`)
	classes := filepath.Join(root, "force-app", "main", "default", "classes")
	if route == "isTest" {
		for _, suffix := range []string{".cls", ".cls-meta.xml"} {
			body, err := os.ReadFile(filepath.Join(whatIDTiebreakFixture, "project", "classes", "WhatIdTie"+major+suffix))
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(classes, "WhatIdTie"+major+suffix), string(body))
		}
	}
	index := loadTestIndex(t, root)
	if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("project compilation: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	var observed string
	if route == "isTest" {
		run := Run(index, Options{SelectedClasses: []string{"WhatIdTie" + major}, SelectedMethod: "probe", NoDiskCache: true, Parallelism: 1})
		if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
			t.Fatalf("expected the probe observation: %+v", run.Summary())
		}
		_, body, ok := strings.Cut(run.Suites[0].Cases[0].Problem.Message, "CAPTURE|")
		if !ok {
			t.Fatalf("missing observation: %s", run.Suites[0].Cases[0].Problem.Message)
		}
		observed = body
	} else {
		org := capturedFixtureOrg(t, index, api)
		runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
		program, err := vm.CompileAnonymousWithOptions(string(anon), vm.CompileOptions{APIVersion: api})
		if err != nil {
			t.Fatal(err)
		}
		result, err := runner.execute(program)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range result.Debug {
			if _, body, ok := strings.Cut(line, "CAPTURE|"); ok {
				observed = body
			}
		}
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(observed), &got); err != nil {
		t.Fatalf("observation %q: %v", observed, err)
	}
	return got
}

// Native rows: whatid_targets_tiebreak org.tsv (two fresh plain Developer
// orgs, 2026-11-07). Both orgs agree at every API and route; earlier captures
// that disagree are recorded as drift in ORG-RUN.txt. Glade follows these rows
// at the captured endpoints 62.0 and 67.0 (Task and Event WhatId target lists
// and the global describe membership of the eight disputed objects), and on
// the anonymous route at 63.0 to 66.0, where the eight objects join one by one.
func TestWhatIDTargetsTiebreakNativeConformance(t *testing.T) {
	rows := whatIDTiebreakRows(t)
	for _, key := range []string{"62.0|anonymous", "62.0|isTest", "67.0|anonymous", "67.0|isTest"} {
		for _, probe := range []string{"task", "event", "global8"} {
			if rows["A"][key+"|"+probe] != rows["B"][key+"|"+probe] {
				t.Fatalf("orgs A and B disagree on %s %s", key, probe)
			}
		}
	}
	for _, api := range []string{"62.0", "63.0", "64.0", "65.0", "66.0", "67.0"} {
		for _, route := range []string{"anonymous", "isTest"} {
			if rows["A"][api+"|"+route+"|task"] == "" {
				continue // isTest was captured at 62.0 and 67.0 only
			}
			if rows["A"][api+"|"+route+"|task"] != rows["B"][api+"|"+route+"|task"] || rows["A"][api+"|"+route+"|global8"] != rows["B"][api+"|"+route+"|global8"] {
				t.Fatalf("orgs A and B disagree on %s %s", api, route)
			}
			t.Run(api+"/"+route, func(t *testing.T) {
				got := whatIDTiebreakObservation(t, api, route)
				for _, probe := range []string{"task", "event", "global8"} {
					var want any
					if err := json.Unmarshal([]byte(rows["A"][api+"|"+route+"|"+probe]), &want); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(want, got[probe]) {
						t.Errorf("%s: native %v; Glade %v", probe, want, got[probe])
					}
				}
			})
		}
	}
}

// whatIDTargetsDriftObjects are the eight targets the earlier orgs disagree on.
var whatIDTargetsDriftObjects = []string{"DevopsActivityLog", "DevopsEnvDeployment", "DevopsEnvDeploymentMbr", "DevopsEnvironment", "DevopsRequestInfo", "PayGroup", "PayPeriod", "SfdcPartnerSbscrOffer"}

// The earlier captures that disagree with the tiebreak (record_type_shape org
// A at 67.0: 85 targets; plain_global_describe at 62.0: 93 targets) differ from
// it by exactly the eight drift objects. The rows stay as captured; Glade
// follows the tiebreak (whatid_targets_tiebreak ORG-RUN.txt).
func TestWhatIDTargetsDriftRows(t *testing.T) {
	majority := map[string][]string{}
	for _, api := range []string{"62.0", "67.0"} {
		var list []string
		if err := json.Unmarshal([]byte(whatIDTiebreakRows(t)["A"][api+"|anonymous|task"]), &list); err != nil {
			t.Fatal(err)
		}
		majority[api] = list
	}
	differ := func(t *testing.T, name string, got, want []string) []string {
		t.Helper()
		in := map[string]int{}
		for _, item := range got {
			in[item]++
		}
		for _, item := range want {
			in[item]--
		}
		var diff []string
		for item, count := range in {
			if count != 0 {
				diff = append(diff, item)
			}
		}
		sort.Strings(diff)
		return diff
	}
	drift := strings.Join(whatIDTargetsDriftObjects, ",")
	body, err := os.ReadFile(filepath.Join(plainGlobalDescribeFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	plain := 0
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n")[1:] {
		cells := strings.Split(line, "\t")
		if cells[3] != "Task.WhatId" && cells[3] != "Event.WhatId" {
			continue
		}
		plain++
		var list []string
		if err := json.Unmarshal([]byte(cells[4]), &list); err != nil {
			t.Fatal(err)
		}
		want := ""
		if cells[0] == "62.0" {
			want = drift
		}
		if got := strings.Join(differ(t, cells[3], list, majority[cells[0]]), ","); got != want {
			t.Errorf("plain_global_describe %s %s: drift %q; want %q", cells[0], cells[3], got, want)
		}
	}
	body, err = os.ReadFile(filepath.Join("testdata", "conformance", "record_type_shape", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	shape := 0
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n")[1:] {
		cells := strings.SplitN(line, "\t", 5)
		var observation map[string]any
		if err := json.Unmarshal([]byte(cells[4]), &observation); err != nil {
			t.Fatal(err)
		}
		refs, ok := observation["whatRefs"]
		if !ok {
			continue
		}
		shape++
		var list []string
		switch value := refs.(type) {
		case string:
			list = strings.Split(value, ",")
		case []any:
			for _, item := range value {
				list = append(list, item.(string))
			}
		}
		want := ""
		if cells[0] == "67.0" {
			want = drift
		}
		if got := strings.Join(differ(t, cells[1], list, majority[cells[0]]), ","); got != want {
			t.Errorf("record_type_shape %s %s %s: drift %q; want %q", cells[0], cells[1], cells[2], got, want)
		}
	}
	if plain != 4 || shape != 8 {
		t.Fatalf("drift rows: plain %d, record type shape %d", plain, shape)
	}
}

// Native rows: whatid_targets_fresh org.tsv (2026-11-08). Three more fresh orgs
// repeat the tiebreak, and the REST describe gives the same lists as Apex. The
// Developer orgs list 85 targets at 62.0 and 93 at 67.0; Glade compares whole
// lists against them on the plain Developer shape. Enterprise lists 27 and 33,
// so the list depends on edition: TestWhatIDTargetsFreshEnterpriseRows records it.
func TestWhatIDTargetsFreshNativeConformance(t *testing.T) {
	rows := whatIDFreshRows(t)
	for _, org := range []string{"P", "R"} {
		for _, api := range []string{"62.0", "67.0"} {
			t.Run(org+"/"+api, func(t *testing.T) {
				got := whatIDTiebreakObservation(t, api, "anonymous")
				for _, probe := range []string{"task", "event"} {
					want := rows[org+"|"+api+"|anonymous|"+probe]
					if !reflect.DeepEqual(want, whatIDFreshSorted(rows[org+"|"+api+"|restDescribe|"+probe])) {
						t.Errorf("%s: native Apex and REST describe differ", probe)
					}
					if !reflect.DeepEqual(any(want), whatIDFreshStrings(got[probe])) {
						t.Errorf("%s: native %v; Glade %v", probe, want, got[probe])
					}
				}
			})
		}
	}
}

func whatIDFreshStrings(value any) any {
	items, ok := value.([]any)
	if !ok {
		return value
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		text, _ := item.(string)
		out = append(out, text)
	}
	return out
}

func whatIDFreshSorted(list []string) []string {
	out := append([]string(nil), list...)
	sort.Strings(out)
	return out
}

// whatIDFreshRows reads the target lists as org|api|route|probe -> list.
func whatIDFreshRows(t *testing.T) map[string][]string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", "conformance", "whatid_targets_fresh", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string][]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n")[1:] {
		cells := strings.SplitN(line, "\t", 5)
		if cells[3] != "task" && cells[3] != "event" {
			continue
		}
		var list []string
		if err := json.Unmarshal([]byte(cells[4]), &list); err != nil {
			t.Fatal(err)
		}
		rows[strings.Join(cells[:4], "|")] = list
	}
	if len(rows) != 24 {
		t.Fatalf("target list rows = %d; want 24", len(rows))
	}
	return rows
}

// The Enterprise twin is a strict subset of the Developer lists at both APIs.
func TestWhatIDTargetsFreshEnterpriseRows(t *testing.T) {
	rows := whatIDFreshRows(t)
	for api, count := range map[string]int{"62.0": 27, "67.0": 33} {
		enterprise := rows["E|"+api+"|anonymous|task"]
		if len(enterprise) != count {
			t.Errorf("%s: Enterprise targets = %d; want %d", api, len(enterprise), count)
		}
		developer := map[string]bool{}
		for _, name := range rows["P|"+api+"|anonymous|task"] {
			developer[name] = true
		}
		for _, name := range enterprise {
			if !developer[name] {
				t.Errorf("%s: Enterprise target %s is not a Developer target", api, name)
			}
		}
	}
}
