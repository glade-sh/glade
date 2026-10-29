package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/frontendcache"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestFrontendCacheHitSkipsBuilderAndRejectsMutation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Sample.cls")
	if err := os.WriteFile(path, []byte("public class Sample { Integer left; }"), 0600); err != nil {
		t.Fatal(err)
	}
	p := project.Project{Root: root, ApexFiles: []string{path}, SourceAPIVersion: "67.0"}
	builds := 0
	build := func() (typesys.Index, typesys.BuildArtifacts) {
		builds++
		return typesys.BuildWithArtifacts(p, schema.Schema{})
	}
	first := frontendcache.LoadOrBuild(p, schema.Schema{}, build)
	if first.Hit || builds != 1 {
		t.Fatal("first access did not build")
	}
	second := frontendcache.LoadOrBuild(p, schema.Schema{}, build)
	if !second.Hit || builds != 1 {
		t.Fatalf("validated hit rebuilt index: hit=%v builds=%d", second.Hit, builds)
	}
	if !typesys.SameProjectIdentity(first.Index, second.Index) {
		t.Fatal("lost private identity")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("public class Sample { Integer rght; }"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	third := frontendcache.LoadOrBuild(p, schema.Schema{}, build)
	if third.Hit || builds != 2 {
		t.Fatal("same-size preserved-mtime edit returned stale frontend")
	}
}

func TestFrontendCacheCorruptionAndPrivatePathsFallBack(t *testing.T) {
	for _, mode := range []string{"corruption", "file_symlink", "directory_symlink", "version"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "Sample.cls")
			if err := os.WriteFile(path, []byte("public class Sample {}"), 0600); err != nil {
				t.Fatal(err)
			}
			p := project.Project{Root: root, ApexFiles: []string{path}}
			build := func() (typesys.Index, typesys.BuildArtifacts) { return typesys.BuildWithArtifacts(p, schema.Schema{}) }
			frontendcache.LoadOrBuild(p, schema.Schema{}, build)
			cache := filepath.Join(root, ".glade", "semantic", "frontend.json")
			switch mode {
			case "corruption", "version":
				if err := os.WriteFile(cache, []byte(`{"version":999}`), 0600); err != nil {
					t.Fatal(err)
				}
			case "file_symlink":
				if err := os.Remove(cache); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(path, cache); err != nil {
					t.Fatal(err)
				}
			case "directory_symlink":
				dir := filepath.Dir(cache)
				if err := os.Rename(dir, dir+"-old"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(dir+"-old", dir); err != nil {
					t.Fatal(err)
				}
			}
			got := frontendcache.LoadOrBuild(p, schema.Schema{}, build)
			if got.Hit {
				t.Fatal("untrusted cache accepted")
			}
			source, err := os.ReadFile(path)
			if err != nil || string(source) != "public class Sample {}" {
				t.Fatal("cache fallback altered project input")
			}
		})
	}
}

