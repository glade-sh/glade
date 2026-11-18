package gladecli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/inputview"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

func runtimeHasMethod(runtime apextest.CompiledProjectRuntime, fragment string) bool {
	for key := range runtime.Methods {
		if strings.Contains(strings.ToLower(key), fragment) {
			return true
		}
	}
	return false
}

func withoutDAPRetryBackoff(t *testing.T) {
	t.Helper()
	saved := dapRetryBackoff
	dapRetryBackoff = nil
	t.Cleanup(func() { dapRetryBackoff = saved })
}

// fileSnapshot holds a path's bytes and timestamps, and its parent's, so a
// seam can restore A exactly after writing B.
type fileSnapshot struct {
	path       string
	data       []byte
	exists     bool
	mode       os.FileMode
	modTime    time.Time
	parentTime time.Time
}

func snapshotFile(t *testing.T, path string) fileSnapshot {
	t.Helper()
	snap := fileSnapshot{path: path}
	if parent, err := os.Stat(filepath.Dir(path)); err == nil {
		snap.parentTime = parent.ModTime()
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return snap
	}
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	snap.data, snap.exists, snap.mode, snap.modTime = data, true, info.Mode(), info.ModTime()
	return snap
}

func (s fileSnapshot) restore(t *testing.T) {
	t.Helper()
	if s.exists {
		if err := os.WriteFile(s.path, s.data, s.mode.Perm()); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(s.path, s.modTime, s.modTime); err != nil {
			t.Fatal(err)
		}
	} else if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if !s.parentTime.IsZero() {
		_ = os.Chtimes(filepath.Dir(s.path), s.parentTime, s.parentTime)
	}
}

func writeDriftFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func removeDriftFile(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}

const driftObjectA = `<?xml version="1.0" encoding="UTF-8"?>
<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata">
    <label>Widget Alpha</label>
    <pluralLabel>Widgets</pluralLabel>
    <nameField><label>Widget Name</label><type>Text</type></nameField>
    <deploymentStatus>Deployed</deploymentStatus>
    <sharingModel>Private</sharingModel>
</CustomObject>
`

const driftObjectB = `<?xml version="1.0" encoding="UTF-8"?>
<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata">
    <label>Widget Bravo</label>
    <pluralLabel>Widgets</pluralLabel>
    <nameField><label>Widget Name</label><type>Text</type></nameField>
    <deploymentStatus>Deployed</deploymentStatus>
    <sharingModel>Private</sharingModel>
</CustomObject>
`

