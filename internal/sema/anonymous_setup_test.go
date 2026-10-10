package sema

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func resetAnonymousSetupCache() {
	anonymousSetupCache.mu.Lock()
	anonymousSetupCache.entries = nil
	anonymousSetupCache.keys = nil
	anonymousSetupCache.mu.Unlock()
}

func anonymousSetupTestIndex(t *testing.T) (typesys.Index, string) {
	t.Helper()
	root := t.TempDir()
	classPath := filepath.Join(root, "force-app", "main", "default", "classes", "AnonymousSetupProbe.cls")
	files := map[string]string{
		"sfdx-project.json": `{"sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`,
		"force-app/main/default/classes/AnonymousSetupProbe.cls": `public class AnonymousSetupProbe {
 public static Integer count() { return [SELECT COUNT() FROM Account WHERE Name != null]; }
}`,
		"force-app/main/default/classes/AnonymousSetupProbe.cls-meta.xml": `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>`,
	}
	for name, contents := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		writeSemaFile(t, path, contents)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	return typesys.Build(p, s), classPath
}

// Rows run back to back through one shared cached setup must get exactly the
// diagnostics a fresh AnalyzeAnonymous gives each row alone, including after
// earlier rows made the query checker resolve standard objects lazily.
func TestAnalyzeAnonymousCachedSetupMatchesFreshSetup(t *testing.T) {
	projectIndex, _ := anonymousSetupTestIndex(t)
	// A decl row follows gladecli's execute-anonymous flow: the local class is
	// indexed from its own temporary root and blanked out of the body.
	rows := []struct {
		name   string
		decl   string
		source string
	}{
		{"soql", "", `List<Account> rows = [SELECT Id, Name FROM Account];`},
		{"soql unknown field", "", `List<Contact> rows = [SELECT Id, NoSuchField__c FROM Contact];`},
		{"local class", `class LocalCounter { Integer total = 0; public Integer add(Integer value) { total += value; return total; } }`, ` LocalCounter counter = new LocalCounter(); Integer total = counter.add(2);`},
		{"local class misuse", `class LocalBox { public String label; }`, ` LocalBox box = new LocalBox(); Integer size = box.label;`},
		{"operator", "", `Integer flags = 1 ^ 2;`},
		{"dml on string", "", `String value = 'x'; insert value;`},
		{"project class and Name", "", `Integer total = AnonymousSetupProbe.count(); Name n = [SELECT Id FROM Name LIMIT 1];`},
		{"soql bind", "", `String label = 'x'; List<Account> rows = [SELECT Id, Name FROM Account WHERE Name = :label];`},
		{"soql activity", "", `List<Task> rows = [SELECT Id, Subject FROM Task];`},
		{"soql again", "", `List<Account> rows = [SELECT Id, Name FROM Account];`},
	}
	localIndex := func(base typesys.Index, decl, api string) typesys.Index {
		t.Helper()
		root := t.TempDir()
		path := filepath.Join(root, "force-app", "main", "default", "classes", "GladeAnonymous0.cls")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		writeSemaFile(t, path, decl)
		transient := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
		if transient.HasErrors() {
			t.Fatalf("local class index: %#v", transient.Diagnostics)
		}
		merged := base
		merged.Types = append(append([]typesys.TypeSymbol(nil), base.Types...), transient.Types...)
		if base.Project.Root == "" {
			merged.Project = transient.Project
		}
		return merged
	}
	for _, tc := range []struct {
		name  string
		index typesys.Index
	}{{"empty", typesys.Index{}}, {"project", projectIndex}} {
		for _, api := range []string{"62.0", "67.0"} {
			resetAnonymousSetupCache()
			shared := anonymousSetupFor(tc.index, api)
			indexes := make([]typesys.Index, len(rows))
			sources := make([]string, len(rows))
			cached := make([]Result, len(rows))
			for i, row := range rows {
				indexes[i], sources[i] = tc.index, row.source
				if row.decl != "" {
					indexes[i] = localIndex(tc.index, row.decl, api)
					sources[i] = strings.Repeat(" ", len(row.decl)) + row.source
				}
				cached[i] = AnalyzeAnonymous(indexes[i], sources[i], api)
				if row.decl == "" && anonymousSetupFor(tc.index, api) != shared {
					t.Fatalf("%s API %s row %q: rows did not share one cached setup", tc.name, api, row.name)
				}
			}
			if cached[2].HasErrors() || !cached[3].HasErrors() || !cached[1].HasErrors() {
				t.Fatalf("%s API %s: local class or SOQL rows lost their expected outcome: %#v / %#v / %#v", tc.name, api, cached[2].Diagnostics, cached[3].Diagnostics, cached[1].Diagnostics)
			}
			freshSetup, _ := buildAnonymousSetup(tc.index)
			cachedChecker, freshChecker := shared.queryChecker, freshSetup.queryChecker
			if len(cachedChecker.objects) != len(freshChecker.objects) || len(cachedChecker.providers) != len(freshChecker.providers) || len(cachedChecker.declaredFields) != len(freshChecker.declaredFields) {
				t.Fatalf("%s API %s: rows wrote into the cached query checker: objects %d/%d providers %d/%d declared %d/%d", tc.name, api, len(cachedChecker.objects), len(freshChecker.objects), len(cachedChecker.providers), len(freshChecker.providers), len(cachedChecker.declaredFields), len(freshChecker.declaredFields))
			}
			if len(shared.deps) != len(freshSetup.deps) {
				t.Fatalf("%s API %s: rows wrote into the cached dependency set", tc.name, api)
			}
			for i, row := range rows {
				resetAnonymousSetupCache()
				fresh := AnalyzeAnonymous(indexes[i], sources[i], api)
				if !reflect.DeepEqual(fresh, cached[i]) {
					t.Fatalf("%s API %s row %q: cached setup diverged\nfresh:  %#v\ncached: %#v", tc.name, api, row.name, fresh, cached[i])
				}
			}
		}
	}
}