func TestFrontendCacheCLICheckAndTestRoutes(t *testing.T) {
	t.Setenv("GLADE_DISABLE_DISK_CACHE", "")
	restore := apextest.EnableDiskCacheForTesting()
	t.Cleanup(restore)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			fixture := newDaemonEquivalenceFixture(t, daemonEquivalenceScenario{sourceAPIVersion: api, classes: map[string]string{"SampleTest": `@isTest private class SampleTest { @isTest static void pass() { System.assert(true); } }`}})
			root := fixture.projectRoot
			first, err := loadTestIndexWithPerfPhasesContext(context.Background(), root, true)
			if err != nil {
				t.Fatal(err)
			}
			warm, err := loadTestIndexWithPerfPhasesContext(context.Background(), root, true)
			if err != nil {
				t.Fatal(err)
			}
			if warm.Artifacts.Sources.Stats().PhysicalReadAttempts != 0 {
				t.Fatal("warm test route rebuilt source arena")
			}
			if !reflect.DeepEqual(first.Index.Project, warm.Index.Project) || !typesys.SameProjectIdentity(first.Index, warm.Index) {
				t.Fatal("test route provenance changed")
			}
			if err := typesys.ValidateBuildGeneration(warm.Index, &warm.Artifacts); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestFrontendCacheCheckResultsAndClearRoutes(t *testing.T) {
	t.Setenv("GLADE_DISABLE_DISK_CACHE", "")
	restore := apextest.EnableDiskCacheForTesting()
	t.Cleanup(restore)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			fixture := newDaemonEquivalenceFixture(t, daemonEquivalenceScenario{sourceAPIVersion: api, classes: map[string]string{"Checked": `public class Checked {}`}})
			// Use the existing local-routing rejection fixture; unresolved field
			// types are accepted by the baseline check and are not a rejection oracle.
			poisonDaemonRoutingLocalLoad(t, fixture.projectRoot)
			var expected string
			for i := 0; i < 3; i++ {
				var stdout, stderr bytes.Buffer
				args := []string{"check", "--project", fixture.projectRoot, "--json", "--no-progress"}
				perf := filepath.Join(t.TempDir(), "perf.json")
				args = append(args, "--perf-json", perf)
				if i == 2 {
					args = append(args, "--no-cache")
				}
				if code := Run(context.Background(), args, &stdout, &stderr); code != 1 {
					t.Fatalf("expected unchanged semantic rejection: code=%d stderr=%s", code, stderr.String())
				}
				if i == 0 {
					expected = stdout.String()
				} else if stdout.String() != expected {
					t.Fatal("cached and uncached complete check JSON differ")
				}
				data, err := os.ReadFile(perf)
				if err != nil {
					t.Fatal(err)
				}
				var stats struct {
					FrontendCacheHit    bool
					FrontendIndexBuilds int
				}
				if err := json.Unmarshal(data, &stats); err != nil {
					t.Fatal(err)
				}
				if i == 1 && (!stats.FrontendCacheHit || stats.FrontendIndexBuilds != 0) {
					t.Fatalf("warm check rebuilt index: %+v", stats)
				}
				if i == 2 && (stats.FrontendCacheHit || stats.FrontendIndexBuilds != 1) {
					t.Fatalf("no-cache route reused snapshot: %+v", stats)
				}
			}
			var stdout, stderr bytes.Buffer
			if code := Run(context.Background(), []string{"test", "clear-cache", "--project", fixture.projectRoot}, &stdout, &stderr); code != 0 {
				t.Fatalf("clear cache: code=%d stderr=%s", code, stderr.String())
			}
			if _, err := os.Stat(filepath.Join(fixture.projectRoot, ".glade", "semantic", "frontend.json")); !os.IsNotExist(err) {
				t.Fatalf("frontend survived cache clear: %v", err)
			}
		})
	}
}

func TestFrontendCacheOneFileEditReusesUnchangedParsing(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	first := filepath.Join(root, "force-app/main/default/classes/First.cls")
	second := filepath.Join(root, "force-app/main/default/classes/Second.cls")
	writeTestFile(t, first, "public class First { public Integer left; }")
	writeTestFile(t, second, "public class Second { public String stable; }")
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	builds := 0
	build := func() (typesys.Index, typesys.BuildArtifacts) { builds++; return typesys.BuildWithArtifacts(p, s) }
	before := frontendcache.LoadOrBuild(p, s, build)
	writeTestFile(t, first, "public class First { public Integer rght; }")
	edited := frontendcache.LoadOrBuild(p, s, build)
	if edited.Hit || !edited.Reused || builds != 1 {
		t.Fatalf("one-file edit rebuilt unchanged parsing: hit=%v reused=%v builds=%d", edited.Hit, edited.Reused, builds)
	}
	fresh, freshArtifacts := typesys.BuildWithArtifacts(p, s)
	a, _ := json.Marshal(fresh)
	b, _ := json.Marshal(edited.Index)
	if string(a) != string(b) || !typesys.SameProjectIdentity(fresh, edited.Index) {
		t.Fatal("incremental frontend differs from fresh index")
	}
	for i, source := range freshArtifacts.Sources.All() {
		if !reflect.DeepEqual(source, edited.Artifacts.Sources.All()[i]) {
			t.Fatal("incremental source artifacts differ from fresh")
		}
	}
	if reflect.DeepEqual(before.Index.Types, edited.Index.Types) {
		t.Fatal("source edit returned the old parsed declaration")
	}
}

