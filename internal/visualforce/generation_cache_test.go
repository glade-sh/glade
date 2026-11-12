package visualforce

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/project"
)

func controllerClassPath(p project.Project) string {
	return filepath.Join(p.Root, "force-app/main/default/classes/ProbeController.cls")
}

type gateOutcome struct {
	Names    []string
	Rejected []PageRejection
	Strict   string
}

// gateOutcomeOf is everything a caller can observe from the controller gate:
// registered names, rejected files, the strict loader's verdict and the
// compile diagnostics.
func gateOutcomeOf(t *testing.T, p project.Project) (gateOutcome, []string) {
	t.Helper()
	names, rejected := PageNamesWithDiagnostics(p, nil)
	out := gateOutcome{Names: names, Rejected: rejected}
	if _, err := LoadProject(p); err != nil {
		out.Strict = err.Error()
	}
	diags, err := ControllerCompileDiagnostics(p)
	if err != nil {
		t.Fatal(err)
	}
	messages := make([]string, 0, len(diags))
	for _, diag := range diags {
		messages = append(messages, diag.Code+": "+diag.Message)
	}
	return out, messages
}

func TestControllerGateOutcomeEqualWithAndWithoutGenerationCache(t *testing.T) {
	for name, body := range map[string]string{"accepted": validProbeController, "rejected": rejectedProbeController} {
		t.Run(name, func(t *testing.T) {
			p := writeControllerPageProject(t, body)
			resetGenerationCache()
			cold, coldDiags := gateOutcomeOf(t, p)
			for pass := range 3 {
				warm, warmDiags := gateOutcomeOf(t, p)
				if !reflect.DeepEqual(cold, warm) || !reflect.DeepEqual(coldDiags, warmDiags) {
					t.Fatalf("pass %d differs from the cold result:\ncold %+v %v\nwarm %+v %v", pass, cold, coldDiags, warm, warmDiags)
				}
			}
			resetGenerationCache()
			again, againDiags := gateOutcomeOf(t, p)
			if !reflect.DeepEqual(cold, again) || !reflect.DeepEqual(coldDiags, againDiags) {
				t.Fatalf("recomputed result differs: %+v vs %+v", cold, again)
			}
		})
	}
}

func TestControllerGateComputedOncePerGeneration(t *testing.T) {
	for name, body := range map[string]string{"accepted": validProbeController, "rejected": rejectedProbeController} {
		t.Run(name, func(t *testing.T) {
			p := writeControllerPageProject(t, body)
			resetGenerationCache()
			compiles, parses := controllerCompiles.Load(), apexSourceParses.Load()
			for range 3 {
				PageNames(p)
				if _, err := LoadProject(p); err != nil && name == "accepted" {
					t.Fatal(err)
				}
				if _, err := ControllerCompileDiagnostics(p); err != nil {
					t.Fatal(err)
				}
			}
			if got := controllerCompiles.Load() - compiles; got != 1 {
				t.Fatalf("controller gate compiled the project %d times for 9 callers of one generation, want 1", got)
			}
			if got := apexSourceParses.Load() - parses; got > 1 {
				t.Fatalf("expression sources parsed %d times for 9 callers of one generation, want at most 1", got)
			}
		})
	}
}

func TestControllerGateSharedByConcurrentCallers(t *testing.T) {
	p := writeControllerPageProject(t, validProbeController)
	resetGenerationCache()
	compiles := controllerCompiles.Load()
	var wg sync.WaitGroup
	results := make([][]string, 8)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = PageNames(p)
		}()
	}
	wg.Wait()
	if got := controllerCompiles.Load() - compiles; got != 1 {
		t.Fatalf("%d concurrent callers compiled %d times, want 1", len(results), got)
	}
	for i, got := range results {
		if !reflect.DeepEqual(got, results[0]) || len(got) != 2 {
			t.Fatalf("caller %d got %v, caller 0 got %v", i, got, results[0])
		}
	}
}