func TestAnonymousSetupKeyIsContentIdentity(t *testing.T) {
	projectIndex, _ := anonymousSetupTestIndex(t)
	key := func(index typesys.Index, api string) anonymousSetupKey {
		t.Helper()
		k, ok := newAnonymousSetupKey(index, api)
		if !ok {
			t.Fatal("index is not cacheable")
		}
		return k
	}
	if key(typesys.Index{}, "67.0") != key(typesys.Index{}, "67.0") {
		t.Fatal("empty index key is not stable")
	}
	if key(typesys.Index{}, "62.0") == key(typesys.Index{}, "67.0") {
		t.Fatal("empty index key ignores the API version")
	}
	if key(typesys.Index{}, "67.0") == key(projectIndex, "67.0") {
		t.Fatal("empty and project index share a key")
	}
	if key(typesys.Index{}, "67.0") == key(typesys.Index{Project: typesys.ProjectInfo{Root: t.TempDir()}}, "67.0") {
		t.Fatal("empty index key ignores the project root")
	}
	copied := projectIndex
	copied.Types = append([]typesys.TypeSymbol(nil), projectIndex.Types...)
	if key(copied, "67.0") != key(projectIndex, "67.0") {
		t.Fatal("equal index content in a new slice changed the key")
	}
	copied.Types[0].ConstructorsAuthoritative = !copied.Types[0].ConstructorsAuthoritative
	if key(copied, "67.0") == key(projectIndex, "67.0") {
		t.Fatal("key ignores a field omitted from index JSON")
	}
	copied.Types[0] = projectIndex.Types[0]
	copied.Types[0].SuperClass = "Exception"
	if key(copied, "67.0") == key(projectIndex, "67.0") {
		t.Fatal("key ignores an in-place type edit")
	}
}