func TestFrontendCacheInvalidEditsUseCompleteBuildArtifacts(t *testing.T) {
	for _, source := range []string{"public class First {", "public class Second {}"} {
		t.Run(source, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
			first := filepath.Join(root, "force-app/main/default/classes/First.cls")
			second := filepath.Join(root, "force-app/main/default/classes/Second.cls")
			writeTestFile(t, first, "public class First {}")
			writeTestFile(t, second, "public class Second {}")
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			s, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			builds := 0
			build := func() (typesys.Index, typesys.BuildArtifacts) { builds++; return typesys.BuildWithArtifacts(p, s) }
			frontendcache.LoadOrBuild(p, s, build)
			writeTestFile(t, first, source)
			edited := frontendcache.LoadOrBuild(p, s, build)
			if edited.Reused || edited.Hit || builds != 2 || len(edited.Artifacts.Sources.All()) != 2 {
				t.Fatalf("invalid edit bypassed full build artifacts: reused=%v hit=%v builds=%d sources=%d", edited.Reused, edited.Hit, builds, len(edited.Artifacts.Sources.All()))
			}
			fresh, _ := typesys.BuildWithArtifacts(p, s)
			a, _ := json.Marshal(fresh)
			b, _ := json.Marshal(edited.Index)
			if string(a) != string(b) {
				t.Fatal("fallback index or diagnostics differ from fresh build")
			}
		})
	}
}

func TestFrontendCacheKeepsSourceOnlyOccurrencesOnEdit(t *testing.T) {
	for _, unrepresented := range []string{"", "// owned source without a declaration\n"} {
		t.Run(unrepresented, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
			first := filepath.Join(root, "force-app/main/default/classes/First.cls")
			second := filepath.Join(root, "force-app/main/default/classes/Second.cls")
			writeTestFile(t, first, "public class First { public Integer left; }")
			writeTestFile(t, second, unrepresented)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			s, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			builds := 0
			build := func() (typesys.Index, typesys.BuildArtifacts) { builds++; return typesys.BuildWithArtifacts(p, s) }
			frontendcache.LoadOrBuild(p, s, build)
			writeTestFile(t, first, "public class First { public Integer rght; }")
			edited := frontendcache.LoadOrBuild(p, s, build)
			if edited.Reused || builds != 2 || len(edited.Artifacts.Sources.All()) != 2 {
				t.Fatalf("source-only occurrence lost: reused=%v builds=%d sources=%d", edited.Reused, builds, len(edited.Artifacts.Sources.All()))
			}
		})
	}
}

func TestFrontendCacheNamedProjectOutcomesAtFloors(t *testing.T) {
	t.Setenv("GLADE_DISABLE_DISK_CACHE", "")
	restore := apextest.EnableDiskCacheForTesting()
	t.Cleanup(restore)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			fixture := newDaemonEquivalenceFixture(t, daemonEquivalenceScenario{sourceAPIVersion: api, classes: map[string]string{"SampleTest": `@isTest private class SampleTest { @isTest static void pass() { System.assert(true); } }`}})
			var expected []byte
			for i := 0; i < 3; i++ {
				var stdout, stderr bytes.Buffer
				args := []string{"test", "--project", fixture.projectRoot, "--no-serve", "--parallelism", "1", "--json", "--no-progress"}
				if i == 2 {
					args = append(args, "--no-cache")
				}
				if code := Run(context.Background(), args, &stdout, &stderr); code != 0 {
					t.Fatalf("named project route failed: code=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
				}
				var result any
				if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				var normalize func(any)
				normalize = func(value any) {
					switch item := value.(type) {
					case map[string]any:
						for key, child := range item {
							switch key {
							case "durationMs", "duration", "startedAt", "finishedAt":
								delete(item, key)
							default:
								normalize(child)
							}
						}
					case []any:
						for _, child := range item {
							normalize(child)
						}
					}
				}
				normalize(result)
				normalized, err := json.Marshal(result)
				if err != nil {
					t.Fatal(err)
				}
				if i == 0 {
					expected = normalized
				} else if !bytes.Equal(expected, normalized) {
					t.Fatalf("complete named outcomes changed on route %d: %s versus %s", i, expected, normalized)
				}
			}
		})
	}
}

