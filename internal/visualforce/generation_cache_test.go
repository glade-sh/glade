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
	"github.com/glade-sh/glade/internal/semanticcache"
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
			// A short rejected controller (unknown local type), padded to the
			// accepted body's size so only the content differs.
			edited := []byte("public with sharing class ProbeController {\n  public void run() { Missing__Type x; }\n}\n")
			for len(edited) < int(info.Size()) {
				edited = append(edited, ' ')
			}
			if len(edited) != int(info.Size()) {
				t.Fatalf("rejected body is %d bytes, longer than the accepted %d", len(edited), info.Size())
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

// Inputs the compile reads that no project string names: each edit must give
// the same outcome as a fresh compile.
func TestControllerGateSeesOrgShapeAndMarkerEdits(t *testing.T) {
	const controller = `public with sharing class ProbeController {
  public String value { get; set; }
  public ProbeController() { ConnectApi.CdpCatalog x; }
}
`
	cases := map[string]struct {
		before, after func(t *testing.T, root string)
	}{
		"scratch definition features": {
			before: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"features":["DevelopmentWave"]}`)
			},
			after: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"features":["AnalyticsAdminPerms"]}`)
			},
		},
		"extra scratch definition": {
			before: func(t *testing.T, root string) {},
			after: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, "config/extra-scratch-def.json"), `{"features":["DevelopmentWave"]}`)
			},
		},
		"nested sfdx-project marker": {
			before: func(t *testing.T, root string) {},
			after: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/sfdx-project.json"), `{"packageDirectories":[{"path":"."}]}`)
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := writeControllerPageProject(t, controller)
			tc.before(t, p.Root)
			p, err := project.Load(p.Root)
			if err != nil {
				t.Fatal(err)
			}
			resetGenerationCache()
			first, _ := gateOutcomeOf(t, p)
			time.Sleep(20 * time.Millisecond)
			tc.after(t, p.Root)
			p, err = project.Load(p.Root)
			if err != nil {
				t.Fatal(err)
			}
			cached, cachedDiags := gateOutcomeOf(t, p)
			resetGenerationCache()
			fresh, freshDiags := gateOutcomeOf(t, p)
			if !reflect.DeepEqual(cached, fresh) || !reflect.DeepEqual(cachedDiags, freshDiags) {
				t.Fatalf("cached gate differs from a fresh compile after the edit:\ncached %+v %v\nfresh  %+v %v\n(first %+v)", cached, cachedDiags, fresh, freshDiags, first)
			}
		})
	}
	// The scratch-definition case must actually flip the outcome, or it proves nothing.
	p := writeControllerPageProject(t, controller)
	writeFile(t, filepath.Join(p.Root, "config/project-scratch-def.json"), `{"features":["DevelopmentWave"]}`)
	p, _ = project.Load(p.Root)
	resetGenerationCache()
	_, a := gateOutcomeOf(t, p)
	writeFile(t, filepath.Join(p.Root, "config/project-scratch-def.json"), `{"features":["AnalyticsAdminPerms"]}`)
	p, _ = project.Load(p.Root)
	resetGenerationCache()
	_, b := gateOutcomeOf(t, p)
	if reflect.DeepEqual(a, b) {
		t.Fatalf("setup: feature edit does not change diagnostics (%v)", a)
	}
}

func enablePersistedGateForTest(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "gate")
	t.Cleanup(semanticcache.SetPersistedGateDirForTesting(dir))
	SetPersistentGateCache(true)
	t.Cleanup(func() { SetPersistentGateCache(false) })
	return dir
}

func countingCompile(p project.Project, calls *int) func() (controllerGate, error) {
	return func() (controllerGate, error) {
		*calls++
		index, diagnostics, err := compileControllerProject(p)
		if err != nil {
			return controllerGate{}, err
		}
		return computeControllerGate(p, index, diagnostics), nil
	}
}

func TestPersistedControllerGateSurvivesProcessRestart(t *testing.T) {
	for name, body := range map[string]string{"accepted": validProbeController, "rejected": rejectedProbeController} {
		t.Run(name, func(t *testing.T) {
			if !semanticcacheFingerprintUsable() {
				t.Skip("build fingerprint is per process")
			}
			enablePersistedGateForTest(t)
			p := writeControllerPageProject(t, body)
			resetGenerationCache()
			calls := 0
			first, err := sharedControllerGate(p, countingCompile(p, &calls))
			if err != nil || calls != 1 {
				t.Fatalf("first: err=%v calls=%d", err, calls)
			}
			resetGenerationCache() // a new process has an empty memory cache
			second, err := sharedControllerGate(p, countingCompile(p, &calls))
			if err != nil || calls != 1 {
				t.Fatalf("second should come from disk: err=%v calls=%d", err, calls)
			}
			if !reflect.DeepEqual(first, second) {
				t.Fatalf("persisted gate differs:\n%+v\n%+v", first, second)
			}
		})
	}
}

func semanticcacheFingerprintUsable() bool {
	probe := "probe-" + fmt.Sprint(time.Now().UnixNano())
	dir, _ := os.MkdirTemp("", "gatefp")
	defer os.RemoveAll(dir)
	defer semanticcache.SetPersistedGateDirForTesting(dir)()
	semanticcache.StorePersistedGate(probe, []byte("x"))
	_, ok := semanticcache.LoadPersistedGate(probe)
	return ok
}