func driftSharingReason(name string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<SharingReason xmlns="http://soap.sforce.com/2006/04/metadata">
    <fullName>` + name + `</fullName>
    <label>` + name + `</label>
</SharingReason>
`
}

// writeDriftProject is the A state every row starts from: the shared test
// project plus a custom object with one sharing reason, a scratch definition,
// and a source-root dependency with its own object.
func writeDriftProject(t *testing.T) (root, dep string) {
	t.Helper()
	base := t.TempDir()
	root = filepath.Join(base, "consumer")
	dep = filepath.Join(base, "dep")
	writeTestProject(t, root)
	objectDir := filepath.Join(root, "force-app", "main", "objects", "Widget__c")
	writeTestFile(t, filepath.Join(objectDir, "Widget__c.object-meta.xml"), driftObjectA)
	writeTestFile(t, filepath.Join(objectDir, "sharingReasons", "First__c.sharingReason-meta.xml"), driftSharingReason("First__c"))
	writeTestFile(t, filepath.Join(root, "config", "project-scratch-def.json"), `{"edition":"Developer"}`)
	writeTestFile(t, filepath.Join(root, "glade.yml"), "project:\n  managedPackageDependencies: [\"drift:../dep:1.0\"]\n")
	writeTestFile(t, filepath.Join(dep, "sfdx-project.json"), `{"namespace":"drift","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(dep, "force-app", "main", "objects", "Gadget__c", "Gadget__c.object-meta.xml"), strings.ReplaceAll(driftObjectA, "Widget", "Gadget"))
	return root, dep
}

type dapDriftResult struct {
	attempts int
	org      storage.OrgState
	api      string
	index    *typesys.Index
	err      error
}

func loadDAPWithDrift(t *testing.T, root string, seams dapLoadSeams) dapDriftResult {
	t.Helper()
	attempts := 0
	userBeforeLoad := seams.beforeLoad
	seams.beforeLoad = func(attempt int) {
		attempts = attempt
		if userBeforeLoad != nil {
			userBeforeLoad(attempt)
		}
	}
	org, _, api, index, err := loadDAPStartupStateWithIndexAndSeams(context.Background(), root, seams)
	return dapDriftResult{attempts: attempts, org: org, api: api, index: index, err: err}
}

func objectSharingReasons(index *typesys.Index, name string) []string {
	if index == nil {
		return nil
	}
	for _, object := range index.Objects {
		if strings.EqualFold(object.Name, name) {
			return object.SharingReasons
		}
	}
	return nil
}

func publishedManifestSHA(t *testing.T, root, path string) (string, bool) {
	t.Helper()
	entry, err := startupcache.Read(root, startupcache.SubdirDAP)
	if err != nil || entry == nil {
		t.Fatalf("no published entry: entry=%v err=%v", entry != nil, err)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		t.Fatal(err)
	}
	rel = filepath.ToSlash(rel)
	for _, files := range [][]startupcache.File{entry.Manifest.Files, entry.Manifest.ConfigFiles} {
		for _, file := range files {
			if file.Path == rel {
				return file.SHA256, true
			}
		}
	}
	return "", false
}

func fileSHA(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func assertServedWarm(t *testing.T, root string) {
	t.Helper()
	_, _, _, index, err := loadDAPStartupStateWithIndex(context.Background(), root)
	if err != nil {
		t.Fatalf("warm load: %v", err)
	}
	if index != nil {
		t.Fatal("following load rebuilt instead of serving the published entry")
	}
}

// Each row writes B before the first load and restores A, bytes and
// timestamps, before the first recheck. Endpoint proofs cannot see that swap;
// the input view must, so the first attempt is discarded and the second,
// stable attempt is served and published.
func TestLoadDAPStartupStateRetriesTransientInputSwaps(t *testing.T) {
	withoutDAPRetryBackoff(t)
	type row struct {
		name   string
		absent bool
		path   func(root, dep string) string
		swap   func(t *testing.T, path string)
		assert func(t *testing.T, root string, got dapDriftResult)
	}
	objectPath := func(root, _ string) string {
		return filepath.Join(root, "force-app", "main", "objects", "Widget__c", "Widget__c.object-meta.xml")
	}
	reasonPath := func(name string) func(root, dep string) string {
		return func(root, _ string) string {
			return filepath.Join(root, "force-app", "main", "objects", "Widget__c", "sharingReasons", name+".sharingReason-meta.xml")
		}
	}
	rows := []row{
		{
			name: "manifest-listed file",
			path: objectPath,
			swap: func(t *testing.T, path string) { writeDriftFile(t, path, driftObjectB) },
			assert: func(t *testing.T, root string, got dapDriftResult) {
				if label := got.org.Objects["Widget__c"].Definition.Label; label != "Widget Alpha" {
					t.Fatalf("label=%q, want the restored A label", label)
				}
				path := objectPath(root, "")
				if sha, ok := publishedManifestSHA(t, root, path); !ok || sha != fileSHA(t, path) {
					t.Fatalf("published manifest does not carry A's digest: ok=%v", ok)
				}
			},
		},
		{
			name:   "optional file absent then present",
			absent: true,
			path:   func(root, _ string) string { return filepath.Join(root, "sfdx-project.json") },
			swap: func(t *testing.T, path string) {
				writeDriftFile(t, path, `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"64.0"}`)
			},
			assert: func(t *testing.T, root string, got dapDriftResult) {
				if got.api == "64.0" {
					t.Fatal("served the API version of a project file that existed only mid-load")
				}
				if _, ok := publishedManifestSHA(t, root, filepath.Join(root, "sfdx-project.json")); ok {
					t.Fatal("published manifest lists a project file that is absent")
				}
			},
		},
		{
			name: "optional file present then absent",
			path: func(root, _ string) string { return filepath.Join(root, "config", "project-scratch-def.json") },
			swap: func(t *testing.T, path string) { removeDriftFile(t, path) },
			assert: func(t *testing.T, root string, got dapDriftResult) {
				path := filepath.Join(root, "config", "project-scratch-def.json")
				if sha, ok := publishedManifestSHA(t, root, path); !ok || sha != fileSHA(t, path) {
					t.Fatalf("published manifest does not carry the restored scratch definition: ok=%v", ok)
				}
			},
		},
		{
			name: "directory member added",
			path: reasonPath("Second__c"),
			swap: func(t *testing.T, path string) { writeDriftFile(t, path, driftSharingReason("Second__c")) },
			assert: func(t *testing.T, root string, got dapDriftResult) {
				if reasons := objectSharingReasons(got.index, "Widget__c"); len(reasons) != 1 || reasons[0] != "First__c" {
					t.Fatalf("sharing reasons=%v, want only First__c", reasons)
				}
			},
		},
		{
			name: "directory member removed",
			path: reasonPath("First__c"),
			swap: func(t *testing.T, path string) { removeDriftFile(t, path) },
			assert: func(t *testing.T, root string, got dapDriftResult) {
				if reasons := objectSharingReasons(got.index, "Widget__c"); len(reasons) != 1 || reasons[0] != "First__c" {
					t.Fatalf("sharing reasons=%v, want the restored First__c", reasons)
				}
			},
		},
		{
			name: "dependency root file",
			path: func(_, dep string) string {
				return filepath.Join(dep, "force-app", "main", "objects", "Gadget__c", "Gadget__c.object-meta.xml")
			},
			swap: func(t *testing.T, path string) {
				writeDriftFile(t, path, strings.ReplaceAll(driftObjectB, "Widget", "Gadget"))
			},
			assert: func(t *testing.T, root string, got dapDriftResult) {
				if label := dependencyObjectLabel(got.org, "Gadget__c"); label != "Gadget Alpha" {
					t.Fatalf("dependency label=%q, want the restored A label", label)
				}
			},
		},
	}
	for _, tc := range rows {
		t.Run(tc.name, func(t *testing.T) {
			root, dep := writeDriftProject(t)
			path := tc.path(root, dep)
			if tc.absent {
				removeDriftFile(t, path)
			}
			var original fileSnapshot
			seams := dapLoadSeams{
				beforeLoad: func(attempt int) {
					if attempt == 1 {
						original = snapshotFile(t, path)
						tc.swap(t, path)
					}
				},
				beforeRecheck: func(attempt int) {
					if attempt == 1 {
						original.restore(t)
					}
				},
			}
			got := loadDAPWithDrift(t, root, seams)
			if got.err != nil {
				t.Fatalf("load: %v", got.err)
			}
			if got.attempts != 2 {
				t.Fatalf("attempts=%d, want 2", got.attempts)
			}
			tc.assert(t, root, got)
			assertServedWarm(t, root)
		})
	}
}

// dependencyObjectLabel finds a dependency object by its unnamespaced suffix.
func dependencyObjectLabel(org storage.OrgState, suffix string) string {
	for name, object := range org.Objects {
		if strings.HasSuffix(strings.ToLower(name), "__"+strings.ToLower(suffix)) || strings.EqualFold(name, suffix) {
			return object.Definition.Label
		}
	}
	return ""
}

// Renaming a dependency root away and back changes its presence, which the
// project loader stats; the view must catch it.
func TestLoadDAPStartupStateRetriesDependencyRootPresenceSwap(t *testing.T) {
	withoutDAPRetryBackoff(t)
	root, dep := writeDriftProject(t)
	moved := dep + "-moved"
	got := loadDAPWithDrift(t, root, dapLoadSeams{
		beforeLoad: func(attempt int) {
			if attempt == 1 {
				if err := os.Rename(dep, moved); err != nil {
					t.Fatal(err)
				}
			}
		},
		beforeRecheck: func(attempt int) {
			if attempt == 1 {
				if err := os.Rename(moved, dep); err != nil {
					t.Fatal(err)
				}
			}
		},
	})
	if got.err != nil {
		t.Fatalf("load: %v", got.err)
	}
	if got.attempts != 2 {
		t.Fatalf("attempts=%d, want 2", got.attempts)
	}
	if label := dependencyObjectLabel(got.org, "Gadget__c"); label != "Gadget Alpha" {
		t.Fatalf("dependency object missing after the root was restored: label=%q", label)
	}
	assertServedWarm(t, root)
}

// A file the build reads twice with different bytes sets the view's sticky
// error. The write is left in place, so the second attempt builds and
// publishes B.
func TestLoadDAPStartupStateRetriesWhenOnePathReadsTwoWays(t *testing.T) {
	withoutDAPRetryBackoff(t)
	root, _ := writeDriftProject(t)
	path := filepath.Join(root, "force-app", "main", "objects", "Widget__c", "Widget__c.object-meta.xml")
	got := loadDAPWithDrift(t, root, dapLoadSeams{
		afterIndex: func(attempt int) {
			if attempt == 1 {
				writeDriftFile(t, path, driftObjectB)
			}
		},
	})
	if got.err != nil {
		t.Fatalf("load: %v", got.err)
	}
	if got.attempts != 2 {
		t.Fatalf("attempts=%d, want 2", got.attempts)
	}
	if label := got.org.Objects["Widget__c"].Definition.Label; label != "Widget Bravo" {
		t.Fatalf("label=%q, want B", label)
	}
	if sha, ok := publishedManifestSHA(t, root, path); !ok || sha != fileSHA(t, path) {
		t.Fatalf("published manifest does not carry B's digest: ok=%v", ok)
	}
	assertServedWarm(t, root)
}

// An Apex edit after the index build is never paired with the old runtime:
// the next load serves the edited source.
func TestLoadDAPStartupStateRebuildsAfterApexEditDuringBuild(t *testing.T) {
	withoutDAPRetryBackoff(t)
	root := t.TempDir()
	writeTestProject(t, root)
	source := filepath.Join(root, "force-app", "main", "classes", "MathUtil.cls")
	edited := false
	attempts := 0
	_, first, _, err := loadDAPStartupStateWithSeams(root, dapLoadSeams{beforeLoad: func(attempt int) { attempts = attempt }, afterIndex: func(int) {
		if edited {
			return
		}
		edited = true
		writeDriftFile(t, source, `
public class MathUtil {
  public static Integer add(Integer a, Integer b) {
    return a + b;
  }
  public static Integer driftmarker(Integer a) {
    return a;
  }
}
`)
	}})
	if err != nil {
		t.Fatalf("first load: %v", err)
	}
	if attempts != 2 || !runtimeHasMethod(first, "driftmarker") {
		t.Fatalf("first load: attempts=%d, edited method=%v", attempts, runtimeHasMethod(first, "driftmarker"))
	}
	if sha, ok := publishedManifestSHA(t, root, source); !ok || sha != fileSHA(t, source) {
		t.Fatal("published digest does not match edited source")
	}
	assertServedWarm(t, root)
	if !edited {
		t.Fatal("index hook did not run")
	}
	_, second, _, err := loadDAPStartupState(root)
	if err != nil {
		t.Fatalf("second load: %v", err)
	}
	if !runtimeHasMethod(second, "driftmarker") {
		t.Fatal("second load served a stale runtime without the edited method")
	}
}

// A source that changes or disappears while the runtime compiles and is then
// restored with its original bytes and timestamp must not reach the served
// runtime or the published entry.
func TestLoadDAPStartupStateHandlesTransientSourceChangeDuringCompile(t *testing.T) {
	withoutDAPRetryBackoff(t)
	cases := map[string]func(t *testing.T, path string){
		"removed": func(t *testing.T, path string) { removeDriftFile(t, path) },
		"replaced": func(t *testing.T, path string) {
			writeDriftFile(t, path, "public class MathUtil { public static Integer other() { return 9; } }\n")
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeTestProject(t, root)
			source := filepath.Join(root, "force-app", "main", "classes", "MathUtil.cls")
			var original fileSnapshot
			seams := dapLoadSeams{
				afterIndex: func(attempt int) {
					if attempt == 1 {
						original = snapshotFile(t, source)
						change(t, source)
					}
				},
				beforeRecheck: func(attempt int) {
					if attempt == 1 {
						original.restore(t)
					}
				},
			}
			_, runtime, _, err := loadDAPStartupStateWithSeams(root, seams)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			if runtimeHasMethod(runtime, "other") || !runtimeHasMethod(runtime, "add") {
				t.Fatal("served a runtime that does not match the restored source")
			}
			if sha, ok := publishedManifestSHA(t, root, source); !ok || sha != fileSHA(t, source) {
				t.Fatalf("published manifest does not carry the restored source digest: ok=%v", ok)
			}
			assertServedWarm(t, root)
		})
	}
}

func TestLoadDAPStartupStateFailsClosedWhenInputsKeepChanging(t *testing.T) {
	withoutDAPRetryBackoff(t)
	root, _ := writeDriftProject(t)
	path := filepath.Join(root, "force-app", "main", "objects", "Widget__c", "Widget__c.object-meta.xml")
	var original fileSnapshot
	got := loadDAPWithDrift(t, root, dapLoadSeams{
		beforeLoad: func(int) {
			original = snapshotFile(t, path)
			writeDriftFile(t, path, driftObjectB)
		},
		beforeRecheck: func(int) { original.restore(t) },
	})
	if got.attempts != dapLoadAttempts {
		t.Fatalf("attempts=%d, want %d", got.attempts, dapLoadAttempts)
	}
	if got.err == nil {
		t.Fatal("load succeeded although the inputs changed on every attempt")
	}
	var changed *inputview.ChangedError
	if !errors.As(got.err, &changed) {
		t.Fatalf("error does not unwrap to a ChangedError: %v", got.err)
	}
	want := "project files changed while the debugger was starting: force-app/main/objects/Widget__c/Widget__c.object-meta.xml (file content).\nGlade retried 3 times and the project was still changing. Wait for the save, checkout or sync to finish, then start debugging again."
	if got.err.Error() != want {
		t.Fatalf("message:\n%s\nwant:\n%s", got.err.Error(), want)
	}
	if len(got.org.Objects) != 0 || got.api != "" || got.index != nil {
		t.Fatal("fail-closed load returned state")
	}
	if _, err := os.Stat(filepath.Join(root, ".glade", "dap", "startup.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cache file written after fail-closed load: %v", err)
	}
}

func TestLoadDAPStartupStateReturnsRealLoaderErrorWithoutRetry(t *testing.T) {
	withoutDAPRetryBackoff(t)
	root := t.TempDir()
	writeTestProject(t, root)
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":`)
	_, base := project.Load(root)
	if base == nil {
		t.Fatal("fixture did not make project.Load fail")
	}
	got := loadDAPWithDrift(t, root, dapLoadSeams{})
	if got.attempts != 1 {
		t.Fatalf("attempts=%d, want 1", got.attempts)
	}
	var changing *DAPInputsChangingError
	if got.err == nil || errors.As(got.err, &changing) || got.err.Error() != base.Error() {
		t.Fatalf("err=%v, want the loader error %v", got.err, base)
	}
}

func TestConcurrentDAPLoadsDoNotInterfere(t *testing.T) {
	withoutDAPRetryBackoff(t)
	first, _ := writeDriftProject(t)
	second, _ := writeDriftProject(t)
	for _, root := range []string{first, second} {
		for i := 0; i < runtime.GOMAXPROCS(0)+1; i++ {
			name := fmt.Sprintf("ViewWorker%d", i)
			writeDriftFile(t, filepath.Join(root, "force-app", "main", "classes", name+".cls"), "public class "+name+" {}\n")
		}
	}
	stop := make(chan struct{})
	var readers sync.WaitGroup
	readers.Add(1)
	go func() {
		defer readers.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			for _, root := range []string{first, second} {
				_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
					if err == nil && !d.IsDir() {
						_, _ = os.ReadFile(path)
					}
					return nil
				})
			}
		}
	}()
	results := make([]dapDriftResult, 3)
	var loads, sameRoot sync.WaitGroup
	sameRoot.Add(2)
	for i, root := range []string{first, first, second} {
		loads.Add(1)
		go func(i int, root string) {
			defer loads.Done()
			attempts := 0
			_, _, _, _, err := loadDAPStartupStateWithIndexAndSeams(context.Background(), root, dapLoadSeams{beforeLoad: func(attempt int) {
				attempts = attempt
				if root == first && attempt == 1 {
					sameRoot.Done()
					sameRoot.Wait()
				}
			}})
			results[i] = dapDriftResult{attempts: attempts, err: err}
		}(i, root)
	}
	loads.Wait()
	close(stop)
	readers.Wait()
	for i, got := range results {
		if got.err != nil || got.attempts != 1 {
			t.Fatalf("load %d: attempts=%d err=%v", i, got.attempts, got.err)
		}
	}
}

// A sharing reason found by listing its directory is not a manifest file.
// The entry's persisted observations still cover the listing, so adding a
// member after publication rebuilds instead of serving the old entry.
func TestDAPWarmPathRebuildsWhenUnlistedInputChanges(t *testing.T) {
	withoutDAPRetryBackoff(t)
	root, _ := writeDriftProject(t)
	if _, _, _, err := loadDAPStartupState(root); err != nil {
		t.Fatal(err)
	}
	entry, err := startupcache.Read(root, startupcache.SubdirDAP)
	if err != nil || entry == nil || len(entry.Inputs) == 0 {
		t.Fatalf("published entry has no observations: entry=%v err=%v", entry != nil, err)
	}
	assertServedWarm(t, root)
	writeDriftFile(t, filepath.Join(root, "force-app", "main", "objects", "Widget__c", "sharingReasons", "Second__c.sharingReason-meta.xml"), driftSharingReason("Second__c"))
	_, _, _, index, err := loadDAPStartupStateWithIndex(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if index == nil {
		t.Fatal("warm path served an entry built before a sharing reason was added")
	}
	if reasons := objectSharingReasons(index, "Widget__c"); len(reasons) != 2 {
		t.Fatalf("sharing reasons=%v, want both", reasons)
	}
}