func TestFrontendCacheClearFencesActivePublication(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Owned.cls")
	writeTestFile(t, path, "public class Owned {}")
	p := project.Project{Root: root, ApexFiles: []string{path}}
	first := frontendcache.LoadOrBuild(p, schema.Schema{}, func() (typesys.Index, typesys.BuildArtifacts) {
		var stdout, stderr bytes.Buffer
		if code := Run(context.Background(), []string{"test", "clear-cache", "--project", root}, &stdout, &stderr); code != 0 {
			t.Fatalf("clear-cache failed: %s", stderr.String())
		}
		return typesys.BuildWithArtifacts(p, schema.Schema{})
	})
	if first.Hit {
		t.Fatal("first build reported a hit")
	}
	if _, err := os.Stat(filepath.Join(root, ".glade", "semantic", "frontend.json")); !os.IsNotExist(err) {
		t.Fatalf("pre-clear flight recreated frontend cache: %v", err)
	}
	next := frontendcache.LoadOrBuild(p, schema.Schema{}, func() (typesys.Index, typesys.BuildArtifacts) { return typesys.BuildWithArtifacts(p, schema.Schema{}) })
	if next.Hit {
		t.Fatal("cleared generation remained published")
	}
	warm := frontendcache.LoadOrBuild(p, schema.Schema{}, func() (typesys.Index, typesys.BuildArtifacts) {
		t.Fatal("post-clear cache failed to publish")
		return typesys.Index{}, typesys.BuildArtifacts{}
	})
	if !warm.Hit {
		t.Fatal("post-clear generation failed to publish")
	}
}

func TestFrontendCacheBodyEditInvalidatesWholeSemanticResult(t *testing.T) {
	t.Setenv("GLADE_DISABLE_DISK_CACHE", "")
	restore := apextest.EnableDiskCacheForTesting()
	t.Cleanup(restore)
	checkSemanticResults.Reset()
	t.Cleanup(checkSemanticResults.Reset)
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	path := filepath.Join(root, "force-app/main/default/classes/Owned.cls")
	writeTestFile(t, path, "public class Owned { public static void run() { System.debug('old'); } }")
	perf := filepath.Join(t.TempDir(), "perf.json")
	run := func(extra ...string) string {
		t.Helper()
		var stdout, stderr bytes.Buffer
		args := []string{"check", "--project", root, "--json", "--no-progress", "--perf-json", perf}
		if code := Run(context.Background(), append(args, extra...), &stdout, &stderr); code != 0 {
			t.Fatalf("check failed: %s %s", stdout.String(), stderr.String())
		}
		return stdout.String()
	}
	run()
	before, err := os.ReadFile(perf)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, path, "public class Owned { public static void run() { System.debug('new'); } }")
	edited := run()
	after, err := os.ReadFile(perf)
	if err != nil {
		t.Fatal(err)
	}
	var oldStats, newStats struct {
		SemanticCache struct {
			Source   string
			Identity struct{ ProjectContentSHA256 string }
		}
	}
	if err := json.Unmarshal(before, &oldStats); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(after, &newStats); err != nil {
		t.Fatal(err)
	}
	if newStats.SemanticCache.Source != "build" || newStats.SemanticCache.Identity.ProjectContentSHA256 == oldStats.SemanticCache.Identity.ProjectContentSHA256 {
		t.Fatal("same-shape source body edit reused an old semantic result")
	}
	if current := run("--no-cache"); current != edited {
		t.Fatal("edited cached check differs from fresh complete check JSON")
	}
}