func TestAnonymousSetupRebuildsWhenReadSourceChanges(t *testing.T) {
	resetAnonymousSetupCache()
	index, classPath := anonymousSetupTestIndex(t)
	first := anonymousSetupFor(index, "67.0")
	if len(first.sourceReads) == 0 {
		t.Fatal("setup read no project source; the test does not exercise revalidation")
	}
	if again := anonymousSetupFor(index, "67.0"); again != first {
		t.Fatal("unchanged index and sources did not reuse the cached setup")
	}
	writeSemaFile(t, classPath, `public class AnonymousSetupProbe {
 public static Integer count() { return [SELECT COUNT() FROM Contact WHERE LastName != null]; }
}`)
	if first.inputsUnchanged() {
		t.Fatal("edited source still reported unchanged")
	}
	if rebuilt := anonymousSetupFor(index, "67.0"); rebuilt == first {
		t.Fatal("edited source reused the stale setup")
	}
}

// Concurrent bodies share one cached setup; run with -race.
func TestAnalyzeAnonymousConcurrentCallsShareSetup(t *testing.T) {
	resetAnonymousSetupCache()
	sources := []string{
		`List<Account> rows = [SELECT Id, Name FROM Account];`,
		`List<Contact> rows = [SELECT Id, NoSuchField__c FROM Contact];`,
		`List<Task> rows = [SELECT Id, Subject FROM Task];`,
		`String value = 'x'; insert value;`,
	}
	want := make([]Result, len(sources))
	for i, source := range sources {
		want[i] = AnalyzeAnonymous(typesys.Index{}, source, "67.0")
	}
	var wg sync.WaitGroup
	errs := make(chan string, 4*len(sources))
	for range 4 {
		for i, source := range sources {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if got := AnalyzeAnonymous(typesys.Index{}, source, "67.0"); !reflect.DeepEqual(got, want[i]) {
					errs <- source
				}
			}()
		}
	}
	wg.Wait()
	close(errs)
	for source := range errs {
		t.Errorf("concurrent result diverged for %q", source)
	}
}

