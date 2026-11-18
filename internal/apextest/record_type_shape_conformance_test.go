package apextest

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// recordTypeShapeKnownGaps are captured facets Glade does not reproduce yet.
// Each must still differ; a matching facet fails as stale.
//   - infos without profile metadata: Glade keeps every active record type
//     available; a fresh scratch org reports deployed record types unavailable
//     until a profile grants them (record_type_visibility plain and
//     account_rt rows; applyProjectProfileRecordTypeAvailability).
//   - Idea/Solution infos: catalog record type shape.
//   - whatRef*@67.0: org drift, not a Glade gap. Org A listed 85 Task/Event
//     WhatId targets at 67.0; the tiebreak orgs (whatid_targets_tiebreak) and
//     org D list 93, adding the eight objects that join the global describe by
//     67.0. Glade follows the majority (TestWhatIDTargetsTiebreakNativeConformance,
//     TestWhatIDTargetsDriftRows, TestWhatIDTargetsFreshNativeConformance). At
//     62.0 these orgs give 85 targets; the plain_global_describe 62.0 isTest row
//     gives 93 and stays the minority.
//   - list_forms refs: polymorphic target catalogs (Attachment, Note,
//     ContentDocumentLink, EntitySubscription, FeedItem, Event.WhatId) and
//     NetworkId fields the plain shape lacks; keySetSize: Account catalog.
var recordTypeShapeKnownGaps = map[string]bool{
	"list_forms/forms/keySetSize":         true,
	"list_forms/refs/Attachment.ParentId": true, "list_forms/refs/CollaborationGroup.NetworkId": true,
	"list_forms/refs/ContentDocumentLink.LinkedEntityId": true, "list_forms/refs/EntitySubscription.ParentId": true,
	"list_forms/refs/Event.WhatId": true, "list_forms/refs/FeedItem.ParentId": true,
	"list_forms/refs/Note.ParentId": true, "list_forms/refs/Topic.NetworkId": true,
	"plain/rt/infos": true, "account_rt/rt/infos": true,
	"plain/shape/std.V35Rt__c.infos": true, "account_rt/shape/std.V35Rt__c.infos": true,
	"account_rt/shape/std.Account.infos": true,
	"plain/shape/std.Idea.infos":         true, "plain/shape/std.Idea.rtField": true, "plain/shape/std.Solution.infos": true,
	"account_rt/shape/std.Idea.infos": true, "account_rt/shape/std.Idea.rtField": true, "account_rt/shape/std.Solution.infos": true,
	"profile/shape/std.Idea.infos": true, "profile/shape/std.Idea.rtField": true, "profile/shape/std.Solution.infos": true,
	"plain/shape/whatRefs@67.0": true, "plain/shape/whatRefCount@67.0": true, "plain/shape/whatRefInGlobal@67.0": true, "plain/shape/eventWhatRefCount@67.0": true,
	"account_rt/shape/whatRefs@67.0": true, "account_rt/shape/whatRefCount@67.0": true, "account_rt/shape/whatRefInGlobal@67.0": true, "account_rt/shape/eventWhatRefCount@67.0": true,
	"profile/shape/whatRefs@67.0": true, "profile/shape/whatRefCount@67.0": true, "profile/shape/whatRefInGlobal@67.0": true, "profile/shape/eventWhatRefCount@67.0": true,
}

var recordTypeShapePhases = map[string][]string{
	"plain":         nil,
	"account_rt":    {"account_rt"},
	"profile":       {"account_rt", "profile"},
	"plain_targets": nil,
	// Orgs C and D: the object_settings scratch definition, nothing deployed.
	"object_settings": nil,
	"list_forms":      nil,
}