// Setup stages (unpackaged/pre and unpackaged/post) deploy before tests run and
// are startup-cache roots. A change inside them must change the frontend input
// identity, so the compile cache never serves a generation built without it.
func TestFrontendCacheSetupStageChangesInvalidateIdentity(t *testing.T) {
	field := func(kind string) string {
		return `<?xml version="1.0" encoding="UTF-8"?><CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Amount__c</fullName><label>Amount</label><type>` + kind + `</type>` +
			map[string]string{"Number": `<precision>18</precision><scale>0</scale>`, "Text": `<length>40</length>`}[kind] + `</CustomField>`
	}
	object := `<?xml version="1.0" encoding="UTF-8"?><CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Setup</label><pluralLabel>Setups</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`
	for _, change := range []string{"none", "post_class_edit", "pre_field_type", "post_add_object", "new_pre_stage"} {
		t.Run(change, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
			writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/Owned.cls"), "public class Owned { public Integer left; }")
			setupClass := filepath.Join(root, "unpackaged/post/classes/SetupData.cls")
			writeTestFile(t, setupClass, "public class SetupData { public Integer left; }")
			writeTestFile(t, filepath.Join(root, "unpackaged/pre/objects/Setup__c/Setup__c.object-meta.xml"), object)
			fieldPath := filepath.Join(root, "unpackaged/pre/objects/Setup__c/fields/Amount__c.field-meta.xml")
			writeTestFile(t, fieldPath, field("Number"))
			if change == "new_pre_stage" {
				if err := os.RemoveAll(filepath.Join(root, "unpackaged/pre")); err != nil {
					t.Fatal(err)
				}
			}
			load := func() (project.Project, schema.Schema) {
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				s, err := schema.LoadProject(p)
				if err != nil {
					t.Fatal(err)
				}
				return p, s
			}
			p, s := load()
			if len(p.SetupMetadataRoots()) == 0 {
				t.Fatal("fixture has no setup stage roots")
			}
			before, err := typesys.FrontendInputIdentity(p, s)
			if err != nil {
				t.Fatal(err)
			}
			builds := 0
			frontendcache.LoadOrBuild(p, s, func() (typesys.Index, typesys.BuildArtifacts) { builds++; return typesys.BuildWithArtifacts(p, s) })
			switch change {
			case "post_class_edit":
				info, err := os.Stat(setupClass)
				if err != nil {
					t.Fatal(err)
				}
				writeTestFile(t, setupClass, "public class SetupData { public Integer rght; }")
				if err := os.Chtimes(setupClass, info.ModTime(), info.ModTime()); err != nil {
					t.Fatal(err)
				}
			case "pre_field_type":
				writeTestFile(t, fieldPath, field("Text"))
			case "post_add_object":
				writeTestFile(t, filepath.Join(root, "unpackaged/post/objects/Later__c/Later__c.object-meta.xml"), strings.ReplaceAll(object, "Setup", "Later"))
			case "new_pre_stage":
				writeTestFile(t, filepath.Join(root, "unpackaged/pre/classes/PreData.cls"), "public class PreData {}")
			}
			p, s = load()
			after, err := typesys.FrontendInputIdentity(p, s)
			if err != nil {
				t.Fatal(err)
			}
			result := frontendcache.LoadOrBuild(p, s, func() (typesys.Index, typesys.BuildArtifacts) { builds++; return typesys.BuildWithArtifacts(p, s) })
			if change == "none" {
				if after != before || !result.Hit || builds != 1 {
					t.Fatalf("unchanged setup stages missed the cache: same=%v hit=%v builds=%d", after == before, result.Hit, builds)
				}
				return
			}
			if after == before || result.Hit {
				t.Fatalf("setup stage change kept the frontend identity: same=%v hit=%v", after == before, result.Hit)
			}
			fresh, _ := typesys.BuildWithArtifacts(p, s)
			want, _ := json.Marshal(fresh)
			got, _ := json.Marshal(result.Index)
			if string(want) != string(got) {
				t.Fatal("setup stage change did not produce the fresh index")
			}
		})
	}
}