// writeMarkerChainProject lays out duplicate Base classes in two trees under
// one force-app directory. Child inherits Inner through whichever Base shares
// its source unit, and a nested sfdx-project.json decides those units.
func writeMarkerChainProject(t *testing.T, root string) typesys.Index {
	t.Helper()
	files := map[string]string{
		"sfdx-project.json":     `{"sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`,
		"force-app/a/Base.cls":  `public virtual class Base extends BaseA {}`,
		"force-app/a/BaseA.cls": `public virtual class BaseA { public class Inner { public Integer n; } }`,
		"force-app/a/Child.cls": `public class Child extends Base { public Inner f; }`,
		"force-app/b/Base.cls":  `public virtual class Base extends BaseB {}`,
		"force-app/b/BaseB.cls": `public virtual class BaseB { public class Inner { public String s; } }`,
	}
	for name, contents := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		writeSemaFile(t, path, contents)
		if strings.HasSuffix(name, ".cls") {
			writeSemaFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>`)
		}
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	return typesys.Build(p, s)
}

// Adding or removing a nested sfdx-project.json regroups source units, which
// changes inherited nested-type resolution in the type-member model. A cached
// setup must notice, exactly as a fresh setup would.
func TestAnalyzeAnonymousCachedSetupFollowsProjectMarkers(t *testing.T) {
	const source = `Child c = new Child(); Integer x = c.f.n;`
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct {
			name   string
			before bool
		}{{"add", false}, {"remove", true}} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				index := writeMarkerChainProject(t, root)
				marker := filepath.Join(root, "force-app", "a", "sfdx-project.json")
				setMarker := func(present bool) {
					t.Helper()
					if present {
						writeSemaFile(t, marker, `{"packageDirectories":[{"path":".","default":true}]}`)
					} else if err := os.Remove(marker); err != nil {
						t.Fatal(err)
					}
				}
				if tc.before {
					setMarker(true)
				}
				resetAnonymousSetupCache()
				first := AnalyzeAnonymous(index, source, api)
				setMarker(!tc.before)
				cached := AnalyzeAnonymous(index, source, api)
				resetAnonymousSetupCache()
				fresh := AnalyzeAnonymous(index, source, api)
				if reflect.DeepEqual(first.Diagnostics, fresh.Diagnostics) {
					t.Fatalf("marker change did not change fresh diagnostics; the control exercises nothing: %#v", fresh.Diagnostics)
				}
				if !reflect.DeepEqual(cached, fresh) {
					t.Fatalf("cached setup ignored the marker change\ncached: %#v\nfresh:  %#v", cached.Diagnostics, fresh.Diagnostics)
				}
			})
		}
	}
}

// Concurrent misses on one key share one build.
func TestAnonymousSetupConcurrentMissesBuildOnce(t *testing.T) {
	root := t.TempDir()
	index := writeMarkerChainProject(t, root)
	for _, api := range []string{"62.0", "67.0"} {
		resetAnonymousSetupCache()
		setups := make(chan *anonymousSetup, 16)
		var wg sync.WaitGroup
		for range 16 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				setups <- anonymousSetupFor(index, api)
			}()
		}
		wg.Wait()
		close(setups)
		var first *anonymousSetup
		for setup := range setups {
			if first == nil {
				first = setup
			} else if setup != first {
				t.Fatalf("API %s: concurrent misses built more than one setup", api)
			}
		}
	}
}

// writeColorEnumProject writes a project whose Color enum is read from source
// by the type-member model. A switch branch on BIG is rejected until an edit
// renames RED to BIG, which keeps the file length.
func writeColorEnumProject(t *testing.T) (typesys.Index, string) {
	t.Helper()
	root := t.TempDir()
	classPath := filepath.Join(root, "force-app", "main", "default", "classes", "Color.cls")
	files := map[string]string{
		"sfdx-project.json":                                 `{"sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`,
		"force-app/main/default/classes/Color.cls":          `public enum Color { RED, GREEN }`,
		"force-app/main/default/classes/Color.cls-meta.xml": `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>`,
	}
	for name, contents := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		writeSemaFile(t, path, contents)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	return typesys.Build(p, s), classPath
}

// A call that joins an in-flight build must not receive a build that read a
// source before an edit this call has to see. The leader is held after it
// read RED, the waiter joins, the file becomes BIG (same length, same mtime),
// and the build is released: the waiter must match a fresh analysis.
func TestAnalyzeAnonymousWaiterSeesEditDuringLeaderBuild(t *testing.T) {
	const source = `Color c = Color.GREEN; switch on c { when BIG { } when else { } }`
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index, classPath := writeColorEnumProject(t)
			before, err := os.Stat(classPath)
			if err != nil {
				t.Fatal(err)
			}
			ours := func(candidate typesys.Index) bool { return candidate.Project.Root == index.Project.Root }
			held, release, joined := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var holdOnce, joinOnce sync.Once
			anonymousSetupTestHooks.Store(&anonymousSetupHooks{
				afterSourceReads: func(candidate typesys.Index) {
					if !ours(candidate) {
						return
					}
					holdOnce.Do(func() {
						close(held)
						<-release
					})
				},
				waiting: func(candidate typesys.Index) {
					if ours(candidate) {
						joinOnce.Do(func() { close(joined) })
					}
				},
			})
			t.Cleanup(func() { anonymousSetupTestHooks.Store(nil) })
			resetAnonymousSetupCache()
			var leader, waiter Result
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				leader = AnalyzeAnonymous(index, source, api)
			}()
			<-held
			go func() {
				defer wg.Done()
				waiter = AnalyzeAnonymous(index, source, api)
			}()
			<-joined
			writeSemaFile(t, classPath, `public enum Color { BIG, GREEN }`)
			if err := os.Chtimes(classPath, before.ModTime(), before.ModTime()); err != nil {
				t.Fatal(err)
			}
			close(release)
			wg.Wait()
			anonymousSetupTestHooks.Store(nil)
			resetAnonymousSetupCache()
			fresh := AnalyzeAnonymous(index, source, api)
			if reflect.DeepEqual(leader.Diagnostics, fresh.Diagnostics) {
				t.Fatalf("leader read the source after the edit; the control exercises nothing: %#v", leader.Diagnostics)
			}
			if !reflect.DeepEqual(waiter, fresh) {
				t.Fatalf("waiter received a build from before the edit\nwaiter: %#v\nfresh:  %#v", waiter.Diagnostics, fresh.Diagnostics)
			}
		})
	}
}