func TestControllerGateNeverServesStaleGeneration(t *testing.T) {
	cases := map[string]struct {
		edit func(t *testing.T, p project.Project)
	}{
		"controller becomes rejected": {func(t *testing.T, p project.Project) {
			writeFile(t, controllerClassPath(p), rejectedProbeController)
		}},
		"same size edit with restored mtime": {func(t *testing.T, p project.Project) {
			path := controllerClassPath(p)
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			edited := []byte(rejectedProbeController)
			// Pad to the original size so only the content differs.
			for len(edited) < int(info.Size()) {
				edited = append(edited, ' ')
			}
			if len(edited) != int(info.Size()) {
				t.Skip("rejected body is longer than the accepted one")
			}
			if err := os.WriteFile(path, edited, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
				t.Fatal(err)
			}
		}},
		"class deleted from disk": {func(t *testing.T, p project.Project) {
			if err := os.Remove(controllerClassPath(p)); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := writeControllerPageProject(t, validProbeController)
			resetGenerationCache()
			before, _ := gateOutcomeOf(t, p)
			if len(before.Names) != 2 {
				t.Fatalf("setup: accepted project registered %v", before.Names)
			}
			time.Sleep(20 * time.Millisecond)
			tc.edit(t, p)
			after, _ := gateOutcomeOf(t, p)
			resetGenerationCache()
			fresh, _ := gateOutcomeOf(t, p)
			if !reflect.DeepEqual(after, fresh) {
				t.Fatalf("a cached generation was served after the edit:\ncached %+v\nfresh  %+v", after, fresh)
			}
			if reflect.DeepEqual(after.Names, before.Names) && name != "same size edit with restored mtime" {
				t.Fatalf("edit did not change the registered pages: %v", after.Names)
			}
		})
	}
}

func TestControllerGateSeesSchemaEditBehindUnchangedFileList(t *testing.T) {
	const object = `<?xml version="1.0" encoding="UTF-8"?>
<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Probe</label><pluralLabel>Probes</pluralLabel><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel><nameField><label>Name</label><type>Text</type></nameField>
<fields><fullName>%s__c</fullName><label>F</label><type>Text</type><length>20</length></fields></CustomObject>`
	controller := `public with sharing class ProbeController {
  public String value { get; set; }
  public ProbeController() { for (Probe__c r : [SELECT Id, Extra__c FROM Probe__c LIMIT 1]) { value = r.Extra__c; } }
}
`
	p := writeControllerPageProject(t, controller)
	objectPath := filepath.Join(p.Root, "force-app/main/default/objects/Probe__c/Probe__c.object-meta.xml")
	writeFile(t, objectPath, fmt.Sprintf(object, "Other"))
	p, err := project.Load(p.Root)
	if err != nil {
		t.Fatal(err)
	}
	resetGenerationCache()
	before, _ := gateOutcomeOf(t, p)
	if !reflect.DeepEqual(before.Names, []string{"Plain"}) {
		t.Fatalf("query on a missing field should reject the controller page, got %v", before.Names)
	}
	info, _ := os.Stat(objectPath)
	time.Sleep(20 * time.Millisecond)
	writeFile(t, objectPath, fmt.Sprintf(object, "Extra"))
	if err := os.Chtimes(objectPath, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	after, _ := gateOutcomeOf(t, p)
	if len(after.Names) != 2 {
		t.Fatalf("object edit not seen by the gate (stale generation): %v", after.Names)
	}
}

func TestControllerGateDiagnosticsAreCopiedToCallers(t *testing.T) {
	p := writeControllerPageProject(t, rejectedProbeController)
	resetGenerationCache()
	first, err := ControllerCompileDiagnostics(p)
	if err != nil || len(first) == 0 {
		t.Fatalf("diagnostics = %v, %v", first, err)
	}
	want := first[0]
	first[0].Message = "mutated by the caller"
	second, err := ControllerCompileDiagnostics(p)
	if err != nil || len(second) == 0 || !reflect.DeepEqual(second[0], want) {
		t.Fatalf("a caller's edit reached the shared generation: %+v, want %+v", second, want)
	}
}

func TestGenerationKeyCoversNestedProjectMarkers(t *testing.T) {
	p := writeControllerPageProject(t, validProbeController)
	before, ok := projectGenerationKey(p)
	if !ok {
		t.Fatal("no key")
	}
	marker := filepath.Join(p.Root, "force-app/main/default/classes/sfdx-project.json")
	writeFile(t, marker, `{"packageDirectories":[{"path":"."}]}`)
	added, _ := projectGenerationKey(p)
	if added == before {
		t.Fatal("adding a nested sfdx-project.json did not change the key")
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	if again, _ := projectGenerationKey(p); again != before {
		t.Fatal("removing the marker did not restore the key")
	}
}