func TestPersistedControllerGateHonorsDisableAndEdits(t *testing.T) {
	if !semanticcacheFingerprintUsable() {
		t.Skip("build fingerprint is per process")
	}
	dir := enablePersistedGateForTest(t)
	p := writeControllerPageProject(t, validProbeController)
	resetGenerationCache()
	calls := 0
	if _, err := sharedControllerGate(p, countingCompile(p, &calls)); err != nil {
		t.Fatal(err)
	}
	// Disabled: neither reads nor writes.
	SetPersistentGateCache(false)
	resetGenerationCache()
	if _, err := sharedControllerGate(p, countingCompile(p, &calls)); err != nil || calls != 2 {
		t.Fatalf("disabled run must compile: err=%v calls=%d", err, calls)
	}
	SetPersistentGateCache(true)
	// An edit changes the key, so the stored outcome is not served.
	if err := os.WriteFile(controllerClassPath(p), []byte(rejectedProbeController), 0o600); err != nil {
		t.Fatal(err)
	}
	resetGenerationCache()
	edited, err := sharedControllerGate(p, countingCompile(p, &calls))
	if err != nil || calls != 3 {
		t.Fatalf("edit must recompute: err=%v calls=%d", err, calls)
	}
	if len(edited.rejected) == 0 {
		t.Fatalf("edited controller should be rejected")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
}

func TestPersistedControllerGateToleratesCorruption(t *testing.T) {
	if !semanticcacheFingerprintUsable() {
		t.Skip("build fingerprint is per process")
	}
	dir := enablePersistedGateForTest(t)
	p := writeControllerPageProject(t, rejectedProbeController)
	resetGenerationCache()
	calls := 0
	want, err := sharedControllerGate(p, countingCompile(p, &calls))
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("want one entry, got %d", len(entries))
	}
	path := filepath.Join(dir, entries[0].Name())
	good, _ := os.ReadFile(path)
	for name, bad := range map[string][]byte{
		"empty":     {},
		"garbage":   []byte("not a gate"),
		"truncated": good[:len(good)/2],
		"flipped":   append(append([]byte{}, good[:len(good)-3]...), 'Z', 'Z', 'Z'),
	} {
		if err := os.WriteFile(path, bad, 0o600); err != nil {
			t.Fatal(err)
		}
		resetGenerationCache()
		before := calls
		got, err := sharedControllerGate(p, countingCompile(p, &calls))
		if err != nil || calls != before+1 || !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: err=%v recompute=%d equal=%v", name, err, calls-before, reflect.DeepEqual(got, want))
		}
	}
}

func TestPersistedControllerGateConcurrentProcesses(t *testing.T) {
	if !semanticcacheFingerprintUsable() {
		t.Skip("build fingerprint is per process")
	}
	dir := enablePersistedGateForTest(t)
	p := writeControllerPageProject(t, validProbeController)
	key, _ := projectGenerationKey(p)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 20 {
				storePersistedControllerGate(key, controllerGate{rejected: map[string]bool{"a": true}})
				if g, ok := loadPersistedControllerGate(key); ok && !g.rejected["a"] {
					t.Error("served a partial entry")
				}
			}
		}()
	}
	wg.Wait()
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("temporary files left behind: %d entries", len(entries))
	}
}

func TestPersistedControllerGateRejectsUnsafeDirectory(t *testing.T) {
	if !semanticcacheFingerprintUsable() {
		t.Skip("build fingerprint is per process")
	}
	dir := enablePersistedGateForTest(t)
	if err := os.MkdirAll(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	storePersistedControllerGate("k", controllerGate{rejected: map[string]bool{"a": true}})
	if _, ok := loadPersistedControllerGate("k"); ok {
		t.Fatal("a group- and world-writable directory must not be trusted")
	}
}

func TestWarmGenerationKeyDoesNotRereadUnchangedFiles(t *testing.T) {
	p := writeControllerPageProject(t, validProbeController)
	// ctime cannot be backdated: shrink the trust window and let the files age past it.
	fileDigestTrustAge = 50 * time.Millisecond
	t.Cleanup(func() { fileDigestTrustAge = 2 * time.Second })
	time.Sleep(120 * time.Millisecond)
	resetFileDigestMemo()
	reads := 0
	fileDigestReadHook = func(string) { reads++ }
	t.Cleanup(func() { fileDigestReadHook = nil; resetFileDigestMemo() })
	cold, _ := projectGenerationKey(p)
	coldReads := reads
	if coldReads == 0 {
		t.Fatal("cold key read no files")
	}
	warm, _ := projectGenerationKey(p)
	if warm != cold || reads != coldReads {
		t.Fatalf("warm key re-read files: reads %d -> %d, equal=%v", coldReads, reads, warm == cold)
	}
	// An edit with different size is seen.
	if err := os.WriteFile(controllerClassPath(p), []byte(rejectedProbeController), 0o600); err != nil {
		t.Fatal(err)
	}
	if edited, _ := projectGenerationKey(p); edited == cold {
		t.Fatal("edit not reflected in key")
	}
}

func TestFileDigestMemoNeverTrustsFreshFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, []byte("aaaa"), 0o600); err != nil {
		t.Fatal(err)
	}
	resetFileDigestMemo()
	a, _ := fileContentDigest(path)
	// Same size, same second: a stamp-only memo could miss this write.
	if err := os.WriteFile(path, []byte("bbbb"), 0o600); err != nil {
		t.Fatal(err)
	}
	b, _ := fileContentDigest(path)
	if a == b {
		t.Fatal("fresh file served from memo")
	}
}