// A same-length edit that keeps the exact modification time (cp -p, git
// checkout, rsync -t, coarse timestamps) must still reach a cached setup.
// Enum values are read from the source file, not from the index, and a switch
// branch must name one of them.
func TestAnalyzeAnonymousCachedSetupFollowsSameStampEdit(t *testing.T) {
	const source = `Color c = Color.GREEN; switch on c { when BIG { } when else { } }`
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index, classPath := writeColorEnumProject(t)
			before, err := os.Stat(classPath)
			if err != nil {
				t.Fatal(err)
			}
			resetAnonymousSetupCache()
			first := AnalyzeAnonymous(index, source, api)
			writeSemaFile(t, classPath, `public enum Color { BIG, GREEN }`)
			if err := os.Chtimes(classPath, before.ModTime(), before.ModTime()); err != nil {
				t.Fatal(err)
			}
			after, err := os.Stat(classPath)
			if err != nil {
				t.Fatal(err)
			}
			if after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) || after.Mode() != before.Mode() {
				t.Fatalf("edit changed the stat: before %v %v %v, after %v %v %v", before.Size(), before.ModTime(), before.Mode(), after.Size(), after.ModTime(), after.Mode())
			}
			cached := AnalyzeAnonymous(index, source, api)
			resetAnonymousSetupCache()
			fresh := AnalyzeAnonymous(index, source, api)
			if reflect.DeepEqual(first.Diagnostics, fresh.Diagnostics) {
				t.Fatalf("edit did not change fresh diagnostics; the control exercises nothing: %#v", fresh.Diagnostics)
			}
			if !reflect.DeepEqual(cached, fresh) {
				t.Fatalf("cached setup ignored a same-stamp edit\ncached: %#v\nfresh:  %#v", cached.Diagnostics, fresh.Diagnostics)
			}
		})
	}
}