// Native observations: testdata/conformance/record_type_shape (ORG-RUN.txt).
func TestRecordTypeShapeNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "record_type_shape")
	data, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 89 || lines[0] != "source_api\tphase\troute\tprobe\tobservation" {
		t.Fatal("incomplete native record type shape observations")
	}
	for _, line := range lines[1:] {
		row := strings.SplitN(line, "\t", 5)
		if len(row) != 5 || (row[0] != "62.0" && row[0] != "67.0") || (row[2] != "anonymous" && row[2] != "isTest") {
			t.Fatalf("invalid row %q", line)
		}
		api, phase, route, probe := row[0], row[1], row[2], row[3]
		overlays, ok := recordTypeShapePhases[phase]
		if !ok {
			t.Fatalf("unknown phase %s", phase)
		}
		t.Run(api+"/"+phase+"/"+route+"/"+probe, func(t *testing.T) {
			var want map[string]any
			if err := json.Unmarshal([]byte(row[4]), &want); err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			sources := []string{filepath.Join(dir, "project")}
			for _, overlay := range overlays {
				sources = append(sources, filepath.Join(dir, "phases", overlay))
			}
			if phase == "plain_targets" {
				sources = []string{filepath.Join(dir, "project", "config"), filepath.Join(dir, "project", "sfdx-project.json")}
			}
			if phase == "object_settings" || phase == "list_forms" {
				sources = []string{filepath.Join(dir, "phases", "object_settings"), filepath.Join(dir, "project", "sfdx-project.json")}
			}
			for _, source := range sources {
				base := filepath.Join(dir, "project")
				if strings.Contains(source, "phases") {
					base = source
				}
				if strings.HasSuffix(source, "sfdx-project.json") {
					base = filepath.Dir(source)
				}
				err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
					if err != nil || entry.IsDir() {
						return err
					}
					body, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					name, err := filepath.Rel(base, path)
					if err != nil {
						return err
					}
					text := string(body)
					// One deploy kept RtPresenceProbe at its apiVersion 62.0 for
					// both source APIs: a 67.0 class accepts the explicit null
					// RecordTypeId these rows reject (record_type_visibility).
					if !strings.HasPrefix(filepath.Base(path), "RtPresenceProbe") {
						text = strings.ReplaceAll(text, "62.0", api)
					}
					writeFile(t, filepath.Join(root, name), text)
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			if phase == "plain_targets" || phase == "object_settings" || phase == "list_forms" {
				if err := os.MkdirAll(filepath.Join(root, "force-app"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			index := loadTestIndex(t, root)
			got := map[string]any{}
			switch {
			case strings.HasPrefix(probe, "c"):
				source := readTestFile(t, filepath.Join(dir, "anon", probe+".apex"))
				got["compiled"] = !sema.AnalyzeAnonymous(index, source, api).HasErrors()
				delete(want, "problem")
			case route == "isTest":
				class := map[string]string{"rt": "RtPresenceProbeTest", "shape": "LgShapeProbeTest"}[probe]
				run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
					t.Fatalf("expected one observation: %+v", run.Summary())
				}
				_, observed, _ := strings.Cut(run.Suites[0].Cases[0].Problem.Message, "OBS|")
				if err := json.Unmarshal([]byte(observed), &got); err != nil {
					t.Fatalf("invalid observation %q", observed)
				}
			default:
				source := readTestFile(t, filepath.Join(dir, "anon", probe+".apex"))
				if analysis := sema.AnalyzeAnonymous(index, source, api); analysis.HasErrors() {
					t.Fatal(analysis.Diagnostics)
				}
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				result, err := runner.execute(program)
				if strings.HasPrefix(probe, "assert") {
					if err == nil {
						t.Fatal("native assertion failed; Glade completed")
					}
					got["exception"] = err.Error()
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				for i, message := range result.Debug {
					if marker, ok := strings.CutPrefix(message, "MARK|"); ok && i+1 < len(result.Debug) {
						got["debug."+marker] = result.Debug[i+1]
					}
					if at := strings.Index(message, "OBS|"); at >= 0 {
						if err := json.Unmarshal([]byte(message[at+len("OBS|"):]), &got); err != nil {
							t.Fatalf("invalid observation %q", message)
						}
					}
				}
			}
			compareRecordTypeShapeFacets(t, api, phase, probe, flattenRecordTypeShape(got), flattenRecordTypeShape(want))
		})
	}
}

// flattenRecordTypeShape splits the shape probe's std map into one facet per
// object property so each gap is named.
func flattenRecordTypeShape(observed map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range observed {
		std, ok := value.(map[string]any)
		if key != "std" || !ok {
			out[key] = value
			continue
		}
		for object, facets := range std {
			if fields, ok := facets.(map[string]any); ok {
				for name, facet := range fields {
					out["std."+object+"."+name] = facet
				}
				continue
			}
			out["std."+object] = facets
		}
	}
	return out
}

func compareRecordTypeShapeFacets(t *testing.T, api, phase, probe string, got, want map[string]any) {
	t.Helper()
	keys := make([]string, 0, len(want))
	for key := range want {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		gap := recordTypeShapeKnownGaps[phase+"/"+probe+"/"+key] || recordTypeShapeKnownGaps[phase+"/"+probe+"/"+key+"@"+api]
		same := reflect.DeepEqual(got[key], want[key])
		switch {
		case gap && same:
			t.Errorf("%s: stale known gap; Glade now matches native=%v", key, want[key])
		case !gap && !same:
			t.Errorf("%s: native=%v; Glade=%v", key, want[key], got[key])
		}
	}
}