// legacySemaSourceUnitKey is semaSourceUnitKeyCached as it was before the walk
// moved into semaSourceUnitMarkerWalk, kept to prove the refactor is exact.
func legacySemaSourceUnitKey(file, sourceRoot string) string {
	file = filepath.Clean(strings.TrimSpace(file))
	if file == "" || file == "." {
		return ""
	}
	dir := filepath.Dir(file)
	root := filepath.Clean(strings.TrimSpace(sourceRoot))
	for current := dir; current != "" && current != "."; current = filepath.Dir(current) {
		if _, err := os.Stat(filepath.Join(current, "sfdx-project.json")); err == nil {
			return semaSourceTreeKey(current, file)
		}
		if root != "." && root != "" && current == root {
			break
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
	}
	return "file:" + file
}

// The shared walk must give the same source-unit keys as before, and the
// setup's marker record must cover every path that walk probes.
func TestSemaSourceUnitMarkerWalkEquivalence(t *testing.T) {
	base := t.TempDir()
	mkdir := func(rel string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(base, filepath.FromSlash(rel)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	marker := func(rel string) {
		t.Helper()
		writeSemaFile(t, filepath.Join(base, filepath.FromSlash(rel), "sfdx-project.json"), `{}`)
	}
	mkdir("outer/root/pkg/classes")
	mkdir("outer/root/pkg/nested/classes")
	mkdir("plain/classes")
	marker("outer")
	marker("outer/root")
	marker("outer/root/pkg/nested")
	abs := func(rel string) string { return filepath.Join(base, filepath.FromSlash(rel)) }
	cases := []struct {
		name, file, sourceRoot string
		probes                 []string
	}{
		{name: "empty", file: ""},
		{name: "blank", file: "   "},
		{name: "dot", file: "."},
		{name: "relative", file: "a/b/C.cls", probes: []string{"a/b/sfdx-project.json", "a/sfdx-project.json"}},
		{name: "relative dot dir", file: "./C.cls"},
		{name: "nearest of several markers", file: abs("outer/root/pkg/nested/classes/A.cls"), sourceRoot: abs("outer/root"),
			probes: []string{abs("outer/root/pkg/nested/classes/sfdx-project.json"), abs("outer/root/pkg/nested/sfdx-project.json"), abs("outer/root/pkg/sfdx-project.json"), abs("outer/root/sfdx-project.json")}},
		{name: "stops at source root", file: abs("outer/root/pkg/classes/B.cls"), sourceRoot: abs("outer/root/pkg"),
			probes: []string{abs("outer/root/pkg/classes/sfdx-project.json"), abs("outer/root/pkg/sfdx-project.json")}},
		{name: "source root with spaces", file: abs("outer/root/pkg/classes/B.cls"), sourceRoot: "  " + abs("outer/root/pkg") + "  ",
			probes: []string{abs("outer/root/pkg/classes/sfdx-project.json"), abs("outer/root/pkg/sfdx-project.json")}},
		{name: "marker above source root", file: abs("outer/root/pkg/classes/B.cls"), sourceRoot: abs("outer/root"),
			probes: []string{abs("outer/root/pkg/classes/sfdx-project.json"), abs("outer/root/pkg/sfdx-project.json"), abs("outer/root/sfdx-project.json")}},
		{name: "no source root", file: abs("plain/classes/P.cls")},
		{name: "dot source root", file: abs("plain/classes/P.cls"), sourceRoot: "."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got, want := semaSourceUnitKey(tc.file, tc.sourceRoot), legacySemaSourceUnitKey(tc.file, tc.sourceRoot); got != want {
				t.Fatalf("source-unit key = %q, legacy walk = %q", got, want)
			}
			recorded := anonymousProjectMarkers(typesys.Index{Types: []typesys.TypeSymbol{{File: tc.file, SourceRoot: tc.sourceRoot}}})
			for _, probe := range tc.probes {
				if _, ok := recorded[probe]; !ok {
					t.Fatalf("marker record misses probed path %s; recorded %v", probe, recorded)
				}
			}
			if tc.file != "" && strings.TrimSpace(tc.file) != "" && strings.TrimSpace(tc.file) != "." && len(tc.probes) > 0 {
				// The record ends where the walk ends: it never goes above the
				// source root when one is set.
				if root := strings.TrimSpace(tc.sourceRoot); root != "" && root != "." {
					for path := range recorded {
						if !strings.HasPrefix(path, filepath.Clean(root)+string(filepath.Separator)) {
							t.Fatalf("marker record walked above the source root: %s", path)
						}
					}
				}
			}
			if strings.TrimSpace(tc.file) == "" || strings.TrimSpace(tc.file) == "." {
				if len(recorded) != 0 {
					t.Fatalf("empty path recorded markers: %v", recorded)
				}
			}
			for path, exists := range recorded {
				_, err := os.Stat(path)
				if exists != (err == nil) {
					t.Fatalf("marker record for %s says %v, stat says %v", path, exists, err == nil)
				}
			}
		})
	}
	// Every path the walk visits for a deep file reaches the filesystem root
	// when no source root bounds it, nearest first.
	var visited []string
	semaSourceUnitMarkerWalk(abs("plain/classes"), "", func(marker string) bool {
		visited = append(visited, marker)
		return false
	})
	if len(visited) < 3 || visited[0] != abs("plain/classes/sfdx-project.json") || visited[len(visited)-1] != filepath.Join(string(filepath.Separator), "sfdx-project.json") {
		t.Fatalf("unbounded walk = %v", visited)
	}
}
