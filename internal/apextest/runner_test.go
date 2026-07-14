package apextest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/packageartifact"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func firstRunProblem(run testreport.Run) string {
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			if testCase.Problem != nil {
				return testCase.Problem.Message
			}
		}
	}
	return ""
}

func TestRuntimeKeyWithSourceDigestsAvoidsRereadsAndMatchesDiskFallback(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "force-app", "main", "default", "classes", "Unicode.cls")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, path, "public class Unicode { String value = '雪'; }\r\n")
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	index, artifacts := typesys.BuildWithArtifacts(p, gladeschema.Schema{})
	if len(p.ApexFiles) != 1 || len(index.Types) != 1 {
		t.Fatalf("loaded Apex/index types = %#v/%#v diagnostics=%#v", p.ApexFiles, index.Types, index.Diagnostics)
	}
	reads := 0
	readFile := func(string) ([]byte, error) {
		reads++
		return nil, os.ErrNotExist
	}

	fromSnapshot := runtimeKeyWithSourceDigests(index, artifacts.SourceDigests, readFile)
	if reads != 0 {
		t.Fatalf("complete snapshot source reads = %d, want 0", reads)
	}
	fromDisk := runtimeKeyWithSourceDigests(index, nil, os.ReadFile)
	if fromSnapshot != fromDisk {
		t.Fatalf("snapshot key %q differs from disk key %q", fromSnapshot, fromDisk)
	}
	writeFile(t, path, "public class Unicode { String value = '雨'; }\r\n")
	changedIndex, changedArtifacts := typesys.BuildWithArtifacts(p, gladeschema.Schema{})
	if changed := runtimeKeyWithSourceDigests(changedIndex, changedArtifacts.SourceDigests, readFile); changed == fromSnapshot {
		t.Fatal("raw source change did not change runtime key")
	}
	if reads != 0 {
		t.Fatalf("changed complete snapshot source reads = %d, want 0", reads)
	}
}

func TestRuntimeKeyWithIncompleteSourceDigestsFallsBackEntirely(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "First.cls")
	second := filepath.Join(root, "Second.cls")
	writeFile(t, first, "public class First {}\n")
	writeFile(t, second, "public class Second {}\n")
	index, artifacts := typesys.BuildWithArtifacts(project.Project{Root: root, ApexFiles: []string{first}}, gladeschema.Schema{})
	index.Types = append(index.Types, typesys.TypeSymbol{Name: "Second", File: second})
	reads := 0
	readFile := func(path string) ([]byte, error) {
		reads++
		return os.ReadFile(path)
	}

	got := runtimeKeyWithSourceDigests(index, artifacts.SourceDigests, readFile)
	if reads != 2 {
		t.Fatalf("incomplete snapshot source reads = %d, want all 2 unique sources", reads)
	}
	if want := runtimeKeyWithSourceDigests(index, nil, os.ReadFile); got != want {
		t.Fatalf("incomplete snapshot key %q differs from all-disk key %q", got, want)
	}
}

func TestRunWithSourceDigestsPersistsExactV5RuntimeKey(t *testing.T) {
	if testRuntimeCacheABI != "apextest-runtime-v5" {
		t.Fatalf("test runtime ABI = %q, want apextest-runtime-v5", testRuntimeCacheABI)
	}
	wasDisabled := disableDiskCache.Load()
	disableDiskCache.Store(false)
	t.Cleanup(func() {
		disableDiskCache.Store(wasDisabled)
		InvalidateRuntimeCaches()
		ResetPerfCounters()
	})
	InvalidateRuntimeCaches()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app", "main", "classes", "DigestCacheTest.cls"), `
@isTest
private class DigestCacheTest {
  @isTest static void runs() { System.assert(true); }
}
`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := gladeschema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	index, artifacts := typesys.BuildWithArtifacts(p, s)
	wantKey := RuntimeContentKey(index, artifacts.SourceDigests)
	if len(p.ApexFiles) != 1 || len(index.Types) != 1 {
		t.Fatalf("loaded Apex/index types = %#v/%#v diagnostics=%#v", p.ApexFiles, index.Types, index.Diagnostics)
	}
	run := Run(index, Options{SourceDigests: artifacts.SourceDigests})
	if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
		t.Fatalf("summary = %#v problem=%q", summary, firstRunProblem(run))
	}
	entry, err := startupcache.Read(root, startupcache.SubdirTest)
	if err != nil || entry == nil {
		t.Fatalf("Read() = %#v, %v", entry, err)
	}
	if entry.RuntimeABI != testRuntimeCacheABI || entry.RuntimeKey != wantKey {
		t.Fatalf("persisted ABI/key = %q/%q, want %q/%q", entry.RuntimeABI, entry.RuntimeKey, testRuntimeCacheABI, wantKey)
	}
	InvalidateRuntimeCaches()
	if _, ok := tryLoadDiskRuntimeWithSourceDigests(index, artifacts.SourceDigests, runtimeCacheKey(wantKey)); !ok {
		t.Fatal("exact SourceDigestSet runtime cache did not reload")
	}

	InvalidateRuntimeCaches()
	ResetPerfCounters()
	cached := Run(index, Options{SourceDigests: artifacts.SourceDigests, PerfCounters: true})
	if phases := SnapshotPerfCounters().Phases; phases.DiskCacheHits != 1 || phases.CacheMisses != 0 {
		t.Fatalf("source-digest cached phases = %#v", phases)
	}
	InvalidateRuntimeCaches()
	ResetPerfCounters()
	uncached := Run(index, Options{SourceDigests: artifacts.SourceDigests, PerfCounters: true, NoDiskCache: true})
	if phases := SnapshotPerfCounters().Phases; phases.DiskCacheHits != 0 || phases.CacheMisses != 1 {
		t.Fatalf("source-digest no-cache phases = %#v", phases)
	}
	normalize := func(run testreport.Run) testreport.Run {
		run.DurationMS = 0
		for suiteIndex := range run.Suites {
			run.Suites[suiteIndex].DurationMS = 0
			for caseIndex := range run.Suites[suiteIndex].Cases {
				run.Suites[suiteIndex].Cases[caseIndex].DurationMS = 0
			}
		}
		return run
	}
	if got, want := normalize(cached), normalize(uncached); !reflect.DeepEqual(got, want) {
		t.Fatalf("source-digest cache/no-cache results differ:\n cache=%#v\n no-cache=%#v", got, want)
	}
}

func TestRunWithStaleSourceDigestsFailsClosedWithoutCachePublish(t *testing.T) {
	wasDisabled := disableDiskCache.Load()
	disableDiskCache.Store(false)
	t.Cleanup(func() {
		disableDiskCache.Store(wasDisabled)
		InvalidateRuntimeCaches()
		ResetPerfCounters()
	})
	InvalidateRuntimeCaches()
	root := t.TempDir()
	helperPath := filepath.Join(root, "force-app", "main", "classes", "DigestMutationHelper.cls")
	testPath := filepath.Join(root, "force-app", "main", "classes", "DigestMutationTest.cls")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, helperPath, `public class DigestMutationHelper { public static Integer value() { return 1; } }`)
	writeFile(t, testPath, `@isTest private class DigestMutationTest {
  @isTest static void runsFirst() { System.assertEquals(1, DigestMutationHelper.value()); }
  @isTest static void runsSecond() { System.assertEquals(1, DigestMutationHelper.value()); }
}`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := gladeschema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	index, artifacts := typesys.BuildWithArtifacts(p, s)
	staleKey := RuntimeContentKey(index, artifacts.SourceDigests)
	writeFile(t, helperPath, `public class DigestMutationHelper { public static Integer value() { return 2; } }`)
	actualKey := RuntimeContentKey(index, nil)
	if actualKey == staleKey {
		t.Fatal("same-size source mutation did not change runtime key")
	}
	warmErr := WarmRuntimeWithSourceDigests(index, artifacts.SourceDigests)
	var mismatch *SourceSnapshotMismatchError
	if !errors.As(warmErr, &mismatch) {
		t.Fatalf("WarmRuntimeWithSourceDigests error = %T %v", warmErr, warmErr)
	}
	if mismatch.File != helperPath || mismatch.ExpectedSHA256 == "" || mismatch.ActualSHA256 == "" {
		t.Fatalf("changed-source mismatch = %#v", mismatch)
	}
	if _, compileErr := CompileProjectRuntimeForRequestWithSourceDigests(index, artifacts.SourceDigests); !errors.As(compileErr, &mismatch) {
		t.Fatalf("CompileProjectRuntimeForRequestWithSourceDigests error = %T %v", compileErr, compileErr)
	}
	if err := os.Remove(helperPath); err != nil {
		t.Fatal(err)
	}
	warmErr = WarmRuntimeWithSourceDigests(index, artifacts.SourceDigests)
	mismatch = nil
	if !errors.As(warmErr, &mismatch) || mismatch.ActualSHA256 != "" || !errors.Is(warmErr, os.ErrNotExist) {
		t.Fatalf("unreadable-source mismatch = %#v, %v", mismatch, warmErr)
	}
	writeFile(t, helperPath, `public class DigestMutationHelper { public static Integer value() { return 2; } }`)

	ResetPerfCounters()
	first := Run(index, Options{SourceDigests: artifacts.SourceDigests, PerfCounters: true})
	if summary := first.Summary(); summary.Total != 2 || summary.CompileErrors != 2 {
		t.Fatalf("cold mutated summary = %#v problem=%q", summary, firstRunProblem(first))
	}
	if len(first.Suites) != 1 || len(first.Suites[0].Cases) != 2 ||
		first.Suites[0].Cases[0].MethodName != "runsFirst" || first.Suites[0].Cases[1].MethodName != "runsSecond" {
		t.Fatalf("cold mutated case order = %#v", first.Suites)
	}
	if problem := firstRunProblem(first); !strings.Contains(problem, "source snapshot mismatch") {
		t.Fatalf("cold mutated problem = %q", problem)
	}
	runtimeCacheMu.RLock()
	_, stalePublished := runtimeCache[runtimeCacheKey(staleKey)]
	_, actualPublished := runtimeCache[runtimeCacheKey(actualKey)]
	runtimeCacheMu.RUnlock()
	if stalePublished || actualPublished {
		t.Fatalf("mutated runtime cache published stale=%v actual=%v", stalePublished, actualPublished)
	}
	entry, err := startupcache.Read(root, startupcache.SubdirTest)
	if err != nil {
		t.Fatal(err)
	}
	if entry != nil {
		t.Fatalf("mutated disk cache published key %q", entry.RuntimeKey)
	}

	writeFile(t, helperPath, `public class DigestMutationHelper { public static Integer value() { return 1; } }`)
	restored := Run(index, Options{SourceDigests: artifacts.SourceDigests})
	if summary := restored.Summary(); summary.Total != 2 || summary.Passed != 2 {
		t.Fatalf("restored summary = %#v problem=%q", summary, firstRunProblem(restored))
	}
}

func TestRunWithSourceDigestsRejectsLazySourceMutationWithoutCachePoison(t *testing.T) {
	tests := []struct {
		name     string
		original string
		mutated  string
	}{
		{
			name: "ordinary test",
			original: `@isTest private class LazySnapshotTest {
  @isTest static void runs() { System.assertEquals(1, 1); }
}`,
			mutated: `@isTest private class LazySnapshotTest {
  @isTest static void runs() { System.assertEquals(1, 2); }
}`,
		},
		{
			name: "test setup",
			original: `@isTest private class LazySnapshotTest {
  @TestSetup static void setup() { System.assertEquals(1, 1); }
  @isTest static void runs() { System.assert(true); }
}`,
			mutated: `@isTest private class LazySnapshotTest {
  @TestSetup static void setup() { System.assertEquals(1, 2); }
  @isTest static void runs() { System.assert(true); }
}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restoreDisk := DisableDiskCacheForTesting()
			t.Cleanup(restoreDisk)
			t.Cleanup(InvalidateRuntimeCaches)
			InvalidateRuntimeCaches()
			root := t.TempDir()
			path := filepath.Join(root, "LazySnapshotTest.cls")
			writeFile(t, path, tt.original)
			p := project.Project{Root: root, ApexFiles: []string{path}}
			index, artifacts := typesys.BuildWithArtifacts(p, gladeschema.Schema{})
			WarmRuntimeWithSourceDigests(index, artifacts.SourceDigests)

			writeFile(t, path, tt.mutated)
			mutated := Run(index, Options{SourceDigests: artifacts.SourceDigests})
			if summary := mutated.Summary(); summary.Total != 1 || summary.CompileErrors != 1 {
				t.Fatalf("mutated summary = %#v problem=%q", summary, firstRunProblem(mutated))
			}
			if problem := firstRunProblem(mutated); !strings.Contains(problem, "source snapshot mismatch") {
				t.Fatalf("mutated problem = %q", problem)
			}
			if tt.name == "ordinary test" {
				testCacheMu.RLock()
				cached := len(testCache)
				testCacheMu.RUnlock()
				if cached != 0 {
					t.Fatalf("mismatched ordinary test cached %d entries", cached)
				}
			} else {
				setupCacheMu.RLock()
				cached := len(setupCache)
				setupCacheMu.RUnlock()
				if cached != 0 {
					t.Fatalf("mismatched test setup cached %d entries", cached)
				}
			}

			writeFile(t, path, tt.original)
			restored := Run(index, Options{SourceDigests: artifacts.SourceDigests})
			if summary := restored.Summary(); summary.Total != 1 || summary.Passed != 1 {
				t.Fatalf("restored summary = %#v problem=%q", summary, firstRunProblem(restored))
			}
		})
	}
}

func TestRunWithSourceDigestsFullyWarmSkipsSourceReread(t *testing.T) {
	restoreDisk := DisableDiskCacheForTesting()
	t.Cleanup(restoreDisk)
	t.Cleanup(InvalidateRuntimeCaches)
	InvalidateRuntimeCaches()
	root := t.TempDir()
	path := filepath.Join(root, "FullyWarmSnapshotTest.cls")
	writeFile(t, path, `@isTest private class FullyWarmSnapshotTest {
  @TestSetup static void setup() { System.assert(true); }
  @isTest static void runs() { System.assert(true); }
}`)
	p := project.Project{Root: root, ApexFiles: []string{path}}
	index, artifacts := typesys.BuildWithArtifacts(p, gladeschema.Schema{})
	first := Run(index, Options{SourceDigests: artifacts.SourceDigests})
	if summary := first.Summary(); summary.Total != 1 || summary.Passed != 1 {
		t.Fatalf("initial summary = %#v problem=%q", summary, firstRunProblem(first))
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	ResetPerfCounters()
	warm := Run(index, Options{SourceDigests: artifacts.SourceDigests, PerfCounters: true})
	if summary := warm.Summary(); summary.Total != 1 || summary.Passed != 1 {
		t.Fatalf("fully warm summary = %#v problem=%q", summary, firstRunProblem(warm))
	}
	if phases := SnapshotPerfCounters().Phases; phases.MemoryCacheHits != 1 || phases.CacheMisses != 0 {
		t.Fatalf("fully warm phases = %#v", phases)
	}
}

func TestSourceDigestRuntimeWrappersPreserveNilCompatibility(t *testing.T) {
	restore := DisableDiskCacheForTesting()
	t.Cleanup(restore)
	t.Cleanup(InvalidateRuntimeCaches)
	root := t.TempDir()
	path := filepath.Join(root, "Wrapper.cls")
	writeFile(t, path, "public class Wrapper { public static Integer value() { return 1; } }\n")
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{path}}, gladeschema.Schema{})
	if got, want := RuntimeContentKey(index, nil), string(runtimeKey(index)); got != want {
		t.Fatalf("nil-digest public key = %q, want wrapper key %q", got, want)
	}

	InvalidateRuntimeCaches()
	legacy := CompileProjectRuntimeForRequest(index)
	InvalidateRuntimeCaches()
	bridged, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(legacy.Methods, bridged.Methods) ||
		!reflect.DeepEqual(legacy.Triggers, bridged.Triggers) ||
		!reflect.DeepEqual(legacy.PageNames, bridged.PageNames) {
		t.Fatal("nil-digest compiled runtime project methods/triggers/pages differ")
	}

	InvalidateRuntimeCaches()
	WarmRuntime(index)
	runtimeCacheMu.RLock()
	_, legacyWarm := runtimeCache[runtimeKey(index)]
	runtimeCacheMu.RUnlock()
	InvalidateRuntimeCaches()
	if err := WarmRuntimeWithSourceDigests(index, nil); err != nil {
		t.Fatal(err)
	}
	runtimeCacheMu.RLock()
	_, bridgedWarm := runtimeCache[runtimeKey(index)]
	runtimeCacheMu.RUnlock()
	if !legacyWarm || !bridgedWarm {
		t.Fatalf("warm wrapper cache presence = legacy %v bridged %v", legacyWarm, bridgedWarm)
	}
}

func TestRunExecutesAnonymousSubsetTestMethods(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"samplepkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MathTest.cls"), `
@isTest
private class MathTest {
  @isTest static void adds() {
    Integer x = 1 + 1;
    System.assertEquals(2, x);
  }
  @TestSetup static void setup() {
    System.debug('setup');
  }
}

`)

	index := loadTestIndex(t, root)
	run := Run(index, Options{})
	summary := run.Summary()
	if summary.Total != 1 || summary.Passed != 1 {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestRunRejectsSetArgumentWhenOnlyListObjectOverloadExists(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Q.cls"), `
public class Q {
  public static QCondition condition(String fieldName) {
    return new QCondition();
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/QCondition.cls"), `
public class QCondition {
  public QCondition isIn(List<Object> values) {
    return this;
  }
  public QCondition isIn(String value) {
    return this;
  }
  public QCondition isIn(Q subquery) {
    return this;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/QConditionTest.cls"), `
@isTest
private class QConditionTest {
  @isTest static void rejectsSetIdArgument() {
    Set<Id> productIds = new Set<Id>{'001000000000001AAA'};
    Q.condition('Parent__c').isIn(productIds);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Failed != 1 {
		t.Fatalf("summary = %#v problem=%q run=%#v", got, firstRunProblem(run), run)
	}
	if problem := firstRunProblem(run); !strings.Contains(problem, "isIn") {
		t.Fatalf("problem = %q, want isIn overload failure", problem)
	}
}

func TestCapturedPackageMethodFailsWithNamedBoundaryWithoutShim(t *testing.T) {
	root := t.TempDir()
	writeCapturedBillingArtifact(t, filepath.Join(root, "packages/pkg.glade-package.json"))
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "glade.yml"), `project:
  managedPackageDependencies: ["pkg:artifact:packages/pkg.glade-package.json:1.0"]
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BillingTest.cls"), `
@isTest
private class BillingTest {
  @isTest static void runs() {
    System.assertEquals(true, pkg.BillingGateway.authorize(1.00));
  }
}
`)
	run := Run(loadTestIndex(t, root), Options{})
	problem := firstRunProblem(run)
	if got := run.Summary(); got.Total != 1 || got.Failed != 1 || !strings.Contains(problem, "captured package member has no local body") {
		t.Fatalf("summary = %#v problem=%q run=%#v", got, problem, run)
	}
}

func TestRunRemapsProductionNamespaceSourceDependencyToProxyNamespace(t *testing.T) {
	root := t.TempDir()
	depRoot := filepath.Join(root, "base-source")
	consumerRoot := filepath.Join(root, "consumer")
	writeFile(t, filepath.Join(depRoot, "sfdx-project.json"), `{"namespace":"BasePkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/objects/Billing__c/Billing__c.object-meta.xml"), `
<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata">
  <label>Billing</label>
  <pluralLabel>Billings</pluralLabel>
</CustomObject>
`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/objects/Billing__c/fields/Amount__c.field-meta.xml"), `
<CustomField xmlns="http://soap.sforce.com/2006/04/metadata">
  <fullName>Amount__c</fullName>
  <label>Amount</label>
  <type>Number</type>
  <precision>16</precision>
  <scale>0</scale>
</CustomField>
`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/classes/Helper.cls"), `
global class Helper {
  global static Integer amount(BasePkg__Billing__c row) {
    return Integer.valueOf(row.Amount__c);
  }
}
`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/classes/Gateway.cls"), `
global class Gateway {
  global static Integer createAmount(Integer amount) {
    BasePkg__Billing__c row = new BasePkg__Billing__c(Amount__c = amount);
    insert row;
    List<BasePkg__Billing__c> rows = [SELECT Amount__c FROM BasePkg__Billing__c];
    return BasePkg.Helper.amount(rows[0]);
  }
}
`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/triggers/BillingTrigger.trigger"), `
trigger BillingTrigger on BasePkg__Billing__c (before insert) {
  for (BasePkg__Billing__c row : Trigger.new) {
    row.Amount__c = BasePkg.Helper.amount(row) + 1;
  }
}
`)
	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(consumerRoot, "glade.yml"), `project:
  managedPackageDependencies: ["stagepkg:../base-source"]
  namespaceRemaps: ["BasePkg:stagepkg"]
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/classes/NamespaceRemapTest.cls"), `
@isTest
private class NamespaceRemapTest {
  @isTest static void remappedDependencyRuns() {
    System.assertEquals(43, stagepkg.Gateway.createAmount(42));
    List<stagepkg__Billing__c> rows = [SELECT Amount__c FROM stagepkg__Billing__c];
    System.assertEquals(1, rows.size());
    System.assertEquals(43, Integer.valueOf(rows[0].Amount__c));
  }
}
`)

	run := Run(loadTestIndex(t, consumerRoot), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestCapturedPackageConstructorFailsWithNamedBoundaryWithoutShim(t *testing.T) {
	root := t.TempDir()
	writeCapturedBillingArtifact(t, filepath.Join(root, "packages/pkg.glade-package.json"))
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "glade.yml"), `project:
  managedPackageDependencies: ["pkg:artifact:packages/pkg.glade-package.json:1.0"]
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BillingTest.cls"), `
@isTest
private class BillingTest {
  @isTest static void runs() {
    Object gateway = new pkg.BillingGateway(1.00);
    System.assertNotEquals(null, gateway);
  }
}
`)
	run := Run(loadTestIndex(t, root), Options{})
	problem := firstRunProblem(run)
	if got := run.Summary(); got.Total != 1 || got.Failed != 1 || !strings.Contains(problem, "captured package member has no local body") {
		t.Fatalf("summary = %#v problem=%q run=%#v", got, problem, run)
	}
}

func TestCapturedPackageMethodUsesConfiguredShim(t *testing.T) {
	root := t.TempDir()
	writeCapturedBillingArtifact(t, filepath.Join(root, "packages/pkg.glade-package.json"))
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "glade.yml"), `project:
  managedPackageDependencies: ["pkg:artifact:packages/pkg.glade-package.json:1.0"]
  packageShims: ["pkg:test-support/package-shims/pkg"]
`)
	writeFile(t, filepath.Join(root, "test-support/package-shims/pkg/sfdx-project.json"), `{"packageDirectories":[{"path":"classes","default":true}]}`)
	writeFile(t, filepath.Join(root, "test-support/package-shims/pkg/classes/BillingGateway.cls"), `
global class BillingGateway {
  global static Boolean authorize(Decimal amount) {
    return amount > 0;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BillingTest.cls"), `
@isTest
private class BillingTest {
  @isTest static void runs() {
    System.assertEquals(true, pkg.BillingGateway.authorize(1.00));
  }
}
`)
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v problem=%q run=%#v", got, firstRunProblem(run), run)
	}
}

func TestRunNoDiskCachePreservesExistingGlobalDisable(t *testing.T) {
	wasDisabled := disableDiskCache.Load()
	disableDiskCache.Store(true)
	t.Cleanup(func() { disableDiskCache.Store(wasDisabled) })
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/CacheToggleTest.cls"), `
@isTest
private class CacheToggleTest {
  @isTest static void runs() {
    System.assert(true);
  }
}

`)

	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	summary := run.Summary()
	if summary.Total != 1 || summary.Passed != 1 {
		t.Fatalf("summary = %#v", summary)
	}
	if diskCacheEnabled() {
		t.Fatal("NoDiskCache run re-enabled a globally disabled disk cache")
	}
}

func TestRunNoDiskCacheDoesNotToggleGlobalDisable(t *testing.T) {
	wasDisabled := disableDiskCache.Load()
	disableDiskCache.Store(false)
	t.Cleanup(func() { disableDiskCache.Store(wasDisabled) })
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/CacheToggleEnabledTest.cls"), `
@isTest
private class CacheToggleEnabledTest {
  @isTest static void runs() {
    System.assert(true);
  }
}

`)

	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	summary := run.Summary()
	if summary.Total != 1 || summary.Passed != 1 {
		t.Fatalf("summary = %#v", summary)
	}
	if !diskCacheEnabled() {
		t.Fatal("NoDiskCache run disabled the package-global disk cache flag")
	}
}

func TestTryLoadDiskRuntimeRejectsRuntimeABIMismatch(t *testing.T) {
	wasDisabled := disableDiskCache.Load()
	disableDiskCache.Store(false)
	t.Cleanup(func() {
		disableDiskCache.Store(wasDisabled)
		InvalidateRuntimeCaches()
	})
	InvalidateRuntimeCaches()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/CacheABIProbeTest.cls"), `
@isTest
private class CacheABIProbeTest {
  @isTest static void runs() {
    System.assert(true);
  }
}
`)
	index := loadTestIndex(t, root)
	loadedProject, err := project.Load(root)
	if err != nil {
		t.Fatalf("project.Load() error = %v", err)
	}
	entry := startupcache.NewEntry(root, loadedProject, index, storage.NewOrgState(), startupcache.CompiledRuntime{})
	entry.RuntimeABI = "old-runtime-abi"
	if err := startupcache.Write(&entry, startupcache.SubdirTest); err != nil {
		t.Fatalf("Write(old ABI) error = %v", err)
	}
	if _, ok := tryLoadDiskRuntime(index); ok {
		t.Fatal("tryLoadDiskRuntime accepted a stale runtime ABI")
	}

	entry.RuntimeABI = testRuntimeCacheABI
	if err := startupcache.Write(&entry, startupcache.SubdirTest); err != nil {
		t.Fatalf("Write(current ABI) error = %v", err)
	}
	if _, ok := tryLoadDiskRuntime(index); ok {
		t.Fatal("tryLoadDiskRuntime accepted a keyless current runtime ABI")
	}

	entry.RuntimeKey = string(runtimeKey(index))
	if err := startupcache.Write(&entry, startupcache.SubdirTest); err != nil {
		t.Fatalf("Write(current ABI/key) error = %v", err)
	}
	if _, ok := tryLoadDiskRuntime(index); !ok {
		t.Fatal("tryLoadDiskRuntime rejected the current runtime ABI/key")
	}
}

func TestRunParallelMethodsBypassesDiskRuntimeCache(t *testing.T) {
	wasDisabled := disableDiskCache.Load()
	disableDiskCache.Store(false)
	t.Cleanup(func() {
		disableDiskCache.Store(wasDisabled)
		InvalidateRuntimeCaches()
	})
	InvalidateRuntimeCaches()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/CacheProbe.cls"), `
public class CacheProbe {
  public static Boolean value() {
    return true;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/CacheProbeTest.cls"), `
@isTest
private class CacheProbeTest {
  @isTest static void first() {
    System.assertEquals(true, CacheProbe.value());
  }

  @isTest static void second() {
    System.assertEquals(true, CacheProbe.value());
  }
}
`)
	index := loadTestIndex(t, root)
	loadedProject, err := project.Load(root)
	if err != nil {
		t.Fatalf("project.Load() error = %v", err)
	}
	entry := startupcache.NewEntry(root, loadedProject, index, storage.NewOrgState(), startupcache.CompiledRuntime{})
	entry.RuntimeABI = testRuntimeCacheABI
	if err := startupcache.Write(&entry, startupcache.SubdirTest); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	run := Run(index, Options{ParallelMethods: true, Parallelism: 2})

	if summary := run.Summary(); summary.Total != 2 || summary.Passed != 2 {
		t.Fatalf("summary = %#v problem=%q", summary, firstRunProblem(run))
	}
}

func TestRunStaticTestClassCallWinsOverSameNameField(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"samplepkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/TestCustomizationSettings.cls"), `
@isTest
private class TestCustomizationSettings {
  private static Account testCustomizationSettings;

  @isTest static void runs() {
    System.assertEquals(null, testCustomizationSettings);
  }
}

`)

	run := Run(loadTestIndex(t, root), Options{})
	summary := run.Summary()
	if summary.Total != 1 || summary.Passed != 1 {
		t.Fatalf("summary = %#v, run = %#v", summary, run)
	}
}

func TestRunNamespacedProjectLabelReferences(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/labels/CustomLabels.labels"), `<CustomLabels>
  <labels><fullName>Greeting</fullName><language>en_US</language><value>Hello</value></labels>
</CustomLabels>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/translations/fr.translation-meta.xml"), `<Translations>
  <customLabels><name>Greeting</name><label>Bonjour</label></customLabels>
</Translations>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/LabelTest.cls"), `
@isTest
private class LabelTest {
  @isTest static void resolvesLabels() {
    System.assertEquals('Hello', Label.Greeting);
    System.assertEquals('Hello', Label.pkg.Greeting);
    System.assertEquals('Bonjour', System.Label.get('pkg', 'Greeting', 'fr'));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	summary := run.Summary()
	if summary.Total != 1 || summary.Passed != 1 {
		t.Fatalf("summary = %#v, problem = %s, run = %#v", summary, firstRunProblem(run), run)
	}
}

func TestRunSkipsSetupPhaseWhenNoTestSetupsExist(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/FirstTest.cls"), `
@isTest
private class FirstTest {
  @isTest static void runs() {
    System.assertEquals(1, 1);
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/SecondTest.cls"), `
@isTest
private class SecondTest {
  @isTest static void runs() {
    System.assertEquals(2, 2);
  }
}
`)

	var setupStarts int
	run := Run(loadTestIndex(t, root), Options{
		Parallelism: 1,
		Progress: func(progress TestProgress) {
			if progress.Event == "setup_start" {
				setupStarts++
			}
		},
	})
	summary := run.Summary()
	if summary.Total != 2 || summary.Passed != 2 {
		t.Fatalf("summary = %#v, run = %#v", summary, run)
	}
	if setupStarts != 0 {
		t.Fatalf("setup_start events = %d, want 0", setupStarts)
	}
}

func TestRunDataWeaveScriptResourceExecutesRuntimeStub(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/dw/helloWorld.dwl"), `%dw 2.0
output text/plain
---
"Hello World"`)
	writeFile(t, filepath.Join(root, "force-app/main/default/dw/helloWorld.dwl-meta.xml"), `<DataWeaveResource/>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/DataWeaveHarness.cls"), `
public class DataWeaveHarness {
  public static String staticInvocation() {
    DataWeave.Script script = new DataWeaveScriptResource.helloWorld();
    return script.execute(new Map<String, Object>()).getValueAsString();
  }

  public static void dynamicError() {
    DataWeave.Script script = DataWeave.Script.createScript('error');
    script.execute(new Map<String, Object>());
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/DataWeaveHarnessTest.cls"), `
@isTest
private class DataWeaveHarnessTest {
  @isTest static void staticResourceExecuteReturnsScriptOutput() {
    System.assertEquals('"Hello World"', DataWeaveHarness.staticInvocation());
  }

  @isTest static void dynamicCreateScriptThrowsScriptException() {
    try {
      DataWeaveHarness.dynamicError();
      System.assert(false, 'expected exception');
    } catch (Exception ex) {
      System.assertEquals('System.DataWeaveScriptException', ex.getTypeName());
      System.assert(ex.getMessage().startsWith('Division by zero'));
    }
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		var details strings.Builder
		for _, testCase := range run.Suites[0].Cases {
			details.WriteString(testCase.ClassName + "." + testCase.MethodName + " " + string(testCase.Status))
			if testCase.Problem != nil {
				details.WriteString(" " + testCase.Problem.Type + ": " + testCase.Problem.Message)
				if testCase.Problem.Detail != "" {
					details.WriteString(" (" + testCase.Problem.Detail + ")")
				}
			}
			details.WriteByte('\n')
		}
		t.Fatalf("summary = %#v\n%s", got, details.String())
	}
}

func TestSortClassRunOrderUsesDurationHistory(t *testing.T) {
	classOrder := []string{"ShortMany", "LongOne", "Middle"}
	classIndexes := map[string][]int{
		"ShortMany": {0, 1, 2, 3},
		"LongOne":   {4},
		"Middle":    {5, 6},
	}
	sortClassRunOrder(classOrder, classIndexes, map[string]int64{
		"LongOne": 90000,
		"Middle":  10000,
	})
	want := []string{"LongOne", "Middle", "ShortMany"}
	for i := range want {
		if classOrder[i] != want[i] {
			t.Fatalf("classOrder[%d] = %q, want %q (order=%v)", i, classOrder[i], want[i], classOrder)
		}
	}
}

func TestSortClassRunOrderFallsBackToMethodCount(t *testing.T) {
	classOrder := []string{"One", "Three", "Two"}
	classIndexes := map[string][]int{
		"One":   {0},
		"Three": {1, 2, 3},
		"Two":   {4, 5},
	}
	sortClassRunOrder(classOrder, classIndexes, nil)
	want := []string{"Three", "Two", "One"}
	for i := range want {
		if classOrder[i] != want[i] {
			t.Fatalf("classOrder[%d] = %q, want %q (order=%v)", i, classOrder[i], want[i], classOrder)
		}
	}
}

func TestSortMethodIndexesUsesDurationHistory(t *testing.T) {
	planned := []testCasePlan{
		{TestCase: TestCase{ClassName: "SlowClass", MethodName: "fast"}},
		{TestCase: TestCase{ClassName: "SlowClass", MethodName: "slow"}},
	}
	indexes := []int{0, 1}

	sortMethodIndexes(indexes, planned, map[string]int64{
		"SlowClass.slow": 8000,
		"SlowClass.fast": 20,
	})

	if indexes[0] != 1 || indexes[1] != 0 {
		t.Fatalf("indexes = %#v, want slow method first", indexes)
	}
}

func TestEnsureProjectDataReferencedObjectFieldPreservesExistingLookupTarget(t *testing.T) {
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	storage.EnsureStandardObject(&org, "Account")
	storage.EnsureStandardObject(&org, "pkg__Facility__c")
	org.Objects["FacilityLicense__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "FacilityLicense__c",
			Fields: map[string]storage.Field{
				"Facility__c": {
					APIName:               "Facility__c",
					Type:                  storage.FieldReference,
					DisplayType:           "REFERENCE",
					ReferenceTo:           []string{"Account"},
					RelationshipName:      "Facility__r",
					ChildRelationshipName: "FacilityLicenses__r",
				},
			},
			Relations: []storage.Relationship{{
				Field:              "Facility__c",
				ParentObjects:      []string{"Account"},
				ParentRelationship: "Facility__r",
				ChildRelationship:  "FacilityLicenses__r",
			}},
		},
		Records: make(map[storage.ID]storage.Record),
	}

	ensureProjectDataReferencedObjectField(&org, "FacilityLicense__c", "Facility__c")

	state := org.Objects["FacilityLicense__c"]
	field := state.Definition.Fields["Facility__c"]
	if len(field.ReferenceTo) != 1 || field.ReferenceTo[0] != "Account" {
		t.Fatalf("ReferenceTo = %v, want [Account]", field.ReferenceTo)
	}
	if len(state.Definition.Relations) != 1 || len(state.Definition.Relations[0].ParentObjects) != 1 || state.Definition.Relations[0].ParentObjects[0] != "Account" {
		t.Fatalf("Relations = %#v, want existing Account relationship", state.Definition.Relations)
	}
}

func TestRunHttpSendWithoutMockReturnsStubInTestContext(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/HttpHarnessTest.cls"), `
@isTest
private class HttpHarnessTest {
  @isTest static void sendsWithoutExternalNetwork() {
    HttpRequest req = new HttpRequest();
    req.setEndpoint('https://example.invalid/probe');
    req.setMethod('GET');
    new Http().send(req);
  }
}
	`)

	index := loadTestIndex(t, root)
	run := Run(index, Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v cases=%#v", got, run.Suites[0].Cases)
	}
}

func TestRunResolvesLowercaseClassPropertyBeforeDML(t *testing.T) {
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	consumerRoot := filepath.Join(root, "consumer")
	writeFile(t, filepath.Join(depRoot, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/classes/AddressValue.cls"), `
global class AddressValue {
  global AddressValue(String state) {
    this.State = state;
  }
  global String State { get; set; }
}
`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/classes/AddressUtil.cls"), `
global class AddressUtil {
  @testVisible private static final AddressValue ALT_ADDRESS {
    get {
      String stateValue = 'AK';
      return new AddressValue(stateValue);
    }
  }
}
`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/classes/AddressFactory.cls"), `
@isTest
global class AddressFactory {
  global static void setDefaultBillingState(Account account) {
    account.BillingState = AddressUtil.ALT_ADDRESS.State;
  }
}
`)
	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(consumerRoot, "glade.yml"), `project:
  managedPackageDependencies: ["pkg:../dep:1.0"]
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/classes/AddressUtil.cls"), `
public class AddressUtil {
  public static String LocalOnly = 'local';
}
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/classes/AddressValueTest.cls"), `
@isTest
private class AddressValueTest {
  @isTest static void lowercasePropertyStoresStringField() {
    Account account = new Account(Name = 'Acme');
    pkg.AddressFactory.setDefaultBillingState(account);
    insert account;
    Account loaded = [SELECT BillingState FROM Account WHERE Id = :account.Id];
    System.assertEquals('AK', loaded.BillingState);
  }
}
`)

	run := Run(loadTestIndex(t, consumerRoot), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunSupportsAuthValueObjectDefaultConstructors(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/AuthValueObjectTest.cls"), `
@isTest
private class AuthValueObjectTest {
  @isTest static void defaultConstructorsAreUsable() {
    Auth.UserData data = new Auth.UserData();
    Auth.VerificationResult result = new Auth.VerificationResult();
    Auth.UserData populated = new Auth.UserData('003000000000001', 'Ada', 'Lovelace', 'Ada Lovelace', 'ada@example.invalid', null, 'ada@example.invalid', 'en_US', 'local', null, null);
    Auth.VerificationResult verified = new Auth.VerificationResult(new PageReference('/welcome'), true, 'ok');
    System.assertNotEquals(null, data);
    System.assertNotEquals(null, result);
    System.assertEquals('003000000000001', populated.identifier);
    System.assertEquals(true, verified.success);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunBareInstanceMapFieldMembers(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MapFieldOwner.cls"), `
public class MapFieldOwner {
  private Map<Id, Account> accountsById;

  public MapFieldOwner(Account account) {
    accountsById = new Map<Id, Account>{ account.Id => account };
  }

  public Account getAccount(Id accountId) {
    return accountsById.get(accountId);
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MapFieldOwnerTest.cls"), `
@isTest
private class MapFieldOwnerTest {
  @isTest static void mapGetUsesInstanceFieldType() {
    Account account = new Account(Name = 'Acme');
    insert account;
    MapFieldOwner owner = new MapFieldOwner(account);
    System.assertEquals(account.Id, owner.getAccount(account.Id).Id);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunClassConstructorCanShadowSObjectName(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/CartItem.cls"), `
public class CartItem {
  public String Name { get; private set; }
  public CartItem(Account account) {
    this.Name = account.Name;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/CartItemConstructorTest.cls"), `
@isTest
private class CartItemConstructorTest {
  @isTest static void classConstructorWinsOverSObjectName() {
    CartItem item = new CartItem(new Account(Name = 'Acme'));
    System.assertEquals('Acme', item.Name);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestDiscoverCapturesSeeAllDataAnnotation(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/SeeAllDataTest.cls"), `
@isTest
private class SeeAllDataTest {
  @isTest(SeeAllData=true) static void seesData() {}
  @isTest static void siloed() {}
}
`)

	cases := Discover(loadTestIndex(t, root), Options{})
	seen := map[string]bool{}
	for _, testCase := range cases {
		seen[testCase.MethodName] = testCase.SeeAllData
	}
	if !seen["seesData"] {
		t.Fatalf("SeeAllData annotation was not captured: %#v", cases)
	}
	if seen["siloed"] {
		t.Fatalf("plain @isTest method marked SeeAllData: %#v", cases)
	}
}

func TestCloneRuntimeOrgIsolatesRecordsAndDefinitions(t *testing.T) {
	ResetPerfCounters()
	t.Cleanup(ResetPerfCounters)
	counters := newRunPerfCounters(true)
	org := storage.NewOrgState()
	org.OrgID = "00D000000000001"
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: "Text"},
			},
		},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {
				ID:     "001000000000001",
				Object: "Account",
				Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")},
			},
		},
	}

	cloned := cloneRuntimeOrg(org, counters)
	account := cloned.Objects["Account"]
	account.Records["001000000000001"].Fields["Name"] = storage.StringValue("Changed")
	cloned.Objects["Account"] = account
	if _, clonedDef := storage.EnsureMutableObjectDefinition(&cloned, "Account"); !clonedDef {
		t.Fatalf("definition was not made mutable")
	}
	accountValue := cloned.Objects["Account"]
	accountValue.Definition.Fields["RuntimeOnly__c"] = storage.Field{APIName: "RuntimeOnly__c", Type: storage.FieldString}
	cloned.Objects["Account"] = accountValue

	if got := org.Objects["Account"].Records["001000000000001"].Fields["Name"].String; got != "Acme" {
		t.Fatalf("clone shared records with base org: %q", got)
	}
	if _, ok := org.Objects["Account"].Definition.Fields["RuntimeOnly__c"]; ok {
		t.Fatalf("runtime clone shared definition fields with base org")
	}
	stats := snapshotPerfCounters(counters)
	if stats.CloneRuntimeOrgCalls != 1 {
		t.Fatalf("cloneRuntimeOrg calls = %d, want 1", stats.CloneRuntimeOrgCalls)
	}
}

func TestRunUsesClassJournalForSequentialMethodsAfterTestSetup(t *testing.T) {
	ResetPerfCounters()
	t.Cleanup(ResetPerfCounters)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/SetupJournalTest.cls"), `
@isTest
private class SetupJournalTest {
  @testSetup static void setupData() {
    insert new Account(Name = 'Seed');
  }

  @isTest static void firstMethodAddsRecord() {
    insert new Account(Name = 'First');
    System.assertEquals(2, [SELECT COUNT() FROM Account]);
  }

  @isTest static void secondMethodDoesNotSeeFirstMethodRecord() {
    System.assertEquals(1, [SELECT COUNT() FROM Account]);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{Parallelism: 1, PerfCounters: true})
	if summary := run.Summary(); summary.Total != 2 || summary.Passed != 2 {
		if len(run.Suites) > 0 {
			t.Fatalf("summary = %#v cases = %#v", summary, run.Suites[0].Cases)
		}
		t.Fatalf("summary = %#v suites = %#v", summary, run.Suites)
	}
	stats := SnapshotPerfCounters()
	if stats.CloneRuntimeOrgCalls != 1 {
		t.Fatalf("cloneRuntimeOrg calls = %d, want setup clone only", stats.CloneRuntimeOrgCalls)
	}
	if stats.CloneFallbacks != 0 {
		t.Fatalf("clone fallbacks = %d, want class journal path", stats.CloneFallbacks)
	}
	if stats.JournalRollbacks != 2 {
		t.Fatalf("journal rollbacks = %d, want one per method", stats.JournalRollbacks)
	}
}

func TestRunUsesClassJournalForSequentialMethodsWithoutTestSetup(t *testing.T) {
	ResetPerfCounters()
	t.Cleanup(ResetPerfCounters)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/NoSetupJournalTest.cls"), `
@isTest
private class NoSetupJournalTest {
  @isTest static void aFirstMethodAddsRecord() {
    insert new Account(Name = 'First');
    System.assertEquals(1, [SELECT COUNT() FROM Account]);
  }

  @isTest static void bSecondMethodDoesNotSeeFirstMethodRecord() {
    System.assertEquals(0, [SELECT COUNT() FROM Account]);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{Parallelism: 1, PerfCounters: true})
	if summary := run.Summary(); summary.Total != 2 || summary.Passed != 2 {
		if len(run.Suites) > 0 {
			t.Fatalf("summary = %#v cases = %#v", summary, run.Suites[0].Cases)
		}
		t.Fatalf("summary = %#v suites = %#v", summary, run.Suites)
	}
	stats := SnapshotPerfCounters()
	if stats.CloneRuntimeOrgCalls != 1 {
		t.Fatalf("cloneRuntimeOrg calls = %d, want class org clone only", stats.CloneRuntimeOrgCalls)
	}
	if stats.CloneFallbacks != 0 {
		t.Fatalf("clone fallbacks = %d, want class journal path", stats.CloneFallbacks)
	}
	if stats.JournalRollbacks != 2 {
		t.Fatalf("journal rollbacks = %d, want one per method", stats.JournalRollbacks)
	}
}

func TestRunUsesWorkerJournalsForParallelMethods(t *testing.T) {
	ResetPerfCounters()
	t.Cleanup(ResetPerfCounters)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ParallelJournalTest.cls"), `
@isTest
private class ParallelJournalTest {
  @isTest static void aFirstMethodAddsRecord() {
    insert new Account(Name = 'First');
    System.assertEquals(1, [SELECT COUNT() FROM Account]);
  }
  @isTest static void bSecondMethodDoesNotSeeFirstMethodRecord() {
    System.assertEquals(0, [SELECT COUNT() FROM Account]);
  }
  @isTest static void cThirdMethodAddsRecord() {
    insert new Account(Name = 'Third');
    System.assertEquals(1, [SELECT COUNT() FROM Account]);
  }
  @isTest static void dFourthMethodDoesNotSeeThirdMethodRecord() {
    System.assertEquals(0, [SELECT COUNT() FROM Account]);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{Parallelism: 2, ParallelMethods: true, PerfCounters: true})
	if summary := run.Summary(); summary.Total != 4 || summary.Passed != 4 {
		if len(run.Suites) > 0 {
			t.Fatalf("summary = %#v cases = %#v", summary, run.Suites[0].Cases)
		}
		t.Fatalf("summary = %#v suites = %#v", summary, run.Suites)
	}
	stats := SnapshotPerfCounters()
	if stats.CloneRuntimeOrgCalls != 2 {
		t.Fatalf("cloneRuntimeOrg calls = %d, want one journal org per worker", stats.CloneRuntimeOrgCalls)
	}
	if stats.CloneFallbacks != 0 {
		t.Fatalf("clone fallbacks = %d, want worker journal path", stats.CloneFallbacks)
	}
	if stats.JournalRollbacks != 4 {
		t.Fatalf("journal rollbacks = %d, want one per method", stats.JournalRollbacks)
	}
}

func TestRunPerfCountersOptionEnablesVMPerfCounters(t *testing.T) {
	ResetPerfCounters()
	t.Cleanup(ResetPerfCounters)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/PerfCounterTest.cls"), `
@isTest
private class PerfCounterTest {
  @isTest static void passes() {
    System.assertEquals(1, 1);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{PerfCounters: true})
	if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
		t.Fatalf("summary = %#v suites = %#v", summary, run.Suites)
	}
	if stats := SnapshotPerfCounters(); !stats.VMPerf.Enabled {
		t.Fatalf("VM perf counters not enabled: %#v", stats.VMPerf)
	}
}

func TestPerfCountersCapturePreRunAndRunnerPhases(t *testing.T) {
	ResetPerfCounters()
	t.Cleanup(ResetPerfCounters)
	InvalidateRuntimeCaches()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/PhaseTest.cls"), `
@isTest
private class PhaseTest {
	@testSetup static void setupData() {}
  @isTest static void passes() { System.assertEquals(1, 1); }
}
`)

	pre := PreRunPhaseDurations{
		ProjectLoad: 3 * time.Millisecond,
		SchemaLoad:  5 * time.Millisecond,
		IndexBuild:  7 * time.Millisecond,
		Discover:    11 * time.Millisecond,
	}
	run := Run(loadTestIndex(t, root), Options{PerfCounters: true, NoDiskCache: true, PreRunPhaseDurations: pre})
	if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
		t.Fatalf("summary = %#v suites = %#v", summary, run.Suites)
	}
	stats := SnapshotPerfCounters()
	if !stats.Enabled {
		t.Fatalf("Enabled = false: %#v", stats)
	}
	if stats.Phases.ProjectLoadNS != pre.ProjectLoad.Nanoseconds() || stats.Phases.SchemaLoadNS != pre.SchemaLoad.Nanoseconds() || stats.Phases.IndexBuildNS != pre.IndexBuild.Nanoseconds() || stats.Phases.DiscoverNS != pre.Discover.Nanoseconds() {
		t.Fatalf("pre-run phases = %#v, want %#v", stats.Phases, pre)
	}
	if stats.Phases.DiscoverNS <= 0 || stats.Phases.RuntimeKeyNS <= 0 || stats.Phases.OrgBuildNS <= 0 || stats.Phases.ProjectCompileNS <= 0 || stats.Phases.TestCompileNS <= 0 || stats.Phases.ClassSetupNS <= 0 || stats.Phases.MethodRunNS <= 0 || stats.Phases.ReportAssemblyNS <= 0 {
		t.Fatalf("runner phases incomplete: %#v", stats.Phases)
	}
}

func TestPerfCountersCopySuppliedPreRunDiscoverDuration(t *testing.T) {
	ResetPerfCounters()
	t.Cleanup(ResetPerfCounters)
	InvalidateRuntimeCaches()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/PreDiscoveredTest.cls"), `
@isTest private class PreDiscoveredTest {
  @isTest static void passes() { System.assertEquals(1, 1); }
}
`)
	index := loadTestIndex(t, root)
	cases := Discover(index, Options{})
	supplied := 13 * time.Millisecond
	run := RunCasesContext(context.Background(), index, Options{
		PerfCounters: true,
		NoDiskCache:  true,
		PreRunPhaseDurations: PreRunPhaseDurations{
			Discover: supplied,
		},
	}, cases)
	if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
		t.Fatalf("summary = %#v suites = %#v", summary, run.Suites)
	}
	if got := SnapshotPerfCounters().Phases.DiscoverNS; got != supplied.Nanoseconds() {
		t.Fatalf("DiscoverNS = %d, want supplied %d", got, supplied.Nanoseconds())
	}
}

func TestPerfCountersDiscoverRemainsZeroWithoutPreRunWiring(t *testing.T) {
	ResetPerfCounters()
	t.Cleanup(ResetPerfCounters)
	InvalidateRuntimeCaches()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/UnwiredDiscoverTest.cls"), `
@isTest private class UnwiredDiscoverTest {
  @isTest static void passes() { System.assertEquals(1, 1); }
}
`)
	run := Run(loadTestIndex(t, root), Options{PerfCounters: true, NoDiskCache: true})
	if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
		t.Fatalf("summary = %#v suites = %#v", summary, run.Suites)
	}
	if got := SnapshotPerfCounters().Phases.DiscoverNS; got != 0 {
		t.Fatalf("DiscoverNS = %d without pre-run wiring, want 0", got)
	}
}

func TestDiskStartupCacheInvalidationMatchesNoCache(t *testing.T) {
	ResetPerfCounters()
	InvalidateRuntimeCaches()
	wasDiskCacheDisabled := disableDiskCache.Load()
	disableDiskCache.Store(false)
	t.Cleanup(func() {
		disableDiskCache.Store(wasDiskCacheDisabled)
		InvalidateRuntimeCaches()
		ResetPerfCounters()
	})

	root := t.TempDir()
	helperPath := filepath.Join(root, "force-app/main/classes/CacheExactHelper.cls")
	testPath := filepath.Join(root, "force-app/main/classes/CacheExactTest.cls")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, helperPath, `
public class CacheExactHelper {
  public static Integer value() { return 1; }
}
`)
	writeFile(t, testPath, `
@isTest
private class CacheExactTest {
  @isTest static void seesCurrentHelper() {
    System.assertEquals(1, CacheExactHelper.value());
  }
}
`)

	initial := Run(loadTestIndex(t, root), Options{PerfCounters: true})
	if summary := initial.Summary(); summary.Total != 1 || summary.Passed != 1 {
		t.Fatalf("initial summary = %#v cases = %#v", summary, initial.Suites)
	}
	initialCounters := SnapshotPerfCounters().Phases
	if initialCounters.CacheMisses != 1 || initialCounters.DiskCacheHits != 0 || initialCounters.CacheEncodeNS <= 0 {
		t.Fatalf("initial counters = %#v", initialCounters)
	}

	helperInfo, err := os.Stat(helperPath)
	if err != nil {
		t.Fatalf("Stat(helper) error = %v", err)
	}
	testInfo, err := os.Stat(testPath)
	if err != nil {
		t.Fatalf("Stat(test) error = %v", err)
	}
	writeFile(t, helperPath, `
public class CacheExactHelper {
  public static Integer value() { return 2; }
}
`)
	writeFile(t, testPath, `
@isTest
private class CacheExactTest {
  @isTest static void seesCurrentHelper() {
    System.assertEquals(2, CacheExactHelper.value());
  }
}
`)
	if err := os.Chtimes(helperPath, helperInfo.ModTime(), helperInfo.ModTime()); err != nil {
		t.Fatalf("Chtimes(helper) error = %v", err)
	}
	if err := os.Chtimes(testPath, testInfo.ModTime(), testInfo.ModTime()); err != nil {
		t.Fatalf("Chtimes(test) error = %v", err)
	}
	currentIndex := loadTestIndex(t, root)

	InvalidateRuntimeCaches()
	cachedPath := Run(currentIndex, Options{PerfCounters: true})
	cachedCounters := SnapshotPerfCounters().Phases
	if cachedCounters.CacheMisses != 1 || cachedCounters.DiskCacheHits != 0 || cachedCounters.MemoryCacheHits != 0 || cachedCounters.CacheValidateNS <= 0 || cachedCounters.CacheDecodeNS != 0 {
		t.Fatalf("invalidated disk-cache counters = %#v", cachedCounters)
	}

	InvalidateRuntimeCaches()
	noCachePath := Run(currentIndex, Options{PerfCounters: true, NoDiskCache: true})
	noCacheCounters := SnapshotPerfCounters().Phases
	if noCacheCounters.CacheMisses != 1 || noCacheCounters.DiskCacheHits != 0 || noCacheCounters.MemoryCacheHits != 0 || noCacheCounters.CacheValidateNS != 0 || noCacheCounters.CacheDecodeNS != 0 {
		t.Fatalf("no-cache counters = %#v", noCacheCounters)
	}

	normalizeDurations := func(run testreport.Run) testreport.Run {
		run.DurationMS = 0
		for suiteIndex := range run.Suites {
			run.Suites[suiteIndex].DurationMS = 0
			for caseIndex := range run.Suites[suiteIndex].Cases {
				run.Suites[suiteIndex].Cases[caseIndex].DurationMS = 0
			}
		}
		return run
	}
	if got, want := normalizeDurations(cachedPath), normalizeDurations(noCachePath); !reflect.DeepEqual(got, want) {
		t.Fatalf("invalidated disk-cache result != no-cache result:\ncache=%#v\nno-cache=%#v", got, want)
	}
	if summary := cachedPath.Summary(); summary.Total != 1 || summary.Passed != 1 {
		t.Fatalf("invalidated disk-cache summary = %#v cases = %#v", summary, cachedPath.Suites)
	}

	type runtimeInputMutation struct {
		name   string
		setup  func(t *testing.T, root, helperPath string)
		mutate func(t *testing.T, root, helperPath string)
		assert func(t *testing.T, runtime runtimeCacheEntry)
	}
	notificationPath := func(root string) string {
		return filepath.Join(root, "force-app", "main", "default", "notificationtypes", "Cache_Notice.notiftype-meta.xml")
	}
	dataPath := func(root string) string {
		return filepath.Join(root, "fixtures", "data", "CacheShape.json")
	}
	apexMeta := func(apiVersion string) string {
		return `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + apiVersion + `</apiVersion></ApexClass>`
	}
	notification := func(label string) string {
		return `<CustomNotificationType xmlns="http://soap.sforce.com/2006/04/metadata"><customNotifTypeName>` + label + `</customNotifTypeName></CustomNotificationType>`
	}
	data := func(field string) string {
		return `{"records":{"CacheShape__c":[{"` + field + `":"value"}]}}`
	}
	assertAPIVersion := func(want string) func(*testing.T, runtimeCacheEntry) {
		return func(t *testing.T, runtime runtimeCacheEntry) {
			t.Helper()
			for _, method := range runtime.Methods {
				if method.Name == "CacheInputHelper.value" {
					if method.APIVersion != want {
						t.Fatalf("CacheInputHelper.value APIVersion = %q, want %q", method.APIVersion, want)
					}
					return
				}
			}
			t.Fatalf("CacheInputHelper.value was not compiled: methods=%#v", runtime.Methods)
		}
	}
	assertNotification := func(wantLabel string, wantPresent bool) func(*testing.T, runtimeCacheEntry) {
		return func(t *testing.T, runtime runtimeCacheEntry) {
			t.Helper()
			state := runtime.restored.CloneOrg().Objects["CustomNotificationType"]
			present := recordWithFieldValueExists(state, "DeveloperName", "Cache_Notice")
			if present != wantPresent {
				t.Fatalf("CustomNotificationType Cache_Notice present = %v, want %v: records=%#v", present, wantPresent, state.Records)
			}
			if wantPresent && !recordWithFieldValueExists(state, "MasterLabel", wantLabel) {
				t.Fatalf("CustomNotificationType MasterLabel %q was not created: records=%#v", wantLabel, state.Records)
			}
		}
	}
	assertDataFields := func(wantPresent []string, wantAbsent []string) func(*testing.T, runtimeCacheEntry) {
		return func(t *testing.T, runtime runtimeCacheEntry) {
			t.Helper()
			fields := runtime.restored.CloneOrg().Objects["CacheShape__c"].Definition.Fields
			for _, name := range wantPresent {
				if _, ok := fields[name]; !ok {
					t.Fatalf("CacheShape__c.%s was not inferred: fields=%#v", name, fields)
				}
			}
			for _, name := range wantAbsent {
				if _, ok := fields[name]; ok {
					t.Fatalf("CacheShape__c.%s remained inferred: fields=%#v", name, fields)
				}
			}
		}
	}
	runtimeInputs := []runtimeInputMutation{
		{
			name: "add Apex metadata sidecar",
			mutate: func(t *testing.T, _ string, helperPath string) {
				writeFile(t, helperPath+"-meta.xml", apexMeta("61.0"))
			},
			assert: assertAPIVersion("61.0"),
		},
		{
			name: "edit Apex metadata sidecar",
			setup: func(t *testing.T, _ string, helperPath string) {
				writeFile(t, helperPath+"-meta.xml", apexMeta("61.0"))
			},
			mutate: func(t *testing.T, _ string, helperPath string) {
				writeFile(t, helperPath+"-meta.xml", apexMeta("62.0"))
			},
			assert: assertAPIVersion("62.0"),
		},
		{
			name: "delete Apex metadata sidecar",
			setup: func(t *testing.T, _ string, helperPath string) {
				writeFile(t, helperPath+"-meta.xml", apexMeta("61.0"))
			},
			mutate: func(t *testing.T, _ string, helperPath string) {
				if err := os.Remove(helperPath + "-meta.xml"); err != nil {
					t.Fatalf("Remove(Apex metadata sidecar) error = %v", err)
				}
			},
			assert: assertAPIVersion(""),
		},
		{
			name: "add notification type",
			mutate: func(t *testing.T, root, _ string) {
				writeFile(t, notificationPath(root), notification("Added Label"))
			},
			assert: assertNotification("Added Label", true),
		},
		{
			name: "edit notification type",
			setup: func(t *testing.T, root, _ string) {
				writeFile(t, notificationPath(root), notification("Before Label"))
			},
			mutate: func(t *testing.T, root, _ string) {
				writeFile(t, notificationPath(root), notification("After Label"))
			},
			assert: assertNotification("After Label", true),
		},
		{
			name: "delete notification type",
			setup: func(t *testing.T, root, _ string) {
				writeFile(t, notificationPath(root), notification("Deleted Label"))
			},
			mutate: func(t *testing.T, root, _ string) {
				if err := os.Remove(notificationPath(root)); err != nil {
					t.Fatalf("Remove(notification type) error = %v", err)
				}
			},
			assert: assertNotification("", false),
		},
		{
			name: "add project data JSON",
			mutate: func(t *testing.T, root, _ string) {
				writeFile(t, dataPath(root), data("Added__c"))
			},
			assert: assertDataFields([]string{"Added__c"}, nil),
		},
		{
			name: "edit project data JSON",
			setup: func(t *testing.T, root, _ string) {
				writeFile(t, dataPath(root), data("Before__c"))
			},
			mutate: func(t *testing.T, root, _ string) {
				writeFile(t, dataPath(root), data("After___c"))
			},
			assert: assertDataFields([]string{"After___c"}, []string{"Before__c"}),
		},
		{
			name: "delete project data JSON",
			setup: func(t *testing.T, root, _ string) {
				writeFile(t, dataPath(root), data("Deleted__c"))
			},
			mutate: func(t *testing.T, root, _ string) {
				if err := os.Remove(dataPath(root)); err != nil {
					t.Fatalf("Remove(project data JSON) error = %v", err)
				}
			},
			assert: assertDataFields(nil, []string{"Deleted__c"}),
		},
	}
	for _, tc := range runtimeInputs {
		t.Run(tc.name, func(t *testing.T) {
			ResetPerfCounters()
			InvalidateRuntimeCaches()
			root := t.TempDir()
			helperPath := filepath.Join(root, "force-app", "main", "default", "classes", "CacheInputHelper.cls")
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, helperPath, `
public class CacheInputHelper {
  public static Integer value() { return 1; }
}
`)
			writeFile(t, filepath.Join(root, "force-app", "main", "default", "classes", "CacheInputTest.cls"), `
@isTest
private class CacheInputTest {
  @isTest static void passes() {
    System.assertEquals(1, CacheInputHelper.value());
  }
}
`)
			if tc.setup != nil {
				tc.setup(t, root, helperPath)
			}

			initial := Run(loadTestIndex(t, root), Options{PerfCounters: true})
			if summary := initial.Summary(); summary.Total != 1 || summary.Passed != 1 {
				t.Fatalf("initial summary = %#v cases = %#v", summary, initial.Suites)
			}
			initialCounters := SnapshotPerfCounters().Phases
			if initialCounters.CacheMisses != 1 || initialCounters.DiskCacheHits != 0 || initialCounters.CacheEncodeNS <= 0 {
				t.Fatalf("initial counters = %#v", initialCounters)
			}

			tc.mutate(t, root, helperPath)
			currentIndex := loadTestIndex(t, root)
			InvalidateRuntimeCaches()
			cachedPath := Run(currentIndex, Options{PerfCounters: true})
			cachedCounters := SnapshotPerfCounters().Phases
			if cachedCounters.CacheMisses != 1 || cachedCounters.DiskCacheHits != 0 || cachedCounters.MemoryCacheHits != 0 || cachedCounters.CacheValidateNS <= 0 || cachedCounters.CacheDecodeNS != 0 {
				t.Fatalf("invalidated disk-cache counters = %#v", cachedCounters)
			}
			runtimeCacheMu.RLock()
			cachedRuntime, ok := runtimeCache[runtimeKey(currentIndex)]
			runtimeCacheMu.RUnlock()
			if !ok {
				t.Fatal("invalidated disk-cache runtime was not retained in memory")
			}
			cachedRuntime = cloneRuntimeCacheEntry(cachedRuntime)

			InvalidateRuntimeCaches()
			noCachePath := Run(currentIndex, Options{PerfCounters: true, NoDiskCache: true})
			noCacheCounters := SnapshotPerfCounters().Phases
			if noCacheCounters.CacheMisses != 1 || noCacheCounters.DiskCacheHits != 0 || noCacheCounters.MemoryCacheHits != 0 || noCacheCounters.CacheValidateNS != 0 || noCacheCounters.CacheDecodeNS != 0 {
				t.Fatalf("no-cache counters = %#v", noCacheCounters)
			}
			runtimeCacheMu.RLock()
			noCacheRuntime, ok := runtimeCache[runtimeKey(currentIndex)]
			runtimeCacheMu.RUnlock()
			if !ok {
				t.Fatal("no-cache runtime was not retained in memory")
			}
			noCacheRuntime = cloneRuntimeCacheEntry(noCacheRuntime)

			if got, want := normalizeDurations(cachedPath), normalizeDurations(noCachePath); !reflect.DeepEqual(got, want) {
				t.Fatalf("invalidated disk-cache result != no-cache result:\ncache=%#v\nno-cache=%#v", got, want)
			}
			cachedOrg := cachedRuntime.restored.CloneOrg()
			noCacheOrg := noCacheRuntime.restored.CloneOrg()
			if cachedOrg.OrgID != noCacheOrg.OrgID ||
				cachedOrg.APIVersion != noCacheOrg.APIVersion ||
				cachedOrg.Namespace != noCacheOrg.Namespace {
				t.Fatalf("invalidated disk-cache org identity = (%q, %q, %q), no-cache = (%q, %q, %q)",
					cachedOrg.OrgID, cachedOrg.APIVersion, cachedOrg.Namespace,
					noCacheOrg.OrgID, noCacheOrg.APIVersion, noCacheOrg.Namespace)
			}
			if !reflect.DeepEqual(cachedRuntime.Methods, noCacheRuntime.Methods) {
				t.Fatalf("invalidated disk-cache methods != no-cache methods:\ncache=%#v\nno-cache=%#v", cachedRuntime.Methods, noCacheRuntime.Methods)
			}
			if summary := cachedPath.Summary(); summary.Total != 1 || summary.Passed != 1 {
				t.Fatalf("invalidated disk-cache summary = %#v cases = %#v", summary, cachedPath.Suites)
			}
			tc.assert(t, cachedRuntime)
			tc.assert(t, noCacheRuntime)
		})
	}
}

func TestDiskStartupCacheInvalidatesLoadedDependencyData(t *testing.T) {
	wasDiskCacheDisabled := disableDiskCache.Load()
	disableDiskCache.Store(false)
	t.Cleanup(func() {
		disableDiskCache.Store(wasDiskCacheDisabled)
		InvalidateRuntimeCaches()
		ResetPerfCounters()
	})
	dataQuery := func(recordTypeName string) string {
		return `{"query":"SELECT Id FROM RecordType WHERE SObjectType = 'pkg__Product__c' AND Name = '` + recordTypeName + `'"}`
	}
	type mutationCase struct {
		name       string
		initial    string
		mutate     func(t *testing.T, path string)
		want       string
		wantAbsent string
	}
	cases := []mutationCase{
		{
			name: "add",
			mutate: func(t *testing.T, path string) {
				writeFile(t, path, dataQuery("Added Plan"))
			},
			want: "Added Plan",
		},
		{
			name:    "edit",
			initial: dataQuery("Before Plan"),
			mutate: func(t *testing.T, path string) {
				writeFile(t, path, dataQuery("After Plan"))
			},
			want:       "After Plan",
			wantAbsent: "Before Plan",
		},
		{
			name:    "delete",
			initial: dataQuery("Deleted Plan"),
			mutate: func(t *testing.T, path string) {
				if err := os.Remove(path); err != nil {
					t.Fatalf("Remove(dependency data) error = %v", err)
				}
			},
			wantAbsent: "Deleted Plan",
		},
	}
	normalizeDurations := func(run testreport.Run) testreport.Run {
		run.DurationMS = 0
		for suiteIndex := range run.Suites {
			run.Suites[suiteIndex].DurationMS = 0
			for caseIndex := range run.Suites[suiteIndex].Cases {
				run.Suites[suiteIndex].Cases[caseIndex].DurationMS = 0
			}
		}
		return run
	}
	recordTypeExists := func(runtime runtimeCacheEntry, label string) bool {
		for _, recordType := range runtime.restored.CloneOrg().Objects["pkg__Product__c"].Definition.RecordTypes {
			if strings.EqualFold(recordType.Name, label) && strings.EqualFold(recordType.DeveloperName, recordTypeDeveloperNameFromLabel(label)) {
				return true
			}
		}
		return false
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ResetPerfCounters()
			InvalidateRuntimeCaches()
			root := t.TempDir()
			consumerRoot := filepath.Join(root, "consumer")
			dependencyRoot := filepath.Join(root, "dependency")
			dataPath := filepath.Join(dependencyRoot, "fixtures", "data", "RecordTypes.json")
			writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, filepath.Join(consumerRoot, "glade.yml"), "project:\n  managedPackageDependencies: [\"pkg:../dependency\"]\n")
			writeFile(t, filepath.Join(consumerRoot, "force-app", "main", "default", "classes", "DependencyDataTest.cls"), `
@isTest private class DependencyDataTest {
  @isTest static void passes() { System.assertEquals(1, 1); }
}
`)
			writeFile(t, filepath.Join(dependencyRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"pkg"}`)
			writeFile(t, filepath.Join(dependencyRoot, "force-app", "main", "default", "objects", "Product__c", "Product__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Product</label></CustomObject>`)
			if tc.initial != "" {
				writeFile(t, dataPath, tc.initial)
			}

			initial := Run(loadTestIndex(t, consumerRoot), Options{PerfCounters: true})
			if summary := initial.Summary(); summary.Total != 1 || summary.Passed != 1 {
				t.Fatalf("initial summary = %#v cases = %#v", summary, initial.Suites)
			}
			initialCounters := SnapshotPerfCounters().Phases
			if initialCounters.CacheMisses != 1 || initialCounters.DiskCacheHits != 0 || initialCounters.CacheEncodeNS <= 0 {
				t.Fatalf("initial counters = %#v", initialCounters)
			}

			tc.mutate(t, dataPath)
			currentIndex := loadTestIndex(t, consumerRoot)
			InvalidateRuntimeCaches()
			cachedPath := Run(currentIndex, Options{PerfCounters: true})
			cachedCounters := SnapshotPerfCounters().Phases
			if cachedCounters.CacheMisses != 1 || cachedCounters.DiskCacheHits != 0 || cachedCounters.MemoryCacheHits != 0 || cachedCounters.CacheValidateNS <= 0 || cachedCounters.CacheDecodeNS != 0 {
				t.Fatalf("invalidated disk-cache counters = %#v", cachedCounters)
			}
			runtimeCacheMu.RLock()
			cachedRuntime, ok := runtimeCache[runtimeKey(currentIndex)]
			runtimeCacheMu.RUnlock()
			if !ok {
				t.Fatal("invalidated disk-cache runtime was not retained in memory")
			}
			cachedRuntime = cloneRuntimeCacheEntry(cachedRuntime)

			InvalidateRuntimeCaches()
			noCachePath := Run(currentIndex, Options{PerfCounters: true, NoDiskCache: true})
			noCacheCounters := SnapshotPerfCounters().Phases
			if noCacheCounters.CacheMisses != 1 || noCacheCounters.DiskCacheHits != 0 || noCacheCounters.MemoryCacheHits != 0 || noCacheCounters.CacheValidateNS != 0 || noCacheCounters.CacheDecodeNS != 0 {
				t.Fatalf("no-cache counters = %#v", noCacheCounters)
			}
			runtimeCacheMu.RLock()
			noCacheRuntime, ok := runtimeCache[runtimeKey(currentIndex)]
			runtimeCacheMu.RUnlock()
			if !ok {
				t.Fatal("no-cache runtime was not retained in memory")
			}
			noCacheRuntime = cloneRuntimeCacheEntry(noCacheRuntime)

			if got, want := normalizeDurations(cachedPath), normalizeDurations(noCachePath); !reflect.DeepEqual(got, want) {
				t.Fatalf("invalidated disk-cache result != no-cache result:\ncache=%#v\nno-cache=%#v", got, want)
			}
			cachedRecordTypes := cachedRuntime.restored.CloneOrg().Objects["pkg__Product__c"].Definition.RecordTypes
			noCacheRecordTypes := noCacheRuntime.restored.CloneOrg().Objects["pkg__Product__c"].Definition.RecordTypes
			if !reflect.DeepEqual(cachedRecordTypes, noCacheRecordTypes) {
				t.Fatalf("invalidated disk-cache record types != no-cache record types:\ncache=%#v\nno-cache=%#v", cachedRecordTypes, noCacheRecordTypes)
			}
			for name, runtime := range map[string]runtimeCacheEntry{"cache": cachedRuntime, "no-cache": noCacheRuntime} {
				if tc.want != "" && !recordTypeExists(runtime, tc.want) {
					t.Fatalf("%s runtime lacks dependency data record type %q: %#v", name, tc.want, runtime.restored.CloneOrg().Objects["pkg__Product__c"].Definition.RecordTypes)
				}
				if tc.wantAbsent != "" && recordTypeExists(runtime, tc.wantAbsent) {
					t.Fatalf("%s runtime retained dependency data record type %q: %#v", name, tc.wantAbsent, runtime.restored.CloneOrg().Objects["pkg__Product__c"].Definition.RecordTypes)
				}
			}
		})
	}
}

func TestRunPerfCountersSeparateMemoryDiskAndBuildPaths(t *testing.T) {
	ResetPerfCounters()
	t.Cleanup(ResetPerfCounters)
	InvalidateRuntimeCaches()
	wasDiskCacheDisabled := disableDiskCache.Load()
	disableDiskCache.Store(false)
	t.Cleanup(func() { disableDiskCache.Store(wasDiskCacheDisabled) })
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/CachePhaseTest.cls"), `
@isTest
private class CachePhaseTest {
  @isTest static void passes() { System.assertEquals(1, 1); }
}
`)
	index := loadTestIndex(t, root)
	if _, err := project.Load(root); err != nil {
		t.Fatalf("project.Load() error = %v", err)
	}

	Run(index, Options{PerfCounters: true})
	build := SnapshotPerfCounters().Phases
	if build.CacheMisses != 1 || build.OrgBuildNS <= 0 || build.ProjectCompileNS <= 0 || build.CacheEncodeNS <= 0 {
		t.Fatalf("build phases = %#v", build)
	}

	Run(index, Options{PerfCounters: true})
	memory := SnapshotPerfCounters().Phases
	if memory.MemoryCacheHits != 1 || memory.DiskCacheHits != 0 || memory.CacheMisses != 0 {
		t.Fatalf("memory phases = %#v", memory)
	}

	InvalidateRuntimeCaches()
	Run(index, Options{PerfCounters: true})
	disk := SnapshotPerfCounters().Phases
	if disk.DiskCacheHits != 1 || disk.MemoryCacheHits != 0 || disk.CacheValidateNS <= 0 || disk.CacheDecodeNS <= 0 || disk.CacheMisses != 0 {
		t.Fatalf("disk phases = %#v", disk)
	}
}

func TestRunPerfCountersAttributeDiskMaterializationPhases(t *testing.T) {
	ResetPerfCounters()
	t.Cleanup(ResetPerfCounters)
	InvalidateRuntimeCaches()
	wasDiskCacheDisabled := disableDiskCache.Load()
	disableDiskCache.Store(false)
	t.Cleanup(func() { disableDiskCache.Store(wasDiskCacheDisabled) })
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/DiskMaterializationTest.cls"), `
@isTest private class DiskMaterializationTest {
  @isTest static void passes() { System.assertEquals(1, 1); }
}
`)
	index := loadTestIndex(t, root)
	Run(index, Options{PerfCounters: true})
	InvalidateRuntimeCaches()

	counters := newRunPerfCounters(true)
	_, _ = runtimeFromIndexWithPerf(index, newSourceCache(), true, counters)
	phases := snapshotPerfCounters(counters).Phases
	if phases.DiskCacheHits != 1 || phases.CacheDecodeNS <= 0 {
		t.Fatalf("disk cache phases = %#v", phases)
	}
	if phases.ProjectCompileNS <= 0 || phases.OrgBuildNS <= 0 {
		t.Fatalf("disk materialization phases = %#v, want project compile and org build", phases)
	}
}

func TestRunPerfCountersOwnVMRecordersAcrossOverlapAndPostRunActivity(t *testing.T) {
	ResetPerfCounters()
	t.Cleanup(ResetPerfCounters)
	makeRun := func(name, mutations string) (typesys.Index, []TestCase) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
		writeFile(t, filepath.Join(root, "force-app/main/classes", name+".cls"), `
@isTest private class `+name+` {
  @isTest static void aliases() {
    List<Integer> values = new List<Integer>();
    List<List<Integer>> roots = new List<List<Integer>>{values};
    `+mutations+`
    System.assertEquals(values.size(), roots[0].size());
  }
}
`)
		index := loadTestIndex(t, root)
		return index, Discover(index, Options{})
	}
	indexA, casesA := makeRun("OwnedRecorderATest", "values.add(1);")
	indexB, casesB := makeRun("OwnedRecorderBTest", "values.add(1); values.add(2);")
	normalize := func(stats vm.ScopeAliasPerfCounters) vm.ScopeAliasPerfCounters {
		stats.ContainmentNS = 0
		stats.ReplacementNS = 0
		stats.DurationNS = 0
		return stats
	}
	runAndSnapshot := func(index typesys.Index, cases []TestCase, opts Options) vm.ScopeAliasPerfCounters {
		InvalidateRuntimeCaches()
		run := RunCasesContext(context.Background(), index, opts, cases)
		if summary := run.Summary(); summary.Failed != 0 || summary.Unsupported != 0 {
			t.Fatalf("run summary = %#v suites = %#v", summary, run.Suites)
		}
		return normalize(SnapshotPerfCounters().VMPerf.ScopeAlias)
	}
	expectedA := runAndSnapshot(indexA, casesA, Options{PerfCounters: true, NoDiskCache: true})
	expectedB := runAndSnapshot(indexB, casesB, Options{PerfCounters: true, NoDiskCache: true})
	if expectedA.Calls == 0 || expectedB.Calls == 0 {
		t.Fatalf("fixtures did not exercise scope alias instrumentation: A=%#v B=%#v", expectedA, expectedB)
	}

	InvalidateRuntimeCaches()
	startedA := make(chan struct{})
	releaseA := make(chan struct{})
	doneA := make(chan testreport.Run, 1)
	var blockOnce sync.Once
	go func() {
		doneA <- RunCasesContext(context.Background(), indexA, Options{
			PerfCounters: true,
			NoDiskCache:  true,
			Progress: func(progress TestProgress) {
				if progress.Event == "test_start" {
					blockOnce.Do(func() { close(startedA) })
					<-releaseA
				}
			},
		}, casesA)
	}()
	<-startedA
	runB := RunCasesContext(context.Background(), indexB, Options{PerfCounters: true, NoDiskCache: true}, casesB)
	if summary := runB.Summary(); summary.Failed != 0 || summary.Unsupported != 0 {
		t.Fatalf("overlapping B summary = %#v suites = %#v", summary, runB.Suites)
	}
	gotB := normalize(SnapshotPerfCounters().VMPerf.ScopeAlias)
	close(releaseA)
	runA := <-doneA
	if summary := runA.Summary(); summary.Failed != 0 || summary.Unsupported != 0 {
		t.Fatalf("overlapping A summary = %#v suites = %#v", summary, runA.Suites)
	}
	gotA := normalize(SnapshotPerfCounters().VMPerf.ScopeAlias)
	if !reflect.DeepEqual(gotA, expectedA) || !reflect.DeepEqual(gotB, expectedB) {
		t.Fatalf("overlapping recorder counts mixed: gotA=%#v wantA=%#v gotB=%#v wantB=%#v", gotA, expectedA, gotB, expectedB)
	}

	beforeReset := SnapshotPerfCounters()
	vm.ResetPerfCounters()
	if afterReset := SnapshotPerfCounters(); !reflect.DeepEqual(afterReset, beforeReset) {
		t.Fatalf("post-run VM activity changed published snapshot:\nbefore=%#v\nafter=%#v", beforeReset.VMPerf, afterReset.VMPerf)
	}
}

func TestRunDoesNotResetCompatibilityStorageCloneStats(t *testing.T) {
	storage.ResetCloneStats()
	t.Cleanup(storage.ResetCloneStats)
	_ = storage.NewOrgState().CloneRollbackSnapshot()
	prior := storage.SnapshotCloneStats()
	if prior.CloneRuntimeCalls == 0 || prior.CloneRollbackSnapshotCalls == 0 {
		t.Fatalf("precondition clone stats = %#v, want real runtime and rollback clones", prior)
	}

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/StorageCompatibilityTest.cls"), `
@isTest private class StorageCompatibilityTest {
  @isTest static void passes() { System.assertEquals(1, 1); }
}
`)
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
		t.Fatalf("summary = %#v suites = %#v", summary, run.Suites)
	}
	after := storage.SnapshotCloneStats()
	if after.CloneRuntimeCalls < prior.CloneRuntimeCalls || after.CloneRollbackSnapshotCalls < prior.CloneRollbackSnapshotCalls {
		t.Fatalf("runner cleared compatibility clone stats: before=%#v after=%#v", prior, after)
	}
}

func TestRunPerfCountersPreserveSuiteAndCaseOrder(t *testing.T) {
	ResetPerfCounters()
	t.Cleanup(ResetPerfCounters)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/AlphaOrderTest.cls"), `
@isTest private class AlphaOrderTest {
  @isTest static void first() {}
  @isTest static void second() {}
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BetaOrderTest.cls"), `
@isTest private class BetaOrderTest {
  @isTest static void only() {}
}
`)
	index := loadTestIndex(t, root)
	cases := Discover(index, Options{})

	InvalidateRuntimeCaches()
	without := RunCasesContext(context.Background(), index, Options{NoDiskCache: true}, cases)
	if stats := SnapshotPerfCounters(); stats.Enabled || stats.Phases != (RunnerPhasePerfCounters{}) || len(stats.CloneReasons) != 0 || !reflect.DeepEqual(stats.VMPerf, vm.PerfCounters{}) {
		t.Fatalf("disabled runner detailed snapshot = %#v, want zero phases/reasons/VM", stats)
	}
	InvalidateRuntimeCaches()
	with := RunCasesContext(context.Background(), index, Options{PerfCounters: true, NoDiskCache: true}, cases)
	identities := func(run testreport.Run) []string {
		var out []string
		for _, suite := range run.Suites {
			for _, testCase := range suite.Cases {
				problem := ""
				if testCase.Problem != nil {
					problem = testCase.Problem.Type + ":" + testCase.Problem.Message
				}
				out = append(out, suite.Name+"/"+testCase.ClassName+"."+testCase.MethodName+":"+string(testCase.Status)+":"+problem)
			}
		}
		return out
	}
	if got, want := strings.Join(identities(with), "\n"), strings.Join(identities(without), "\n"); got != want {
		t.Fatalf("instrumented run changed identities/status/order:\ngot=%s\nwant=%s", got, want)
	}
	normalizeDurations := func(run testreport.Run) testreport.Run {
		run.DurationMS = 0
		run.Suites = append([]testreport.Suite(nil), run.Suites...)
		for suiteIndex := range run.Suites {
			run.Suites[suiteIndex].DurationMS = 0
			run.Suites[suiteIndex].Cases = append([]testreport.Case(nil), run.Suites[suiteIndex].Cases...)
			for caseIndex := range run.Suites[suiteIndex].Cases {
				run.Suites[suiteIndex].Cases[caseIndex].DurationMS = 0
			}
		}
		return run
	}
	withoutJSON, err := json.Marshal(normalizeDurations(without))
	if err != nil {
		t.Fatal(err)
	}
	withJSON, err := json.Marshal(normalizeDurations(with))
	if err != nil {
		t.Fatal(err)
	}
	if string(withJSON) != string(withoutJSON) {
		t.Fatalf("instrumented run changed JSON object data:\ngot=%s\nwant=%s", withJSON, withoutJSON)
	}
}

func TestRunPerfCountersReportAssemblyAndHistoryApply(t *testing.T) {
	ResetPerfCounters()
	t.Cleanup(ResetPerfCounters)
	InvalidateRuntimeCaches()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/HistoryOneTest.cls"), `@isTest private class HistoryOneTest { @isTest static void one() {} }`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/HistoryTwoTest.cls"), `@isTest private class HistoryTwoTest { @isTest static void two() {} }`)

	run := Run(loadTestIndex(t, root), Options{
		PerfCounters:    true,
		NoDiskCache:     true,
		Parallelism:     2,
		ClassDurationMS: map[string]int64{"HistoryOneTest": 1, "HistoryTwoTest": 2},
	})
	if summary := run.Summary(); summary.Total != 2 || summary.Passed != 2 {
		t.Fatalf("summary = %#v suites = %#v", summary, run.Suites)
	}
	phases := SnapshotPerfCounters().Phases
	if phases.HistoryApplyNS <= 0 || phases.ReportAssemblyNS <= 0 {
		t.Fatalf("history/report phases = %#v", phases)
	}
}

func TestClassJournalDisabledForMergeDML(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "MergeJournalTest.cls")
	writeFile(t, file, `
@isTest
private class MergeJournalTest {
  @isTest static void first() {
    merge new Account(Id = '001000000000001') new Account(Id = '001000000000002');
  }
  @isTest static void second() {}
}
`)
	planned := []testCasePlan{
		{TestCase: TestCase{ClassName: "MergeJournalTest", MethodName: "first", File: file}},
		{TestCase: TestCase{ClassName: "MergeJournalTest", MethodName: "second", File: file}},
	}
	if classSupportsJournalIsolation([]int{0, 1}, planned) {
		t.Fatalf("merge DML class should use per-method org clones")
	}
}

func TestClassJournalDisabledForMetadataDeployment(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "MetadataJournalTest.cls")
	writeFile(t, file, `
@isTest
private class MetadataJournalTest {
  @isTest static void first() {
    Metadata.DeployContainer container = new Metadata.DeployContainer();
  }
  @isTest static void second() {}
}
`)
	planned := []testCasePlan{
		{TestCase: TestCase{ClassName: "MetadataJournalTest", MethodName: "first", File: file}},
		{TestCase: TestCase{ClassName: "MetadataJournalTest", MethodName: "second", File: file}},
	}
	if classSupportsJournalIsolation([]int{0, 1}, planned) {
		t.Fatalf("metadata deployment class should use per-method org clones")
	}
}

func TestClassJournalIsolationProbeReadsEachFileOnce(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "ReadOnceJournalTest.cls")
	writeFile(t, file, `
@isTest
private class ReadOnceJournalTest {
  @isTest static void first() {}
  @isTest static void second() {}
  @isTest static void third() {}
}
`)
	planned := []testCasePlan{
		{TestCase: TestCase{ClassName: "ReadOnceJournalTest", MethodName: "first", File: file}},
		{TestCase: TestCase{ClassName: "ReadOnceJournalTest", MethodName: "second", File: file}},
		{TestCase: TestCase{ClassName: "ReadOnceJournalTest", MethodName: "third", File: file}},
	}
	reads := 0
	cache := newClassIsolationProbeCache(func(path string) ([]byte, error) {
		reads++
		return os.ReadFile(path)
	})

	if !cache.supportsJournalIsolation([]int{0, 1, 2}, planned) {
		t.Fatalf("class should support journal isolation")
	}
	if reads != 1 {
		t.Fatalf("source reads = %d, want 1", reads)
	}
}

func TestRunKeepsPageParametersAndDynamicSelectorBinds(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/Template__c/Template__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Template</label><pluralLabel>Templates</pluralLabel><nameField><type>Text</type><label>Name</label></nameField></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/Template__c/fields/SOQLQuery__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>SOQLQuery__c</fullName><label>SOQL Query</label><type>LongTextArea</type></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/Template__c/fields/TemplateSource__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>TemplateSource__c</fullName><label>Template Source</label><type>LongTextArea</type></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/TemplateSelector.cls"), `
public class TemplateSelector {
  public static Template__c selectById(Id recordId) {
    Set<Id> escapedIdSet = new Set<Id>{ recordId };
    List<Template__c> rows = Database.query('SELECT Id, Name, SOQLQuery__c, TemplateSource__c FROM Template__c WHERE Id IN :escapedIdSet');
    if (rows.isEmpty()) {
      return null;
    }
    return rows[0];
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/TemplateController.cls"), `
public class TemplateController {
  public String Source { get; private set; }
  public String Query { get; private set; }
  public TemplateController() {
    Id templateId = ApexPages.currentPage().getParameters().get('templateId');
    Template__c row = TemplateSelector.selectById(templateId);
    if (row != null) {
      Source = row.TemplateSource__c;
      Query = row.SOQLQuery__c;
    }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/TemplateControllerTest.cls"), `
@isTest
private class TemplateControllerTest {
  @isTest static void loadsFromCurrentPageParameter() {
    Template__c tpl = new Template__c(Name = 'T', SOQLQuery__c = 'SELECT Id FROM Account', TemplateSource__c = 'Hello');
    insert tpl;
    PageReference pageRef = new PageReference('/apex/Template');
    Test.setCurrentPage(pageRef);
    ApexPages.currentPage().getParameters().put('templateId', tpl.Id);
    TemplateController controller = new TemplateController();
    System.assertEquals('Hello', controller.Source);
    System.assertEquals('SELECT Id FROM Account', controller.Query);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
		var problem *testreport.Problem
		if len(run.Suites) > 0 && len(run.Suites[0].Cases) > 0 {
			problem = run.Suites[0].Cases[0].Problem
		}
		t.Fatalf("summary = %#v problem = %#v cases = %#v", summary, problem, run.Suites[0].Cases)
	}
}

func TestRunAllowsListReturnForIterableObject(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/IterableBatch.cls"), `
public class IterableBatch {
  private List<Account> records;
  public IterableBatch(List<Account> records) {
    this.records = records;
  }
  public Iterable<Object> start() {
    return this.records;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/IterableBatchTest.cls"), `
@isTest
private class IterableBatchTest {
  @isTest static void listSatisfiesIterableObject() {
    List<Account> records = new List<Account>{ new Account(Name = 'Acme') };
    Iterable<Object> items = new IterableBatch(records).start();
    Integer count = 0;
    for (Object item : items) {
      count++;
    }
    System.assertEquals(1, count);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
		if len(run.Suites) > 0 && len(run.Suites[0].Cases) > 0 {
			t.Fatalf("summary = %#v problem=%#v", summary, run.Suites[0].Cases[0].Problem)
		}
		t.Fatalf("summary = %#v suites = %#v", summary, run.Suites)
	}
}

func TestRunDispatchesGeneratedProductCallbackImplementations(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/CommerceResolver.cls"), `
public class CommerceResolver implements CommerceExtension.ResolutionStrategy {
  public CommerceExtension.Resolution resolve() {
    return new CommerceExtension.Resolution(CommerceExtension.ResolutionStates.OFF);
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ReadinessEvaluator.cls"), `
public class ReadinessEvaluator implements Readiness.ProductEvaluator {
  public Boolean isActive() {
    return true;
  }
  public List<Readiness.ProductScoreDetail> evaluateReadiness(Readiness.ProductEvaluationContext ctx) {
    return new List<Readiness.ProductScoreDetail>{
      new Readiness.ProductScoreDetail('01t000000000001AAA', 'local-rule', 100, 'ready')
    };
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ProductCallbackTest.cls"), `
@isTest
private class ProductCallbackTest {
  @isTest static void userCallbacksDispatchLocally() {
    CommerceExtension.ResolutionStrategy strategy = new CommerceResolver();
    System.assertEquals(CommerceExtension.ResolutionStates.OFF, strategy.resolve().getResolutionState());

    Readiness.ProductEvaluator evaluator = new ReadinessEvaluator();
    System.assertEquals(true, evaluator.isActive());
    Readiness.ProductEvaluationContext context =
      new Readiness.ProductEvaluationContext(new Set<Id>{ '01t000000000001AAA' });
    List<Readiness.ProductScoreDetail> scores = evaluator.evaluateReadiness(context);
    System.assertEquals(1, scores.size());
    System.assertEquals('local-rule', scores[0].getRuleName());
    System.assertEquals(100, scores[0].getRuleScore());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
		if len(run.Suites) > 0 && len(run.Suites[0].Cases) > 0 {
			t.Fatalf("summary = %#v problem=%#v", summary, run.Suites[0].Cases[0].Problem)
		}
		t.Fatalf("summary = %#v suites = %#v", summary, run.Suites)
	}
}

func TestRuntimeEvaluatesTemplateLexemsWithInnerClassGaps(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MergeValues.cls"), `
public class MergeValues {
  private Map<String, Object> values = new Map<String, Object>();
  public MergeValues(Map<String, Object> values) {
    this.values.putAll(values);
  }
  public Object get(String path) {
    return values.get(path);
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/TemplateEvaluator.cls"), `
public class TemplateEvaluator {
  private static final Pattern MERGE_FIELD_PATTERN = Pattern.compile('\\{!([\\w\\.]+)\\}');
  public final String content;
  private Object[] lexems;
  private TemplateEvaluator(String content) {
    this.content = content;
  }
  public static TemplateEvaluator newInstance(String content) {
    return new TemplateEvaluator(content);
  }
  public String evaluate(Map<String, Object> values) {
    compile();
    String buffer = '';
    MergeValues bag = new MergeValues(values);
    for (Object lexem : lexems) {
      Object value = evaluate(lexem, bag);
      buffer += value == null ? '' : String.valueOf(value);
    }
    return buffer;
  }
  private void compile() {
    lexems = new List<Object>();
    Matcher contentMatcher = MERGE_FIELD_PATTERN.matcher(content);
    Integer processedEnd = 0;
    while (contentMatcher.find()) {
      if (processedEnd < contentMatcher.start()) {
        lexems.add(content.substring(processedEnd, contentMatcher.start()));
      }
      lexems.add(new Gap(contentMatcher.group(1)));
      processedEnd = contentMatcher.end();
    }
    if (processedEnd < content.length()) {
      lexems.add(content.substring(processedEnd));
    }
  }
  private static Object evaluate(Object lexem, MergeValues values) {
    if (lexem instanceof String) {
      return lexem;
    }
    if (lexem instanceof Gap) {
      return values.get(((Gap)lexem).key);
    }
    return null;
  }
  private class Gap {
    public final String key;
    Gap(String key) {
      this.key = key;
    }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/TemplateEvaluatorTest.cls"), `
@isTest
private class TemplateEvaluatorTest {
  @isTest static void evaluatesGaps() {
    String result = TemplateEvaluator.newInstance('-start-{!valueA}-inner-{!valueB}-end-')
      .evaluate(new Map<String, Object>{ 'valueA' => 'A', 'valueB' => 'B' });
    System.assertEquals('-start-A-inner-B-end-', result);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
		var problem *testreport.Problem
		if len(run.Suites) > 0 && len(run.Suites[0].Cases) > 0 {
			problem = run.Suites[0].Cases[0].Problem
		}
		t.Fatalf("summary = %#v problem = %#v cases = %#v", summary, problem, run.Suites[0].Cases)
	}
}

func TestRuntimeInstanceOfUsesConcreteRuntimeTypeForInterfaceValues(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MatcherProbe.cls"), `
public class MatcherProbe {
  public interface IMatcher {
    Boolean matches(Object value);
  }
  public class Captor implements IMatcher {
    public Object value;
    public Boolean matches(Object value) {
      this.value = value;
      return true;
    }
    public void store(List<Object> values) {
      values.add(value);
    }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MatcherProbeTest.cls"), `
@isTest
private class MatcherProbeTest {
  @isTest static void interfaceValueKeepsRuntimeType() {
    MatcherProbe.IMatcher original = new MatcherProbe.Captor();
    List<MatcherProbe.IMatcher> matchers = new List<MatcherProbe.IMatcher>{ original };
    System.assert(original === matchers[0], 'list literal should keep object identity');
    List<MatcherProbe.IMatcher> cloned = matchers.clone();
    System.assert(original === cloned[0], 'List.clone should be shallow');
    List<Object> values = new List<Object>();
    for (MatcherProbe.IMatcher matcher : cloned) {
      System.assert(matcher instanceof MatcherProbe.Captor);
      matcher.matches('Fred');
      ((MatcherProbe.Captor)matcher).store(values);
    }
    System.assertEquals('Fred', (String)values[0]);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
		var problem *testreport.Problem
		if len(run.Suites) > 0 && len(run.Suites[0].Cases) > 0 {
			problem = run.Suites[0].Cases[0].Problem
		}
		t.Fatalf("summary = %#v problem = %#v cases = %#v", summary, problem, run.Suites[0].Cases)
	}
}

func TestRuntimeSObjectMatcherTreatsUnqueriedAuditFieldAsMismatch(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/TrailMatcherDefinitions.cls"), `
public class TrailMatcherDefinitions {
  public class SObjectWith {
    private Map<Schema.SObjectField, Object> toMatch;
    public SObjectWith(Map<Schema.SObjectField, Object> toMatch) {
      this.toMatch = toMatch;
    }
    public Boolean matches(Object arg) {
      if (arg != null && arg instanceof SObject) {
        SObject record = (SObject)arg;
        for (Schema.SObjectField fieldToken : this.toMatch.keySet()) {
          Object valueToMatch = this.toMatch.get(fieldToken);
          try {
            if (record.get(fieldToken) != valueToMatch) {
              return false;
            }
          } catch (Exception e) {
            return false;
          }
        }
        return true;
      }
      return false;
    }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/TrailMatcherDefinitionsTest.cls"), `
@IsTest
private class TrailMatcherDefinitionsTest {
  @IsTest static void unqueriedAuditFieldIsMismatch() {
    SObject inserted = Account.SObjectType.newSObject();
    inserted.put('Name', 'Acme');
    insert inserted;
    Id accountId = inserted.Id;
    SObject queried = Database.query('SELECT Id, Name FROM Account WHERE Id = :accountId LIMIT 1');
    Map<String, Schema.SObjectField> fields = Account.SObjectType.getDescribe().fields.getMap();
    Map<Schema.SObjectField, Object> expected = new Map<Schema.SObjectField, Object>{
      fields.get('CreatedDate') => System.now()
    };
    System.assert(!new TrailMatcherDefinitions.SObjectWith(expected).matches(queried));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
		var problem *testreport.Problem
		if len(run.Suites) > 0 && len(run.Suites[0].Cases) > 0 {
			problem = run.Suites[0].Cases[0].Problem
		}
		t.Fatalf("summary = %#v problem = %#v cases = %#v", summary, problem, run.Suites[0].Cases)
	}
}

func TestExtractMethodBodyHandlesBackslashEscapedApexStrings(t *testing.T) {
	source := `@IsTest
private class DataRequestTest {
    @IsTest
    private static void setParam1_validParams_expectSet() {
        List<String> testParams = new List<String> { 'it\'s', 'wednesday' };
        System.assertEquals('it\'s', testParams[0]);
    }

    @IsTest
    private static void nextTest() {
        System.assert(true);
    }
}`
	start := strings.Index(source, "private static void setParam1")
	end := strings.Index(source, "    @IsTest\n    private static void nextTest")
	if start < 0 || end < 0 {
		t.Fatal("test source markers not found")
	}
	body, err := extractMethodBody(source, diagnostic.Range{
		Start: diagnostic.Position{Offset: start},
		End:   diagnostic.Position{Offset: end},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(body, "nextTest") || !strings.Contains(body, `'it\'s'`) {
		t.Fatalf("body = %q", body)
	}
}

func TestSourcePositionPrefixPreservesLineAndColumnWithoutFullPadding(t *testing.T) {
	source := "class Sample {\n    @isTest\n    static void test() { System.assert(true);\n    }\n}"
	bodyStart := strings.Index(source, "System.assert")
	if bodyStart < 0 {
		t.Fatal("body marker not found")
	}
	prefix := sourcePositionPrefix(source[:bodyStart])
	if strings.Count(prefix, "\n") != strings.Count(source[:bodyStart], "\n") {
		t.Fatalf("prefix newlines = %d, want %d", strings.Count(prefix, "\n"), strings.Count(source[:bodyStart], "\n"))
	}
	lastLine := prefix[strings.LastIndex(prefix, "\n")+1:]
	if len(lastLine) != 25 {
		t.Fatalf("prefix final column padding = %d, want 25", len(lastLine))
	}
	if len(prefix) >= bodyStart {
		t.Fatalf("prefix length = %d, want less than %d", len(prefix), bodyStart)
	}
}

func TestRunCoversProtectedOverrideAndHandlerDispatchPatterns(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/SelectorBase.cls"), `
public abstract class SelectorBase {
  public String run() {
    return getName();
  }
  protected abstract String getName();
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ConcreteSelector.cls"), `
public class ConcreteSelector extends SelectorBase {
  protected override String getName() {
    return 'selector';
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/PrivateSelectorBase.cls"), `
public abstract class PrivateSelectorBase {
  public String run() {
    return fieldListString();
  }
  String fieldListString() {
    return getSObjectFieldList();
  }
  abstract String getSObjectFieldList();
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/PrivateSelectorChild.cls"), `
public class PrivateSelectorChild extends PrivateSelectorBase {
  private override String getSObjectFieldList() {
    return 'private-fields';
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/DMLHelper.cls"), `
public virtual class DMLHelper {
  public static DMLHelper Instance {
    get {
      if (Instance == null) {
        Instance = new WithoutSharing();
      }
      return Instance;
    }
  }
  public virtual String updateRecords(List<SObject> records) {
    return 'base';
  }
  private without sharing class WithoutSharing extends DMLHelper {
    public override String updateRecords(List<SObject> records) {
      return super.updateRecords(records) + '-without';
    }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/TriggerHandlersBase.cls"), `
global virtual class TriggerHandlersBase {
  global virtual void onBeforeUpdate(Map<Id, SObject> newRecordMap, Map<Id, SObject> oldRecordMap) {
    DispatchState.Value = 'base';
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ConcreteTriggerHandlers.cls"), `
public class ConcreteTriggerHandlers extends TriggerHandlersBase {
  public override void onBeforeUpdate(Map<Id, SObject> newRecordMap, Map<Id, SObject> oldRecordMap) {
    DispatchState.Value = 'child';
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/TriggerHandlerManager.cls"), `
public class TriggerHandlerManager {
  public static void executeHandlers(TriggerHandlersBase triggerHandler) {
    triggerHandler.onBeforeUpdate(new Map<Id, SObject>(), new Map<Id, SObject>());
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/DispatchState.cls"), `
public class DispatchState {
  public static String Value;
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/DispatchPatternsTest.cls"), `
@isTest
private class DispatchPatternsTest {
  @isTest static void dispatches() {
    System.assertEquals('selector', new ConcreteSelector().run());
    System.assertEquals('private-fields', new PrivateSelectorChild().run());
    System.assertEquals('base-without', DMLHelper.Instance.updateRecords(new List<Widget__c>()));
    TriggerHandlerManager.executeHandlers(new ConcreteTriggerHandlers());
    System.assertEquals('child', DispatchState.Value);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	summary := run.Summary()
	if summary.Total != 1 || summary.Passed != 1 {
		t.Fatalf("summary = %#v case = %#v problem = %#v", summary, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunCoversNestedServiceFactoryWithTypeMap(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/Thing__c/Thing__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Thing</label><pluralLabel>Things</pluralLabel><nameField><type>Text</type><label>Name</label></nameField></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/FactoryBase.cls"), `
public virtual class FactoryBase {
  public virtual class ServiceFactory {
    private Map<Type, Type> implByInterface;
    public ServiceFactory(Map<Type, Type> registrations) {
      implByInterface = registrations;
    }
    public Object newInstance(Type serviceInterfaceType) {
      Type impl = implByInterface.get(serviceInterfaceType);
      return impl.newInstance();
    }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Application.cls"), `
public class Application {
  private static final List<SObjectType> OBJECTS = new List<SObjectType>{ Thing__c.SObjectType };
  public static final FactoryBase.ServiceFactory Service = new FactoryBase.ServiceFactory(
    new Map<Type, Type>{ ILocatorService.class => LocatorServiceImpl.class, IOtherLocatorService.class => OtherLocatorServiceImpl.class, IStaticIntervalService.class => StaticIntervalServiceImpl.class }
  );
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ILocatorService.cls"), `
public interface ILocatorService {
  String name();
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/IOtherLocatorService.cls"), `
public interface IOtherLocatorService {
  String other();
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/LocatorServiceImpl.cls"), `
public class LocatorServiceImpl implements ILocatorService {
  public String name() {
    return 'located';
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/OtherLocatorServiceImpl.cls"), `
public class OtherLocatorServiceImpl implements IOtherLocatorService {
  public String other() {
    return 'other';
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/IStaticIntervalService.cls"), `
public interface IStaticIntervalService {
  List<String> intervals(String name);
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/StaticIntervalServiceImpl.cls"), `
public class StaticIntervalServiceImpl implements IStaticIntervalService {
  public static List<String> intervals(String name) {
    return new List<String>{ name, 'daily' };
  }
  public static List<String> viaSelf() {
    return intervals('self');
  }
  public static List<String> shadowedStatic() {
    StaticIntervalServiceImpl staticIntervalServiceImpl = new StaticIntervalServiceImpl();
    return StaticIntervalServiceImpl.intervals('shadow');
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/LocatorFacade.cls"), `
public class LocatorFacade {
  public static String name() {
    return ((ILocatorService) Application.Service.newInstance(ILocatorService.class)).name();
  }
  public static List<String> intervals() {
    return ((IStaticIntervalService) Application.Service.newInstance(IStaticIntervalService.class)).intervals('trail');
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MapOverloadProbe.cls"), `
public class MapOverloadProbe {
  public static String choose(Map<Object, Type> values) {
    return 'object';
  }
  public static String choose(Map<SObjectType, Type> values) {
    return 'sobject';
  }
  public static Integer keyCount(Map<SObjectType, Type> values) {
    Integer count = 0;
    for (SObjectType key : values.keySet()) {
      count++;
    }
    return count;
  }
  public static String typeKeyRoundTrip(Map<Type, String> values) {
    for (Type key : values.keySet()) {
      return values.get(key);
    }
    return null;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/LocatorFactoryTest.cls"), `
@isTest
private class LocatorFactoryTest {
  @isTest static void locatesService() {
    System.assertEquals('located', LocatorFacade.name());
    System.assertEquals(new List<String>{ 'trail', 'daily' }, LocatorFacade.intervals());
    System.assertEquals(new List<String>{ 'self', 'daily' }, StaticIntervalServiceImpl.viaSelf());
    System.assertEquals(new List<String>{ 'shadow', 'daily' }, StaticIntervalServiceImpl.shadowedStatic());
    System.assertEquals('sobject', MapOverloadProbe.choose(new Map<SObjectType, Type>{ Thing__c.SObjectType => LocatorServiceImpl.class }));
    System.assertEquals(1, MapOverloadProbe.keyCount(new Map<SObjectType, Type>{ Thing__c.SObjectType => LocatorServiceImpl.class }));
    System.assertEquals('locator', MapOverloadProbe.typeKeyRoundTrip(new Map<Type, String>{ LocatorServiceImpl.class => 'locator' }));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	summary := run.Summary()
	if summary.Total != 1 || summary.Passed != 1 {
		problem := run.Suites[0].Cases[0].Problem
		if problem == nil {
			t.Fatalf("summary = %#v case = %#v problem = nil", summary, run.Suites[0].Cases[0])
		}
		t.Fatalf("summary = %#v case = %#v problem = %#v", summary, run.Suites[0].Cases[0], *problem)
	}
}

func TestRunExecutesJSONParserFieldOnInnerHandler(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/JSONParserHolder.cls"), `
public class JSONParserHolder {
  private interface ParserEvents {
    String nextToken();
  }
  private class InjectChildrenEventHandler implements ParserEvents {
    private JSONParser childrenParser;
    public InjectChildrenEventHandler(JSONParser childrenParser) {
      this.childrenParser = childrenParser;
      this.childrenParser.nextToken();
    }
    public String nextToken() {
      JSONToken token = childrenParser.nextToken();
      return token == null ? null : token.name();
    }
  }
  public static String firstChildToken(String payload) {
    ParserEvents handler = new InjectChildrenEventHandler(JSON.createParser(payload));
    return handler.nextToken();
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/JSONParserHolderTest.cls"), `
@isTest
private class JSONParserHolderTest {
  @isTest static void storesParserOnInnerHandlerField() {
    System.assertEquals('VALUE_STRING', JSONParserHolder.firstChildToken('["child"]'));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	summary := run.Summary()
	if summary.Total != 1 || summary.Passed != 1 {
		problem := run.Suites[0].Cases[0].Problem
		if problem == nil {
			t.Fatalf("summary = %#v case = %#v problem = nil", summary, run.Suites[0].Cases[0])
		}
		t.Fatalf("summary = %#v case = %#v problem = %#v", summary, run.Suites[0].Cases[0], *problem)
	}
}

func TestExtractMethodBodyFallsBackPastShortRange(t *testing.T) {
	source := `public class BigClass {
  public static void run() {
    // a comment with { that should not count
    if (true) {
      System.debug('}');
    }
  }
}`
	start := strings.Index(source, "public static void run")
	shortEnd := strings.Index(source, "if (true)")
	body, err := extractMethodBody(source, diagnostic.Range{
		Start: diagnostic.Position{Offset: start},
		End:   diagnostic.Position{Offset: shortEnd},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "System.debug('}')") {
		t.Fatalf("body = %q", body)
	}
}

func TestExtractMethodSourceRecoversOneLineSignature(t *testing.T) {
	source := `public class Hooks {
  public virtual void onApplyDefaults() { }
}`
	start := strings.Index(source, "{ }")
	text, err := extractMethodSource(source, diagnostic.Range{
		Start: diagnostic.Position{Offset: start},
		End:   diagnostic.Position{Offset: start + len("{ }")},
	})
	if err != nil {
		t.Fatal(err)
	}
	params, err := parseParams(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(params) != 0 {
		t.Fatalf("params = %#v", params)
	}
	body, err := extractMethodBody(source, diagnostic.Range{
		Start: diagnostic.Position{Offset: start + 1},
		End:   diagnostic.Position{Offset: start + 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(body) != "" {
		t.Fatalf("body = %q", body)
	}
}

func TestRunContextReportsCanceledCases(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/CanceledTest.cls"), `
@isTest
private class CanceledTest {
  @isTest static void stops() {
    System.assert(true);
  }
}
`)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	run := RunContext(ctx, loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Unsupported != 1 {
		t.Fatalf("summary = %#v", got)
	}
	if run.Suites[0].Cases[0].Problem.Type != "Canceled" {
		t.Fatalf("case = %#v", run.Suites[0].Cases[0])
	}
}

func TestRunContextPerTestDeadlineDoesNotCancelFollowingTests(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/PerTestTimeoutTest.cls"), `
@isTest
private class PerTestTimeoutTest {
  @isTest static void a_hangs() {
    while (true) {}
  }
  @isTest static void z_passes() {
    System.assert(true);
  }
}
`)

	run := RunContext(context.Background(), loadTestIndex(t, root), Options{TimeoutMS: 500})
	if got := run.Summary(); got.Total != 2 || got.Passed != 1 || got.Failed+got.Unsupported != 1 {
		t.Fatalf("summary = %#v cases=%#v", got, run.Suites[0].Cases)
	}
	cases := run.Suites[0].Cases
	if cases[0].Problem == nil {
		t.Fatalf("first case = %#v", cases[0])
	}
	if cases[1].Status != testreport.StatusPass {
		t.Fatalf("second case = %#v", cases[1])
	}
}

func TestRunReportsAssertionFailures(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/FailingTest.cls"), `
@isTest
private class FailingTest {
  @isTest static void fails() {
    System.assertEquals(3, 1 + 1);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Failed != 1 {
		t.Fatalf("summary = %#v", got)
	}
	if run.Suites[0].Cases[0].Status != testreport.StatusFail {
		t.Fatalf("case = %#v", run.Suites[0].Cases[0])
	}
}

func TestRunExecutesStaticHelperMethod(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MathUtil.cls"), `
public class MathUtil {
  public static Integer add(Integer a, Integer b) {
    return a + b;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MathUtilTest.cls"), `
@isTest
private class MathUtilTest {
  @isTest static void adds() {
    System.assertEquals(3, MathUtil.add(1, 2));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v problem=%#v run=%#v", got, run.Suites[0].Cases[0].Problem, run)
	}
}

func TestRunExecutesStaticHelperMethodWithBranching(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MathUtil.cls"), `
public class MathUtil {
  public static Integer max(Integer a, Integer b) {
    if (a > b) {
      return a;
    } else {
      return b;
    }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MathUtilTest.cls"), `
@isTest
private class MathUtilTest {
  @isTest static void maxChoosesLargerValue() {
    System.assertEquals(5, MathUtil.max(5, 2));
    System.assertEquals(7, MathUtil.max(3, 7));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		index := loadTestIndex(t, root)
		methods := compileProjectMethods(index)
		for _, class := range compileProjectClasses(index, methods) {
			if class.Name == "ListDowncastDomain" {
				t.Logf("constructors=%#v fields=%#v", class.Constructors, class.Fields)
			}
		}
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunExecutesCaseFoldedOverloadWithNestedGenericParams(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/CaseFoldHelper.cls"), `
public class CaseFoldHelper {
  public static Integer reapplyCartCoupons(Set<Id> ids, Boolean preventIfExpired) {
    Map<Id, Integer> results = new Map<Id, Integer>();
    Map<Id, List<Account>> records = new Map<Id, List<Account>>();
    return reapplyCartCoupons(results, ids, records, new Set<Id>(), preventIfExpired);
  }

  private static Integer reApplyCartCoupons(
      Map<Id, Integer> results,
      Set<Id> ids,
      Map<Id, List<Account>> records,
      Set<Id> seen,
      Boolean preventIfExpired) {
    return 7;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/CaseFoldHelperTest.cls"), `
@isTest
private class CaseFoldHelperTest {
  @isTest static void helperOverloadRuns() {
    System.assertEquals(7, CaseFoldHelper.reapplyCartCoupons(new Set<Id>(), true));
  }
}
`)

	index := loadTestIndex(t, root)
	run := Run(index, Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		methods := compileProjectMethods(index)
		keys := make([]string, 0, len(methods))
		for key := range methods {
			keys = append(keys, key)
		}
		t.Logf("methods=%v", keys)
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestCompileProjectMethodsIncludesDependencyTestHelpers(t *testing.T) {
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	consumerRoot := filepath.Join(root, "consumer")
	writeFile(t, filepath.Join(depRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/classes/SharedTestHelper.cls"), `
@isTest
public class SharedTestHelper {
  public static String value() {
    return 'dep';
  }
}
`)
	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(consumerRoot, "glade.yml"), `project:
  managedPackageDependencies: ["pkg:../dep:1.0"]
`)
	index := loadTestIndex(t, consumerRoot)

	if cases := Discover(index, Options{}); len(cases) != 0 {
		t.Fatalf("discovered dependency test helpers as runnable cases: %#v", cases)
	}
	methods := compileProjectMethods(index)
	found := false
	for _, method := range methods {
		if method.Name == "SharedTestHelper.value" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("dependency @isTest helper method was not compiled; methods=%#v", methods)
	}
}

func TestCompileProjectMethodsIncludesNonVoidIsTestHelpers(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/LocalTestHelper.cls"), `
@isTest
private class LocalTestHelper {
  @isTest public static String value() {
    return 'helper';
  }
  @isTest static void callsHelper() {
    System.assertEquals('helper', value());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{Filter: "callsHelper"})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestCompileProjectClassesKeepsDuplicateDependencyMethodsSeparate(t *testing.T) {
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	consumerRoot := filepath.Join(root, "consumer")
	writeFile(t, filepath.Join(depRoot, "sfdx-project.json"), `{"namespace":"PKG","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/classes/Shared.cls"), `
public class Shared {
  private String DepMarker;
  public String depOnly() {
    return DepMarker;
  }
}
`)
	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(consumerRoot, "glade.yml"), `project:
  defaultNamespace: pkg
  managedPackageDependencies: ["pkg:../dep:1.0"]
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/classes/Shared.cls"), `
public class Shared {
  private String LocalMarker;
  public String localOnly() {
    return LocalMarker;
  }
}
`)

	index := loadTestIndex(t, consumerRoot)
	methods := compileProjectMethods(index)
	classes := compileProjectClasses(index, methods)
	for _, class := range classes {
		if _, ok := class.Fields["LocalMarker"]; !ok {
			continue
		}
		if _, ok := class.Methods["depOnly#"]; ok {
			t.Fatalf("local duplicate class received dependency method: %#v", class.Methods)
		}
		return
	}
	t.Fatalf("local duplicate class not found: %#v", classes)
}

func TestCompileProjectMethodsKeepsDuplicateSameSignatureMethodsSeparate(t *testing.T) {
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	consumerRoot := filepath.Join(root, "consumer")
	writeFile(t, filepath.Join(depRoot, "sfdx-project.json"), `{"namespace":"PKG","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/classes/SObjectTestData.cls"), `
public abstract class SObjectTestData {
  public abstract String getSObjectType();
}
`)
	writeFile(t, filepath.Join(consumerRoot, "glade.yml"), `project:
  defaultNamespace: pkg
  managedPackageDependencies: ["pkg:../dep:1.0"]
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/classes/SObjectTestData.cls"), `
public abstract class SObjectTestData {
  protected abstract String getSObjectType();
}
`)

	index := loadTestIndex(t, consumerRoot)
	methods := compileProjectMethods(index)
	count := 0
	for _, method := range methods {
		if method.Name == "SObjectTestData.getSObjectType" {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("compiled %d duplicate getSObjectType methods, want 2; methods=%#v", count, methods)
	}
}

func TestRunDependencyDuplicateSelectorUsesDependencyProjection(t *testing.T) {
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	consumerRoot := filepath.Join(root, "consumer")
	writeFile(t, filepath.Join(depRoot, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/classes/DuplicateSelector.cls"), `
public class DuplicateSelector {
  public static DuplicateSelector Instance {
    get {
      if (Instance == null) {
        Instance = new DuplicateSelector();
      }
      return Instance;
    }
    private set;
  }
  public List<Account> selectById(Set<Id> ids) {
    return Database.query('SELECT ' + fieldList() + ' FROM Account WHERE Id IN :ids');
  }
  private String fieldList() {
    return 'Id,Name,AccountNumber';
  }
}
`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/classes/DependencyCartService.cls"), `
public class DependencyCartService {
  public static String accountNumber(Id accountId) {
    Account account = DuplicateSelector.Instance.selectById(new Set<Id>{accountId})[0];
    return account.AccountNumber;
  }
}
`)
	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(consumerRoot, "glade.yml"), `project:
  managedPackageDependencies: ["pkg:../dep:1.0"]
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/classes/DuplicateSelector.cls"), `
public class DuplicateSelector {
  public List<Account> selectById(Set<Id> ids) {
    return Database.query('SELECT ' + fieldList() + ' FROM Account WHERE Id IN :ids');
  }
  private String fieldList() {
    return 'Id,Name';
  }
}
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/classes/DuplicateSelectorProjectionTest.cls"), `
@isTest
private class DuplicateSelectorProjectionTest {
  @isTest static void dependencyProjectionWinsForDependencyCaller() {
    Account account = new Account(Name = 'Acme', AccountNumber = '42');
    insert account;
    System.assertEquals('42', DependencyCartService.accountNumber(account.Id));
  }
}
`)

	run := Run(loadTestIndex(t, consumerRoot), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunConsumerNamespaceAccessibleDuplicatePrefersConsumerNamespace(t *testing.T) {
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	consumerRoot := filepath.Join(root, "consumer")
	writeFile(t, filepath.Join(depRoot, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/classes/SharedHelper.cls"), `
public class SharedHelper {
  public static String value() {
    return 'dep';
  }
}
`)
	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{"namespace":"otherpkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(consumerRoot, "glade.yml"), `project:
  managedPackageDependencies: ["pkg:../dep:1.0"]
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/classes/SharedHelper.cls"), `
@NamespaceAccessible
public class SharedHelper {
  @NamespaceAccessible
  public static String value() {
    return 'local';
  }
}
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/classes/SharedHelperTest.cls"), `
@isTest
private class SharedHelperTest {
  @isTest static void unqualifiedTypeUsesConsumerNamespace() {
    System.assertEquals('local', SharedHelper.value());
  }
}
`)

	run := Run(loadTestIndex(t, consumerRoot), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunNamespacedDependencyNestedConstructorStaysInDependencyNamespace(t *testing.T) {
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	consumerRoot := filepath.Join(root, "consumer")
	writeFile(t, filepath.Join(depRoot, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/classes/Module.cls"), `
global class Module {}
`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/classes/Binding.cls"), `
global class Binding {
  global class Resolver {
    global String Source;
    global Resolver(List<Module> modules) {
      Source = 'dep';
    }
  }
}
`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/classes/Injector.cls"), `
global class Injector {
  global static String make() {
    Binding.Resolver resolver = new Binding.Resolver(new List<Module>{ new Module() });
    return resolver.Source;
  }
}
`)
	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{"namespace":"otherpkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(consumerRoot, "glade.yml"), `project:
  managedPackageDependencies: ["pkg:../dep:1.0"]
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/classes/Module.cls"), `
public class Module {}
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/classes/Binding.cls"), `
public class Binding {
  public class Resolver {
    public String Source;
    public Resolver(List<Module> modules) {
      Source = 'local';
    }
  }
}
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/classes/InjectorTest.cls"), `
@isTest
private class InjectorTest {
  @isTest static void dependencyKeepsItsNestedType() {
    System.assertEquals('dep', pkg.Injector.make());
  }
}
`)

	run := Run(loadTestIndex(t, consumerRoot), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestNamespacedEnumConstructorFieldEqualityUsesNamespaceType(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/TokenType.cls"), `
public enum TokenType {
  LEFT_PAREN, RIGHT_PAREN
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Token.cls"), `
public class Token {
  public final TokenType type;
  public Token(TokenType type) {
    this.type = type;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Parser.cls"), `
public class Parser {
  public static Boolean check() {
    Token token = new Token(TokenType.RIGHT_PAREN);
    System.assertNotEquals(null, token.type);
    System.assertEquals('RIGHT_PAREN', String.valueOf(token.type));
    System.assertEquals(true, token.type.equals(token.type));
    System.assertEquals('RIGHT_PAREN', String.valueOf(TokenType.RIGHT_PAREN));
    System.assertEquals(true, token.type.equals(TokenType.RIGHT_PAREN));
    return token.type == TokenType.RIGHT_PAREN;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ParserTest.cls"), `
@isTest
private class ParserTest {
  @isTest static void enumFieldMatchesShortLiteral() {
    System.assertEquals(true, Parser.check());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestNestedEnumNamedRecordTypeWinsOverSObjectFieldToken(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/EnumCollisionService.cls"), `
public class EnumCollisionService {
  public enum RecordType { PRODUCT, PURCHASE }
  private static final Map<RecordType, String> TYPE_BY_STRING =
    new Map<RecordType, String>{
      RecordType.PRODUCT => 'product',
      RecordType.PURCHASE => 'purchase'
    };
  public static String product() {
    return TYPE_BY_STRING.get(RecordType.PRODUCT);
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/EnumCollisionServiceTest.cls"), `
@isTest
private class EnumCollisionServiceTest {
  @isTest static void nestedEnumNamedRecordTypeWins() {
    System.assertEquals('product', EnumCollisionService.product());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestStandardSObjectConstructorWinsOverNestedClassWithSameName(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Model.cls"), `
public class Model {
  public class Contact {
    public String firstName;
  }
  public static SObject makeContact() {
    return new Contact(FirstName = 'Ada', LastName = 'Lovelace');
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/OtherModel.cls"), `
public class OtherModel {
  public class Account {
    public String name;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/OtherService.cls"), `
public class OtherService {
  public static SObject makeAccount() {
    return new Account(Name = 'Acme');
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ModelTest.cls"), `
@isTest
private class ModelTest {
  @isTest static void sobjectConstructorWins() {
    SObject row = Model.makeContact();
    System.assertEquals(Contact.SObjectType, row.getSObjectType());
    System.assertEquals('Ada', (String)row.get('FirstName'));
  }
  @isTest static void unrelatedNestedTypeDoesNotWin() {
    SObject row = OtherService.makeAccount();
    System.assertEquals(Account.SObjectType, row.getSObjectType());
    System.assertEquals('Acme', (String)row.get('Name'));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestNamespacedTypeParameterCanBeForwardedToCreateStub(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Widget.cls"), `
public interface Widget {
  String name();
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Provider.cls"), `
public class Provider implements System.StubProvider {
  public Object handleMethodCall(Object stubbedObject, String stubbedMethodName, Type returnType, List<Type> listOfParamTypes, List<String> listOfParamNames, List<Object> listOfArgs) {
    return 'ok';
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Monitor.cls"), `
public class Monitor {
  public enum Type { Example }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Mocks.cls"), `
public class Mocks extends Provider {
  public Object mock(Type classToMock) {
    return Test.createStub(classToMock, this);
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MocksTest.cls"), `
@isTest
private class MocksTest {
  private static final Mocks MY_MOCKS = new Mocks();
  private static final Widget MY_WIDGET = (Widget)MY_MOCKS.mock(Widget.class);
  @isTest static void forwardsTypeParameter() {
    Widget widget = (Widget)new Mocks().mock(Widget.class);
    System.assertEquals('ok', widget.name());
    System.assertEquals('ok', MY_WIDGET.name());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunNamespacedDependencyNestedSubclassStaysInDependencyNamespace(t *testing.T) {
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	consumerRoot := filepath.Join(root, "consumer")
	writeFile(t, filepath.Join(depRoot, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/classes/Binding.cls"), `
global abstract class Binding {
  global String Source;
  global enum BindingType { Apex }
  private static final Map<BindingType, Type> impls =
    new Map<BindingType, Type> {
      BindingType.Apex => ApexBinding.class
    };
  global static Binding newInstance() {
    return (Binding) impls.get(BindingType.Apex).newInstance();
  }
  private class ApexBinding extends Binding {
    private ApexBinding() {
      Source = 'dep';
    }
  }
}
`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/classes/Module.cls"), `
global class Module {
  private List<Binding> bindings = new List<Binding>();
  global String addBinding() {
    Binding binding = Binding.newInstance();
    bindings.add(binding);
    return binding.Source;
  }
  global static String make() {
    return new Module().addBinding();
  }
}
`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/classes/Injector.cls"), `
global class Injector {
  global class CustomMetadataModule extends Module {
  }
  global static String makeCustom() {
    return new CustomMetadataModule().addBinding();
  }
}
`)
	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{"namespace":"otherpkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(consumerRoot, "glade.yml"), `project:
  managedPackageDependencies: ["pkg:../dep:1.0"]
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/classes/Binding.cls"), `
public abstract class Binding {
  public String Source;
  public enum BindingType { Apex }
  private static final Map<BindingType, Type> impls =
    new Map<BindingType, Type> {
      BindingType.Apex => ApexBinding.class
    };
  public static Binding newInstance() {
    return (Binding) impls.get(BindingType.Apex).newInstance();
  }
  private class ApexBinding extends Binding {
    private ApexBinding() {
      Source = 'local';
    }
  }
}
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/classes/Module.cls"), `
public class Module {
  private List<Binding> bindings = new List<Binding>();
  public String addBinding() {
    Binding binding = Binding.newInstance();
    bindings.add(binding);
    return binding.Source;
  }
  public static String make() {
    return new Module().addBinding();
  }
}
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/classes/Injector.cls"), `
public class Injector {
  public class CustomMetadataModule extends Module {
  }
  public static String makeCustom() {
    return new CustomMetadataModule().addBinding();
  }
}
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/classes/ModuleTest.cls"), `
@isTest
private class ModuleTest {
  @isTest static void dependencyKeepsItsNestedSubclass() {
    System.assertEquals('dep', pkg.Module.make());
    System.assertEquals('dep', pkg.Injector.makeCustom());
  }
}
`)

	run := Run(loadTestIndex(t, consumerRoot), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunDependencyDuplicateSuperCallUsesDependencyBaseMethod(t *testing.T) {
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	consumerRoot := filepath.Join(root, "consumer")
	writeFile(t, filepath.Join(depRoot, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/classes/DuplicateSelector.cls"), `
public virtual class DuplicateSelector {
  public virtual List<Account> selectById(Set<Id> ids) {
    return Database.query('SELECT ' + fieldList() + ' FROM Account WHERE Id IN :ids');
  }
  protected virtual String fieldList() {
    return 'Id,Name,AccountNumber';
  }
  public class Impl extends DuplicateSelector {
    public override List<Account> selectById(Set<Id> ids) {
      return super.selectById(ids);
    }
  }
}
`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/classes/DependencyCartService.cls"), `
public class DependencyCartService {
  public static String accountNumber(Id accountId) {
    Account account = new DuplicateSelector.Impl().selectById(new Set<Id>{accountId})[0];
    return account.AccountNumber;
  }
}
`)
	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(consumerRoot, "glade.yml"), `project:
  managedPackageDependencies: ["pkg:../dep:1.0"]
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/classes/DuplicateSelector.cls"), `
public virtual class DuplicateSelector {
  public virtual List<Account> selectById(Set<Id> ids) {
    return Database.query('SELECT ' + fieldList() + ' FROM Account WHERE Id IN :ids');
  }
  protected virtual String fieldList() {
    return 'Id,Name';
  }
}
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/classes/DuplicateSelectorSuperTest.cls"), `
@isTest
private class DuplicateSelectorSuperTest {
  @isTest static void dependencySuperUsesDependencyBaseMethod() {
    Account account = new Account(Name = 'Acme', AccountNumber = '42');
    insert account;
    System.assertEquals('42', DependencyCartService.accountNumber(account.Id));
  }
}
`)

	run := Run(loadTestIndex(t, consumerRoot), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunDependencyDuplicateClassUsesDependencyFieldShape(t *testing.T) {
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	consumerRoot := filepath.Join(root, "consumer")
	writeFile(t, filepath.Join(depRoot, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/classes/DuplicateNode.cls"), `
public class DuplicateNode {
  protected String field;
  public DuplicateNode(String field) {
    this.field = field;
  }
  public String value() {
    return field;
  }
}
`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/classes/DependencyNodeService.cls"), `
public class DependencyNodeService {
  public static String value() {
    return new DuplicateNode('Name').value();
  }
}
`)
	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(consumerRoot, "glade.yml"), `project:
  managedPackageDependencies: ["pkg:../dep:1.0"]
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/classes/DuplicateNode.cls"), `
public class DuplicateNode {
  protected Schema.SObjectField field;
  public DuplicateNode(Schema.SObjectField field) {
    this.field = field;
  }
  public String value() {
    return field.getDescribe().getName();
  }
}
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/classes/DuplicateNodeFieldTest.cls"), `
@isTest
private class DuplicateNodeFieldTest {
  @isTest static void dependencyFieldShapeWinsForDependencyCaller() {
    System.assertEquals('Name', DependencyNodeService.value());
  }
}
`)

	run := Run(loadTestIndex(t, consumerRoot), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunProjectDuplicateClassKeepsProjectReceiverState(t *testing.T) {
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	consumerRoot := filepath.Join(root, "consumer")
	writeFile(t, filepath.Join(depRoot, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/classes/OperationResult.cls"), `
public class OperationResult {
  public enum OperationStatus { SUCCESS, FAILURE }
  public OperationStatus Status { get; set; }
  public OperationResult() {
    Status = OperationStatus.SUCCESS;
  }
  public static OperationResult newInstance() {
    return new OperationResult();
  }
  public static void addErrorMessage(OperationResult result, String errMsg) {
    result.Status = OperationStatus.FAILURE;
  }
  public void addErrorMessage(String errMsg) {
    OperationResult.addErrorMessage(this, errMsg);
  }
  public Boolean isSuccessful() {
    return Status == OperationStatus.SUCCESS;
  }
}
`)
	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(consumerRoot, "glade.yml"), `project:
  managedPackageDependencies: ["pkg:../dep:1.0"]
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/classes/OperationResult.cls"), `
public class OperationResult {
  private Status resultStatus;
  private List<Message> messages;
  public enum Status { SUCCESS, ERROR }
  private OperationResult() {
    resultStatus = Status.SUCCESS;
    messages = new List<Message>();
  }
  public static OperationResult newInstance() {
    return new OperationResult();
  }
  public OperationResult addErrorMessage(String message) {
    resultStatus = Status.ERROR;
    messages.add(new ErrorMessage(message));
    return this;
  }
  public Boolean isSuccessful() {
    return resultStatus == Status.SUCCESS;
  }
  public Boolean isNotSuccessful() {
    return !isSuccessful();
  }
  private virtual class Message {
    protected String messageStr;
    protected OperationResult.Status messageStatus;
    public Message(String messageStr) {
      messageStatus = OperationResult.Status.SUCCESS;
      this.messageStr = messageStr;
    }
  }
  private class ErrorMessage extends Message {
    public ErrorMessage(String messageStr) {
      super(messageStr);
      this.messageStatus = OperationResult.Status.ERROR;
    }
  }
}
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/classes/OperationResponse.cls"), `
public class OperationResponse {
  public OperationResult Result { get; private set; }
  public OperationResponse(OperationResult failedResult) {
    this.Result = failedResult;
  }
}
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/classes/OperationResultTest.cls"), `
@isTest
private class OperationResultTest {
  @isTest static void projectDuplicateReceiverStateWins() {
    OperationResult result = OperationResult.newInstance();
    result.addErrorMessage('failed');
    System.assert(result.isNotSuccessful(), 'consumer result should stay failed');
    OperationResponse response = new OperationResponse(result);
    System.assert(response.Result.isNotSuccessful(), 'response result should stay failed');
  }
}
`)

	run := Run(loadTestIndex(t, consumerRoot), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunValidateMethodDoesNotInjectFieldSetRequiredErrors(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Widget__c/Widget__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Widget</label><pluralLabel>Widgets</pluralLabel><nameField><type>Text</type><label>Widget Name</label></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Widget__c/fields/Reason__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Reason__c</fullName><label>Reason</label><type>Text</type><length>80</length></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Widget__c/fieldSets/Review.fieldSet-meta.xml"), `<FieldSet xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Review</fullName><displayedFields><field>Reason__c</field><isRequired>true</isRequired></displayedFields></FieldSet>`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/OperationResult.cls"), `
public class OperationResult {
  private Boolean successful = true;
  public static OperationResult newInstance() {
    return new OperationResult();
  }
  public OperationResult addErrorMessage(String message) {
    successful = false;
    return this;
  }
  public Boolean isSuccessful() {
    return successful;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Card.cls"), `
public class Card {
  public String FieldSetName;
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/FieldSetController.cls"), `
public class FieldSetController {
  public Card Card;
  public Widget__c getRecord() {
    return new Widget__c();
  }
  public OperationResult validate() {
    return OperationResult.newInstance();
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/FieldSetControllerTest.cls"), `
@isTest
private class FieldSetControllerTest {
  @isTest static void validateMethodOwnsItsResult() {
    FieldSetController controller = new FieldSetController();
    controller.Card = new Card();
    controller.Card.FieldSetName = 'Review';
    OperationResult result = controller.validate();
    System.assert(result.isSuccessful(), 'Apex validate methods should own their returned result.');
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunCallsInstanceMethodThroughStaticProperty(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Context.cls"), `
public class Context {
  public static Context Instance {
    get {
      if (Instance == null) {
        Instance = new Context();
      }
      return Instance;
    }
  }
  public Context() {
    Object duringConstruction = Context.Instance;
  }
  public String value(Schema.SObjectType typ) {
    return typ.getDescribe().getName();
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ContextTest.cls"), `
@isTest
private class ContextTest {
  @isTest static void callsThroughProperty() {
    System.assertEquals('Account', Context.Instance.value(Account.SObjectType));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunCallsStaticPropertyReceiverInsideMapLiteral(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Context.cls"), `
public class Context {
  public static Context Instance {
    get {
      if (Instance == null) {
        Instance = new Context();
      }
      return Instance;
    }
  }
  public Id getId(Schema.SObjectType typ) {
    return '001000000000001AAA';
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ContextTest.cls"), `
@isTest
private class ContextTest {
  @isTest static void callsThroughPropertyInMapLiteral() {
    Map<Schema.SObjectField, Object> values = new Map<Schema.SObjectField, Object>{
      Account.Name => Context.Instance.getId(Account.SObjectType)
    };
    System.assertEquals('001000000000001AAA', values.get(Account.Name));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunAllowsNestedInheritedPropertyGetterOnDifferentInstances(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BaseBuilder.cls"), `
public abstract class BaseBuilder {
  private Map<String, Object> defaultsPriv;
  private Map<String, Object> defaults {
    get {
      if (defaultsPriv == null) {
        defaultsPriv = getDefaults();
      }
      return defaultsPriv;
    }
  }
  protected abstract Map<String, Object> getDefaults();
  public Integer countDefaults() {
    return defaults.keySet().size();
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ChildBuilder.cls"), `
public class ChildBuilder extends BaseBuilder {
  public static ChildBuilder Instance {
    get {
      if (Instance == null) {
        Instance = new ChildBuilder();
      }
      return Instance;
    }
  }
  protected override Map<String, Object> getDefaults() {
    Map<String, Object> values = new Map<String, Object>{'self' => 'ok'};
    values.put('other', OtherBuilder.Instance.countDefaults());
    return values;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/OtherBuilder.cls"), `
public class OtherBuilder extends BaseBuilder {
  public static OtherBuilder Instance {
    get {
      if (Instance == null) {
        Instance = new OtherBuilder();
      }
      return Instance;
    }
  }
  protected override Map<String, Object> getDefaults() {
    return new Map<String, Object>{'other' => 'ok'};
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BuilderTest.cls"), `
@isTest
private class BuilderTest {
  @isTest static void nestedInheritedGetterUsesOwnReceiver() {
    System.assertEquals(2, ChildBuilder.Instance.countDefaults());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunExecutesStaticHelperMethodWithWhileLoop(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MathUtil.cls"), `
public class MathUtil {
  public static Integer sumTo(Integer n) {
    Integer total = 0;
    Integer i = 1;
    while (i <= n) {
      total = total + i;
      i = i + 1;
    }
    return total;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MathUtilTest.cls"), `
@isTest
private class MathUtilTest {
  @isTest static void sumsRange() {
    System.assertEquals(15, MathUtil.sumTo(5));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v problem=%#v", got, run.Suites[0].Cases[0].Problem)
	}
}

func TestRunExecutesInstanceHelperMethod(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Calculator.cls"), `
public class Calculator {
  public Integer add(Integer a, Integer b) {
    return a + b;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/CalculatorTest.cls"), `
@isTest
private class CalculatorTest {
  @isTest static void instanceMethodAdds() {
    Calculator calc = new Calculator();
    System.assertEquals(7, calc.add(3, 4));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v cases=%#v problem=%#v", got, run.Suites[0].Cases, run.Suites[0].Cases[0].Problem)
	}
}

func TestRunDispatchesCreateStubToStubProvider(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Greeter.cls"), `
public interface Greeter {
  String greet(String name);
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/GreeterProvider.cls"), `
private class GreeterProvider implements System.StubProvider {
  public Object handleMethodCall(Object stubbedObject, String stubbedMethodName, Type returnType, List<Type> listOfParamTypes, List<String> listOfParamNames, List<Object> listOfArgs) {
    System.assertEquals('greet', stubbedMethodName);
    return 'stubbed';
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/GreeterTest.cls"), `
@isTest
private class GreeterTest {
  @isTest static void routesThroughProvider() {
    Greeter greeter = Test.createStub(Greeter.class, new GreeterProvider());
    System.assertEquals('stubbed', greeter.greet('Ada'));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunDispatchesCreateStubToStubProviderWithSystemTypeList(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Greeter.cls"), `
public interface Greeter {
  String greet(String name);
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/GreeterProvider.cls"), `
private class GreeterProvider implements System.StubProvider {
  public Object handleMethodCall(Object stubbedObject, String stubbedMethodName, Type returnType, List<System.Type> listOfParamTypes, List<String> listOfParamNames, List<Object> listOfArgs) {
    System.assertEquals('greet', stubbedMethodName);
    System.assertEquals(String.class, listOfParamTypes.get(0));
    return 'stubbed';
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/GreeterTest.cls"), `
@isTest
private class GreeterTest {
  @isTest static void routesThroughProvider() {
    Greeter greeter = Test.createStub(Greeter.class, new GreeterProvider());
    System.assertEquals('stubbed', greeter.greet('Ada'));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunSObjectTypeEnumMapMissingKeyFallsThroughSwitchElse(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/Widget__c/Widget__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Widget</label><pluralLabel>Widgets</pluralLabel><nameField><type>Text</type><label>Name</label></nameField></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/fflib_IDGenerator.cls"), `
public class fflib_IDGenerator {
  private static Integer fakeIdCount = 0;
  private static final String ID_PATTERN = '000000000000';
  public static Id generate(Schema.SObjectType sobjectType) {
    String keyPrefix = sobjectType.getDescribe().getKeyPrefix();
    fakeIdCount++;
    String fakeIdPrefix = ID_PATTERN.substring(0, ID_PATTERN.length() - String.valueOf(fakeIdCount).length());
    return Id.valueOf(keyPrefix + fakeIdPrefix + fakeIdCount);
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/SObjectTypeMapSwitchProbe.cls"), `
public class SObjectTypeMapSwitchProbe {
  public enum Dataset { Widget }
  public static Map<SObjectType, Dataset> objectTypeToDataset = new Map<SObjectType, Dataset>{
    Widget__c.SObjectType => Dataset.Widget
  };
  public static void requireSupported(Id recordId) {
    Dataset datasetType = objectTypeToDataset.get(recordId.getSObjectType());
    switch on datasetType {
      when Widget {
      }
      when else {
        throw new AuraHandledException('Unsupported object type.');
      }
    }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/SObjectTypeMapSwitchProbeTest.cls"), `
@isTest
private class SObjectTypeMapSwitchProbeTest {
  @isTest static void accountIsNotMapped() {
    Boolean caught = false;
    try {
      SObjectTypeMapSwitchProbe.requireSupported(fflib_IDGenerator.generate(Account.SObjectType));
    } catch (Exception e) {
      caught = true;
    }
    System.assert(caught);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v cases=%#v problem=%#v", got, run.Suites[0].Cases, run.Suites[0].Cases[0].Problem)
	}
}

func TestRunInvokesCallableImplementation(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/LocalCallable.cls"), `
public class LocalCallable implements System.Callable {
  public Object call(String action, Map<String, Object> args) {
    return action;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/LocalCallableTest.cls"), `
@isTest
private class LocalCallableTest {
  @isTest static void invokesCallable() {
    System.Callable callable = new LocalCallable();
    System.assert(callable instanceof System.Callable);
    System.assertEquals('go', callable.call('go', new Map<String, Object>()));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunExecutesClassStateConstructorLoopsAndExceptions(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Counter.cls"), `
public class Counter {
  public Integer value { get; set; }
  public static Integer created = 0;

  public Counter(Integer seed) {
    value = seed;
    created++;
  }

  public Integer addAll(List<Integer> values) {
    for (Integer value : values) {
      if (value == 2) {
        continue;
      }
      this.value = this.value + value;
    }
    return this.value;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/CounterTest.cls"), `
@isTest
private class CounterTest {
  @isTest static void statefulRuntimeFeaturesWork() {
    Counter c = new Counter(1);
    List<Integer> values = new List<Integer>{1, 2, 3};
    System.assertEquals(5, c.addAll(values));
    System.assertEquals(1, Counter.created);
    Integer cleanup = 0;
    try {
      throw new MyException();
    } catch (Exception e) {
      cleanup = cleanup + 1;
    } finally {
      cleanup = cleanup + 2;
    }
    System.assertEquals(3, cleanup);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunExecutesInitializerBlocks(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/InitCounter.cls"), `
public class InitCounter {
  public Integer value { get; set; }
  public static Integer seed = 0;

  static {
    seed = 4;
  }

  {
    value = seed + 1;
  }

  public Integer score() {
    return value;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/InitCounterTest.cls"), `
@isTest
private class InitCounterTest {
  @isTest static void initializersRun() {
    System.assertEquals(4, InitCounter.seed);
    InitCounter counter = new InitCounter();
    System.assertEquals(5, counter.score());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunExecutesFieldInitializerExpressionsInSourceOrder(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/InitOrder.cls"), `
public class InitOrder {
  public static Integer seed = 2;
  public static Integer doubled = seed * 2;
  static {
    seed = doubled + 1;
  }

  public Integer first = seed + 1;
  public Integer second = first + 1;
  {
    second = second + 1;
  }

  public Integer score() {
    return second;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/InitOrderTest.cls"), `
@isTest
private class InitOrderTest {
  @isTest static void fieldInitializersRunInOrder() {
    System.assertEquals(5, InitOrder.seed);
    System.assertEquals(4, InitOrder.doubled);
    InitOrder ordered = new InitOrder();
    System.assertEquals(8, ordered.score());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestFieldInitializerExprFallsBackToFullDeclarationStatement(t *testing.T) {
	source := "\t\tprivate List<Error> errorList = new List<Error>(); \n\t\tprivate Boolean enabled = false;\n"
	start := strings.Index(source, "new List<Error>()")
	end := strings.Index(source, "private Boolean")
	expr, ok := fieldInitializerExpr("errorList", diagnostic.Range{
		Start: diagnostic.Position{Offset: start},
		End:   diagnostic.Position{Offset: end},
	}, source)
	if !ok || expr != "new List<Error>()" {
		t.Fatalf("expr = %q ok=%v, want new List<Error>()", expr, ok)
	}
}

func TestTypeDeclarationSourceFallsBackToFullDeclarationLine(t *testing.T) {
	source := "\t\tpublic class InterfaceBackedFactory implements DomainFactory.IConstructable\n\t\t{\n\t\t\tpublic Object construct() { return null; }\n\t\t}\n"
	start := strings.Index(source, "InterfaceBackedFactory")
	end := strings.LastIndex(source, "}") + 1
	typeSource, err := typeDeclarationSource(source, diagnostic.Range{
		Start: diagnostic.Position{Offset: start},
		End:   diagnostic.Position{Offset: end},
	})
	if err != nil {
		t.Fatal(err)
	}
	if interfaces := parseImplements(typeSource); len(interfaces) != 1 || interfaces[0] != "DomainFactory.IConstructable" {
		t.Fatalf("interfaces = %#v", interfaces)
	}
}

func TestCompileProjectClassesPrefersIndexedInterfaces(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "Outer.cls")
	writeFile(t, file, "public class Outer {}\n")
	index := typesys.Index{Types: []typesys.TypeSymbol{
		{
			Kind:  apexast.DeclarationInterface,
			Name:  "Outer.Marker",
			File:  file,
			Range: diagnostic.Range{Start: diagnostic.Position{Offset: 0}, End: diagnostic.Position{Offset: 1}},
		},
		{
			Kind:       apexast.DeclarationClass,
			Name:       "Outer.Impl",
			File:       file,
			Interfaces: []string{"Outer.Marker"},
			Range:      diagnostic.Range{Start: diagnostic.Position{Offset: 0}, End: diagnostic.Position{Offset: 1}},
		},
	}}
	classes := compileProjectClasses(index, nil)
	for _, class := range classes {
		if class.Name == "Outer.Impl" {
			if len(class.Interfaces) != 1 || class.Interfaces[0] != "Outer.Marker" {
				t.Fatalf("interfaces = %#v", class.Interfaces)
			}
			return
		}
	}
	t.Fatal("Outer.Impl class not compiled")
}

func TestRunRegistersPassiveGeneratedSystemStubClasses(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/PassiveGeneratedStubTest.cls"), `
@IsTest
private class PassiveGeneratedStubTest {
  @IsTest static void generatedDtoAccessorsWork() {
    List<String> coupons = new List<String>{'WELCOME'};
    commercepromotions.PromotionRequest request = new commercepromotions.PromotionRequest(new Account(Name = 'Acme'), 'buyer-one', 'store-one', coupons);
    System.assertEquals('buyer-one', request.getBuyerAccountId());
    System.assertEquals('store-one', request.getWebStoreId());
    System.assertEquals('WELCOME', request.getCouponCodes().get(0));
    System.assertEquals('Acme', ((Account)request.getSalesTransaction()).Name);
    Map<String,Object> values = request.getAsMap();
    System.assertEquals('buyer-one', (String)values.get('buyerAccountId'));
    commercepromotions.PromotionRequest cloned = (commercepromotions.PromotionRequest)request.clone();
    System.assertEquals('buyer-one', cloned.getBuyerAccountId());
    System.assertEquals('INVALIDCOUPON', commercepromotions.ErrorCode.INVALIDCOUPON.name());
    commercepromotions.PromotionRequest namedRequest = new commercepromotions.PromotionRequest(salesTransaction = new Account(Name = 'Named'), buyerAccountId = 'named-buyer', webStoreId = 'named-store', couponCodes = coupons);
    System.assertEquals('named-buyer', namedRequest.getBuyerAccountId());
    System.assertEquals('Named', ((Account)namedRequest.getSalesTransaction()).Name);
    Invocable.Action action = Invocable.Action.createCustomAction('apex', 'pkg', 'DoIt');
    System.assertEquals('apex', action.getType());
    System.assertEquals('pkg', action.getNamespace());
    System.assertEquals('DoIt', action.getName());
    System.assertEquals('Audience', ConnectApi.AudienceCriteriaType.Audience.name());
    System.assertEquals('INVALIDCOUPON', commercepromotions.CouponInfo.ErrorCode.INVALIDCOUPON.name());
    System.assertEquals('NO_FILTER', Database.PaginationCursor.DeleteFilter.NO_FILTER.name());
    System.assertEquals(4, Database.PaginationCursor.DeleteFilter.values().size());
    System.assertEquals('EmailActivity', sfdatakit.DeployComponentBundleAccountEngagementConfig.AccountEngagmentDataStreamTypeEnum.EmailActivity.name());
    Slack.ApiTestRequest slackRequest = Slack.ApiTestRequest.builder().foo('bar').build();
    System.assert(slackRequest != null);
    Slack.ApiTestRequest.Builder slackBuilder = Slack.ApiTestRequest.builder();
    slackBuilder.foo('stored');
    slackBuilder.error('none');
    Slack.ApiTestRequest storedSlackRequest = slackBuilder.build();
    System.assert(storedSlackRequest != null);
    LoyaltyManagement.ChangeTierInputBuilder tierBuilder = new LoyaltyManagement.ChangeTierInputBuilder();
    tierBuilder.setProgramName('Rewards');
    tierBuilder.setTargetTierName('Gold');
    LoyaltyManagement.ChangeTierInput tierInput = tierBuilder.build();
    Map<String,Object> tierValues = tierInput.getAsMap();
    System.assertEquals('Rewards', (String)tierValues.get('programName'));
    System.assertEquals('Gold', (String)tierValues.get('targetTierName'));
    inventorypricing.GetInventoryPricing inventoryService = new inventorypricing.GetInventoryPricing();
    Object response = inventoryService.createResponse(new inventorypricing.InventoryPricingData());
    System.assertNotEquals(null, response);
    Map<String,Object> flowInputs = new Map<String,Object>{'recordId' => '001000000000001'};
    Flow.Interview interview = Flow.Interview.createInterview('Demo_Flow', flowInputs);
    interview.start();
    Map<String,Object> interviewValues = interview.getAsMap();
    System.assertEquals('Demo_Flow', (String)interviewValues.get('flowName'));
    System.assertEquals(true, (Boolean)interviewValues.get('started'));
    Flow.Interview namespacedInterview = Flow.Interview.createInterview('pkg', 'Demo_Flow', flowInputs);
    Map<String,Object> namespacedValues = namespacedInterview.getAsMap();
    System.assertEquals('pkg', (String)namespacedValues.get('namespace'));
    CartExtension.BuyerActionDetails.Builder actionBuilder = new CartExtension.BuyerActionDetails.Builder();
    actionBuilder.withCheckoutStarted(true);
    CartExtension.BuyerActionDetails details = actionBuilder.build();
    System.assertEquals(true, (Boolean)details.getAsMap().get('isCheckoutStarted'));
    CartExtension.BuyerActionDetails chainedDetails = new CartExtension.BuyerActionDetails.Builder()
      .withCouponChanges(new List<CartExtension.CouponChange>())
      .build();
    System.assertEquals(0, chainedDetails.getCouponChanges().size());
    CartExtension.Cart cart = CartExtension.CartTestUtil.createCart();
    System.assert(cart != null);
  }
}
`)
	run := Run(loadTestIndex(t, root), Options{})
	summary := run.Summary()
	if summary.Total != 1 || summary.Passed != 1 {
		if len(run.Suites) > 0 && len(run.Suites[0].Cases) > 0 {
			t.Fatalf("summary = %#v problem=%#v", summary, run.Suites[0].Cases[0].Problem)
		}
		t.Fatalf("summary = %#v suites=%#v", summary, run.Suites)
	}
}

func TestExtractMethodSourceUsesByteOffsets(t *testing.T) {
	source := "// café comment before the method\npublic Integer runIt() {\n  return 7;\n}\n"
	start := strings.Index(source, "public Integer")
	end := len(source)
	methodSource, err := extractMethodSource(source, diagnostic.Range{
		Start: diagnostic.Position{Offset: start},
		End:   diagnostic.Position{Offset: end},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(methodSource, "public Integer runIt()") {
		t.Fatalf("methodSource = %q", methodSource)
	}
}

func TestRunExecutesConstructorChaining(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BaseCounter.cls"), `
public class BaseCounter {
  public Integer base { get; set; }

  public BaseCounter(Integer seed) {
    base = seed;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ChainedCounter.cls"), `
public class ChainedCounter extends BaseCounter {
  public Integer bonus { get; set; }

  public ChainedCounter() {
    this(4);
  }

  public ChainedCounter(Integer bonusSeed) {
    super(3);
    bonus = bonusSeed;
  }

  public Integer score() {
    return base + bonus;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ChainedCounterTest.cls"), `
@isTest
private class ChainedCounterTest {
  @isTest static void constructorsChain() {
    ChainedCounter counter = new ChainedCounter();
    System.assertEquals(7, counter.score());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunExecutesConstructorChainingWithSafeNavigation(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/AccountWrapper.cls"), `
public class AccountWrapper {
  private String accountName;

  public AccountWrapper(Account account) {
    this(account, account?.Name);
  }

  public AccountWrapper(Account account, String name) {
    accountName = name;
  }

  public String name() {
    return accountName;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/AccountWrapperTest.cls"), `
@isTest
private class AccountWrapperTest {
  @isTest static void safeNavigationConstructorChains() {
    AccountWrapper wrapper = new AccountWrapper(new Account(Name = 'Acme'));
    System.assertEquals('Acme', wrapper.name());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunMatchesNamespacedSObjectConstructorArgumentFromSOQL(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/FiscalProbe.cls"), `
public class FiscalProbe {
  public Integer Month;

  public FiscalProbe(Organization org) {
    Month = org.FiscalYearStartMonth;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/FiscalProbeTest.cls"), `
@isTest
private class FiscalProbeTest {
  @isTest static void constructorAcceptsQueriedOrganization() {
    FiscalProbe probe = new FiscalProbe([
      SELECT FiscalYearStartMonth
      FROM Organization
      WHERE Id = :UserInfo.getOrganizationId()
    ]);
    System.assertEquals(1, probe.Month);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunMatchesSObjectConstructorArgumentFromDynamicSOQL(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/AccountQueryWrapper.cls"), `
public class AccountQueryWrapper {
  private Account account;

  public AccountQueryWrapper(Account account) {
    this.account = account;
  }

  public String name() {
    return account.Name;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/AccountQueryWrapperTest.cls"), `
@isTest
private class AccountQueryWrapperTest {
  @isTest static void constructorAcceptsDynamicQueryResult() {
    Account account = new Account(Name = 'Acme');
    insert account;
    String soql = 'SELECT Id, Name FROM Account WHERE Id = \'' + account.Id + '\' LIMIT 1';
    AccountQueryWrapper wrapper = new AccountQueryWrapper(Database.query(soql));
    System.assertEquals('Acme', wrapper.name());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunMatchesSystemStatusCodeMethodParameter(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/StatusCodeAliasMapper.cls"), `
public class StatusCodeAliasMapper {
  public static String map(system.StatusCode status) {
    if (status == system.StatusCode.REQUIRED_FIELD_MISSING) {
      return 'required';
    }
    return 'other';
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/StatusCodeAliasMapperTest.cls"), `
@isTest
private class StatusCodeAliasMapperTest {
  @isTest static void mapsDmlStatusCodeThroughSystemQualifiedParameter() {
    Database.SaveResult result = Database.insert(new Account(), false);
    System.assertEquals(false, result.isSuccess());
    System.assertEquals('required', StatusCodeAliasMapper.map(result.getErrors()[0].getStatusCode()));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunExecutesPropertyAccessorBodies(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/PropertyBox.cls"), `
public class PropertyBox {
  private String backing;

  public String Name {
    get {
      return backing + '!';
    }
    set {
      backing = value.toUpperCase();
    }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/PropertyBoxTest.cls"), `
@isTest
private class PropertyBoxTest {
  @isTest static void accessorsRun() {
    PropertyBox box = new PropertyBox();
    box.Name = 'acme';
    System.assertEquals('ACME!', box.Name);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunExecutesImplicitSuperBeforeSourceConstructorPropertySetters(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BaseWrapper.cls"), `
public abstract class BaseWrapper {
  private Map<String, String> values;

  protected BaseWrapper() {
    values = new Map<String, String>();
  }

  protected void setValue(String name, String value) {
    values.put(name, value);
  }

  public String getValue(String name) {
    return values.get(name);
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ConcreteWrapper.cls"), `
public class ConcreteWrapper extends BaseWrapper {
  public ConcreteWrapper() {
    this.Name = 'Ada';
  }

  public String Name {
    set { setValue('name', value); }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ConcreteWrapperTest.cls"), `
@isTest
private class ConcreteWrapperTest {
  @isTest static void constructorRunsSuperBeforeSetter() {
    ConcreteWrapper wrapper = new ConcreteWrapper();
    System.assertEquals('Ada', wrapper.getValue('name'));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunJSONDeserializeRunsZeroArgConstructorBeforePropertySetters(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BaseWrapper.cls"), `
public abstract class BaseWrapper {
  private Map<String, String> values;

  protected BaseWrapper() {
    values = new Map<String, String>();
  }

  protected void setValue(String name, String value) {
    values.put(name, value);
  }

  protected String getValueInternal(String name) {
    return values.get(name);
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ConcreteWrapper.cls"), `
public class ConcreteWrapper extends BaseWrapper {
  public ConcreteWrapper() {}

  public String Name {
    get { return getValueInternal('name'); }
    set { setValue('name', value); }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ConcreteWrapperTest.cls"), `
@isTest
private class ConcreteWrapperTest {
  @isTest static void deserializeInitializesSetterState() {
    ConcreteWrapper wrapper = new ConcreteWrapper();
    wrapper.Name = 'Ada';
    String payload = '{"Name":"Ada"}';
    ConcreteWrapper decoded = (ConcreteWrapper)JSON.deserialize(payload, ConcreteWrapper.class);
    System.assertEquals('Ada', decoded.Name);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunExecutesStaticPropertyNestedSubclassManagerDispatch(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ManagerBase.cls"), `
public abstract class ManagerBase {
  public abstract String required();
  public String callRequired() {
    return required();
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BatchManager.cls"), `
public abstract class BatchManager extends ManagerBase {
  public static BatchManager Instance {
    get {
      if (Instance == null) {
        Instance = (BatchManager)new WithSharing();
      }
      return Instance;
    }
  }

  public virtual override String required() {
    return 'base';
  }

  public virtual String FindBatch(String source) {
    return callRequired() + ':' + source;
  }

  private class WithSharing extends BatchManager {
    public override String required() {
      return super.required();
    }
    public override String FindBatch(String source) {
      return super.FindBatch(source);
    }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BatchManagerTest.cls"), `
@isTest
private class BatchManagerTest {
  @isTest static void staticPropertyDispatches() {
    System.assertEquals('base:SS', BatchManager.Instance.FindBatch('SS'));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunExecutesOverloadedMethodsByArgumentTypes(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/OverloadUtil.cls"), `
public class OverloadUtil {
  public static String pick(Integer value) {
    return 'int';
  }
  public static String pick(String value) {
    return 'string';
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/OverloadUtilTest.cls"), `
@isTest
private class OverloadUtilTest {
  @isTest static void choosesSpecificMethod() {
    System.assertEquals('int', OverloadUtil.pick(1));
    System.assertEquals('string', OverloadUtil.pick('one'));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunExecutesInheritanceAndSuperDispatch(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BaseScore.cls"), `
public virtual class BaseScore {
  public virtual Integer score() {
    return 2;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BonusScore.cls"), `
public class BonusScore extends BaseScore {
  public override Integer score() {
    return super.score() + 3;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BonusScoreTest.cls"), `
@isTest
private class BonusScoreTest {
  @isTest static void dispatchesOverrideAndSuper() {
    BonusScore score = new BonusScore();
    System.assertEquals(5, score.score());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunExecutesEnumValues(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Mood.cls"), `
public enum Mood {
  Happy,
  Sad
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MoodTest.cls"), `
@isTest
private class MoodTest {
  @isTest static void enumValuesCompareByName() {
    System.assertEquals(Mood.Happy, Mood.Happy);
    System.assertNotEquals(Mood.Happy, Mood.Sad);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunExecutesNestedEnumValues(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Result.cls"), `
public class Result {
  public enum Status {
    SUCCESS,
    ERROR
  }
  private Status value;
  public Result() {
    value = Status.SUCCESS;
  }
  public Boolean ok() {
    return value == Status.SUCCESS;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ResultTest.cls"), `
@isTest
private class ResultTest {
  @isTest static void nestedEnumValueResolvesInsideOwner() {
    System.assertEquals(true, new Result().ok());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunNestedEnumValueBeatsMergedDependencyField(t *testing.T) {
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	consumerRoot := filepath.Join(root, "consumer")
	writeFile(t, filepath.Join(depRoot, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/classes/Result.cls"), `
global class Result {
  global String Status { get; set; }
}
`)
	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(consumerRoot, "glade.yml"), `project:
  managedPackageDependencies: ["pkg:../dep:1.0"]
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/classes/Result.cls"), `
global class Result {
  public enum Status {
    SUCCESS,
    ERROR
  }
  private Status value;
  private Result() {
    value = Status.SUCCESS;
  }
  global static Boolean ok() {
    return new Result().value == Status.SUCCESS;
  }
}
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/classes/ResultTest.cls"), `
@isTest
private class ResultTest {
  @isTest static void nestedEnumValueResolvesBeforeMergedField() {
    System.assertEquals(true, Result.ok());
  }
}
`)

	run := Run(loadTestIndex(t, consumerRoot), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v cases=%#v problem=%#v", got, run.Suites[0].Cases, run.Suites[0].Cases[0].Problem)
	}
}

func TestRunExecutesNestedClassMethod(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Outer.cls"), `
public class Outer {
  public class Inner {
    public static Integer count = 1;
    public static String staticLabel() {
      return 'static-inner';
    }
    public String label() {
      return 'inner';
    }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/OuterTest.cls"), `
@isTest
private class OuterTest {
  @isTest static void nestedClassRuns() {
    Outer.Inner inner = new Outer.Inner();
    System.assertEquals('inner', inner.label());
    System.assertEquals('static-inner', Outer.Inner.staticLabel());
    Outer.Inner.count = 3;
    System.assertEquals(3, Outer.Inner.count);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunNestedExceptionGetTypeNameKeepsOuterClass(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Outer.cls"), `
public class Outer {
  public static void raise() {
    throw new InnerException('blocked');
  }
  public class InnerException extends Exception {}
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/OuterTest.cls"), `
@isTest
private class OuterTest {
  @isTest static void nestedExceptionCarriesQualifiedTypeName() {
    try {
      Outer.raise();
      System.assert(false, 'expected exception');
    } catch (Exception e) {
      System.assertEquals('Outer.InnerException', e.getTypeName());
    }
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunPropertyGetterNestedExceptionGetTypeNameKeepsOuterClass(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Outer.cls"), `
public class Outer {
  public static String Value {
    get {
      throw new InnerException('blocked');
    }
  }
  public class InnerException extends Exception {}
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/OuterTest.cls"), `
@isTest
private class OuterTest {
  @isTest static void propertyExceptionCarriesQualifiedTypeName() {
    try {
      String ignored = Outer.Value;
      System.assert(false, 'expected exception');
    } catch (Exception e) {
      System.assertEquals('Outer.InnerException', e.getTypeName());
    }
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunInstancePropertyNestedExceptionGetTypeNameKeepsOuterClass(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/WorkflowProcess.cls"), `
public class WorkflowProcess {
  public static WorkflowProcess Instance {
    get {
      throw new ProcessException('blocked');
    }
  }
  public class ProcessException extends Exception {}
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/WorkflowProcessTest.cls"), `
@isTest
private class WorkflowProcessTest {
  @isTest static void instanceExceptionCarriesQualifiedTypeName() {
    try {
      WorkflowProcess ignored = WorkflowProcess.Instance;
      System.assert(false, 'expected exception');
    } catch (Exception e) {
      System.assertEquals('WorkflowProcess.ProcessException', e.getTypeName());
    }
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunPrefersNestedInstanceFieldOverCaseFoldedInnerType(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Outer.cls"), `
public class Outer {
  public interface Filter {
    Boolean hasValue();
  }
  public class Impl implements Filter {
    public Boolean hasValue() {
      return true;
    }
  }
  public class Adapter {
    private Filter filter;
    public Adapter(Filter filter) {
      this.filter = filter;
    }
    public Boolean run() {
      return filter.hasValue();
    }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/OuterTest.cls"), `
@isTest
private class OuterTest {
  @isTest static void nestedFieldWins() {
    System.assertEquals(true, new Outer.Adapter(new Outer.Impl()).run());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunExecutesLowercaseNestedClassStaticMethodFromInitializer(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/DispatchOuter.cls"), `
public class DispatchOuter {
  public static String initialized = v1.label('init');
  public class v1 {
    public static String label(String input) {
      return input + '-nested';
    }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/DispatchOuterTest.cls"), `
@isTest
private class DispatchOuterTest {
  @isTest static void lowercaseNestedStaticDispatches() {
    System.assertEquals('direct-nested', DispatchOuter.v1.label('direct'));
    System.assertEquals('init-nested', DispatchOuter.initialized);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunExecutesInstanceMethodOnStaticPropertyInitializedByStaticBlock(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/Account.object-meta.xml"), `<CustomObject><label>Account</label><pluralLabel>Accounts</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fields/Name.field-meta.xml"), `<CustomField><fullName>Name</fullName><label>Name</label><type>Text</type></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/DispatchFacade.cls"), `
public class DispatchFacade {
  public static DispatchService v1 { get; private set; }
  static {
    v1 = new DispatchService();
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/DispatchService.cls"), `
public class DispatchService {
  public String label(String input) {
    return input + '-instance';
  }
  public String describeRecords(List<SObject> records, SObjectField fieldToken) {
    return String.valueOf(records.size()) + ':' + fieldToken.getDescribe().getName();
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/DispatchFacadeTest.cls"), `
@isTest
private class DispatchFacadeTest {
  @isTest static void staticPropertyReceiverDispatches() {
    System.assertEquals('direct-instance', DispatchFacade.v1.label('direct'));
    List<Account> records = new List<Account>{new Account(Name = 'Acme')};
    System.assertEquals('1:Name', DispatchFacade.v1.describeRecords(records, Account.Name));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunPersistsUnqualifiedStaticListMutation(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/StaticListRegistry.cls"), `
public class StaticListRegistry {
  private static List<String> values = new List<String>();

  public static void addOne(String value) {
    values.add(value);
  }

  public static Integer countValues() {
    return values.size();
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/StaticListRegistryTest.cls"), `
@IsTest
private class StaticListRegistryTest {
  @IsTest static void staticListMutationPersistsAcrossStaticMethods() {
    StaticListRegistry.addOne('x');
    System.assertEquals(1, StaticListRegistry.countValues());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunExecutesStaticMapInitializerWithInnerClassTypeValues(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BindingShape.cls"), `
public abstract class BindingShape {
  public enum BindingType { Apex, Module }
  private static final Map<BindingType, Type> bindingImplsByType =
    new Map<BindingType, Type> {
      BindingType.Apex => ApexBinding.class,
      BindingType.Module => ApexBinding.class
    };

  public static String lookup() {
    return bindingImplsByType.get(BindingType.Apex).getName();
  }

  private class ApexBinding extends BindingShape {
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BindingShapeTest.cls"), `
@isTest
private class BindingShapeTest {
  @isTest static void staticMapTypeInitializerDispatches() {
    System.assertEquals('BindingShape.ApexBinding', BindingShape.lookup());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunExecutesLowercaseGenericListIsEmptyFromMethodReturn(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BindingCaseProbe.cls"), `
public class BindingCaseProbe {
  public class Binding {
    public String name;
  }

  public static list<Binding> retrieveBindings() {
    list<Binding> bindings = new list<Binding>();
    return bindings;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BindingCaseProbeTest.cls"), `
@isTest
private class BindingCaseProbeTest {
  @isTest static void lowercaseListReturnSupportsIsEmpty() {
    list<BindingCaseProbe.Binding> matchedBindings = BindingCaseProbe.retrieveBindings();
    System.assert(matchedBindings.isEmpty());
    matchedBindings.add(new BindingCaseProbe.Binding());
    System.assert(!matchedBindings.isEmpty());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunExecutesNestedTypesWithConstructorsInterfacesEnumsAndIdentity(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Outer.cls"), `
public class Outer {
  public static Integer seed = 2;
  public interface Named {
    String name();
  }
  public class Inner {
    public Integer value;
    public Inner(Integer input) {
      value = input + Outer.seed;
    }
    public String label() {
      return 'inner-' + value;
    }
  }
  public class NamedImpl implements Named {
    public String name() {
      return 'nested-iface';
    }
  }
  public static Inner makeInner(Integer input) {
    Inner made = new Inner(input);
    return made;
  }
  public enum Choice {
    One,
    Two
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/OuterTypesTest.cls"), `
@isTest
private class OuterTypesTest {
  @isTest static void nestedTypesRun() {
    Outer.Inner first = new Outer.Inner(3);
    Outer.Inner alias = first;
    Outer.Inner second = new Outer.Inner(3);
    System.assertEquals(5, first.value);
    System.assertEquals('inner-5', first.label());
    System.assert(first == alias);
    System.assert(first != second);
    Outer.Named named = new Outer.NamedImpl();
    System.assertEquals('nested-iface', named.name());
    Outer.Inner made = Outer.makeInner(4);
    System.assertEquals(6, made.value);
    System.assertEquals('Two', Outer.Choice.Two.name());
    System.assertEquals(1, Outer.Choice.Two.ordinal());
    List<Outer.Choice> choices = Outer.Choice.values();
    System.assertEquals(2, choices.size());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunExecutesTestSetupAndResetsStatics(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/SetupState.cls"), `
public class SetupState {
  public static Integer value = 1;
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/SetupStateTest.cls"), `
@isTest
private class SetupStateTest {
  @TestSetup static void setup() {
    SetupState.value = 99;
  }

  @isTest static void first() {
    System.assertEquals(1, SetupState.value);
    SetupState.value = 2;
  }

  @isTest static void second() {
    System.assertEquals(1, SetupState.value);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		t.Fatalf("summary = %#v cases=%#v", got, run.Suites[0].Cases)
	}
}

func TestRunIsolatesDMLBetweenTestMethods(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/Account.object-meta.xml"), `
<CustomObject>
  <label>Account</label>
  <pluralLabel>Accounts</pluralLabel>
  <sharingModel>ReadWrite</sharingModel>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fields/Name.field-meta.xml"), `
<CustomField>
  <fullName>Name</fullName>
  <label>Name</label>
  <type>Text</type>
</CustomField>
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/IsolationTest.cls"), `
@isTest
private class IsolationTest {
  @isTest static void insertsData() {
    insert new Account(Name = 'Acme');
    Integer rows = [SELECT COUNT() FROM Account];
    System.assertEquals(1, rows);
  }

  @isTest static void doesNotSeeOtherTestData() {
    Integer rows = [SELECT COUNT() FROM Account];
    System.assertEquals(0, rows);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		t.Fatalf("summary = %#v cases=%#v", got, run.Suites[0].Cases)
	}
}

func TestRunClonesTestSetupDataBetweenMethods(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/Account.object-meta.xml"), `
<CustomObject>
  <label>Account</label>
  <pluralLabel>Accounts</pluralLabel>
  <sharingModel>ReadWrite</sharingModel>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fields/Name.field-meta.xml"), `
<CustomField>
  <fullName>Name</fullName>
  <label>Name</label>
  <type>Text</type>
</CustomField>
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/SetupDataTest.cls"), `
@isTest
private class SetupDataTest {
  @TestSetup static void seed() {
    insert new Account(Name = 'Seed');
  }

  @isTest static void canMutateSetupDataInOwnTransaction() {
    List<Account> rows = [SELECT Id, Name FROM Account WHERE Name = 'Seed'];
    System.assertEquals(1, rows.size());
    Account row = rows.get(0);
    row.Name = 'Changed';
    update row;
    insert new Account(Name = 'Extra');
    Integer total = [SELECT COUNT() FROM Account];
    System.assertEquals(2, total);
  }

  @isTest static void seesFreshSetupSnapshot() {
    Integer seedRows = [SELECT COUNT() FROM Account WHERE Name = 'Seed'];
    Integer changedRows = [SELECT COUNT() FROM Account WHERE Name = 'Changed'];
    Integer extraRows = [SELECT COUNT() FROM Account WHERE Name = 'Extra'];
    System.assertEquals(1, seedRows);
    System.assertEquals(0, changedRows);
    System.assertEquals(0, extraRows);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		t.Fatalf("summary = %#v cases=%#v", got, run.Suites[0].Cases)
	}
}

func TestRunContinuesDeterministicRandomStateAfterTestSetup(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/Account.object-meta.xml"), `
<CustomObject>
  <label>Account</label>
  <pluralLabel>Accounts</pluralLabel>
  <sharingModel>ReadWrite</sharingModel>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fields/Name.field-meta.xml"), `
<CustomField>
  <fullName>Name</fullName>
  <label>Name</label>
  <type>Text</type>
</CustomField>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fields/ExternalId__c.field-meta.xml"), `
<CustomField>
  <fullName>ExternalId__c</fullName>
  <label>ExternalId</label>
  <type>Text</type>
  <unique>true</unique>
</CustomField>
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/SetupRandomTest.cls"), `
@isTest
private class SetupRandomTest {
  @TestSetup static void seed() {
    insert new Account(Name = 'Seed', ExternalId__c = UUID.randomUUID().toString());
  }

  @isTest static void methodRandomDoesNotCollideWithSetupData() {
    insert new Account(Name = 'Method', ExternalId__c = UUID.randomUUID().toString());
    System.assertEquals(2, [SELECT COUNT() FROM Account]);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v cases=%#v", got, run.Suites[0].Cases)
	}
}

func TestRunUpsertUsesNamespacedExternalIDFromSiblingDependency(t *testing.T) {
	root := t.TempDir()
	workspaceRoot := filepath.Join(root, "workspace")
	depRoot := filepath.Join(workspaceRoot, "dep-package")
	consumerRoot := filepath.Join(workspaceRoot, "consumer-package")
	writeFile(t, filepath.Join(depRoot, "sfdx-project.json"), `{
  "namespace": "depns",
  "packageDirectories": [{"path":"force-app","default":true}]
}`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/default/objects/Contact/fields/ExternalID__c.field-meta.xml"), `<CustomField>
  <fullName>ExternalID__c</fullName>
  <type>Text</type>
  <externalId>true</externalId>
  <unique>true</unique>
</CustomField>`)
	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{
  "namespace": "workns",
  "packageDirectories": [{
    "path":"force-app",
    "default":true,
    "package":"consumer-package",
    "dependencies": [{"package":"missing-dep-name"}]
  }]
}`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/default/classes/NamespacedExternalIDTest.cls"), `
@isTest
private class NamespacedExternalIDTest {
  @isTest static void upsertsWithDependencyFieldToken() {
    Contact c = new Contact(LastName = 'External', depns__ExternalId__c = 'E-1');
    Database.upsert(c, Contact.depns__ExternalId__c);
    System.assertNotEquals(null, c.Id);
  }
}
`)

	run := Run(loadTestIndex(t, consumerRoot), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v cases=%#v", got, run.Suites[0].Cases)
	}
}

func TestRunSeedsCurrentUserProfilePackagingPermission(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/SetupPermissionTest.cls"), `
@isTest
private class SetupPermissionTest {
  @isTest static void currentUserHasInstallPackagingPermission() {
    Integer assigned = [
      SELECT COUNT()
      FROM PermissionSetAssignment
      WHERE AssigneeId = :UserInfo.getUserId()
      AND PermissionSet.PermissionsInstallPackaging = true
    ];
    System.assertEquals(1, assigned);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v cases=%#v", got, run.Suites[0].Cases)
	}
}

func TestRunStoresIdScalarInFifteenCharacterTextField(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Install_Request__c/Install_Request__c.object-meta.xml"), `
<CustomObject>
  <label>Install Request</label>
  <pluralLabel>Install Requests</pluralLabel>
  <nameField>
    <label>Name</label>
    <type>Text</type>
  </nameField>
  <sharingModel>ReadWrite</sharingModel>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Install_Request__c/fields/RequestId__c.field-meta.xml"), `
<CustomField>
  <fullName>RequestId__c</fullName>
  <label>Request Id</label>
  <length>15</length>
  <type>Text</type>
</CustomField>
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/TextIdFieldTest.cls"), `
@isTest
private class TextIdFieldTest {
  @isTest static void textFieldLengthFifteenKeepsFifteenCharacterId() {
    Id requestId = '00B000000000001';
    Install_Request__c row = new Install_Request__c(Name = 'One');
    row.RequestId__c = requestId;
    insert row;

    Install_Request__c stored = [SELECT RequestId__c FROM Install_Request__c LIMIT 1];
    System.assertEquals('00B000000000001', stored.RequestId__c);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v cases=%#v", got, run.Suites[0].Cases)
	}
}

func TestProjectRuntimeResolvesCustomObjectFieldTokensFromMetadata(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"samplepkg"}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/SampleAffiliation__c/SampleAffiliation__c.object-meta.xml"), `
<CustomObject>
  <label>Hospital Affiliation</label>
  <pluralLabel>Hospital Affiliations</pluralLabel>
  <sharingModel>ReadWrite</sharingModel>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/SampleAffiliation__c/fields/Type__c.field-meta.xml"), `
<CustomField>
  <fullName>Type__c</fullName>
  <label>Type</label>
  <type>Picklist</type>
  <valueSet>
    <valueSetDefinition>
      <value>
        <fullName>AdmittingPrivileges</fullName>
        <label>Admitting Privileges</label>
      </value>
    </valueSetDefinition>
  </valueSet>
</CustomField>
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/SchemaTokenTest.cls"), `
@isTest
private class SchemaTokenTest {
  @isTest static void resolvesFieldToken() {
    System.assertNotEquals(null, SampleAffiliation__c.Type__c);
    System.assertEquals('samplepkg__SampleAffiliation__c', SampleAffiliation__c.SObjectType.getDescribe().getName());
    System.assertEquals('SampleAffiliation__c', SampleAffiliation__c.SObjectType.getDescribe().getLocalName());
    System.assertEquals('samplepkg__Type__c', SampleAffiliation__c.Type__c.getDescribe().getName());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v cases=%#v problem=%#v", got, run.Suites[0].Cases, run.Suites[0].Cases[0].Problem)
	}
}

func TestRunTestSetupRecordWinsOverSyntheticSetupDataDefault(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Setup_Data__c/Setup_Data__c.object-meta.xml"), `
<CustomObject>
  <label>Setup Data</label>
  <pluralLabel>Setup Data</pluralLabel>
  <sharingModel>ReadWrite</sharingModel>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Setup_Data__c/fields/Config__c.field-meta.xml"), `
<CustomField>
  <fullName>Config__c</fullName>
  <label>Config</label>
  <type>LongTextArea</type>
  <length>32768</length>
  <visibleLines>3</visibleLines>
</CustomField>
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/SetupDataDefaultTest.cls"), `
@isTest
private class SetupDataDefaultTest {
  @TestSetup static void seed() {
    insert new Setup_Data__c(Name = 'Test Setup', Config__c = 'method');
  }

  @isTest static void unorderedLimitPrefersTestSetupRecord() {
    Setup_Data__c setup = [SELECT Config__c FROM Setup_Data__c LIMIT 1];
    System.assertEquals('method', setup.Config__c);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v cases=%#v problem=%#v", got, run.Suites[0].Cases, run.Suites[0].Cases[0].Problem)
	}
}

func TestRunDrainsQueueableAtStopTest(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/AsyncMarker.cls"), `
public class AsyncMarker {
  public static Integer ran = 0;
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MarkJob.cls"), `
public class MarkJob implements Queueable {
  public void execute(QueueableContext qc) {
    AsyncMarker.ran = AsyncMarker.ran + 1;
    insert new Account(Name = 'async ran');
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MarkJobTest.cls"), `
@isTest
private class MarkJobTest {
  @isTest static void stopTestDrainsQueue() {
    Test.startTest();
    System.enqueueJob(new MarkJob());
    AsyncMarker.ran = 41;
    System.assertEquals(41, AsyncMarker.ran);
    Test.stopTest();
    System.assertEquals(42, AsyncMarker.ran);
    Integer asyncRows = [SELECT COUNT() FROM Account WHERE Name = 'async ran'];
    System.assertEquals(1, asyncRows);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunDrainsQueueableWithMultiMethodCloneIsolation(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MultiMethodJob.cls"), `
public class MultiMethodJob implements Queueable {
  public void execute(QueueableContext qc) {
    insert new Account(Name = 'multi async ran');
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MultiMethodJobTest.cls"), `
@isTest
private class MultiMethodJobTest {
  @isTest static void firstDrainsQueue() {
    Test.startTest();
    System.enqueueJob(new MultiMethodJob());
    Test.stopTest();
    System.assertEquals(1, [SELECT COUNT() FROM Account WHERE Name = 'multi async ran']);
  }

  @isTest static void secondSeesFreshData() {
    System.assertEquals(0, [SELECT COUNT() FROM Account WHERE Name = 'multi async ran']);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunDoesNotDrainQueueableEnqueuedBeforeStartTest(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/PreStartJob.cls"), `
public class PreStartJob implements Queueable {
  public void execute(QueueableContext qc) {
    insert new Account(Name = 'pre-start async ran');
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/PreStartJobTest.cls"), `
@isTest
private class PreStartJobTest {
  @isTest static void stopTestSkipsPreStartQueue() {
    System.enqueueJob(new PreStartJob());
	    Test.startTest();
	    Test.stopTest();
	    System.assertEquals(0, [SELECT COUNT() FROM Account WHERE Name = 'pre-start async ran']);
	    System.assertEquals(0, [SELECT COUNT() FROM AsyncApexJob]);
	  }
	}
	`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v problem=%#v", got, run.Suites[0].Cases[0].Problem)
	}
}

func TestRunExecutesQueueableFinalizerAtStopTest(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/FinalizerJob.cls"), `
public class FinalizerJob implements Queueable, Finalizer {
  public void execute(QueueableContext qc) {
    System.attachFinalizer(this);
    insert new Account(Name = 'queueable ran');
  }
  public void execute(FinalizerContext fc) {
    System.assertEquals(ParentJobResult.SUCCESS, fc.getResult());
    System.assertNotEquals('', fc.getAsyncApexJobId());
    System.assertEquals(null, fc.getException());
    insert new Account(Name = 'finalizer ran');
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/FinalizerJobTest.cls"), `
@isTest
private class FinalizerJobTest {
  @isTest static void stopTestRunsFinalizer() {
    Test.startTest();
    System.enqueueJob(new FinalizerJob());
    Test.stopTest();
    System.assertEquals(1, [SELECT COUNT() FROM Account WHERE Name = 'queueable ran']);
	System.assertEquals(1, [SELECT COUNT() FROM Account WHERE Name = 'finalizer ran']);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v problem=%#v", got, run.Suites[0].Cases[0].Problem)
	}
}

func TestRunDrainsFutureBatchScheduleAndChainedQueueables(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/Account.object-meta.xml"), `
<CustomObject>
  <label>Account</label>
  <pluralLabel>Accounts</pluralLabel>
  <sharingModel>ReadWrite</sharingModel>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fields/Name.field-meta.xml"), `
<CustomField>
  <fullName>Name</fullName>
  <label>Name</label>
  <type>Text</type>
</CustomField>
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/AsyncState.cls"), `
public class AsyncState {
  public static Integer futureRan = 0;
  public static Integer batchSum = 0;
  public static Integer batchFinish = 0;
  public static Integer scheduledRan = 0;
  public static Integer queueRan = 0;
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/FutureWorker.cls"), `
public class FutureWorker {
  @future public static void mark(Integer amount) {
    AsyncState.futureRan = amount;
    insert new Account(Name = 'future');
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/CountingBatch.cls"), `
public class CountingBatch implements Database.Batchable<Integer> {
  public List<Integer> start(Database.BatchableContext bc) {
    return new List<Integer>{1, 2, 3};
  }
  public void execute(Database.BatchableContext bc, List<Integer> scope) {
    for (Integer value : scope) {
      AsyncState.batchSum = AsyncState.batchSum + value;
      insert new Account(Name = 'batch-' + value);
    }
  }
  public void finish(Database.BatchableContext bc) {
    AsyncState.batchFinish = 1;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ObjectCountingBatch.cls"), `
public class ObjectCountingBatch implements Database.Batchable<Object> {
  public List<Object> start(Database.BatchableContext bc) {
    return new List<Object>{1, 2, 3};
  }
  public void execute(Database.BatchableContext bc, List<Object> scope) {
    for (Object value : scope) {
      AsyncState.batchSum = AsyncState.batchSum + (Integer)value;
      insert new Account(Name = 'object-batch-' + value);
    }
  }
  public void finish(Database.BatchableContext bc) {
    AsyncState.batchFinish = AsyncState.batchFinish + 1;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ChildObjectCountingBatch.cls"), `
public class ChildObjectCountingBatch extends ObjectCountingBatch {
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ScheduledWorker.cls"), `
public class ScheduledWorker {
  public void execute(Object sc) {
    AsyncState.scheduledRan = 1;
    insert new Account(Name = 'scheduled');
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/FirstQueue.cls"), `
public class FirstQueue {
  public void execute(Object qc) {
    AsyncState.queueRan = AsyncState.queueRan + 1;
    insert new Account(Name = 'queue-1');
    System.enqueueJob(new SecondQueue());
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/SecondQueue.cls"), `
public class SecondQueue {
  public void execute(Object qc) {
    AsyncState.queueRan = AsyncState.queueRan + 1;
    insert new Account(Name = 'queue-2');
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/AsyncSemanticsTest.cls"), `
@isTest
private class AsyncSemanticsTest {
  @isTest static void drainsSupportedAsyncWork() {
    Test.startTest();
    FutureWorker.mark(7);
    String batchId = Database.executeBatch(new CountingBatch(), 2);
    String scheduleId = System.schedule('nightly', '0 0 0 * * ?', new ScheduledWorker());
    String scheduledBatchId = System.scheduleBatch(new CountingBatch(), 'batch later', 1, 2);
    String queueId = System.enqueueJob(new FirstQueue());
    System.assertNotEquals('', batchId);
    System.assertNotEquals('', scheduleId);
    System.assertNotEquals('', scheduledBatchId);
    System.assertNotEquals('', queueId);
    System.assertEquals(0, AsyncState.futureRan);
    System.assertEquals(0, AsyncState.batchSum);
    System.assertEquals(0, AsyncState.scheduledRan);
    System.assertEquals(0, AsyncState.queueRan);
    Integer beforeRows = [SELECT COUNT() FROM Account];
    System.assertEquals(0, beforeRows);
    Test.stopTest();
    Integer afterRows = [SELECT COUNT() FROM Account];
    System.assertEquals(9, afterRows);
    List<AsyncApexJob> jobs = [SELECT Id, Status, JobType FROM AsyncApexJob];
    System.assertEquals(8, jobs.size());
    System.assertEquals(2, [SELECT COUNT() FROM AsyncApexJob WHERE JobType = 'BatchApexWorker']);
    List<CronTrigger> crons = [SELECT Id, State FROM CronTrigger];
    System.assertEquals(2, crons.size());
    CronTrigger cron = crons.get(0);
    System.assertEquals('Complete', cron.State);
    System.assertEquals(7, AsyncState.futureRan);
    System.assertEquals(12, AsyncState.batchSum);
    System.assertEquals(1, AsyncState.batchFinish);
    System.assertEquals(1, AsyncState.scheduledRan);
    System.assertEquals(1, AsyncState.queueRan);
  }

  @isTest static void acceptsInterfaceTypedBatchable() {
    Database.Batchable<Object> batch = new ObjectCountingBatch();
    Test.startTest();
    String batchId = Database.executeBatch(batch, 2);
    String scheduledBatchId = System.scheduleBatch(batch, 'typed batch later', 1, 2);
    System.assertNotEquals('', batchId);
    System.assertNotEquals('', scheduledBatchId);
    Test.stopTest();
    System.assertEquals(6, [SELECT COUNT() FROM Account]);
    System.assertEquals(12, AsyncState.batchSum);
    System.assertEquals(2, AsyncState.batchFinish);
  }

  @isTest static void acceptsBatchableInheritedFromSuperclass() {
    Test.startTest();
    String batchId = Database.executeBatch(new ChildObjectCountingBatch(), 2);
    String scheduledBatchId = System.scheduleBatch(new ChildObjectCountingBatch(), 'child batch later', 1, 2);
    System.assertNotEquals('', batchId);
    System.assertNotEquals('', scheduledBatchId);
    Test.stopTest();
    System.assertEquals(6, [SELECT COUNT() FROM Account]);
    System.assertEquals(12, AsyncState.batchSum);
    System.assertEquals(2, AsyncState.batchFinish);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 3 || got.Passed != 3 {
		if run.Suites[0].Cases[0].Problem != nil {
			t.Logf("problem=%#v", *run.Suites[0].Cases[0].Problem)
		}
		t.Fatalf("summary = %#v cases=%#v", got, run.Suites[0].Cases)
	}
}

func TestRunBatchKeepsCyclicInstanceReferences(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/CyclicBatch.cls"), `
public class CyclicBatch implements Database.Batchable<Integer> {
  public Service service;
  public CyclicBatch() {
    service = new Service();
  }
  public List<Integer> start(Database.BatchableContext bc) {
    return new List<Integer>{1};
  }
  public void execute(Database.BatchableContext bc, List<Integer> scope) {
    service.values = scope;
    service.worker.assertBackPointer();
  }
  public void finish(Database.BatchableContext bc) {}
  public class Service {
    public Worker worker;
    public List<Integer> values;
    public Service() {
      worker = new Worker(this);
    }
  }
  public class Worker {
    public Service service;
    public Worker(Service service) {
      this.service = service;
    }
    public void assertBackPointer() {
      System.assertNotEquals(null, service);
      System.assertEquals(1, service.values.size());
      insert new Account(Name = 'cycle ok');
    }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/CyclicBatchTest.cls"), `
@isTest
private class CyclicBatchTest {
  @isTest static void batchKeepsNestedBackPointer() {
    Test.startTest();
    Database.executeBatch(new CyclicBatch(), 1);
    Test.stopTest();
    System.assertEquals(1, [SELECT COUNT() FROM Account WHERE Name = 'cycle ok']);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunBatchStartCanReturnQueryLocator(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/Account.object-meta.xml"), `
<CustomObject>
  <fullName>Account</fullName>
  <label>Account</label>
  <pluralLabel>Accounts</pluralLabel>
  <nameField><type>Text</type><label>Name</label></nameField>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fields/Name.field-meta.xml"), `
<CustomField>
  <fullName>Name</fullName>
  <label>Name</label>
  <type>Text</type>
</CustomField>
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/LocatorBatch.cls"), `
public class LocatorBatch implements Database.Batchable<Account> {
  public Database.QueryLocator start(Database.BatchableContext bc) {
    return Database.getQueryLocator('SELECT Id, Name FROM Account');
  }
  public void execute(Database.BatchableContext bc, List<Account> scope) {
    for (Account row : scope) {
      insert new Account(Name = 'processed-' + row.Name);
    }
  }
  public void finish(Database.BatchableContext bc) {
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/LocatorBatchTest.cls"), `
@isTest
private class LocatorBatchTest {
  @isTest static void drainsQueryLocatorScope() {
    insert new Account(Name = 'seed');
    Test.startTest();
	    database.executeBatch(new LocatorBatch(), 200);
    Test.stopTest();
    Integer processed = [SELECT COUNT() FROM Account WHERE Name LIKE 'processed%'];
    System.assertEquals(1, processed);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		if run.Suites[0].Cases[0].Problem != nil {
			t.Logf("problem=%#v", *run.Suites[0].Cases[0].Problem)
		}
		t.Fatalf("summary = %#v cases=%#v", got, run.Suites[0].Cases)
	}
}

func TestRunBatchRerunsStaticCollectionInitializerInAsyncTransaction(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/Account.object-meta.xml"), `
<CustomObject>
  <fullName>Account</fullName>
  <label>Account</label>
  <pluralLabel>Accounts</pluralLabel>
  <nameField><type>Text</type><label>Name</label></nameField>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fields/Name.field-meta.xml"), `
<CustomField>
  <fullName>Name</fullName>
  <label>Name</label>
  <type>Text</type>
</CustomField>
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/StaticSetBatch.cls"), `
public class StaticSetBatch implements Database.Batchable<Account> {
  public static Boolean Ready;
  public static Set<String> Fields = new Set<String>();
  public static Boolean isReady() {
    if (Ready == null || Fields.isEmpty()) {
      Fields.add('Name');
      Ready = true;
    }
    return Ready;
  }
  public Database.QueryLocator start(Database.BatchableContext bc) {
    isReady();
    String query = 'SELECT Id';
    for (String fieldName : Fields) {
      query += ', ' + fieldName;
    }
    query += ' FROM Account';
    return Database.getQueryLocator(query);
  }
  public void execute(Database.BatchableContext bc, List<Account> scope) {
    for (Account row : scope) {
      System.assertEquals('seed', row.Name);
    }
  }
  public void finish(Database.BatchableContext bc) {
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/StaticSetBatchTest.cls"), `
@isTest
private class StaticSetBatchTest {
  @isTest static void staticSetInitializerRunsForBatch() {
    System.assertEquals(true, StaticSetBatch.isReady());
    System.assertEquals(true, StaticSetBatch.Fields.contains('Name'));
    insert new Account(Name = 'seed');
    Test.startTest();
    Database.executeBatch(new StaticSetBatch(), 200);
    Test.stopTest();
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		if run.Suites[0].Cases[0].Problem != nil {
			t.Logf("problem=%#v", *run.Suites[0].Cases[0].Problem)
		}
		t.Fatalf("summary = %#v cases=%#v", got, run.Suites[0].Cases)
	}
}

func TestRunBatchStartCanReturnCustomIterable(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/IterableBatchState.cls"), `
public class IterableBatchState {
  public static Integer sum = 0;
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/IterableBatch.cls"), `
public class IterableBatch implements Database.Batchable<Integer> {
  public CounterIterable start(Database.BatchableContext bc) {
    return new CounterIterable(3);
  }
  public void execute(Database.BatchableContext bc, List<Integer> scope) {
    for (Integer value : scope) {
      IterableBatchState.sum = IterableBatchState.sum + value;
    }
  }
  public void finish(Database.BatchableContext bc) {
  }
  public class CounterIterable implements Iterable<Integer>, Iterator<Integer> {
    private List<Integer> values;
    private Integer index = 0;
    public CounterIterable(Integer total) {
      values = new List<Integer>();
      for (Integer i = 0; i < total; i++) {
        values.add(i + 1);
      }
    }
    public Iterator<Integer> iterator() {
      return this;
    }
    public Boolean hasNext() {
      return index < values.size();
    }
    public Integer next() {
      return values[index++];
    }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/IterableBatchTest.cls"), `
@isTest
private class IterableBatchTest {
  @isTest static void drainsCustomIterableScope() {
    Test.startTest();
    Database.executeBatch(new IterableBatch(), 2);
    Test.stopTest();
    System.assertEquals(6, IterableBatchState.sum);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		if len(run.Suites) > 0 && len(run.Suites[0].Cases) > 0 && run.Suites[0].Cases[0].Problem != nil {
			t.Logf("problem=%#v", *run.Suites[0].Cases[0].Problem)
		}
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunAppliesCustomObjectNameDefaultWhenTestSetsNull(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/Widget__c/Widget__c.object-meta.xml"), `
<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata">
  <label>Widget</label>
  <pluralLabel>Widgets</pluralLabel>
  <nameField><type>Text</type><label>Widget Name</label></nameField>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/WidgetNameDefaultTest.cls"), `
@isTest
private class WidgetNameDefaultTest {
  @isTest static void insertsWithNullNameInLocalTestContext() {
    Widget__c widget = new Widget__c(Name = null);
    insert widget;
    Widget__c loaded = [SELECT Name FROM Widget__c WHERE Id = :widget.Id];
    System.assertEquals('Widget', loaded.Name);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		if len(run.Suites) > 0 && len(run.Suites[0].Cases) > 0 && run.Suites[0].Cases[0].Problem != nil {
			t.Logf("problem=%#v", *run.Suites[0].Cases[0].Problem)
		}
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunAsyncContextIdsAndJobFields(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/Account.object-meta.xml"), `<CustomObject><label>Account</label><pluralLabel>Accounts</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fields/Name.field-meta.xml"), `<CustomField><fullName>Name</fullName><label>Name</label><type>Text</type></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ContextQueue.cls"), `
public class ContextQueue {
  public void execute(QueueableContext qc) {
    insert new Account(Name = qc.getJobId());
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ContextBatch.cls"), `
public class ContextBatch implements Database.Batchable<Integer> {
  public List<Integer> start(Database.BatchableContext bc) {
    return new List<Integer>{1, 2, 3};
  }
  public void execute(Database.BatchableContext bc, List<Integer> scope) {
    insert new Account(Name = bc.getJobId());
  }
  public void finish(Database.BatchableContext bc) {
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ContextSchedule.cls"), `
public class ContextSchedule {
  public void execute(SchedulableContext sc) {
    insert new Account(Name = sc.getTriggerId());
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/AsyncContextIdsTest.cls"), `
@isTest
private class AsyncContextIdsTest {
  @isTest static void contextsExposeDeterministicIds() {
    Test.startTest();
    String queueId = System.enqueueJob(new ContextQueue());
    String batchId = Database.executeBatch(new ContextBatch(), 2);
    String schedId = System.schedule('nightly', '0 0 0 * * ?', new ContextSchedule());
    System.assertEquals('707000000000001', queueId);
    System.assertEquals('707000000000002', batchId);
    System.assertEquals('08e000000000003', schedId);
    Test.stopTest();
    Integer batchRows = [SELECT COUNT() FROM Account WHERE Name = '707000000000002'];
    Integer queueRows = [SELECT COUNT() FROM Account WHERE Name = '707000000000001'];
    Integer triggerRows = [SELECT COUNT() FROM Account WHERE Name = '08e000000000003'];
    System.assertEquals(2, batchRows);
    System.assertEquals(1, queueRows);
    System.assertEquals(1, triggerRows);
    List<AsyncApexJob> batches = [SELECT Id, Status, JobType, TotalJobItems, JobItemsProcessed, NumberOfErrors, CompletedDate FROM AsyncApexJob WHERE Id = '707000000000002'];
    System.assertEquals(1, batches.size());
    AsyncApexJob batch = batches.get(0);
    System.assertEquals('Completed', batch.Status);
    System.assertEquals('BatchApex', batch.JobType);
    System.assertEquals(2, batch.TotalJobItems);
    System.assertEquals(2, batch.JobItemsProcessed);
    System.assertEquals(0, batch.NumberOfErrors);
    System.assertNotEquals(null, batch.CompletedDate);
    List<AsyncApexJob> pendingBatches = [
      SELECT Id
      FROM AsyncApexJob
      WHERE Status IN ('Preparing', 'Processing', 'Queued', 'Holding')
      AND JobType = 'BatchApex'
      AND Id = '707000000000002'
    ];
    System.assertEquals(1, pendingBatches.size());
    List<CronTrigger> crons = [SELECT Id, State, CronExpression, CronJobDetail FROM CronTrigger];
    System.assertEquals(1, crons.size());
    CronTrigger cron = crons.get(0);
    System.assertEquals('Complete', cron.State);
    System.assertEquals('0 0 0 * * ?', cron.CronExpression);
    System.assertEquals('nightly', cron.CronJobDetail);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		if len(run.Suites) > 0 && len(run.Suites[0].Cases) > 0 && run.Suites[0].Cases[0].Problem != nil {
			t.Logf("problem=%#v", *run.Suites[0].Cases[0].Problem)
		}
		t.Fatalf("summary = %#v cases=%#v", got, run.Suites[0].Cases)
	}
}

func TestRunScheduledApexExposesPendingJobWithCronTriggerRelationship(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/QueuedWorker.cls"), `
public class QueuedWorker {
  public void execute(QueueableContext qc) {
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ScheduledWorker.cls"), `
public class ScheduledWorker {
  public static Integer Ran = 0;
  public void execute(SchedulableContext sc) {
    Ran = Ran + 1;
    System.enqueueJob(new QueuedWorker());
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ScheduledWorkerTest.cls"), `
@isTest
private class ScheduledWorkerTest {
  @isTest static void exposesScheduledJobRows() {
    Test.startTest();
    String scheduleId = System.schedule('nightly', '0 0 12 * * ?', new ScheduledWorker());
    Test.stopTest();
    System.assertEquals(1, ScheduledWorker.Ran);
    List<AsyncApexJob> jobs = [
      SELECT Id, Status, JobType, ApexClass.Name, CronTriggerId, CronTrigger.Id
      FROM AsyncApexJob
      WHERE ApexClass.Name = 'ScheduledWorker'
      AND Status IN ('Preparing', 'Processing', 'Queued', 'Holding')
      AND JobType = 'ScheduledApex'
    ];
    System.assertEquals(1, jobs.size());
    System.assertEquals(scheduleId, jobs.get(0).CronTriggerId);
    System.assertEquals(scheduleId, jobs.get(0).CronTrigger.Id);
    ApexClass queuedClass = [SELECT Id, Name, NamespacePrefix FROM ApexClass WHERE Name = 'QueuedWorker' AND NamespacePrefix = null LIMIT 1];
    List<AsyncApexJob> queuedJobs = [
      SELECT Id, Status, JobType, ApexClassId
      FROM AsyncApexJob
      WHERE ApexClassId = :queuedClass.Id
      AND Status IN ('Preparing', 'Processing', 'Queued', 'Holding')
      AND JobType = 'Queueable'
    ];
    System.assertEquals(1, queuedJobs.size());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		if len(run.Suites) > 0 && len(run.Suites[0].Cases) > 0 && run.Suites[0].Cases[0].Problem != nil {
			t.Logf("problem=%#v", *run.Suites[0].Cases[0].Problem)
		}
		t.Fatalf("summary = %#v cases=%#v", got, run.Suites[0].Cases)
	}
}

func TestRunAsyncContextFlagsReflectLocalDrainKind(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/Account.object-meta.xml"), `<CustomObject><label>Account</label><pluralLabel>Accounts</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fields/Name.field-meta.xml"), `<CustomField><fullName>Name</fullName><label>Name</label><type>Text</type></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/FlagFuture.cls"), `
public class FlagFuture {
  @future public static void run() {
    System.assertEquals(true, System.isFuture());
    System.assertEquals(false, System.isBatch());
    System.assertEquals(false, System.isQueueable());
    System.assertEquals(false, System.isScheduled());
    insert new Account(Name = 'future');
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/FlagQueue.cls"), `
public class FlagQueue {
  public void execute(QueueableContext qc) {
    System.assertEquals(false, System.isFuture());
    System.assertEquals(false, System.isBatch());
    System.assertEquals(true, System.isQueueable());
    System.assertEquals(false, System.isScheduled());
    insert new Account(Name = 'queueable');
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/FlagBatch.cls"), `
public class FlagBatch implements Database.Batchable<Integer> {
  public List<Integer> start(Database.BatchableContext bc) {
    System.assertEquals(true, System.isBatch());
    System.assertEquals(false, System.isFuture());
    System.assertEquals(false, System.isQueueable());
    System.assertEquals(false, System.isScheduled());
    return new List<Integer>{1};
  }
  public void execute(Database.BatchableContext bc, List<Integer> scope) {
    System.assertEquals(true, System.isBatch());
    insert new Account(Name = 'batch');
  }
  public void finish(Database.BatchableContext bc) {
    System.assertEquals(true, System.isBatch());
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/FlagSchedule.cls"), `
public class FlagSchedule {
  public void execute(SchedulableContext sc) {
    System.assertEquals(false, System.isFuture());
    System.assertEquals(false, System.isBatch());
    System.assertEquals(false, System.isQueueable());
    System.assertEquals(true, System.isScheduled());
    insert new Account(Name = 'scheduled');
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/AsyncFlagTest.cls"), `
@isTest
private class AsyncFlagTest {
  @isTest static void localDrainReportsAsyncContext() {
    System.assertEquals(false, System.isFuture());
    System.assertEquals(false, System.isBatch());
    System.assertEquals(false, System.isQueueable());
    System.assertEquals(false, System.isScheduled());
    Test.startTest();
    FlagFuture.run();
    System.enqueueJob(new FlagQueue());
    Database.executeBatch(new FlagBatch(), 1);
    System.schedule('nightly', '0 0 0 * * ?', new FlagSchedule());
    Test.stopTest();
    Integer rows = [SELECT COUNT() FROM Account];
    System.assertEquals(4, rows);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		if len(run.Suites) > 0 && len(run.Suites[0].Cases) > 0 && run.Suites[0].Cases[0].Problem != nil {
			t.Logf("problem=%#v", *run.Suites[0].Cases[0].Problem)
		}
		t.Fatalf("summary = %#v cases=%#v", got, run.Suites[0].Cases)
	}
}

func TestRunAsSetsUserContextForBlock(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/RunAsTest.cls"), `
@isTest
private class RunAsTest {
  @isTest static void scopesCurrentUser() {
    System.assertEquals('005000000000001', UserInfo.getUserId());
    System.runAs(new User(Id = 'user-a', ProfileId = 'profile-a', Username = 'user-a@example.test')) {
      System.assertEquals('user-a', UserInfo.getUserId());
      System.assertEquals('profile-a', UserInfo.getProfileId());
      System.assertEquals('user-a@example.test', UserInfo.getUserName());
    }
    System.assertEquals('005000000000001', UserInfo.getUserId());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunAsDMLPersistsNonSetupRecordWithAuditUser(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Thing__c/Thing__c.object-meta.xml"), `
<CustomObject>
  <label>Thing</label>
  <pluralLabel>Things</pluralLabel>
  <sharingModel>ReadWrite</sharingModel>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/RunAsDMLTest.cls"), `
@isTest
private class RunAsDMLTest {
  @isTest static void persistsRecord() {
    User u = new User(Id = '005000000000999', ProfileId = '00e000000000006', Username = 'user-a@example.test');
    System.runAs(u) {
      insert new Thing__c();
    }
    List<Thing__c> rows = [SELECT Id, LastModifiedById FROM Thing__c];
    System.assertEquals(1, rows.size());
    System.assertEquals('005000000000999', rows[0].LastModifiedById);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v cases=%#v problem=%#v", got, run.Suites[0].Cases, run.Suites[0].Cases[0].Problem)
	}
}

func TestRunAsPersistsUserContactRelationship(t *testing.T) {
	project := newSalesforceSurfaceProject(t, `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	project.writeClass("RunAsContactTest", `
@isTest
private class RunAsContactTest {
  @isTest static void queriesContactAccount() {
    Account account = new Account(Name = 'acct');
    insert account;
    Contact contact = new Contact(LastName = 'member', AccountId = account.Id);
    insert contact;
    User u = new User(ProfileId = '00e000000000006', Username = 'user-a@example.test', ContactId = contact.Id);

    System.runAs(u) {
      System.assertNotEquals(null, UserInfo.getUserId());
      User current = [SELECT Contact.AccountId FROM User WHERE Id = :UserInfo.getUserId()];
      System.assertEquals(account.Id, current.Contact.AccountId);
    }
  }
}
`)

	project.assertSinglePassingRun()
}

func TestRunAsPersistsStandardPersonContactRelationship(t *testing.T) {
	project := newSalesforceSurfaceProject(t, `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"pkg"}`)
	project.writeRecordType("PersonAccount", "Individual", `
	<RecordType>
	  <fullName>Individual</fullName>
  <label>Individual</label>
  <active>true</active>
</RecordType>`)
	project.writeClass("RunAsPersonContactTest", `
@isTest
private class RunAsPersonContactTest {
  @isTest static void queriesPersonContactAccount() {
	    Id recordTypeId = [SELECT Id FROM RecordType WHERE SobjectType = 'Account' AND DeveloperName = 'Individual' LIMIT 1].Id;
	    Account account = new Account(FirstName = 'Ada', LastName = 'Lovelace', RecordTypeId = recordTypeId);
	    insert account;
	    Account stored = [SELECT PersonContactId FROM Account WHERE Id = :account.Id LIMIT 1];
	    System.assertNotEquals(null, stored.PersonContactId);
	    User u = new User(ProfileId = '00e000000000006', Username = 'person-user@example.test', ContactId = stored.PersonContactId);

    System.runAs(u) {
      User current = [SELECT Contact.AccountId FROM User WHERE Id = :UserInfo.getUserId()];
      System.assertEquals(account.Id, current.Contact.AccountId);
    }
  }
}
`)

	project.assertSinglePassingRun()
}

func TestRunAsUserCanQueryOwnContactAccountWithSharing(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"pkg"}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/Account.object-meta.xml"), `
<CustomObject>
  <sharingModel>Private</sharingModel>
</CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/AccountSelector.cls"), `
public with sharing class AccountSelector {
  public List<Account> selectById(Set<Id> accountIds) {
    return Database.query('SELECT Id, Name FROM Account WHERE Id IN :accountIds');
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/RunAsContactAccountSharingTest.cls"), `
@isTest
private class RunAsContactAccountSharingTest {
  @isTest static void canQueryOwnContactAccountWithSharing() {
    Account account = new Account(Name = 'acct');
    insert account;
    Contact contact = new Contact(AccountId = account.Id, LastName = 'member');
    insert contact;
    User u = new User(ProfileId = '00e000000000006', Username = 'community@example.test', ContactId = contact.Id);

    System.runAs(u) {
      System.assertEquals(1, new AccountSelector().selectById(new Set<Id>{account.Id}).size());
    }
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v cases=%#v problem=%#v", got, run.Suites[0].Cases, run.Suites[0].Cases[0].Problem)
	}
}

func TestRunAsDMLPersistsOuterAssignedSObject(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Thing__c/Thing__c.object-meta.xml"), `
<CustomObject>
  <label>Thing</label>
  <pluralLabel>Things</pluralLabel>
  <sharingModel>ReadWrite</sharingModel>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/RunAsOuterAssignedDMLTest.cls"), `
@isTest
private class RunAsOuterAssignedDMLTest {
  @isTest static void persistsOuterAssignedRecord() {
    User u = new User(Id = '005000000000999', ProfileId = '00e000000000006', Username = 'user-a@example.test');
    Thing__c row;
    System.runAs(u) {
      row = new Thing__c();
      insert row;
    }
    System.assertNotEquals(null, row.Id);
    System.assertEquals(1, [SELECT Id FROM Thing__c].size());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v cases=%#v problem=%#v", got, run.Suites[0].Cases, run.Suites[0].Cases[0].Problem)
	}
}

func TestRunReportsAssertionStackWithFileAndLine(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	testFile := filepath.Join(root, "force-app/main/classes/StackTraceTest.cls")
	writeFile(t, testFile, `
@isTest
private class StackTraceTest {
  @isTest static void failsWithStack() {
    System.assertEquals(1, 2);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Failed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
	problem := run.Suites[0].Cases[0].Problem
	if problem == nil || problem.Type != "System.AssertException" {
		t.Fatalf("problem = %#v", problem)
	}
	if len(problem.Stack) == 0 || problem.Stack[0].File != testFile || problem.Stack[0].Line != 5 || problem.Stack[0].Column != 5 {
		t.Fatalf("stack = %#v", problem.Stack)
	}
}

func TestRunExecutesSObjectSOQLDMLAndTriggers(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/Account.object-meta.xml"), `
<CustomObject>
  <label>Account</label>
  <pluralLabel>Accounts</pluralLabel>
  <sharingModel>ReadWrite</sharingModel>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fields/Name.field-meta.xml"), `
<CustomField>
  <fullName>Name</fullName>
  <label>Name</label>
  <type>Text</type>
</CustomField>
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/AccountNameTrigger.trigger"), `
trigger AccountNameTrigger on Account (before insert) {
  for (Account a : Trigger.new) {
    a.Name = a.Name + '!';
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/DataRuntimeTest.cls"), `
@isTest
private class DataRuntimeTest {
  @isTest static void dmlSoqlAndTriggersWork() {
    Account a = new Account(Name = 'Acme');
    insert a;
    String wanted = 'Acme!';
    List<Account> rows = [SELECT Id, Name FROM Account WHERE Name = :wanted];
    System.assertEquals(1, rows.size());
    Account row = rows.get(0);
    System.assertEquals('Acme!', row.Name);
    row.put('Name', 'Changed');
    update row;
    List<Account> changed = Database.query('SELECT Id, Name FROM Account');
    System.assertEquals(1, changed.size());
    delete row;
    List<Account> remaining = [SELECT Id FROM Account];
    System.assertEquals(0, remaining.size());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunNamespacedFieldMapDoesNotExposeSiblingDeprecatedField(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"PKG","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Application2__c/fields/Account2__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Account2__c</fullName><type>Lookup</type><referenceTo>Account</referenceTo><relationshipName>Account2__r</relationshipName></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Application__c/fields/Account__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Account__c</fullName><type>Lookup</type><referenceTo>Account</referenceTo><relationshipName>Account__r</relationshipName></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/NamespacedFieldMapTest.cls"), `
@isTest
private class NamespacedFieldMapTest {
  static final String legacyTemplate = 'Your application: {!Application2__c.Account__c}';

  @isTest static void activeObjectDoesNotSeeDeprecatedMasterField() {
    Map<String, Schema.SObjectField> fields = Application2__c.SObjectType.getDescribe().fields.getMap();
    System.assert(fields.containsKey('Account2__c'));
    System.assert(!fields.containsKey('Account__c'));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestDiscoverFilter(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ManyTest.cls"), `
@isTest
private class ManyTest {
  @isTest static void first() { System.assert(true); }
  @isTest static void second() { System.assert(true); }
}
`)

	cases := Discover(loadTestIndex(t, root), Options{Filter: "second"})
	if len(cases) != 1 || cases[0].MethodName != "second" {
		t.Fatalf("cases = %#v", cases)
	}
}

func TestDiscoverFilterAllowsCommaSeparatedNames(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/FirstTest.cls"), `
@isTest
private class FirstTest {
  @isTest static void one() { System.assert(true); }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/SecondTest.cls"), `
@isTest
private class SecondTest {
  @isTest static void two() { System.assert(true); }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ThirdTest.cls"), `
@isTest
private class ThirdTest {
  @isTest static void three() { System.assert(true); }
}
`)
	cases := Discover(loadTestIndex(t, root), Options{Filter: "FirstTest, ThirdTest"})
	if len(cases) != 2 {
		t.Fatalf("cases = %#v", cases)
	}
	if cases[0].ClassName != "FirstTest" || cases[1].ClassName != "ThirdTest" {
		t.Fatalf("selected cases = %#v", cases)
	}
}

func TestDiscoverSelectsExactClassAndMethod(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/AccountServiceTest.cls"), `
@isTest
private class AccountServiceTest {
  @isTest static void testCreatesAccount() { System.assert(true); }
  @isTest static void testUpdatesAccount() { System.assert(true); }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/AccountServiceTestExtra.cls"), `
@isTest
private class AccountServiceTestExtra {
  @isTest static void testCreatesAccount() { System.assert(true); }
}
`)

	cases := Discover(loadTestIndex(t, root), Options{
		SelectedClasses: []string{"AccountServiceTest"},
		SelectedMethod:  "testCreatesAccount",
	})
	if len(cases) != 1 || cases[0].ClassName != "AccountServiceTest" || cases[0].MethodName != "testCreatesAccount" {
		t.Fatalf("cases = %#v", cases)
	}
}

func TestSourceCacheConcurrentRead(t *testing.T) {
	cache := newSourceCache()
	files := make([]string, 64)
	for i := range files {
		path := filepath.Join(t.TempDir(), fmt.Sprintf("Source%d.cls", i))
		writeFile(t, path, fmt.Sprintf("public class Source%d {}", i))
		files[i] = path
	}
	var wg sync.WaitGroup
	for _, file := range files {
		wg.Add(1)
		go func(file string) {
			defer wg.Done()
			if _, err := cache.read(file); err != nil {
				t.Errorf("read %s: %v", file, err)
			}
		}(file)
	}
	wg.Wait()
}

func TestDiscoverSkipsHelpersInIsTestClass(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/HelperTest.cls"), `
@isTest
private class HelperTest {
  static String helper() { return 'skip'; }
  @isTest static void runs() { System.assertEquals('skip', helper()); }
}
`)

	cases := Discover(loadTestIndex(t, root), Options{})
	if len(cases) != 1 || cases[0].MethodName != "runs" {
		t.Fatalf("cases = %#v", cases)
	}
}

func TestMethodParallelismSharesClassWorkerBudget(t *testing.T) {
	tests := []struct {
		name              string
		totalParallelism  int
		classParallelism  int
		methods           int
		unfinishedClasses int
		want              int
	}{
		{name: "single class uses full budget", totalParallelism: 8, classParallelism: 1, methods: 20, unfinishedClasses: 1, want: 8},
		{name: "eight class workers run methods serially", totalParallelism: 8, classParallelism: 8, methods: 20, unfinishedClasses: 8, want: 1},
		{name: "last class uses freed workers", totalParallelism: 8, classParallelism: 8, methods: 20, unfinishedClasses: 1, want: 8},
		{name: "two class workers split budget", totalParallelism: 8, classParallelism: 2, methods: 20, unfinishedClasses: 2, want: 4},
		{name: "rounds down to keep total under budget", totalParallelism: 3, classParallelism: 2, methods: 20, unfinishedClasses: 2, want: 1},
		{name: "caps at method count", totalParallelism: 8, classParallelism: 2, methods: 3, unfinishedClasses: 2, want: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := methodParallelismForClassRun(tt.totalParallelism, tt.classParallelism, tt.methods, tt.unfinishedClasses); got != tt.want {
				t.Fatalf("methodParallelismForClassRun(%d, %d, %d, %d) = %d, want %d", tt.totalParallelism, tt.classParallelism, tt.methods, tt.unfinishedClasses, got, tt.want)
			}
		})
	}
}

func TestRunValidationRuleResolvesParentFormulaFieldFromSplitLookupID(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Payment__c/fields/IsCredit__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>IsCredit__c</fullName><type>Checkbox</type><defaultValue>false</defaultValue></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Payment__c/fields/PaymentAmount__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>PaymentAmount__c</fullName><type>Currency</type><precision>18</precision><scale>2</scale><defaultValue>0</defaultValue></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Payment__c/fields/TotalPaymentApplied__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>TotalPaymentApplied__c</fullName><type>Summary</type><summaryOperation>sum</summaryOperation><summaryForeignKey>PaymentLine__c.Payment__c</summaryForeignKey><summarizedField>PaymentLine__c.PaymentAmount__c</summarizedField></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Payment__c/fields/AvailableCreditBalance__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>AvailableCreditBalance__c</fullName><type>Currency</type><precision>18</precision><scale>2</scale><formula>IF(IsCredit__c, PaymentAmount__c - TotalPaymentApplied__c, 0)</formula></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/CartPayment__c/fields/PaymentAmount__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>PaymentAmount__c</fullName><type>Currency</type><precision>18</precision><scale>2</scale></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/CartPayment__c/fields/CreditPayment__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>CreditPayment__c</fullName><type>Lookup</type><referenceTo>Payment__c</referenceTo><relationshipName>CreditPayment</relationshipName></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/CartPayment__c/validationRules/PrepaymentAmountCannotExceedBalance.validationRule-meta.xml"), `<ValidationRule xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>PrepaymentAmountCannotExceedBalance</fullName><active>true</active><errorConditionFormula>IsBlank(CreditPayment__c) = false &amp;&amp; PaymentAmount__c &gt; CreditPayment__r.AvailableCreditBalance__c</errorConditionFormula><errorMessage>The prepayment amount cannot exceed the available credit balance on the prepayment.</errorMessage></ValidationRule>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/CreditValidationTest.cls"), `
@isTest
private class CreditValidationTest {
  @isTest static void splitLookupIdStillRunsParentFormulaValidation() {
    Payment__c credit = new Payment__c(IsCredit__c = true, PaymentAmount__c = 9);
    insert credit;
    String[] parts = ('Pay Up To &&&epm&&&check&&&' + credit.Id).split('&&&');
    CartPayment__c applied = new CartPayment__c(PaymentAmount__c = 10);
    applied.CreditPayment__c = parts[3];
    try {
      upsert applied;
      System.assert(false, 'expected validation failure');
    } catch (DmlException e) {
      System.assert(e.getMessage().contains('available credit balance'), e.getMessage());
    }
    System.assertEquals(0, [SELECT Id FROM CartPayment__c].size());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunResolvesVisualforcePageReferencesAndControllerConstructors(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/AccountView.page"), `<apex:page standardController="Account" extensions="AccountViewExtension">
  <c:AccountBadge value="{!Account.Name}" />
</apex:page>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/components/AccountBadge.component"), `<apex:component controller="AccountBadgeController">
  <apex:attribute name="value" type="String" assignTo="{!value}" />
</apex:component>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/AccountViewExtension.cls"), `
public class AccountViewExtension {
  public String name;
  public AccountViewExtension(ApexPages.StandardController controller) {
    Account account = (Account) controller.getRecord();
    name = account.Name;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/AccountBadgeController.cls"), `
public class AccountBadgeController {
  public String value { get; set; }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/VisualforceControllerContractTest.cls"), `
@isTest
private class VisualforceControllerContractTest {
  @isTest static void resolvesPageTokenAndExtensionConstructor() {
    Account account = new Account(Name = 'Acme');
    ApexPages.StandardController controller = new ApexPages.StandardController(account);
    AccountViewExtension extension = new AccountViewExtension(controller);
    System.assertEquals('Acme', extension.name);
    Test.setCurrentPage(Page.AccountView);
    ApexPages.currentPage().getParameters().put('id', '001000000000001AAA');
    System.assertEquals('/apex/AccountView?id=001000000000001AAA', ApexPages.currentPage().getUrl());
    System.assertEquals('001000000000001AAA', ApexPages.currentPage().getParameters().get('id'));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunResolvesNamespacedDependencyVisualforcePageReferences(t *testing.T) {
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	consumerRoot := filepath.Join(root, "consumer")
	writeFile(t, filepath.Join(depRoot, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/default/pages/Order.page"), `<apex:page/>`)
	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{"namespace":"otherpkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(consumerRoot, "glade.yml"), `project:
  managedPackageDependencies: ["pkg:../dep:1.0"]
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/default/classes/PageDependencyTest.cls"), `
@isTest
private class PageDependencyTest {
  @isTest static void namespacedDependencyPageTokenResolves() {
    PageReference page = Page.pkg__Order;
    System.assertEquals('/apex/pkg__Order', page.getUrl());
  }
}
`)

	run := Run(loadTestIndex(t, consumerRoot), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunResetsApexPagesStateBetweenTestMethods(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/ResetProbe.page"), `<apex:page/>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/PageStateResetTest.cls"), `
@isTest
private class PageStateResetTest {
  @isTest static void addsMessageAndParameter() {
    Test.setCurrentPage(Page.ResetProbe);
    ApexPages.currentPage().getParameters().put('marker', 'dirty');
    ApexPages.addMessage(new ApexPages.Message(ApexPages.Severity.ERROR, 'Summary'));
    System.assert(ApexPages.hasMessages());
  }
  @isTest static void seesCleanPageState() {
    System.assert(!ApexPages.hasMessages());
    System.assertEquals(null, ApexPages.currentPage().getParameters().get('marker'));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestRunInitializesNullURLApexPagesCurrentPage(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/DefaultPageTest.cls"), `
@isTest
private class DefaultPageTest {
  @isTest static void hasBlankCurrentPage() {
    System.assertNotEquals(null, ApexPages.currentPage());
    System.assertEquals(null, ApexPages.currentPage().getUrl());
    ApexPages.currentPage().getParameters().put('id', '001000000000001AAA');
    System.assertEquals('001000000000001AAA', ApexPages.currentPage().getParameters().get('id'));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestProjectRuntimeCompilesStaticMapInitializerWithEscapedStrings(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/StaticMapProbe.cls"), `
public class StaticMapProbe {
  public static String lookup(String key) {
    return Values.get(key);
  }

  private static final Map<String, String> Values = new Map<String, String>{
    'US' => 'US',
    'L\'ANDORRE' => 'AD'
  };
}
`)
	machine := vm.New(nil)
	if err := RegisterProjectRuntimeForRequest(machine, loadTestIndex(t, root)); err != nil {
		t.Fatal(err)
	}
	value, err := machine.CallStatic("StaticMapProbe.lookup", []vm.Value{vm.String("L'ANDORRE")})
	if err != nil {
		t.Fatal(err)
	}
	if value.Kind != vm.ValueString || value.Text != "AD" {
		t.Fatalf("lookup = %#v, want AD", value)
	}
}

func TestProjectRuntimeInitializesStaticFieldsInSourceOrder(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/StaticConstants.cls"), `
public class StaticConstants {
  public static final Boolean IsInternal = false;
  public static final String Endpoint = StaticEndpointService.getEndpoint();
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/StaticEndpointService.cls"), `
public class StaticEndpointService {
  public static String getEndpoint() {
    if (StaticConstants.IsInternal) {
      return 'internal';
    }
    return 'external';
  }
}
`)
	machine := vm.New(nil)
	if err := RegisterProjectRuntimeForRequest(machine, loadTestIndex(t, root)); err != nil {
		t.Fatal(err)
	}
	value, err := machine.CallStatic("StaticEndpointService.getEndpoint", nil)
	if err != nil {
		t.Fatal(err)
	}
	if value.Kind != vm.ValueString || value.Text != "external" {
		t.Fatalf("endpoint = %#v, want external", value)
	}
}

func TestProjectRuntimeStaticFieldKeepsCommonSObjectWhenNestedTypeSharesName(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/SampleDataMapping.cls"), `
public class SampleDataMapping {
  public Organization organization;

  public class Organization {
    public String sfObject;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/constants.cls"), `
public class constants {
  public static final Organization ORG = [
    SELECT Id
    FROM Organization
    LIMIT 1
  ];
}
`)
	writeFile(t, filepath.Join(root, "force-app/test/classes/ConstantsStaticFieldTest.cls"), `
@isTest
private class ConstantsStaticFieldTest {
  @isTest static void orgUsesStandardSObject() {
    System.assertEquals('Organization', constants.ORG.getSObjectType().getDescribe().getName());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunNestedConstructorNamedLikeStandardSObjectWinsInLexicalOwner(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Wrapper.cls"), `
public class Wrapper {
  public static String makeName(ContentDocumentLink link) {
    Attachment item = new Attachment(link);
    return item.name;
  }

  public class Attachment {
    public String name;

    public Attachment(ContentDocumentLink link) {
      this.name = 'wrapped';
    }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/test/classes/WrapperTest.cls"), `
@isTest
private class WrapperTest {
  @isTest static void nestedAttachmentConstructorWins() {
    System.assertEquals('wrapped', Wrapper.makeName(new ContentDocumentLink()));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunNestedInterfaceClassLiteralSupportsTypeIsAssignableFrom(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ScriptHandler.cls"), `
public class ScriptHandler {
  public interface Runnable {
    void run();
  }

  public class Impl implements Runnable {
    public void run() {}
  }

  public static Boolean accepts(Type scriptType) {
    return Runnable.class.isAssignableFrom(scriptType);
  }

  public static Boolean acceptsByName(String scriptName) {
    Type scriptType = Type.forName(scriptName);
    return Runnable.class.isAssignableFrom(scriptType);
  }

  public static Boolean rejectsByName(String scriptName) {
    Type scriptType = Type.forName(scriptName);
    return Runnable.class.isAssignableFrom(scriptType) == false;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/test/classes/ScriptHandlerTest.cls"), `
@isTest
private class ScriptHandlerTest {
  public class TestImpl implements ScriptHandler.Runnable {
    public void run() {}
  }

  @isTest static void nestedInterfaceLiteralIsAssignable() {
    System.assertEquals(true, ScriptHandler.accepts(ScriptHandler.Impl.class));
  }

  @isTest static void nestedInterfaceLiteralIsAssignableFromForName() {
    System.assertEquals(true, ScriptHandler.acceptsByName('pkg.ScriptHandler.Impl'));
  }

  @isTest static void nestedInterfaceLiteralIsAssignableFromTestNestedForName() {
    System.assertEquals(true, ScriptHandler.acceptsByName('pkg.ScriptHandlerTest.TestImpl'));
  }

  @isTest static void nestedInterfaceLiteralComparisonKeepsArgument() {
    System.assertEquals(false, ScriptHandler.rejectsByName('pkg.ScriptHandlerTest.TestImpl'));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 4 || got.Passed != 4 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunAssignsDottedPathThroughStaticFieldRoot(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/BaseControllerProbe.cls"), `
public virtual class BaseControllerProbe {
  private String marker;
  public String Marker {
    get { return marker; }
    set { marker = value; }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/ConcreteControllerProbe.cls"), `
public class ConcreteControllerProbe extends BaseControllerProbe {
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/DottedStaticRootAssignmentTest.cls"), `
@isTest
private class DottedStaticRootAssignmentTest {
  private static ConcreteControllerProbe controller;

  @isTest static void assignsInheritedPropertyThroughStaticField() {
    controller = new ConcreteControllerProbe();
    controller.Marker = 'set';
    System.assertEquals('set', controller.Marker);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunJSONDeserializeUsesPropertySetter(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/JSONPropertySetterPayload.cls"), `
public class JSONPropertySetterPayload {
  public Date StartDate { get; set; }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/JSONPropertySetterEnvelope.cls"), `
public class JSONPropertySetterEnvelope {
  private JSONPropertySetterPayload payload;
  public JSONPropertySetterPayload Payload {
    get {
      if (payload == null) {
        payload = new JSONPropertySetterPayload();
      }
      return payload;
    }
    private set { payload = value; }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/JSONPropertySetterTest.cls"), `
@isTest
private class JSONPropertySetterTest {
  @isTest static void deserializePopulatesBackingFieldThroughSetter() {
    JSONPropertySetterEnvelope envelope = (JSONPropertySetterEnvelope)JSON.deserialize(
      '{"Payload":{"StartDate":"2026-05-07"}}',
      JSONPropertySetterEnvelope.class
    );
    System.assertNotEquals(null, envelope.Payload.StartDate);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunJSONDeserializeUntypedMapsMatchMapStringObjectInstanceOf(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/JSONUntypedMapInstanceOfTest.cls"), `
@isTest
private class JSONUntypedMapInstanceOfTest {
  @isTest static void untypedNestedMapsAreMapStringObject() {
    List<Object> rows = (List<Object>)JSON.deserializeUntyped(
      '[{"payload":{"id":"V-1","settings":{"organizationId":"ORG"}}}]'
    );
    Map<String,Object> row = (Map<String,Object>)rows[0];
    Object payload = row.get('payload');
    System.assert(payload instanceof Map<String,Object>, 'payload should be Map<String,Object>');
    Map<String,Object> payloadMap = (Map<String,Object>)payload;
    System.assertEquals('V-1', (String)payloadMap.get('id'));
    Object settings = payloadMap.get('settings');
    System.assert(settings instanceof Map<String,Object>, 'settings should be Map<String,Object>');
    System.assertEquals('ORG', (String)((Map<String,Object>)settings).get('organizationId'));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunJSONDeserializePopulatesCustomGetterAutoSetterListProperty(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/JSONAutoSetterMapping.cls"), `
public class JSONAutoSetterMapping {
  public JSONAutoSetterMapping provider;
  public class Row {
    public String tpField;
    public String sfField;
    public Boolean isComplete() {
      return String.isNotBlank(tpField) && String.isNotBlank(sfField);
    }
  }
  public List<Row> rows {
    get {
      if (rows?.isEmpty() == false) {
        List<Row> validRows = new List<Row>();
        for (Row row : rows) {
          if (row.isComplete()) {
            validRows.add(row);
          }
        }
        rows = validRows;
      }
      return rows;
    }
    set;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/JSONAutoSetterMappingTest.cls"), `
@isTest
private class JSONAutoSetterMappingTest {
  @isTest static void deserializePopulatesRows() {
    JSONAutoSetterMapping mapping = (JSONAutoSetterMapping)JSON.deserialize(
      '{"provider":{"rows":[{"tpField":"gender","sfField":"LeadSource"}]}}',
      JSONAutoSetterMapping.class
    );
    mapping = mapping.provider;
    System.assertEquals(1, mapping.rows.size());
    System.assertEquals('LeadSource', mapping.rows.get(0).sfField);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunJSONGeneratorRoundTripsCustomPropertyBackingField(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/JSONBackedPropertyPayload.cls"), `
public class JSONBackedPropertyPayload {
  private Boolean enabled = false;
  public Boolean Enabled {
    get { return enabled; }
    set { enabled = value; }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/JSONBackedPropertyEnvelope.cls"), `
public class JSONBackedPropertyEnvelope {
  private JSONBackedPropertyPayload payload;
  public JSONBackedPropertyPayload Payload {
    get {
      if (payload == null) {
        payload = new JSONBackedPropertyPayload();
      }
      return payload;
    }
    set { payload = value; }
  }
  public void saveTo(Account account) {
    JSONGenerator gen = JSON.createGenerator(false);
    gen.writeStartObject();
    gen.writeObjectField('Payload', payload);
    gen.writeEndObject();
    account.Description = gen.getAsString();
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/JSONBackedPropertyTest.cls"), `
@isTest
private class JSONBackedPropertyTest {
  @isTest static void generatorUsesCustomGetterNameForRoundTrip() {
    Account account = new Account();
    JSONBackedPropertyEnvelope envelope = new JSONBackedPropertyEnvelope();
    envelope.Payload.Enabled = true;
    envelope.saveTo(account);
    System.assertEquals('{"Payload":{"Enabled":true}}', account.Description);
    JSONBackedPropertyEnvelope restored = (JSONBackedPropertyEnvelope)JSON.deserialize(
      account.Description,
      JSONBackedPropertyEnvelope.class
    );
    System.assertEquals(true, restored.Payload.Enabled);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunPropertySetterSeesAutoPropertyAssignedInConstructor(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/SetterDispatchControllerProbe.cls"), `
public class SetterDispatchControllerProbe {
  public Account CurrentRecord { get; set; }
  public SetterDispatchControllerProbe() {
    CurrentRecord = new Account();
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/SetterDispatchBaseProbe.cls"), `
public virtual class SetterDispatchBaseProbe {
  private SetterDispatchControllerProbe c;
  public SetterDispatchControllerProbe Controller {
    get { return c; }
    set {
      c = value;
      OnControllerSet();
    }
  }
  public virtual void OnControllerSet() {}
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/SetterDispatchChildProbe.cls"), `
public class SetterDispatchChildProbe extends SetterDispatchBaseProbe {
  public override void OnControllerSet() {
    Controller.CurrentRecord.Name = 'set';
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/SetterDispatchPropertyTest.cls"), `
@isTest
private class SetterDispatchPropertyTest {
  private static SetterDispatchControllerProbe controller;
  private static SetterDispatchChildProbe child;

  @isTest static void setterDispatchSeesConstructorAssignedProperty() {
    controller = new SetterDispatchControllerProbe();
    System.assertNotEquals(null, controller.CurrentRecord);
    child = new SetterDispatchChildProbe();
    child.Controller = controller;
    System.assertEquals('set', controller.CurrentRecord.Name);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunInstanceFieldLookupIsCaseInsensitive(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/CaseInsensitiveFieldProbe.cls"), `
public class CaseInsensitiveFieldProbe {
  public Map<Object, List<Account>> RecordsByKey;
  public CaseInsensitiveFieldProbe() {
    RecordsByKey = new Map<Object, List<Account>>();
    System.assertEquals(0, recordsByKey.keySet().size());
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/CaseInsensitiveFieldLookupTest.cls"), `
@isTest
private class CaseInsensitiveFieldLookupTest {
  @isTest static void constructorReadsFieldWithDifferentCase() {
    new CaseInsensitiveFieldProbe();
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestProjectRuntimeInitializesNestedInstanceFields(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/NestedInitializer.cls"), `
public class NestedInitializer {
  public static Inner StaticInner { get; private set; }
  public static TestFactory Test { get; private set; }
  static {
    StaticInner = new Inner();
    Test = new TestFactory();
  }
  public class Inner {
    private List<String> values = new List<String>();
    public Child child = new Child();
    public Child Database = new Child();
    private Inner() {
    }
    public Integer size() {
      values.add('x');
      return values.size();
    }
  }
  public class Child {
    public Integer value() {
      return 7;
    }
  }
  public class TestFactory {
    public MockDatabase Database = new MockDatabase();
    private TestFactory() {
    }
  }
  public class MockDatabase {
    private List<String> rows = new List<String>();
    private MockDatabase() {
    }
    public Boolean hasRecords() {
      return rows != null;
    }
  }
  public static Integer run() {
    return new Inner().size();
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/NestedInitializerTest.cls"), `
@isTest
private class NestedInitializerTest {
  @isTest static void initializes() {
    System.assertEquals(1, NestedInitializer.run());
    System.assertEquals(7, NestedInitializer.StaticInner.child.value());
    System.assertEquals(7, NestedInitializer.StaticInner.Database.value());
    System.assert(NestedInitializer.Test.Database.hasRecords());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
}

func TestQualifyNestedTypeNameResolvesTopLevelOwnerNestedType(t *testing.T) {
	known := map[string]bool{
		"Outer":       true,
		"Outer.Inner": true,
	}
	if got := qualifyNestedTypeName("Outer", "Inner", known); got != "Outer.Inner" {
		t.Fatalf("qualified type = %q, want Outer.Inner", got)
	}
}

func TestProjectRuntimeMatchesSObjectListDowncastConstructors(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ListDowncastDomain.cls"), `
public class ListDowncastDomain {
  public List<Opportunity> Records;
  public ListDowncastDomain(List<Opportunity> source) {
    Records = source;
  }
  public static Integer run() {
    List<SObject> records = new List<SObject>{ new Opportunity(Name = 'Test') };
    ListDowncastDomain domain = new ListDowncastDomain(records);
    return domain.Records.size();
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/CustomListDowncastDomain.cls"), `
public class CustomListDowncastDomain {
  public List<Thing__c> Records;
  public CustomListDowncastDomain(List<Thing__c> source) {
    Records = source;
  }
  public static Integer run() {
    List<SObject> records = new List<SObject>{ new Thing__c(Name = 'Test') };
    CustomListDowncastDomain domain = new CustomListDowncastDomain(records);
    return domain.Records.size();
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/Thing__c/Thing__c.object-meta.xml"), `
<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata">
  <label>Thing</label>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ListDowncastDomainTest.cls"), `
@isTest
private class ListDowncastDomainTest {
  @isTest static void constructs() {
    System.assertEquals(1, ListDowncastDomain.run());
    System.assertEquals(1, CustomListDowncastDomain.run());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestProjectRuntimeCallsImplicitDefaultSuperConstructor(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ImplicitSuperBase.cls"), `
public virtual class ImplicitSuperBase {
  public List<String> values;
  public ImplicitSuperBase() {
    values = new List<String>();
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ImplicitSuperChild.cls"), `
public class ImplicitSuperChild extends ImplicitSuperBase {
  public Integer size() {
    values.add('x');
    return values.size();
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ImplicitSuperChildTest.cls"), `
@isTest
private class ImplicitSuperChildTest {
  @isTest static void constructsBase() {
    System.assertEquals(1, new ImplicitSuperChild().size());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestProjectRuntimeMatchesChainedConstructorWithSObjectTypeAlias(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ChainedSObjectTypeCtor.cls"), `
public class ChainedSObjectTypeCtor {
  public SObjectType Captured;
  public ChainedSObjectTypeCtor(List<SObject> records) {
    this(records, records.getSObjectType());
  }
  public ChainedSObjectTypeCtor(List<SObject> records, SObjectType objectType) {
    Captured = objectType;
  }
  public static SObjectType run() {
    return new ChainedSObjectTypeCtor(new List<SObject>{ new Account(Name = 'Test') }).Captured;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ChainedSObjectTypeCtorTest.cls"), `
@isTest
private class ChainedSObjectTypeCtorTest {
  @isTest static void constructs() {
    System.assertEquals(Account.SObjectType, ChainedSObjectTypeCtor.run());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestProjectRuntimeSuperclassThisConstructorChainsLexically(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/LexicalBaseCtor.cls"), `
public virtual class LexicalBaseCtor {
  public Integer value;
  public LexicalBaseCtor(Integer seed) {
    this(seed, 2);
  }
  public LexicalBaseCtor(Integer seed, Integer multiplier) {
    value = seed * multiplier;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/LexicalChildCtor.cls"), `
public class LexicalChildCtor extends LexicalBaseCtor {
  public LexicalChildCtor(Integer seed) {
    super(seed);
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/LexicalChildCtorTest.cls"), `
@isTest
private class LexicalChildCtorTest {
  @isTest static void constructs() {
    LexicalChildCtor child = new LexicalChildCtor(3);
    System.assertEquals(6, child.value);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestProjectRuntimeAssignsNestedInterfaceByShortName(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/NestedInterfaceOwner.cls"), `
public class NestedInterfaceOwner {
  public interface Worker {
    Integer run();
  }
  public class Impl implements Worker {
    public Integer run() {
      return 7;
    }
  }
  public static Integer execute() {
    Worker worker = new Impl();
    return worker.run();
  }
  public static Integer executeFromType() {
    Type implType = Type.forName('NestedInterfaceOwner.Impl');
    Worker worker = (Worker) implType.newInstance();
    return worker.run();
  }
  public static Integer executeFromInterfaceMap() {
    Map<String, Worker> workers = new Map<String, Worker>();
    workers.put('one', (Worker) Type.forName('NestedInterfaceOwner.Impl').newInstance());
    return workers.get('one').run();
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/NestedInterfaceOwnerTest.cls"), `
@isTest
private class NestedInterfaceOwnerTest {
  @isTest static void assignsShortName() {
    System.assertEquals(7, NestedInterfaceOwner.execute());
    System.assertEquals(7, NestedInterfaceOwner.executeFromType());
    System.assertEquals(7, NestedInterfaceOwner.executeFromInterfaceMap());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestProjectRuntimeStaticFieldInitializerCanReadHierarchyCustomSetting(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/StaticSettings.cls"), `
public class StaticSettings {
  public static final Boolean IsInternal = Setup_Settings__c.getOrgDefaults().IsInternalOrg__c;
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/Setup_Settings__c/Setup_Settings__c.object-meta.xml"), `
<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata">
  <customSettingsType>Hierarchy</customSettingsType>
  <label>Setup Settings</label>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/Setup_Settings__c/fields/IsInternalOrg__c.field-meta.xml"), `
<CustomField xmlns="http://soap.sforce.com/2006/04/metadata">
  <fullName>IsInternalOrg__c</fullName>
  <type>Checkbox</type>
</CustomField>
`)
	org := storage.NewOrgState()
	org.Objects["Setup_Settings__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:  "Setup_Settings__c",
			Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "Hierarchy"},
			Fields: map[string]storage.Field{
				"SetupOwnerId":     {APIName: "SetupOwnerId", Type: storage.FieldString},
				"IsInternalOrg__c": {APIName: "IsInternalOrg__c", Type: storage.FieldBoolean},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a0s000000000001": {
				ID:     "a0s000000000001",
				Object: "Setup_Settings__c",
				Fields: map[string]storage.Value{
					"SetupOwnerId":     storage.StringValue("00D000000000001"),
					"IsInternalOrg__c": storage.BooleanValue(false),
				},
			},
		},
	}
	machine := vm.New(nil)
	machine.SetOrg(&org)
	if err := RegisterProjectRuntimeForRequest(machine, loadTestIndex(t, root)); err != nil {
		t.Fatal(err)
	}
	program, err := vm.CompileAnonymous(`System.assertEquals(false, StaticSettings.IsInternal);`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRunHierarchyCustomSettingAbsentOrgDefaultsEqualsFreshEmptySObject(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/SetupSettingsDefaultsTest.cls"), `
@isTest
private class SetupSettingsDefaultsTest {
  @isTest static void absentDefaultsEqualsFreshEmpty() {
    Setup_Settings__c defaults = Setup_Settings__c.getOrgDefaults();
    System.assertEquals(false, defaults.IsInternalOrg__c);
    System.assertEquals(new Setup_Settings__c(), defaults);
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/Setup_Settings__c/Setup_Settings__c.object-meta.xml"), `
<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata">
  <customSettingsType>Hierarchy</customSettingsType>
  <label>Setup Settings</label>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/Setup_Settings__c/fields/IsInternalOrg__c.field-meta.xml"), `
<CustomField xmlns="http://soap.sforce.com/2006/04/metadata">
  <fullName>IsInternalOrg__c</fullName>
  <type>Checkbox</type>
  <defaultValue>false</defaultValue>
</CustomField>
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunSparseHierarchyCustomSettingDoesNotReadTextDefault(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/SetupSettingsSparseTest.cls"), `
@isTest
private class SetupSettingsSparseTest {
  private static Setup_Settings__c settings;
  private static Setup_Settings__c forTests(Setup_Settings__c input) {
    if (settings == null) {
      settings = new Setup_Settings__c();
    }
    settings.Mode__c = input.Mode__c;
    return settings;
  }
  @isTest static void sparseSettingsObjectKeepsUnsetTextFieldNull() {
    Setup_Settings__c configured = forTests(new Setup_Settings__c());
    System.assertEquals(null, configured.Mode__c);
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/Setup_Settings__c/Setup_Settings__c.object-meta.xml"), `
<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata">
  <customSettingsType>Hierarchy</customSettingsType>
  <label>Setup Settings</label>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/Setup_Settings__c/fields/Mode__c.field-meta.xml"), `
<CustomField xmlns="http://soap.sforce.com/2006/04/metadata">
  <fullName>Mode__c</fullName>
  <type>Text</type>
  <defaultValue>&quot;Membership&quot;</defaultValue>
  <length>255</length>
</CustomField>
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunDescribeFieldResultReadsInlineHelpTextFromMetadata(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/InlineHelpTextTest.cls"), `
@isTest
private class InlineHelpTextTest {
  @isTest static void describesInlineHelpText() {
    System.assertEquals('Opportunity.Amount', Import_Row__c.Donation_Amount__c.getDescribe().getInlineHelpText());
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/Import_Row__c/Import_Row__c.object-meta.xml"), `
<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata">
  <label>Import Row</label>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/Import_Row__c/fields/Donation_Amount__c.field-meta.xml"), `
<CustomField xmlns="http://soap.sforce.com/2006/04/metadata">
  <fullName>Donation_Amount__c</fullName>
  <inlineHelpText>Opportunity.Amount</inlineHelpText>
  <label>Donation Amount</label>
  <type>Currency</type>
  <precision>18</precision>
  <scale>2</scale>
</CustomField>
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunListCustomSettingRequiredFieldDescribeDrivesInsert(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ListSettingDescribeTest.cls"), `
@isTest
private class ListSettingDescribeTest {
  @isTest static void requiredTextFieldInsertedAndFoundByName() {
    Schema.SObjectType settingType = Schema.getGlobalDescribe().get('Card__c');
    SObject record = settingType.newSObject();
    for (Schema.SObjectField field : settingType.getDescribe().fields.getMap().values()) {
      Schema.DescribeFieldResult describe = field.getDescribe();
      if (describe.isUpdateable() && !describe.isNillable()) {
        record.put(describe.getName(), 'Example');
      }
    }
    insert record;
    Card__c actual = Card__c.getInstance('Example');
    System.assertNotEquals(null, actual);
    System.assertEquals('Example', actual.Type__c);
    Card__c nullName = Card__c.getInstance(null);
    System.assertEquals(null, nullName);
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/Card__c/Card__c.object-meta.xml"), `
<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata">
  <customSettingsType>List</customSettingsType>
  <label>Card</label>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/Card__c/fields/Type__c.field-meta.xml"), `
<CustomField xmlns="http://soap.sforce.com/2006/04/metadata">
  <fullName>Type__c</fullName>
  <label>Type</label>
  <required>true</required>
  <type>Text</type>
</CustomField>
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func loadTestIndex(t *testing.T, root string) typesys.Index {
	t.Helper()
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := gladeschema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	return typesys.Build(p, s)
}

func TestOrgFromIndexIncludesGeneratedStandardSchema(t *testing.T) {
	org := orgFromIndex(typesys.Index{})

	for objectName, fieldName := range map[string]string{
		"Account":             "AccountNumber",
		"Task":                "WhatId",
		"PricebookEntry":      "Product2Id",
		"OpportunityLineItem": "PricebookEntryId",
	} {
		state, ok := org.Objects[objectName]
		if !ok {
			t.Fatalf("%s object was not exposed", objectName)
		}
		if _, ok := state.Definition.Fields[fieldName]; !ok {
			t.Fatalf("%s.%s field was not exposed; fields=%#v", objectName, fieldName, state.Definition.Fields)
		}
	}
}

func TestOrgFromIndexPreservesNamespacedCustomMetadataParentRelationships(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/StateTransition__mdt/StateTransition__mdt.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>State Transition</label></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/StateTransitionCallback__mdt/StateTransitionCallback__mdt.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>State Transition Callback</label></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/StateTransitionCallback__mdt/fields/TriggeringTransition__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>TriggeringTransition__c</fullName><label>Triggering Transition</label><type>MetadataRelationship</type><referenceTo>StateTransition__mdt</referenceTo><relationshipName>StateTransitionCallbacks</relationshipName></CustomField>`)

	org := orgFromIndex(loadTestIndex(t, root))
	program, err := vm.CompileAnonymous(`
Map<String, Schema.SObjectField> fields = StateTransitionCallback__mdt.SObjectType.getDescribe().fields.getMap();
String relationshipName = StateTransitionCallback__mdt.TriggeringTransition__c.getDescribe().getRelationshipName();
System.assertEquals('pkg__TriggeringTransition__r', relationshipName);
System.assertNotEquals(null, fields.get(relationshipName));
Boolean found = false;
for (Schema.SObjectField field : fields.values()) {
    Schema.DescribeFieldResult describe = field.getDescribe();
    if (describe.getRelationshipName() == relationshipName && describe.getReferenceTo().size() > 0) {
        found = true;
    }
}
System.assertEquals(true, found);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := vm.New(nil)
	machine.Org = &org
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestOrgFromIndexIncludesProjectReferencedStandardFields(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/AssetProbe.cls"), `
public class AssetProbe {
	public static void touch() {
		Asset asset = new Asset();
		asset.ExternalIdentifier = 'external';
	}
}
`)
	index := loadTestIndex(t, root)

	org := orgFromIndex(index)
	state, ok := org.Objects["Asset"]
	if !ok {
		t.Fatal("Asset object was not exposed")
	}
	field, ok := state.Definition.Fields["ExternalIdentifier"]
	if !ok {
		t.Fatalf("Asset.ExternalIdentifier was not loaded from standard metadata; fields=%#v", state.Definition.Fields)
	}
	if field.Type != storage.FieldString || field.DisplayType != "STRING" {
		t.Fatalf("Asset.ExternalIdentifier field = %#v", field)
	}
}

func TestOrgFromIndexIncludesCareProgramParentRelationship(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/CareProgramProbe.cls"), `
public class CareProgramProbe {
	public static CareProgram parent(CareProgram program) {
		return program.ParentProgram;
	}
}
`)
	index := loadTestIndex(t, root)

	org := orgFromIndex(index)
	state, ok := org.Objects["CareProgram"]
	if !ok {
		t.Fatal("CareProgram object was not exposed")
	}
	field, ok := state.Definition.Fields["ParentProgramId"]
	if !ok || field.Type != storage.FieldReference || field.RelationshipName != "ParentProgram" || len(field.ReferenceTo) != 1 || field.ReferenceTo[0] != "CareProgram" {
		t.Fatalf("CareProgram.ParentProgramId = %#v", field)
	}
}

func assertUnknownProjectReferencedField(t *testing.T, field storage.Field) {
	t.Helper()
	if field.Type != storage.FieldAny || field.DisplayType != "" || field.DefaultValue != "" ||
		len(field.ReferenceTo) != 0 || field.RelationshipName != "" || field.ChildRelationshipName != "" {
		t.Fatalf("field = %#v, want unknown metadata placeholder", field)
	}
}

func TestOrgFromIndexKeepsProjectReferencedBooleanLikeFieldsUnknownWithoutMetadata(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ManagedBooleanProbe.cls"), `
public class ManagedBooleanProbe {
	public static void touch(Account account, Contact contact) {
		account.pkg__SYSTEMIsIndividual__c = true;
		Schema.SObjectField fieldToken = Account.pkg__SystemIsIndividual__c;
		Boolean privateContact = contact.pkg__Private__c != false;
	}
}
`)
	index := loadTestIndex(t, root)

	org := orgFromIndex(index)
	accountState := org.Objects["Account"]
	accountFieldName, ok := storage.ResolveFieldName(accountState.Definition, org.Namespace, "pkg__SYSTEMIsIndividual__c")
	if !ok {
		t.Fatalf("Account.pkg__SYSTEMIsIndividual__c was not inferred; fields=%#v", accountState.Definition.Fields)
	}
	accountField := accountState.Definition.Fields[accountFieldName]
	assertUnknownProjectReferencedField(t, accountField)
	matches := 0
	for name, field := range accountState.Definition.Fields {
		if strings.EqualFold(name, "pkg__SYSTEMIsIndividual__c") {
			matches++
			assertUnknownProjectReferencedField(t, field)
		}
	}
	if matches != 1 {
		t.Fatalf("Account.pkg__SYSTEMIsIndividual__c case variants = %d; fields=%#v", matches, accountState.Definition.Fields)
	}
	contactField := org.Objects["Contact"].Definition.Fields["pkg__Private__c"]
	assertUnknownProjectReferencedField(t, contactField)
}

func TestOrgFromIndexKeepsManagedBooleanFieldsUnknownFromBooleanUsageWithoutMetadata(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ManagedBooleanProbe.cls"), `
public class ManagedBooleanProbe {
	private pkg__Settings__c settings;
	public void touch(Account account, Contact contact) {
		account.pkg__ManagedFlag__c = true;
		Boolean privateContact = contact.pkg__Private__c != false;
		Boolean defaultValue = !settings.pkg__DefaultEnabled__c;
	}
}
`)
	org := orgFromIndex(loadTestIndex(t, root))
	accountField := org.Objects["Account"].Definition.Fields["pkg__ManagedFlag__c"]
	assertUnknownProjectReferencedField(t, accountField)
	contactField := org.Objects["Contact"].Definition.Fields["pkg__Private__c"]
	assertUnknownProjectReferencedField(t, contactField)
	settingsField := org.Objects["pkg__Settings__c"].Definition.Fields["pkg__DefaultEnabled__c"]
	assertUnknownProjectReferencedField(t, settingsField)
}

func TestOrgFromIndexKeepsManagedNumericFieldsUnknownFromNumericUsageWithoutMetadata(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ManagedNumericProbe.cls"), `
public class ManagedNumericProbe {
	public void touch(Opportunity opp) {
		Boolean noAmounts = opp.pkg__Amount_Total__c == 0 && 0 == opp.pkg__Installment_Count__c;
		opp.pkg__Manual_Count__c = 2;
	}
}
`)
	org := orgFromIndex(loadTestIndex(t, root))
	opportunity := org.Objects["Opportunity"].Definition.Fields
	for _, fieldName := range []string{"pkg__Amount_Total__c", "pkg__Installment_Count__c", "pkg__Manual_Count__c"} {
		field := opportunity[fieldName]
		assertUnknownProjectReferencedField(t, field)
	}
}

func TestRecordProjectReferencedBooleanFieldKeepsUnknownPlaceholder(t *testing.T) {
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	org.Objects["pkg__Settings__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "pkg__Settings__c",
			Fields: map[string]storage.Field{
				"pkg__DefaultEnabled__c": {APIName: "pkg__DefaultEnabled__c", Type: storage.FieldAny},
			},
		},
	}

	recordProjectReferencedBooleanField(&org, map[string]map[string]storage.Field{}, nil, "pkg__Settings__c", "pkg__DefaultEnabled__c")

	field := org.Objects["pkg__Settings__c"].Definition.Fields["pkg__DefaultEnabled__c"]
	assertUnknownProjectReferencedField(t, field)
}

func TestRunInfersListCustomSettingFromGetAllOnReferencedShell(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ListSettingProbeTest.cls"), `
@IsTest
private class ListSettingProbeTest {
  @IsTest static void getAllUsesListCustomSettingShell() {
    Schema.SObjectField token = Payment_Field_Mapping_Settings__c.Payment_Field__c;
    Map<String, Payment_Field_Mapping_Settings__c> rows = Payment_Field_Mapping_Settings__c.getAll();
    System.assertEquals(0, rows.size());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestOrgFromIndexIncludesProjectReferencedSchemaFieldTokens(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/SchemaFieldProbe.cls"), `
public class SchemaFieldProbe {
	public static Schema.SObjectField contactProcessor() {
		return Schema.SObjectType.Contact.fields.pkg__Processor__c;
	}
}
`)
	index := loadTestIndex(t, root)

	org := orgFromIndex(index)
	field, ok := org.Objects["Contact"].Definition.Fields["pkg__Processor__c"]
	if !ok {
		t.Fatalf("Contact.pkg__Processor__c was not inferred; fields=%#v", org.Objects["Contact"].Definition.Fields)
	}
	assertUnknownProjectReferencedField(t, field)
}

func TestOrgFromIndexDoesNotInferChildRelationshipNamesAsFields(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ChildRelationshipProbe.cls"), `
public class ChildRelationshipProbe {
	public static void touch(Account account) {
		Object staticReference = Account.Contacts;
		Object variableReference = account.Contacts;
	}
}
`)
	index := loadTestIndex(t, root)

	org := orgFromIndex(index)
	account := org.Objects["Account"]
	if _, ok := account.Definition.Fields["Contacts"]; ok {
		t.Fatalf("Account.Contacts was inferred as a field; fields=%#v", account.Definition.Fields)
	}
	foundRelationship := false
	for _, relation := range org.Objects["Contact"].Definition.Relations {
		if relation.ChildRelationship == "Contacts" {
			foundRelationship = true
			break
		}
	}
	if !foundRelationship {
		t.Fatalf("Contact -> Account child relationship Contacts missing; relations=%#v", org.Objects["Contact"].Definition.Relations)
	}
}

func TestOrgFromIndexIncludesProjectReferencedSObjectLiteralFields(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/SObjectLiteralProbe.cls"), `
public class SObjectLiteralProbe {
	public static void touch() {
		Contact contact = new Contact(LastName = 'Probe', pkg__PreferredPhone__c = 'Home');
		Account account = new Account(Name = 'Probe', pkg__SystemIsIndividual__c = true);
	}
}
`)
	index := loadTestIndex(t, root)

	org := orgFromIndex(index)
	contactField, ok := org.Objects["Contact"].Definition.Fields["pkg__PreferredPhone__c"]
	if !ok {
		t.Fatalf("Contact.pkg__PreferredPhone__c was not inferred; fields=%#v", org.Objects["Contact"].Definition.Fields)
	}
	assertUnknownProjectReferencedField(t, contactField)
	accountFieldName, ok := storage.ResolveFieldName(org.Objects["Account"].Definition, org.Namespace, "pkg__SYSTEMIsIndividual__c")
	if !ok {
		t.Fatalf("Account.pkg__SYSTEMIsIndividual__c was not inferred; fields=%#v", org.Objects["Account"].Definition.Fields)
	}
	accountField := org.Objects["Account"].Definition.Fields[accountFieldName]
	assertUnknownProjectReferencedField(t, accountField)
}

func TestOrgFromIndexKeepsNamespacedSObjectLiteralFieldsUnknownWithoutMetadata(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/pkg__ProcessingFee__c/pkg__ProcessingFee__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Processing Fee</label></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ProcessingFeeProbe.cls"), `
public class ProcessingFeeProbe {
	public static void touch() {
		List<pkg__ProcessingFee__c> fees = new List<pkg__ProcessingFee__c>();
		fees.add(new pkg__ProcessingFee__c(pkg__Mandatory__c = false));
	}
}
`)
	org := orgFromIndex(loadTestIndex(t, root))
	field := org.Objects["pkg__ProcessingFee__c"].Definition.Fields["pkg__Mandatory__c"]
	assertUnknownProjectReferencedField(t, field)
}

func TestOrgFromIndexKeepsInferredFieldUnknownWhenLaterLiteralProvidesValue(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/pkg__ProcessingFee__c/pkg__ProcessingFee__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Processing Fee</label></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/AProcessingFeeService.cls"), `
public class AProcessingFeeService {
	public static Object mandatory(pkg__ProcessingFee__c fee) {
		return fee.pkg__Mandatory__c;
	}
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ZProcessingFeeTest.cls"), `
public class ZProcessingFeeTest {
	public static void touch() {
		pkg__ProcessingFee__c fee = new pkg__ProcessingFee__c(pkg__Mandatory__c = false);
	}
}
`)
	org := orgFromIndex(loadTestIndex(t, root))
	field := org.Objects["pkg__ProcessingFee__c"].Definition.Fields["pkg__Mandatory__c"]
	assertUnknownProjectReferencedField(t, field)
}

func TestRecordProjectReferencedStandardFieldKeepsInferredShapeUnknown(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["pkg__ProcessingFee__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "pkg__ProcessingFee__c",
			Fields:  map[string]storage.Field{},
		},
		Records: map[storage.ID]storage.Record{},
	}
	inferred := map[string]map[string]storage.Field{}
	recordProjectReferencedStandardField(&org, inferred, nil, "pkg__ProcessingFee__c", "pkg__Mandatory__c")
	applyReferencedStandardFieldSet(&org, inferred, nil)
	field := org.Objects["pkg__ProcessingFee__c"].Definition.Fields["pkg__Mandatory__c"]
	if field.Type != storage.FieldAny {
		t.Fatalf("initial inferred field = %#v", field)
	}

	recordProjectReferencedStandardField(&org, map[string]map[string]storage.Field{}, nil, "pkg__ProcessingFee__c", "pkg__Mandatory__c")
	field = org.Objects["pkg__ProcessingFee__c"].Definition.Fields["pkg__Mandatory__c"]
	if field.Type != storage.FieldAny {
		t.Fatalf("repeated inferred field = %#v", field)
	}
}

func TestOrgFromIndexIncludesProjectReferencedCustomLookupFields(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"otherpkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/pkg__Order__c/pkg__Order__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Order</label></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/pkg__Entity__c/pkg__Entity__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Entity</label></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/OrderProbe.cls"), `
public class OrderProbe {
	public static void touch() {
		Schema.SObjectField field = pkg__Order__c.pkg__Entity__c;
	}
}
`)
	index := loadTestIndex(t, root)

	org := orgFromIndex(index)
	state := org.Objects["pkg__Order__c"]
	field, ok := state.Definition.Fields["pkg__Entity__c"]
	if !ok {
		t.Fatalf("pkg__Order__c.pkg__Entity__c was not inferred; fields=%#v", state.Definition.Fields)
	}
	assertUnknownProjectReferencedField(t, field)
	if parentRelationshipKnown(state.Definition, "pkg__Entity__r") {
		t.Fatalf("pkg__Entity__r relationship inferred without lookup metadata: %#v", state.Definition.Relations)
	}
}

func TestOrgFromIndexInfersLookupFromTypedSObjectIDLiteralAssignment(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/pkg__Fee__c/pkg__Fee__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Fee</label></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/pkg__GatewayLink__c/pkg__GatewayLink__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Gateway Link</label></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/FeeProbe.cls"), `
public class FeeProbe {
	public static void touch() {
		pkg__GatewayLink__c link = new pkg__GatewayLink__c();
		pkg__Fee__c fee = new pkg__Fee__c(pkg__GatewayLink__c = link.Id);
	}
}
`)
	org := orgFromIndex(loadTestIndex(t, root))
	state := org.Objects["pkg__Fee__c"]
	field := state.Definition.Fields["pkg__GatewayLink__c"]
	if field.Type != storage.FieldReference || len(field.ReferenceTo) != 1 || field.ReferenceTo[0] != "pkg__GatewayLink__c" || field.RelationshipName != "pkg__GatewayLink__r" {
		t.Fatalf("pkg__Fee__c.pkg__GatewayLink__c = %#v, want lookup to pkg__GatewayLink__c", field)
	}
	if !parentRelationshipKnown(state.Definition, "pkg__GatewayLink__r") {
		t.Fatalf("pkg__GatewayLink__r relation missing: %#v", state.Definition.Relations)
	}
}

func TestOrgFromIndexKeepsManagedLookupLikeFieldUnknownWithoutMetadata(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/pkg__Payment__c/pkg__Payment__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Payment</label></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/PaymentProbe.cls"), `
public class PaymentProbe {
	public static void touch(Id opportunityId) {
		pkg__Payment__c payment = new pkg__Payment__c(pkg__Opportunity__c = opportunityId);
		Opportunity opp = [SELECT Id, (SELECT Id FROM pkg__Payment__r) FROM Opportunity WHERE Id = :opportunityId];
		System.assertEquals(opportunityId, payment.pkg__Opportunity__c);
		System.assertEquals(0, opp.pkg__Payment__r.size());
	}
}
`)
	index := loadTestIndex(t, root)

	org := orgFromIndex(index)
	state := org.Objects["pkg__Payment__c"]
	field, ok := state.Definition.Fields["pkg__Opportunity__c"]
	if !ok {
		t.Fatalf("pkg__Payment__c.pkg__Opportunity__c was not inferred; fields=%#v", state.Definition.Fields)
	}
	assertUnknownProjectReferencedField(t, field)
	for _, relation := range state.Definition.Relations {
		if relation.ChildRelationship == "pkg__Payment__r" && len(relation.ParentObjects) == 1 && relation.ParentObjects[0] == "Opportunity" {
			t.Fatalf("pkg__Payment__r relationship inferred without lookup metadata: %#v", state.Definition.Relations)
		}
	}
}

func TestOrgFromIndexInfersLookupFromExplicitParentChildRelationshipSource(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/pkg__Payment__c/pkg__Payment__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Payment</label></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/PaymentProbe.cls"), `
public class PaymentProbe {
	public static void touch() {
		Opportunity opp = new Opportunity(Name = 'Donation');
		pkg__Payment__c payment = new pkg__Payment__c(pkg__Opportunity__c = opp.Id);
		String fieldName = String.valueOf(pkg__Payment__c.pkg__Opportunity__c);
		String query = 'SELECT Id, (SELECT Id FROM Opportunity.pkg__Payment__r) FROM Opportunity';
	}
}
`)
	org := orgFromIndex(loadTestIndex(t, root))
	state := org.Objects["pkg__Payment__c"]
	field := state.Definition.Fields["pkg__Opportunity__c"]
	if field.Type != storage.FieldReference || len(field.ReferenceTo) != 1 || field.ReferenceTo[0] != "Opportunity" || field.RelationshipName != "pkg__Opportunity__r" || field.ChildRelationshipName != "pkg__Payment__r" {
		t.Fatalf("pkg__Payment__c.pkg__Opportunity__c = %#v, want Opportunity lookup with child relationship", field)
	}
	for _, relation := range state.Definition.Relations {
		if relation.Field == "pkg__Opportunity__c" && relation.ChildRelationship == "pkg__Payment__r" {
			return
		}
	}
	t.Fatalf("pkg__Payment__r relationship missing: %#v", state.Definition.Relations)
}

func TestOrgFromIndexIncludesProjectReferencedManagedFieldTokens(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"otherpkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/PasscodeProbe.cls"), `
public class PasscodeProbe {
	public static void touch() {
		Schema.SObjectField field = pkg__Passcode__c.pkg__Code__c;
	}
}
`)
	index := loadTestIndex(t, root)

	org := orgFromIndex(index)
	state, ok := org.Objects["pkg__Passcode__c"]
	if !ok {
		t.Fatalf("pkg__Passcode__c object was not inferred")
	}
	field := state.Definition.Fields["pkg__Code__c"]
	assertUnknownProjectReferencedField(t, field)
}

func TestOrgFromIndexProjectReferencedFieldCacheTracksSourceBody(t *testing.T) {
	projectReferencedStandardFieldCache = sync.Map{}
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	probePath := filepath.Join(root, "force-app/main/classes/AccountProbe.cls")
	writeFile(t, probePath, `
public class AccountProbe {
	public static void touch() {
		Account account = new Account(Name = 'Plain');
		System.debug(account.Name);
	}
}
`)
	org := orgFromIndex(loadTestIndex(t, root))
	if _, ok := org.Objects["Account"].Definition.Fields["PersonEmail"]; ok {
		t.Fatal("PersonEmail should stay gated without a person-account reference")
	}

	writeFile(t, probePath, `
public class AccountProbe {
	public static void touch() {
		Account account = new Account(PersonEmail = 'trail@example.com');
		System.debug(account.PersonEmail);
	}
}
`)
	org = orgFromIndex(loadTestIndex(t, root))
	if _, ok := org.Objects["Account"].Definition.Fields["PersonEmail"]; !ok {
		t.Fatal("PersonEmail was not inferred after source changed with the same type count")
	}
}

func TestOrgFromIndexKeepsProjectReferencedCurrencyLikeFieldUnknownWithoutMetadata(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/pkg__Order__c/pkg__Order__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Order</label></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/OrderProbe.cls"), `
public class OrderProbe {
	public static void touch() {
		Schema.SObjectField field = pkg__Order__c.pkg__TotalTax__c;
	}
}
`)
	index := loadTestIndex(t, root)

	org := orgFromIndex(index)
	field := org.Objects["pkg__Order__c"].Definition.Fields["pkg__TotalTax__c"]
	assertUnknownProjectReferencedField(t, field)
}

func TestOrgFromIndexKeepsProjectReferencedSubtotalFieldUnknownWithoutMetadata(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/pkg__Cart__c/pkg__Cart__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Cart</label></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/CartProbe.cls"), `
public class CartProbe {
	public static void touch() {
		Schema.SObjectField field = pkg__Cart__c.pkg__Subtotal__c;
	}
}
`)
	index := loadTestIndex(t, root)

	org := orgFromIndex(index)
	field := org.Objects["pkg__Cart__c"].Definition.Fields["pkg__Subtotal__c"]
	assertUnknownProjectReferencedField(t, field)
}

func TestOrgFromIndexSynthesizesReferencedManagedFieldSets(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/FieldSetProbe.cls"), `
public class FieldSetProbe {
	private static final String ACCOUNT_FIELD_SET_NAME = 'pkg__AccountEdit';
	public Schema.SObjectType getSObjectType() {
		return Account.SObjectType;
	}
}
`)

	org := orgFromIndex(loadTestIndex(t, root))
	for _, fieldSet := range org.Metadata.FieldSets {
		if fieldSet.ObjectName == "Account" && fieldSet.Name == "pkg__AccountEdit" && len(fieldSet.Fields) > 0 {
			for _, field := range fieldSet.Fields {
				if field.Field == "Name" && !field.Required {
					t.Fatalf("synthetic Account field set did not mark Name required")
				}
			}
			return
		}
	}
	t.Fatalf("synthetic Account field set was not created: %#v", org.Metadata.FieldSets)
}

func TestOrgFromIndexSynthesizesReferencedDependencyFieldSets(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "BadgeProbe.cls")
	writeFile(t, source, `
global class BadgeProbe {
	private static final String BADGE_FIELDSET = 'pkg__BadgeAddEdit';
	global pkg__Badge__c getRecord() {
		return new pkg__Badge__c();
	}
}
`)
	org := storage.OrgState{
		Namespace: "pkg",
		Objects: map[string]storage.ObjectState{
			"pkg__Badge__c": {
				Definition: storage.ObjectDefinition{
					APIName: "pkg__Badge__c",
					Fields: map[string]storage.Field{
						"Id":                {APIName: "Id", Type: storage.FieldID},
						"pkg__FirstName__c": {APIName: "pkg__FirstName__c", Type: storage.FieldString},
						"pkg__LastName__c":  {APIName: "pkg__LastName__c", Type: storage.FieldString},
						"pkg__OptOut__c":    {APIName: "pkg__OptOut__c", Type: storage.FieldBoolean},
					},
				},
			},
		},
	}
	index := typesys.Index{Types: []typesys.TypeSymbol{{Name: "BadgeProbe", File: source, Dependency: true}}}
	applyReferencedSyntheticFieldSets(&org, index)
	for _, fieldSet := range org.Metadata.FieldSets {
		if fieldSet.ObjectName == "pkg__Badge__c" && fieldSet.Name == "pkg__BadgeAddEdit" && len(fieldSet.Fields) > 0 {
			for _, field := range fieldSet.Fields {
				if field.Required {
					t.Fatalf("synthetic dependency field set marked %s required", field.Field)
				}
				if field.Field == "pkg__OptOut__c" {
					t.Fatalf("synthetic dependency field set included boolean field %s", field.Field)
				}
			}
			return
		}
	}
	t.Fatalf("synthetic dependency field set was not created: %#v", org.Metadata.FieldSets)
}

func TestOrgFromIndexDoesNotSynthesizeFieldSetsFromTestOnlyConstants(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/FieldSetProbeTest.cls"), `
@IsTest
private class FieldSetProbeTest {
	private static final String ACCOUNT_FIELD_SET_NAME = 'pkg__AccountEdit';
	private static Account getRecord() {
		return new Account();
	}
	private class Helper {
		String value;
	}
}
`)

	org := orgFromIndex(loadTestIndex(t, root))
	for _, fieldSet := range org.Metadata.FieldSets {
		if fieldSet.ObjectName == "Account" && fieldSet.Name == "pkg__AccountEdit" {
			t.Fatalf("test-only constant synthesized field set: %#v", fieldSet)
		}
	}
}

func TestOrgFromIndexKeepsProjectReferencedNumberLikeFieldUnknownWithoutMetadata(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/AccountNumberProbe.cls"), `
public class AccountNumberProbe {
	public static void touch(Account account) {
		System.assertEquals(1, account.pkg__ClosedCount__c);
	}
}
`)
	index := loadTestIndex(t, root)

	org := orgFromIndex(index)
	field := org.Objects["Account"].Definition.Fields["pkg__ClosedCount__c"]
	assertUnknownProjectReferencedField(t, field)
}

func TestOrgFromIndexKeepsProjectReferencedNumericLiteralFieldUnknownWithoutMetadata(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/AccountRollupProbe.cls"), `
public class AccountRollupProbe {
	public static Account zero(Id accountId) {
		return new Account(Id = accountId, pkg__PriorYearCount__c = 0);
	}
}
`)
	index := loadTestIndex(t, root)

	org := orgFromIndex(index)
	field := org.Objects["Account"].Definition.Fields["pkg__PriorYearCount__c"]
	assertUnknownProjectReferencedField(t, field)
}

func TestOrgFromIndexDoesNotInferStandardParentRelationshipAsField(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/AccountProbe.cls"), `
public class AccountProbe {
	public static void touch(Account existingRecord) {
		Boolean linked = existingRecord.Parent?.IsPersonAccount == true;
	}
}
`)
	index := loadTestIndex(t, root)

	org := orgFromIndex(index)
	state := org.Objects["Account"]
	if _, ok := state.Definition.Fields["Parent"]; ok {
		t.Fatalf("Account.Parent was inferred as a concrete field: %#v", state.Definition.Fields["Parent"])
	}
	field, ok := state.Definition.Fields["ParentId"]
	if !ok {
		t.Fatalf("Account.ParentId missing from standard fields")
	}
	if field.Type != storage.FieldReference || !parentRelationshipKnown(state.Definition, "Parent") {
		t.Fatalf("Account.ParentId relationship not preserved: field=%#v relations=%#v", field, state.Definition.Relations)
	}
}

func TestOrgFromIndexIncludesApexClassRows(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/WidgetTestData.cls"), `
public class WidgetTestData {
}
`)
	index := loadTestIndex(t, root)

	org := orgFromIndex(index)
	state, ok := org.Objects["ApexClass"]
	if !ok {
		t.Fatal("ApexClass object was not exposed")
	}
	if _, ok := state.Definition.Fields["ApiVersion"]; !ok {
		t.Fatalf("ApexClass.ApiVersion was not exposed: %#v", state.Definition.Fields)
	}
	if _, ok := state.Definition.Fields["LengthWithoutComments"]; !ok {
		t.Fatalf("ApexClass.LengthWithoutComments was not exposed: %#v", state.Definition.Fields)
	}
	var found bool
	for _, record := range state.Records {
		if record.Fields["Name"].String != "WidgetTestData" {
			continue
		}
		found = true
		if record.Fields["Body"].String == "" {
			t.Fatal("ApexClass.Body was empty")
		}
		if record.Fields["NamespacePrefix"].Kind != storage.ValueString {
			t.Fatalf("NamespacePrefix field = %#v", record.Fields["NamespacePrefix"])
		}
		if record.Fields["ApiVersion"].Decimal != "65.0" {
			t.Fatalf("ApiVersion field = %#v", record.Fields["ApiVersion"])
		}
		if record.Fields["LengthWithoutComments"].Integer == 0 {
			t.Fatalf("LengthWithoutComments field = %#v", record.Fields["LengthWithoutComments"])
		}
	}
	if !found {
		t.Fatalf("ApexClass row for WidgetTestData not found: %#v", state.Records)
	}
}

func TestOrgFromIndexHidesManagedDependencyApexClassSource(t *testing.T) {
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	consumerRoot := filepath.Join(root, "consumer")
	writeFile(t, filepath.Join(depRoot, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/classes/DependencyTestData.cls"), `
@isTest
public class DependencyTestData {
}
`)
	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{"namespace":"otherpkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(consumerRoot, "glade.yml"), `project:
  managedPackageDependencies: ["pkg:../dep:1.0"]
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/classes/LocalTestData.cls"), `
public class LocalTestData {
}
`)

	org := orgFromIndex(loadTestIndex(t, consumerRoot))
	state := org.Objects["ApexClass"]
	var foundDependency, foundLocal bool
	for _, record := range state.Records {
		switch record.Fields["Name"].String {
		case "DependencyTestData":
			foundDependency = true
			if record.Fields["NamespacePrefix"].String != "pkg" {
				t.Fatalf("dependency NamespacePrefix = %#v", record.Fields["NamespacePrefix"])
			}
			if record.Fields["Body"].String != "" || record.Fields["LengthWithoutComments"].Integer != 0 {
				t.Fatalf("dependency source was exposed: %#v", record.Fields)
			}
		case "LocalTestData":
			foundLocal = true
			if record.Fields["Body"].String == "" || record.Fields["LengthWithoutComments"].Integer == 0 {
				t.Fatalf("local source was hidden: %#v", record.Fields)
			}
		}
	}
	if !foundDependency || !foundLocal {
		t.Fatalf("ApexClass rows missing dependency=%v local=%v records=%#v", foundDependency, foundLocal, state.Records)
	}
}

func TestOrgFromIndexIncludesCustomApplicationMenuRows(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/applications/Apex_Recipes.app-meta.xml"), `<CustomApplication xmlns="http://soap.sforce.com/2006/04/metadata"><label>Apex Recipes</label></CustomApplication>`)
	index := loadTestIndex(t, root)

	org := orgFromIndex(index)
	apps := org.Objects["CustomApplication"]
	menu := org.Objects["AppMenuItem"]
	if len(apps.Records) != 1 || len(menu.Records) != 1 {
		t.Fatalf("application records = %d, menu records = %d", len(apps.Records), len(menu.Records))
	}
	var appID storage.ID
	for id, record := range apps.Records {
		appID = id
		if record.Fields["DeveloperName"].String != "Apex_Recipes" || record.Fields["Label"].String != "Apex Recipes" {
			t.Fatalf("CustomApplication fields = %#v", record.Fields)
		}
	}
	for _, record := range menu.Records {
		if record.Fields["Name"].String != "Apex_Recipes" || record.Fields["ApplicationId"].ID != appID {
			t.Fatalf("AppMenuItem fields = %#v, appID=%s", record.Fields, appID)
		}
	}
}

func TestOrgFromIndexUsesNotificationFileNameAsDeveloperName(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/notificationtypes/Processing_Complete.notiftype-meta.xml"), `<CustomNotificationType xmlns="http://soap.sforce.com/2006/04/metadata">
  <customNotifTypeName>Processing Completed</customNotifTypeName>
</CustomNotificationType>`)

	org := orgFromIndex(loadTestIndex(t, root))
	state := org.Objects["CustomNotificationType"]
	if !recordWithFieldValueExists(state, "DeveloperName", "Processing_Complete") {
		t.Fatalf("notification developer name row was not created: %#v", state.Records)
	}
	if !recordWithFieldValueExists(state, "MasterLabel", "Processing Completed") {
		t.Fatalf("notification label row was not created: %#v", state.Records)
	}
}

func TestOrgFromIndexIncludesProjectProfiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/profiles/Managed App Standard.profile-meta.xml"), `<Profile/>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/profiles/Customer Community Login User.profile-meta.xml"), `<Profile>
  <objectPermissions>
    <allowCreate>true</allowCreate>
    <allowDelete>false</allowDelete>
    <allowEdit>true</allowEdit>
    <allowRead>true</allowRead>
    <modifyAllRecords>false</modifyAllRecords>
    <object>pkg__Cart__c</object>
    <viewAllRecords>false</viewAllRecords>
  </objectPermissions>
</Profile>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/permissionsets/Read_access_to_Account_Shipping_Address.permissionset-meta.xml"), `<PermissionSet>
  <label>Read access to Account Shipping Address</label>
  <fieldPermissions>
    <editable>false</editable>
    <field>Account.ShippingStreet</field>
    <readable>true</readable>
  </fieldPermissions>
  <objectPermissions>
    <allowCreate>false</allowCreate>
    <allowDelete>false</allowDelete>
    <allowEdit>false</allowEdit>
    <allowRead>true</allowRead>
    <modifyAllRecords>false</modifyAllRecords>
    <object>Account</object>
    <viewAllRecords>false</viewAllRecords>
  </objectPermissions>
</PermissionSet>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/permissionsets/Guest_Public.permissionset"), `<PermissionSet>
  <label>Guest Public</label>
</PermissionSet>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/permissionsetgroups/Permission_Set_Group_for_testing.permissionsetgroup-meta.xml"), `<PermissionSetGroup/>`)

	org := orgFromIndex(loadTestIndex(t, root))
	profiles := org.Objects["Profile"].Records
	foundProfile := false
	foundGuestProfile := false
	foundCustomerCommunityProfile := false
	foundCommunityLoginProfile := false
	communityLicenseID, ok := recordFieldID(org.Objects["UserLicense"], "LicenseDefinitionKey", "PID_Customer_Community_Login")
	if !ok {
		t.Fatalf("community license row missing: %#v", org.Objects["UserLicense"].Records)
	}
	var communityLoginProfileID storage.ID
	for _, record := range profiles {
		if record.Fields["Name"].String == "Managed App Standard" {
			foundProfile = true
		}
		if record.Fields["Name"].String == "Customer Community Guest User" {
			foundGuestProfile = true
		}
		if record.Fields["Name"].String == "Customer Community User" {
			foundCustomerCommunityProfile = true
		}
		if record.Fields["Name"].String == "Customer Community Login User" {
			foundCommunityLoginProfile = true
			communityLoginProfileID = record.ID
			if got := record.Fields["UserLicenseId"].ID; got != communityLicenseID {
				t.Fatalf("customer community profile license = %q, want %q; record=%#v", got, communityLicenseID, record)
			}
		}
	}
	if !foundProfile {
		t.Fatalf("project profile row was not created; records=%#v", profiles)
	}
	if !foundGuestProfile {
		t.Fatalf("guest profile row was overwritten; records=%#v", profiles)
	}
	if !foundCustomerCommunityProfile {
		t.Fatalf("customer community profile row was not created; records=%#v", profiles)
	}
	if !foundCommunityLoginProfile {
		t.Fatalf("customer community login profile row was not created; records=%#v", profiles)
	}
	foundCartPermission := false
	for _, record := range org.Objects["ObjectPermissions"].Records {
		parent, hasParent := record.GetField("ParentId")
		objectValue, hasObject := record.GetField("SObjectType")
		if hasParent && hasObject &&
			storageIDValueEqualsText(parent, string(communityLoginProfileID)) &&
			storageStringValueEqualsText(objectValue, "pkg__Cart__c") {
			foundCartPermission = true
			break
		}
	}
	if !foundCartPermission {
		t.Fatalf("customer community profile cart permission row was not created; records=%#v", org.Objects["ObjectPermissions"].Records)
	}
	if !recordWithFieldValueExists(org.Objects["PermissionSet"], "Name", "Read_access_to_Account_Shipping_Address") {
		t.Fatalf("project permission set row was not created; records=%#v", org.Objects["PermissionSet"].Records)
	}
	if !guestPermissionSetAssignmentExists(org) {
		t.Fatalf("guest permission set assignment was not created; records=%#v", org.Objects["PermissionSetAssignment"].Records)
	}
	if !recordWithFieldValueExists(org.Objects["ObjectPermissions"], "SObjectType", "Account") {
		t.Fatalf("project object permissions row was not created; records=%#v", org.Objects["ObjectPermissions"].Records)
	}
	if !recordWithFieldValueExists(org.Objects["FieldPermissions"], "Field", "Account.ShippingStreet") {
		t.Fatalf("project field permissions row was not created; records=%#v", org.Objects["FieldPermissions"].Records)
	}
	if !recordWithFieldValueExists(org.Objects["PermissionSetGroup"], "DeveloperName", "Permission_Set_Group_for_testing") {
		t.Fatalf("project permission set group row was not created; records=%#v", org.Objects["PermissionSetGroup"].Records)
	}
}

func TestOrgFromIndexUsesPermissionSetsToSeedManagedObjectShape(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/permissionsets/BasePkgObjectsPermissions.permissionset-meta.xml"), `<PermissionSet>
  <fieldPermissions>
    <editable>true</editable>
    <field>pkg__Affiliation__c.pkg__Account__c</field>
    <readable>true</readable>
  </fieldPermissions>
  <fieldPermissions>
    <editable>true</editable>
    <field>pkg__Affiliation__c.pkg__IsCompanyManager__c</field>
    <readable>true</readable>
  </fieldPermissions>
  <fieldPermissions>
    <editable>true</editable>
    <field>pkg__Affiliation__c.pkg__BillingHelperEnabled__c</field>
    <readable>true</readable>
  </fieldPermissions>
  <fieldPermissions>
    <editable>true</editable>
    <field>pkg__Affiliation__c.pkg__AllowRelatedUser__c</field>
    <readable>true</readable>
  </fieldPermissions>
  <fieldPermissions>
    <editable>true</editable>
    <field>pkg__Affiliation__c.pkg__Hidden__c</field>
    <readable>true</readable>
  </fieldPermissions>
  <fieldPermissions>
    <editable>true</editable>
    <field>pkg__OrderItem__c.pkg__PriceClass__c</field>
    <readable>true</readable>
  </fieldPermissions>
  <fieldPermissions>
    <editable>true</editable>
    <field>pkg__OrderItem__c.pkg__Customer__c</field>
    <readable>true</readable>
  </fieldPermissions>
  <fieldPermissions>
    <editable>true</editable>
    <field>pkg__ExternalPaymentProfile__c.pkg__PaymentType__c</field>
    <readable>true</readable>
  </fieldPermissions>
  <fieldPermissions>
    <editable>true</editable>
    <field>pkg__ExternalPaymentProfile__c.pkg__PaymentTypeIssuerName__c</field>
    <readable>true</readable>
  </fieldPermissions>
  <fieldPermissions>
    <editable>true</editable>
    <field>pkg__Product__c.pkg__TrackInventory__c</field>
    <readable>true</readable>
  </fieldPermissions>
  <recordTypeVisibilities>
    <default>false</default>
    <recordType>pkg__ProductLink__c.pkg__BundleProduct</recordType>
    <visible>true</visible>
  </recordTypeVisibilities>
  <objectPermissions>
    <allowCreate>true</allowCreate>
    <allowDelete>false</allowDelete>
    <allowEdit>true</allowEdit>
    <allowRead>true</allowRead>
    <modifyAllRecords>false</modifyAllRecords>
    <object>pkg__PriceClass__c</object>
    <viewAllRecords>false</viewAllRecords>
  </objectPermissions>
  <objectPermissions>
    <allowCreate>true</allowCreate>
    <allowDelete>false</allowDelete>
    <allowEdit>true</allowEdit>
    <allowRead>true</allowRead>
    <modifyAllRecords>false</modifyAllRecords>
    <object>pkg__Affiliation__c</object>
    <viewAllRecords>false</viewAllRecords>
  </objectPermissions>
</PermissionSet>`)

	org := orgFromIndex(loadTestIndex(t, root))
	affiliation, ok := org.Objects["pkg__Affiliation__c"]
	if !ok {
		t.Fatalf("managed permission-set object was not seeded")
	}
	assertUnknownProjectReferencedField(t, affiliation.Definition.Fields["pkg__IsCompanyManager__c"])
	assertUnknownProjectReferencedField(t, affiliation.Definition.Fields["pkg__BillingHelperEnabled__c"])
	assertUnknownProjectReferencedField(t, affiliation.Definition.Fields["pkg__AllowRelatedUser__c"])
	assertUnknownProjectReferencedField(t, affiliation.Definition.Fields["pkg__Hidden__c"])
	accountField := affiliation.Definition.Fields["pkg__Account__c"]
	assertUnknownProjectReferencedField(t, accountField)
	if parentRelationshipKnown(affiliation.Definition, "pkg__Account__r") {
		t.Fatalf("lookup relationship inferred without metadata: %#v", affiliation.Definition.Relations)
	}
	orderItem, ok := org.Objects["pkg__OrderItem__c"]
	if !ok {
		t.Fatalf("managed permission-set order item object was not seeded")
	}
	priceClassField := orderItem.Definition.Fields["pkg__PriceClass__c"]
	assertUnknownProjectReferencedField(t, priceClassField)
	customerField := orderItem.Definition.Fields["pkg__Customer__c"]
	assertUnknownProjectReferencedField(t, customerField)
	productLink, ok := org.Objects["pkg__ProductLink__c"]
	if !ok {
		t.Fatalf("managed permission-set product link object was not seeded")
	}
	if len(productLink.Definition.RecordTypes) != 1 || productLink.Definition.RecordTypes[0].DeveloperName != "BundleProduct" || productLink.Definition.RecordTypes[0].Name != "Bundle Product" {
		t.Fatalf("product link record types = %#v", productLink.Definition.RecordTypes)
	}
	externalProfile := org.Objects["pkg__ExternalPaymentProfile__c"]
	assertUnknownProjectReferencedField(t, externalProfile.Definition.Fields["pkg__PaymentType__c"])
	assertUnknownProjectReferencedField(t, externalProfile.Definition.Fields["pkg__PaymentTypeIssuerName__c"])
	product := org.Objects["pkg__Product__c"]
	assertUnknownProjectReferencedField(t, product.Definition.Fields["pkg__TrackInventory__c"])
}

func TestOrgFromIndexPermissionSetDoesNotUpgradeUnknownFieldToLookup(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/pkg__OrderItem__c/pkg__OrderItem__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Order Item</label></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/PriceClassShapeHarness.cls"), `
public class PriceClassShapeHarness {
  public void touch() {
    pkg__OrderItem__c item = new pkg__OrderItem__c(pkg__PriceClass__c = 1);
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/permissionsets/BasePkgObjectsPermissions.permissionset-meta.xml"), `<PermissionSet>
  <fieldPermissions>
    <editable>true</editable>
    <field>pkg__OrderItem__c.pkg__PriceClass__c</field>
    <readable>true</readable>
  </fieldPermissions>
  <objectPermissions>
    <allowCreate>true</allowCreate>
    <allowDelete>false</allowDelete>
    <allowEdit>true</allowEdit>
    <allowRead>true</allowRead>
    <modifyAllRecords>false</modifyAllRecords>
    <object>pkg__PriceClass__c</object>
    <viewAllRecords>false</viewAllRecords>
  </objectPermissions>
</PermissionSet>`)

	org := orgFromIndex(loadTestIndex(t, root))
	orderItem := org.Objects["pkg__OrderItem__c"]
	field := orderItem.Definition.Fields["pkg__PriceClass__c"]
	assertUnknownProjectReferencedField(t, field)
}

func TestOrgFromIndexPermissionSetDoesNotUpgradeUnknownFieldToBoolean(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/pkg__Product__c/pkg__Product__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Product</label></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/ProductShapeHarness.cls"), `
public class ProductShapeHarness {
  public void touch() {
    pkg__Product__c product = new pkg__Product__c(pkg__TrackInventory__c = 'false');
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/permissionsets/BasePkgObjectsPermissions.permissionset-meta.xml"), `<PermissionSet>
  <fieldPermissions>
    <editable>true</editable>
    <field>pkg__Product__c.pkg__TrackInventory__c</field>
    <readable>true</readable>
  </fieldPermissions>
</PermissionSet>`)

	org := orgFromIndex(loadTestIndex(t, root))
	product := org.Objects["pkg__Product__c"]
	assertUnknownProjectReferencedField(t, product.Definition.Fields["pkg__TrackInventory__c"])
}

func TestOrgFromIndexPermissionSetDoesNotInferCustomLookupChildRelationship(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/permissionsets/BasePkgObjectsPermissions.permissionset-meta.xml"), `<PermissionSet>
  <fieldPermissions>
    <editable>true</editable>
    <field>pkg__Product__c.pkg__Event__c</field>
    <readable>true</readable>
  </fieldPermissions>
  <fieldPermissions>
    <editable>true</editable>
    <field>pkg__Product__c.pkg__Event2__c</field>
    <readable>true</readable>
  </fieldPermissions>
  <objectPermissions>
    <allowCreate>true</allowCreate>
    <allowDelete>false</allowDelete>
    <allowEdit>true</allowEdit>
    <allowRead>true</allowRead>
    <modifyAllRecords>false</modifyAllRecords>
    <object>pkg__Product__c</object>
    <viewAllRecords>false</viewAllRecords>
  </objectPermissions>
  <objectPermissions>
    <allowCreate>true</allowCreate>
    <allowDelete>false</allowDelete>
    <allowEdit>true</allowEdit>
    <allowRead>true</allowRead>
    <modifyAllRecords>false</modifyAllRecords>
    <object>pkg__Event__c</object>
    <viewAllRecords>false</viewAllRecords>
  </objectPermissions>
</PermissionSet>`)

	org := orgFromIndex(loadTestIndex(t, root))
	product := org.Objects["pkg__Product__c"]
	assertUnknownProjectReferencedField(t, product.Definition.Fields["pkg__Event__c"])
	assertUnknownProjectReferencedField(t, product.Definition.Fields["pkg__Event2__c"])
	for _, relation := range product.Definition.Relations {
		if relation.Field == "pkg__Event2__c" {
			t.Fatalf("Event2 relation inferred without metadata: %#v", relation)
		}
	}
}

func TestOrgFromIndexPermissionSetDoesNotInferPrefixedCustomLookupChildRelationship(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/permissionsets/BasePkgObjectsPermissions.permissionset-meta.xml"), `<PermissionSet>
  <fieldPermissions>
    <editable>true</editable>
    <field>pkg__ProductLink__c.pkg__ParentProduct__c</field>
    <readable>true</readable>
  </fieldPermissions>
  <objectPermissions>
    <allowCreate>true</allowCreate>
    <allowDelete>false</allowDelete>
    <allowEdit>true</allowEdit>
    <allowRead>true</allowRead>
    <modifyAllRecords>false</modifyAllRecords>
    <object>pkg__Product__c</object>
    <viewAllRecords>false</viewAllRecords>
  </objectPermissions>
  <objectPermissions>
    <allowCreate>true</allowCreate>
    <allowDelete>false</allowDelete>
    <allowEdit>true</allowEdit>
    <allowRead>true</allowRead>
    <modifyAllRecords>false</modifyAllRecords>
    <object>pkg__ProductLink__c</object>
    <viewAllRecords>false</viewAllRecords>
  </objectPermissions>
</PermissionSet>`)

	org := orgFromIndex(loadTestIndex(t, root))
	productLink := org.Objects["pkg__ProductLink__c"]
	field := productLink.Definition.Fields["pkg__ParentProduct__c"]
	assertUnknownProjectReferencedField(t, field)
	for _, relation := range productLink.Definition.Relations {
		if relation.Field == "pkg__ParentProduct__c" {
			t.Fatalf("parent product relation inferred without metadata: %#v", relation)
		}
	}
}

func TestOrgFromIndexKeepsDataRelationshipReferencesUnknownWithoutMetadata(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "data/Event_Questions.json"), `{
  "records": {
    "pkg__EventQuestion__c": [
      {
        "pkg__ExternalID__c": "Q1",
        "pkg__QuestionText__c": "Dietary needs?",
        "pkg__IsRequired__c": true,
        "pkg__Event__r.pkg__ExternalID__c": "E1"
      }
    ]
  }
}`)

	org := orgFromIndex(loadTestIndex(t, root))
	eventQuestion := org.Objects["pkg__EventQuestion__c"]
	field := eventQuestion.Definition.Fields["pkg__Event__c"]
	assertUnknownProjectReferencedField(t, field)
	assertUnknownProjectReferencedField(t, eventQuestion.Definition.Fields["pkg__QuestionText__c"])
	assertUnknownProjectReferencedField(t, eventQuestion.Definition.Fields["pkg__IsRequired__c"])
	for _, relation := range eventQuestion.Definition.Relations {
		if relation.Field == "pkg__Event__c" {
			t.Fatalf("event question relation inferred without metadata: %#v", relation)
		}
	}
}

func TestOrgFromIndexLoadsCustomObjectLookupRelationshipAndCheckboxDefault(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/pkg__Product__c/pkg__Product__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Product</label></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/pkg__Event__c/pkg__Event__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Event</label></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/pkg__Product__c/fields/pkg__Event2__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>pkg__Event2__c</fullName><type>Lookup</type><referenceTo>pkg__Event__c</referenceTo><relationshipName>pkg__Event2__r</relationshipName><childRelationshipName>pkg__Products2__r</childRelationshipName></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/pkg__Product__c/fields/pkg__TrackInventory__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>pkg__TrackInventory__c</fullName><type>Checkbox</type><defaultValue>false</defaultValue></CustomField>`)

	org := orgFromIndex(loadTestIndex(t, root))
	product := org.Objects["pkg__Product__c"]
	field := product.Definition.Fields["pkg__Event2__c"]
	if field.Type != storage.FieldReference || len(field.ReferenceTo) != 1 || field.ReferenceTo[0] != "pkg__Event__c" {
		t.Fatalf("Event2 field = %#v", field)
	}
	trackInventory := product.Definition.Fields["pkg__TrackInventory__c"]
	if trackInventory.Type != storage.FieldBoolean || trackInventory.DefaultValue != "false" {
		t.Fatalf("TrackInventory field = %#v", trackInventory)
	}
	for _, relation := range product.Definition.Relations {
		if relation.Field == "pkg__Event2__c" {
			if relation.ChildRelationship != "pkg__Products2__r" {
				t.Fatalf("Event2 relation = %#v", relation)
			}
			return
		}
	}
	t.Fatalf("Event2 relation missing: %#v", product.Definition.Relations)
}

func TestOrgFromIndexDoesNotInferProductDownloadURLFormula(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/permissionsets/BasePkgObjectsPermissions.permissionset-meta.xml"), `<PermissionSet>
  <objectPermissions><allowRead>true</allowRead><object>pkg__OrderItemLine__c</object></objectPermissions>
  <objectPermissions><allowRead>true</allowRead><object>pkg__Product__c</object></objectPermissions>
  <fieldPermissions><readable>true</readable><field>pkg__OrderItemLine__c.pkg__Product2__c</field></fieldPermissions>
  <fieldPermissions><readable>true</readable><field>pkg__OrderItemLine__c.pkg__DownloadUrl__c</field></fieldPermissions>
  <fieldPermissions><readable>true</readable><field>pkg__Product__c.pkg__IsDownloadable__c</field></fieldPermissions>
  <fieldPermissions><readable>true</readable><field>pkg__Product__c.pkg__DownloadUrl__c</field></fieldPermissions>
</PermissionSet>`)

	org := orgFromIndex(loadTestIndex(t, root))
	line := org.Objects["pkg__OrderItemLine__c"]
	field := line.Definition.Fields["pkg__DownloadUrl__c"]
	if field.Type == storage.FieldCalculated || strings.TrimSpace(field.Formula) != "" {
		t.Fatalf("order item line download field = %#v", field)
	}
}

func TestOrgFromIndexKeepsValidCustomMetadataWhenAnotherRecordHasUnknownField(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Feature__mdt/Feature__mdt.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Feature</label></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Feature__mdt/fields/Enabled__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Enabled__c</fullName><label>Enabled</label><type>Checkbox</type></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/customMetadata/Feature.Bad.md-meta.xml"), `<CustomMetadata xmlns="http://soap.sforce.com/2006/04/metadata"><label>Bad</label><values><field>Missing__c</field><value>true</value></values></CustomMetadata>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/bindings/Feature.Good.md-meta.xml"), `<CustomMetadata xmlns="http://soap.sforce.com/2006/04/metadata"><label>Good</label><values><field>Enabled__c</field><value>true</value></values></CustomMetadata>`)

	org := orgFromIndex(loadTestIndex(t, root))
	for _, record := range org.Objects["Feature__mdt"].Records {
		if record.Fields["DeveloperName"].String == "Good" && record.Fields["Enabled__c"].Boolean {
			return
		}
	}
	t.Fatalf("valid custom metadata record missing: %#v", org.Objects["Feature__mdt"].Records)
}

func TestRunClonedMethodsKeepCustomMetadataChildRelationships(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/StateConfiguration__mdt/StateConfiguration__mdt.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>State Configuration</label></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/StateConfiguration__mdt/fields/IsActive__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>IsActive__c</fullName><label>Is Active</label><type>Checkbox</type></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/StateConfiguration__mdt/fields/SupportedStates__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>SupportedStates__c</fullName><label>Supported States</label><type>Text</type></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/StateTransition__mdt/StateTransition__mdt.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>State Transition</label></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/StateTransition__mdt/fields/IsActive__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>IsActive__c</fullName><label>Is Active</label><type>Checkbox</type></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/StateTransition__mdt/fields/FromStates__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>FromStates__c</fullName><label>From States</label><type>Text</type></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/StateTransition__mdt/fields/ToState__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>ToState__c</fullName><label>To State</label><type>Text</type></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/StateTransition__mdt/fields/StateConfiguration__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>StateConfiguration__c</fullName><label>State Configuration</label><type>MetadataRelationship</type><referenceTo>StateConfiguration__mdt</referenceTo><relationshipName>StateConfiguration__r</relationshipName><childRelationshipName>StateTransitions__r</childRelationshipName></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/customMetadata/StateConfiguration.OrderGraph.md-meta.xml"), `<CustomMetadata xmlns="http://soap.sforce.com/2006/04/metadata"><label>Order Graph</label><values><field>IsActive__c</field><value>true</value></values><values><field>SupportedStates__c</field><value>Cart,Pro forma</value></values></CustomMetadata>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/customMetadata/StateTransition.order_submit_as_proforma.md-meta.xml"), `<CustomMetadata xmlns="http://soap.sforce.com/2006/04/metadata"><label>Submit as pro forma</label><values><field>IsActive__c</field><value>true</value></values><values><field>FromStates__c</field><value>Cart</value></values><values><field>ToState__c</field><value>Pro forma</value></values><values><field>StateConfiguration__c</field><value>OrderGraph</value></values></CustomMetadata>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/MetadataChildCloneTest.cls"), `
@IsTest
private class MetadataChildCloneTest {
  @IsTest static void firstCloneRun() {
    assertChildRows();
  }

  @IsTest static void secondCloneRun() {
    assertChildRows();
  }

  private static void assertChildRows() {
    List<StateConfiguration__mdt> configs = [
      SELECT QualifiedApiName, SupportedStates__c,
        (SELECT QualifiedApiName, FromStates__c, ToState__c FROM StateTransitions__r WHERE IsActive__c = TRUE)
      FROM StateConfiguration__mdt
      WHERE IsActive__c = TRUE
    ];
    System.assertEquals(1, configs.size());
    Integer childCount = 0;
    for (StateTransition__mdt transitionRecord : configs[0].StateTransitions__r) {
      childCount++;
      System.assertEquals('order_submit_as_proforma', transitionRecord.QualifiedApiName);
      System.assertEquals('Cart', transitionRecord.FromStates__c);
      System.assertEquals('Pro forma', transitionRecord.ToState__c);
    }
    System.assertEquals(1, childCount);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		t.Fatalf("summary = %#v case0=%#v case1=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[1])
	}
}

func TestRunCustomMetadataEntityDefinitionRelationshipFieldIsTypeName(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Handler__mdt/Handler__mdt.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Handler</label></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Handler__mdt/fields/SObjectType__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>SObjectType__c</fullName><label>SObject Type</label><type>MetadataRelationship</type><referenceTo>EntityDefinition</referenceTo><relationshipName>Handlers</relationshipName></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Handler__mdt/fields/TargetField__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>TargetField__c</fullName><label>Target Field</label><type>MetadataRelationship</type><referenceTo>FieldDefinition</referenceTo><relationshipName>TargetField</relationshipName></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/customMetadata/Handler.Log.md-meta.xml"), `<CustomMetadata xmlns="http://soap.sforce.com/2006/04/metadata"><label>Log</label><values><field>SObjectType__c</field><value>Log__c</value></values><values><field>TargetField__c</field><value>Log__c.Status__c</value></values></CustomMetadata>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Log__c/Log__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Log</label></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Log__c/fields/Status__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Status__c</fullName><label>Status</label><type>Text</type><length>40</length></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/HandlerTest.cls"), `
@isTest
private class HandlerTest {
  @isTest static void metadataRelationshipFieldIsATypeName() {
	    Handler__mdt handler = [
	      SELECT SObjectType__c, TargetField__c
	      FROM Handler__mdt
	      LIMIT 1
	    ];
	    System.assertEquals('Log__c', handler.SObjectType__c);
	    handler.SObjectType__c = handler.SObjectType__r.QualifiedApiName;
	    System.assertEquals('Log__c', Type.forName(handler.SObjectType__c).getName());
	    System.assertEquals('Status__c', handler.TargetField__r.QualifiedApiName);
	    System.assertEquals('Log__c', handler.TargetField__r.EntityDefinition.QualifiedApiName);
	  }
	}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v problem=%#v case=%#v", got, run.Suites[0].Cases[0].Problem, run.Suites[0].Cases[0])
	}
}

func TestOrgFromIndexAddsProfileVisiblePersonAccountRecordTypes(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/PersonAccount/PersonAccount.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Person Account</label></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/PersonAccount/recordTypes/Individual.recordType-meta.xml"), `<RecordType xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Individual</fullName><label>Individual</label><active>true</active></RecordType>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/profiles/Community.profile-meta.xml"), `<Profile xmlns="http://soap.sforce.com/2006/04/metadata">
  <recordTypeVisibilities>
    <default>true</default>
    <recordType>PersonAccount.pkg__Individual</recordType>
    <visible>true</visible>
  </recordTypeVisibilities>
</Profile>`)

	org := orgFromIndex(loadTestIndex(t, root))
	account := org.Objects["Account"]
	foundDefinition := false
	for _, recordType := range account.Definition.RecordTypes {
		if recordType.DeveloperName == "Individual" && recordType.Name == "Individual" && !recordType.Default {
			foundDefinition = true
			break
		}
	}
	if !foundDefinition {
		t.Fatalf("Account record types missing profile-visible Individual: %#v", account.Definition.RecordTypes)
	}
	foundRecord := false
	for _, record := range org.Objects["RecordType"].Records {
		if record.Fields["SobjectType"].String == "PersonAccount" {
			t.Fatalf("RecordType rows should fold PersonAccount into Account: %#v", org.Objects["RecordType"].Records)
		}
		if record.Fields["SobjectType"].String == "Account" && record.Fields["DeveloperName"].String == "Individual" && record.Fields["Name"].String == "Individual" {
			foundRecord = true
			break
		}
	}
	if !foundRecord {
		t.Fatalf("RecordType rows missing Account.Individual: %#v", org.Objects["RecordType"].Records)
	}
	for _, record := range org.Objects["RecordType"].Records {
		if record.Fields["SobjectType"].String == "PersonAccount" {
			t.Fatalf("PersonAccount record type row leaked into RecordType: %#v", record)
		}
	}
}

func TestProfileRecordTypeExistsMatchesNamespaceStrippedNames(t *testing.T) {
	recordTypes := []storage.RecordTypeInfo{{
		DeveloperName: "pkg__Individual",
		Name:          "pkg__Individual",
	}}
	if !profileRecordTypeExists(recordTypes, "Individual") {
		t.Fatalf("profileRecordTypeExists did not match namespace-stripped developer name")
	}
}

func TestUpdateRecordTypeFromProjectReferenceRefreshesExistingMetadata(t *testing.T) {
	recordTypes := []storage.RecordTypeInfo{{
		DeveloperName: "Business_Account",
		Name:          "Business Account",
		Active:        false,
		Available:     false,
	}}
	if !updateRecordTypeFromProjectReference(recordTypes, "Business_Account", "Organization") {
		t.Fatalf("record type was not updated")
	}
	if recordTypes[0].Name != "Organization" || !recordTypes[0].Active || !recordTypes[0].Available {
		t.Fatalf("record type = %#v", recordTypes[0])
	}
}

func TestUpdateRecordTypeFromProjectReferencePreservesExistingLabelWhenReferenceUsesDeveloperName(t *testing.T) {
	recordTypes := []storage.RecordTypeInfo{{
		DeveloperName: "FlatRate",
		Name:          "Flat Rate",
		Active:        true,
		Available:     true,
	}}
	if updateRecordTypeFromProjectReference(recordTypes, "FlatRate", "FlatRate") {
		t.Fatalf("record type should not have been updated")
	}
	if recordTypes[0].Name != "Flat Rate" {
		t.Fatalf("record type = %#v", recordTypes[0])
	}
}

func TestOrgFromIndexAddsProjectReferencedRecordTypes(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/pkg__Product__c/pkg__Product__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Product</label></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/pkg__ProductLink__c/pkg__ProductLink__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Product Link</label></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/ReferencedRecordTypesTest.cls"), `
@IsTest
private class ReferencedRecordTypesTest {
  @IsTest static void referencedRecordTypesHaveIds() {
    Id bundle = SObjectType.pkg__Product__c.getRecordTypeInfosByName().get('Bundle').getRecordTypeId();
    Id bundleProduct = pkg__ProductLink__c.SObjectType.getDescribe().getRecordTypeInfosByDeveloperName().get('BundleProduct').getRecordTypeId();
    System.assertNotEquals(null, bundle);
    System.assertNotEquals(null, bundleProduct);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestProjectReferencedRecordTypesUsesSourceCache(t *testing.T) {
	file := filepath.Join(t.TempDir(), "ReferencedRecordTypesTest.cls")
	cache := &sourceCache{
		files: map[string]string{
			file: `
@IsTest
private class ReferencedRecordTypesTest {
  @IsTest static void referencedRecordTypesHaveIds() {
    Id bundle = SObjectType.pkg__Product__c.getRecordTypeInfosByName().get('Bundle').getRecordTypeId();
  }
}
`,
		},
	}

	refs := projectReferencedRecordTypes(project.Project{ApexFiles: []string{file}}, cache)
	if len(refs) != 1 {
		t.Fatalf("refs = %#v", refs)
	}
	if refs[0].ObjectName != "pkg__Product__c" || refs[0].Name != "Bundle" || refs[0].DeveloperName != "Bundle" {
		t.Fatalf("ref = %#v", refs[0])
	}
}

func TestSourceCacheReadIsConcurrentSafe(t *testing.T) {
	dir := t.TempDir()
	files := make([]string, 0, 8)
	for i := 0; i < 8; i++ {
		path := filepath.Join(dir, fmt.Sprintf("Class%d.cls", i))
		writeFile(t, path, fmt.Sprintf("public class Class%d { Id rt = Schema.SObjectType.Account.getRecordTypeInfosByName().get('Master').getRecordTypeId(); }", i))
		files = append(files, path)
	}
	cache := newSourceCache()
	done := make(chan struct{}, 16)
	for i := 0; i < 16; i++ {
		go func() {
			_ = parallelScanProjectReferencedRecordTypes(files, cache)
			done <- struct{}{}
		}()
	}
	for i := 0; i < 16; i++ {
		<-done
	}
}

func TestOrgFromIndexAddsRecordTypesFromProjectConfig(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "cumulusci.yml"), `
tasks:
  ensure_record_types:
    options:
      record_types:
        - record_type: Account.HH_Account
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/ReferencedRecordTypeConfigTest.cls"), `
@IsTest
private class ReferencedRecordTypeConfigTest {
  @IsTest static void configuredRecordTypeCanBeQueried() {
    Id household = [SELECT Id FROM RecordType WHERE DeveloperName = 'HH_Account'].Id;
    System.assertNotEquals(null, household);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestOrgFromIndexAddsRecordTypesReferencedByTestDataHelpers(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/pkg__Product__c/pkg__Product__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Product</label></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/ProductTestData.cls"), `
@IsTest
private class ProductTestData {
  private static final String PROD_RECORD_TYPE_PROCESSING_FEE = 'Processing Fee';
  public Schema.SObjectType getSObjectType() {
    return pkg__Product__c.SObjectType;
  }
  public Id processingFeeRecordTypeId() {
    return forRecordType(PROD_RECORD_TYPE_PROCESSING_FEE);
  }
  public Id forRecordType(String recordTypeName) {
    return getRecordTypeId(recordTypeName);
  }
  protected Id getRecordTypeId(String name) {
    return getSObjectType().getDescribe().getRecordTypeInfosByName().get(name).getRecordTypeId();
  }
}
`)

	org := orgFromIndex(loadTestIndex(t, root))
	product, ok := org.Objects["pkg__Product__c"]
	if !ok {
		t.Fatalf("product object missing")
	}
	found := false
	for _, recordType := range product.Definition.RecordTypes {
		if recordType.DeveloperName == "ProcessingFee" && recordType.Name == "Processing Fee" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Processing Fee record type missing: %#v", product.Definition.RecordTypes)
	}
}

func TestOrgFromIndexMergesManagedDependencyRecordTypesWithConsumerOverlay(t *testing.T) {
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	consumerRoot := filepath.Join(root, "consumer")
	writeFile(t, filepath.Join(depRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"pkg"}`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/default/objects/Product__c/Product__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Product</label></CustomObject>`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/default/objects/Product__c/recordTypes/Subscription.recordType-meta.xml"), `<RecordType xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Subscription</fullName><label>Subscription</label><active>true</active></RecordType>`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/default/objects/Product__c/recordTypes/Merchandise.recordType-meta.xml"), `<RecordType xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Merchandise</fullName><label>Merchandise</label><active>true</active></RecordType>`)

	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"ext"}`)
	writeFile(t, filepath.Join(consumerRoot, "glade.yml"), `project:
  managedPackageDependencies: ["pkg:../dep"]
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/default/objects/pkg__Product__c/pkg__Product__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Product Overlay</label></CustomObject>`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/default/objects/pkg__Product__c/recordTypes/Program.recordType-meta.xml"), `<RecordType xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Program</fullName><label>Program</label><active>true</active></RecordType>`)

	org := orgFromIndex(loadTestIndex(t, consumerRoot))
	product, ok := org.Objects["pkg__Product__c"]
	if !ok {
		t.Fatalf("product object missing")
	}
	recordTypes := map[string]bool{}
	for _, recordType := range product.Definition.RecordTypes {
		recordTypes[recordType.DeveloperName] = true
		if recordType.ID == "" {
			t.Fatalf("record type missing id: %#v", recordType)
		}
	}
	for _, name := range []string{"Subscription", "Merchandise", "Program"} {
		if !recordTypes[name] {
			t.Fatalf("record types missing %s: %#v", name, product.Definition.RecordTypes)
		}
	}
}

func TestManagedDependencyClassResolvesOwnSObjectRecordTypes(t *testing.T) {
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	consumerRoot := filepath.Join(root, "consumer")
	writeFile(t, filepath.Join(depRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"pkg"}`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/default/objects/Product__c/Product__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Product</label></CustomObject>`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/default/objects/Product__c/recordTypes/Merchandise.recordType-meta.xml"), `<RecordType xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Merchandise</fullName><label>Merchandise</label><active>true</active></RecordType>`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/default/classes/ProductRecordTypes.cls"), `
global class ProductRecordTypes {
  global static Boolean hasMerchandise() {
    Map<String, Schema.RecordTypeInfo> byName = Schema.SObjectType.Product__c.getRecordTypeInfosByName();
    return byName.containsKey('Merchandise') && byName.get('Merchandise').isAvailable();
  }
}
`)

	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"ext"}`)
	writeFile(t, filepath.Join(consumerRoot, "glade.yml"), `project:
  managedPackageDependencies: ["pkg:../dep"]
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/default/objects/pkg__Product__c/pkg__Product__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Product Overlay</label></CustomObject>`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/default/objects/pkg__Product__c/recordTypes/Program.recordType-meta.xml"), `<RecordType xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Program</fullName><label>Program</label><active>true</active></RecordType>`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/default/classes/ProductRecordTypesConsumerTest.cls"), `
@IsTest
private class ProductRecordTypesConsumerTest {
  @IsTest static void dependencyClassSeesDependencyRecordTypes() {
    System.assert(pkg.ProductRecordTypes.hasMerchandise());
  }
}
`)

	run := Run(loadTestIndex(t, consumerRoot), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestManagedDependencyClassQueriesOwnNamespacedSObjectRecordTypeRelationship(t *testing.T) {
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	consumerRoot := filepath.Join(root, "consumer")
	writeFile(t, filepath.Join(depRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"pkg"}`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/default/objects/ShipMethod__c/ShipMethod__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Ship Method</label></CustomObject>`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/default/objects/ShipMethod__c/fields/CommunityEnabled__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>CommunityEnabled__c</fullName><label>Community Enabled</label><type>Checkbox</type><defaultValue>false</defaultValue></CustomField>`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/default/objects/ShipMethod__c/recordTypes/FlatRate.recordType-meta.xml"), `<RecordType xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>FlatRate</fullName><label>Flat Rate</label><active>true</active></RecordType>`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/default/classes/ShippingProbe.cls"), `
global class ShippingProbe {
  global static String firstRecordTypeName() {
    Id recordTypeId = Schema.SObjectType.ShipMethod__c.getRecordTypeInfosByName().get('Flat Rate').getRecordTypeId();
    insert new ShipMethod__c(Name = 'Ground', RecordTypeId = recordTypeId, CommunityEnabled__c = true);
    List<ShipMethod__c> rows = [SELECT Id, RecordType.Name, CommunityEnabled__c FROM ShipMethod__c WHERE CommunityEnabled__c = true];
    Map<Id, ShipMethod__c> byId = new Map<Id, ShipMethod__c>(rows);
    System.assert(!byId.keySet().isEmpty(), 'expected map constructor to key queried rows by Id');
    return rows[0].RecordType.Name;
  }
}
`)

	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"ext"}`)
	writeFile(t, filepath.Join(consumerRoot, "glade.yml"), `project:
  managedPackageDependencies: ["pkg:../dep"]
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/default/classes/ShippingProbeConsumerTest.cls"), `
@IsTest
private class ShippingProbeConsumerTest {
  @IsTest static void dependencyQueryReturnsRecordTypeRelationship() {
    System.assertEquals('Flat Rate', pkg.ShippingProbe.firstRecordTypeName());
  }
}
`)

	run := Run(loadTestIndex(t, consumerRoot), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestManagedDependencyTriggerResolvesOwnShortClassCollision(t *testing.T) {
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	consumerRoot := filepath.Join(root, "consumer")
	writeFile(t, filepath.Join(depRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"pkg"}`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/default/objects/Widget__c/Widget__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Widget</label><pluralLabel>Widgets</pluralLabel><nameField><type>Text</type><label>Name</label></nameField></CustomObject>`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/default/classes/TriggerHandlersBase.cls"), `
global virtual class TriggerHandlersBase {
  global virtual void run() {}
}
`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/default/classes/WidgetTriggerHandlers.cls"), `
public class WidgetTriggerHandlers extends TriggerHandlersBase {
  public override void run() {
    DispatchState.Value = 'pkg';
  }
}
`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/default/classes/DispatchState.cls"), `
global class DispatchState {
  global static String Value;
}
`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/default/classes/TriggerHandlerManager.cls"), `
global class TriggerHandlerManager {
  global static void executeHandlers(String triggerName, TriggerHandlersBase handler) {
    handler.run();
  }
}
`)
	writeFile(t, filepath.Join(depRoot, "force-app/main/default/triggers/WidgetTrigger.trigger"), `
trigger WidgetTrigger on Widget__c (before insert) {
  TriggerHandlerManager.executeHandlers('WidgetTrigger', new WidgetTriggerHandlers());
}
`)

	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"ext"}`)
	writeFile(t, filepath.Join(consumerRoot, "glade.yml"), `project:
  managedPackageDependencies: ["pkg:../dep"]
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/default/classes/TriggerHandlerManager.cls"), `
public class TriggerHandlerManager {
  public static void executeHandlers(String triggerName, Object handler) {
    pkg.DispatchState.Value = 'ext';
  }
}
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/default/classes/DependencyTriggerCollisionTest.cls"), `
@IsTest
private class DependencyTriggerCollisionTest {
  @IsTest static void dependencyTriggerUsesDependencyManager() {
    insert new pkg__Widget__c(Name = 'one');
    System.assertEquals('pkg', pkg.DispatchState.Value);
  }
}
`)

	run := Run(loadTestIndex(t, consumerRoot), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func recordWithFieldValueExists(state storage.ObjectState, fieldName, value string) bool {
	for _, record := range state.Records {
		if strings.EqualFold(record.Fields[fieldName].String, value) {
			return true
		}
	}
	return false
}

func guestPermissionSetAssignmentExists(org storage.OrgState) bool {
	guestUserID, ok := projectGuestUserID(&org)
	if !ok {
		return false
	}
	permissionSetID, ok := recordFieldID(org.Objects["PermissionSet"], "Name", "Guest_Public")
	if !ok {
		return false
	}
	for _, record := range org.Objects["PermissionSetAssignment"].Records {
		assignee, hasAssignee := record.GetField("AssigneeId")
		permissionSet, hasPermissionSet := record.GetField("PermissionSetId")
		if hasAssignee && hasPermissionSet &&
			storageIDValueEqualsText(assignee, string(guestUserID)) &&
			storageIDValueEqualsText(permissionSet, string(permissionSetID)) {
			return true
		}
	}
	return false
}

func TestOrgFromIndexIncludesProjectStaticResources(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/staticresources/resetcss.resource"), "body")
	writeFile(t, filepath.Join(root, "force-app/main/default/staticresources/resetcss.resource-meta.xml"), `<StaticResource><contentType>text/css</contentType><cacheControl>Public</cacheControl></StaticResource>`)
	org := orgFromIndex(loadTestIndex(t, root))
	object := org.Objects["StaticResource"]
	for _, record := range object.Records {
		if record.Fields["Name"].String == "resetcss" {
			return
		}
	}
	t.Fatalf("resetcss StaticResource record was not created; records=%#v", object.Records)
}

func TestRunTestLoadDataHandlesCsvEdgesAndTypedFields(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Load_Row__c/Load_Row__c.object-meta.xml"), `
<CustomObject>
  <label>Load Row</label>
  <pluralLabel>Load Rows</pluralLabel>
  <nameField><type>Text</type><label>Name</label></nameField>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Load_Row__c/fields/Quoted__c.field-meta.xml"), `<CustomField><fullName>Quoted__c</fullName><label>Quoted</label><type>Text</type></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Load_Row__c/fields/Blank__c.field-meta.xml"), `<CustomField><fullName>Blank__c</fullName><label>Blank</label><type>Text</type></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Load_Row__c/fields/When__c.field-meta.xml"), `<CustomField><fullName>When__c</fullName><label>When</label><type>Date</type></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Load_Row__c/fields/At__c.field-meta.xml"), `<CustomField><fullName>At__c</fullName><label>At</label><type>DateTime</type></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Load_Row__c/fields/Ready__c.field-meta.xml"), `<CustomField><fullName>Ready__c</fullName><label>Ready</label><type>Checkbox</type><defaultValue>false</defaultValue></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Load_Row__c/fields/Amount__c.field-meta.xml"), `<CustomField><fullName>Amount__c</fullName><label>Amount</label><type>Number</type><precision>16</precision><scale>2</scale></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Load_Row__c/fields/Lookup__c.field-meta.xml"), `<CustomField><fullName>Lookup__c</fullName><label>Lookup</label><type>Lookup</type><referenceTo>Account</referenceTo></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/staticresources/LoadRows.resource"), "Name,Quoted__c,Blank__c,When__c,At__c,Ready__c,Amount__c,Lookup__c\r\n\"Acme, Inc.\",\"said \"\"hello\"\"\",,2024-02-29,2024-02-29T12:34:56Z,true,42.50,001000000000001AAA\r\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/staticresources/LoadRows.resource-meta.xml"), `<StaticResource><contentType>text/csv</contentType><cacheControl>Private</cacheControl></StaticResource>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/staticresources/BadLoadRows.resource"), "Name,Missing__c\nBad,bad\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/staticresources/BadLoadRows.resource-meta.xml"), `<StaticResource><contentType>text/csv</contentType><cacheControl>Private</cacheControl></StaticResource>`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/LoadDataEdgesTest.cls"), `
@isTest
private class LoadDataEdgesTest {
  @isTest static void loadsCsvEdgesThroughDml() {
    List<Load_Row__c> rows = Test.loadData(Load_Row__c.SObjectType, 'LoadRows');
    System.assertEquals(1, rows.size());
    Load_Row__c row = [SELECT Name, Quoted__c, Blank__c, When__c, At__c, Ready__c, Amount__c, Lookup__c FROM Load_Row__c LIMIT 1];
    System.assertEquals('Acme, Inc.', row.Name);
    System.assertEquals('said "hello"', row.Quoted__c);
    System.assertEquals(null, row.Blank__c);
    System.assertEquals(Date.newInstance(2024, 2, 29), row.When__c);
    System.assertEquals(Datetime.valueOfGmt('2024-02-29T12:34:56Z'), row.At__c);
    System.assertEquals(true, row.Ready__c);
    System.assertEquals(42.50, row.Amount__c);
    System.assertEquals('001000000000001AAA', row.Lookup__c);
    System.assertEquals(1, Limits.getDmlStatements());
    System.assertEquals(1, Limits.getDmlRows());
  }

  @isTest static void reportsMissingResourceAndBadHeader() {
    try {
      Test.loadData(Load_Row__c.SObjectType, 'MissingRows');
      System.assert(false, 'missing resource should fail');
    } catch (Exception e) {
      System.assert(e.getMessage().contains('static resource MissingRows not found'));
    }
    try {
      Test.loadData(Load_Row__c.SObjectType, 'BadLoadRows');
      System.assert(false, 'bad header should fail');
    } catch (Exception e) {
      System.assert(e.getMessage().contains('Unknown field Missing__c'));
    }
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		t.Fatalf("summary = %#v cases=%#v", got, run.Suites[0].Cases)
	}
}

func TestOrgFromIndexNamespacesProjectStaticResources(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/staticresources/Resources.resource"), "body")
	writeFile(t, filepath.Join(root, "force-app/main/default/staticresources/Resources.resource-meta.xml"), `<StaticResource><contentType>text/plain</contentType><cacheControl>Public</cacheControl></StaticResource>`)
	org := orgFromIndex(loadTestIndex(t, root))
	object := org.Objects["StaticResource"]
	for _, record := range object.Records {
		if record.Fields["Name"].String == "Resources" {
			if got := record.Fields["NamespacePrefix"].String; got != "pkg" {
				t.Fatalf("Resources NamespacePrefix = %q, want pkg", got)
			}
			return
		}
	}
	t.Fatalf("Resources StaticResource record was not created; records=%#v", object.Records)
}

func TestRuntimeCallsProjectMergeValuesPutSObject(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/MergeValues.cls"), `
public class MergeValues {
  private Map<String, Object> values = new Map<String, Object>();
  public void registerFieldSecurely(String fieldName) {}
  public void putSObject(String objectName, Id recordId) {
    if (objectName == 'User') {
      values.put('User.FirstName', UserInfo.getFirstName());
      values.put('User.LastName', UserInfo.getLastName());
    }
  }
  public Object get(String fieldName) {
    return values.get(fieldName);
  }
}
`)
	index := loadTestIndex(t, root)
	methods := compileProjectMethods(index)
	found := false
	for _, method := range methods {
		if method.Name == "MergeValues.putSObject" {
			found = true
			if len(method.Params) != 2 || method.Params[0].Type != "String" || method.Params[1].Type != "Id" {
				t.Fatalf("MergeValues.putSObject params = %#v", method.Params)
			}
		}
	}
	if !found {
		t.Fatal("MergeValues.putSObject was not compiled")
	}
	org := orgFromIndex(index)
	machine := vm.New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := registerRuntime(machine, methods, compileProjectClasses(index, methods), nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := machine.Classes["MergeValues"]; !ok {
		t.Fatal("MergeValues class was not registered")
	}
	if candidates := machine.MethodOverloads["MergeValues.putSObject"]; len(candidates) == 0 {
		t.Fatalf("MergeValues.putSObject overloads were not registered; methods has %d entries", len(machine.Methods))
	}
	program, err := vm.CompileAnonymous(`
MergeValues bag = new MergeValues();
bag.registerFieldSecurely('User.FirstName');
bag.registerFieldSecurely('User.LastName');
bag.putSObject('User', UserInfo.getUserId());
System.assertEquals(UserInfo.getFirstName(), bag.get('User.FirstName'));
System.assertEquals(UserInfo.getLastName(), bag.get('User.LastName'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestParseParamsSkipsAnnotationArguments(t *testing.T) {
	params, err := parseParams(`
@AuraEnabled(Cacheable=true)
public static Id getAccountId() {
    return UserInfo.getUserId();
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(params) != 0 {
		t.Fatalf("params = %#v", params)
	}

	params, err = parseParams(`
@AuraEnabled(Cacheable=true)
public static String render(final Id templateId, Map<String, Object> values) {
    return '';
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(params) != 2 || params[0].Type != "Id" || params[0].Name != "templateId" || params[1].Type != "Map<String, Object>" || params[1].Name != "values" {
		t.Fatalf("params = %#v", params)
	}

	params, err = parseParams(`
public static List<SObject> queryByIds(String query, /* do not remove param */ Set<Id> ids) {
    return Database.query(query);
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(params) != 2 || params[0].Type != "String" || params[0].Name != "query" || params[1].Type != "Set<Id>" || params[1].Name != "ids" {
		t.Fatalf("params = %#v", params)
	}

	params, err = parseParams(`
private BulkPriceClassResponse getBulkPriceClassResponse(Map<Id, CartItemPricer>cartItemPricersByCartItemId) {
    return null;
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(params) != 1 || params[0].Type != "Map<Id, CartItemPricer>" || params[0].Name != "cartItemPricersByCartItemId" {
		t.Fatalf("params = %#v", params)
	}
}

func TestProjectRuntimeResolvesNestedEnumConstantsInStaticInitializers(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ObjectMappings.cls"), `
public class ObjectMappings {
  public enum MAPPING_OPERATION_TYPE {
    /**
     * Sets a field value.
     */
    setFieldValue,
    // Selects source records.
    sourceObjectSelectionCriteria,
    targetObjectSelectionCriteria
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MappingOperationFactory.cls"), `
public class MappingOperationFactory {
  private static final Map<ObjectMappings.MAPPING_OPERATION_TYPE, String> operationInstances =
    new Map<ObjectMappings.MAPPING_OPERATION_TYPE, String>{
      ObjectMappings.MAPPING_OPERATION_TYPE.setFieldValue => 'set',
      ObjectMappings.MAPPING_OPERATION_TYPE.sourceObjectSelectionCriteria => 'source',
      ObjectMappings.MAPPING_OPERATION_TYPE.targetObjectSelectionCriteria => 'target'
    };
  public static String get(ObjectMappings.MAPPING_OPERATION_TYPE operationType) {
    return operationInstances.get(operationType);
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MappingOperationFactoryTest.cls"), `
@isTest
private class MappingOperationFactoryTest {
  @isTest static void resolvesNestedEnumConstants() {
    System.assertEquals('set', MappingOperationFactory.get(ObjectMappings.MAPPING_OPERATION_TYPE.setFieldValue));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestProjectRuntimeStaticEnumFieldAvailableToSuperConstructorArg(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/Options.cls"), `
public class Options {
  public enum Kind {
    AccountHardCredit
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BaseJob.cls"), `
public virtual class BaseJob {
  public Options.Kind JobType;
  public BaseJob(Options.Kind jobType) {
    this.JobType = jobType;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/AccountJob.cls"), `
public class AccountJob extends BaseJob {
  private static final Options.Kind jobType = Options.Kind.AccountHardCredit;
  public static Options.Kind getJobType() {
    return jobType;
  }
  public AccountJob() {
    super(jobType);
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/AccountJobTest.cls"), `
@isTest
private class AccountJobTest {
  @isTest static void staticEnumFieldFeedsSuperConstructor() {
    System.assertEquals(Options.Kind.AccountHardCredit, AccountJob.getJobType());
    AccountJob job = new AccountJob();
    System.assertEquals(Options.Kind.AccountHardCredit, job.JobType);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunNestedSubclassInheritsBaseProperties(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BaseRequest.cls"), `
public virtual class BaseRequest {
  public String AccountId { get; set; }
  public Datetime TransactionDate { get; set; }
  public BaseRequest() {
    this.TransactionDate = Datetime.now();
  }
  public virtual void validate() {
    List<String> missing = getNullParams();
    if (!missing.isEmpty()) {
      throw new IllegalArgumentException(String.join(missing, ','));
    }
  }
  public virtual String typeName() {
    throw new IllegalArgumentException('must override');
  }
  protected virtual List<String> getNullParams() {
    List<String> missing = new List<String>();
    if (this.AccountId == null) {
      missing.add('AccountId');
    }
    if (this.TransactionDate == null) {
      missing.add('TransactionDate');
    }
    return missing;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/RequestFactory.cls"), `
public class RequestFactory {
  public void create(BaseRequest request) {
    request.validate();
    request.typeName();
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/NestedRequestTest.cls"), `
@isTest
private class NestedRequestTest {
  private class ChildRequest extends BaseRequest { }

  @isTest static void nestedSubclassUsesInheritedStateAndMethods() {
    ChildRequest request = new ChildRequest();
    request.AccountId = '001000000000001AAA';
    try {
      new RequestFactory().create(request);
      System.assert(false, 'Expected inherited method exception.');
    } catch (IllegalArgumentException e) {
      System.assertEquals('must override', e.getMessage());
    }
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunTypeForNameDoesNotResolveTopLevelTestClass(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ReflectiveTest.cls"), `
@isTest
private class ReflectiveTest {
  public class Helper {
    public String value() {
      return 'ok';
    }
  }

  @isTest static void topLevelTestClassIsHiddenButNestedHelperResolves() {
    System.assertEquals(null, Type.forName('ReflectiveTest'));
    Object helper = Type.forName('ReflectiveTest.Helper').newInstance();
    System.assertEquals('ok', ((Helper) helper).value());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunTypeForNameResolvesQualifiedTopLevelTestClass(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ReflectiveQualifiedTest.cls"), `
@isTest
private class ReflectiveQualifiedTest {
  @isTest static void qualifiedTestClassTokenResolves() {
    System.assertEquals(null, Type.forName('ReflectiveQualifiedTest'));
    Type qualified = Type.forName(ReflectiveQualifiedTest.class.getName());
    System.assertNotEquals(null, qualified);
    Object instance = qualified.newInstance();
    System.assert(instance instanceof ReflectiveQualifiedTest);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunTypeForNameResolvesPublicTopLevelTestHelper(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ReflectiveHelper.cls"), `
@isTest
public class ReflectiveHelper {
  public String value() {
    return 'ok';
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ReflectiveHelperTest.cls"), `
@isTest
private class ReflectiveHelperTest {
  @isTest static void publicTopLevelTestHelperResolves() {
    Object helper = Type.forName('ReflectiveHelper').newInstance();
    System.assertEquals('ok', ((ReflectiveHelper) helper).value());
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunPlatformCacheNestedBuilderUsesStringKey(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "glade.yml"), `org:
  features: [MultiCurrency]
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/CacheHost.cls"), `
public class CacheHost {
  public String accountCode;

  public static Object load(String code) {
    if (code == null) {
      return null;
    }
    return loadRaw(code);
  }

  public static Object loadRaw(String code) {
    Cache.OrgPartition orgCache = Cache.Org.getPartition('local.default');
    return orgCache.get(NestedLoader.class, code);
  }

  public static Object loadFromAccount(Account account) {
    return load((String) account.get('CurrencyIsoCode'));
  }

  public Object loadField() {
    return load(accountCode);
  }

  public class NestedLoader implements Cache.CacheBuilder {
    public Object doLoad(String key) {
      return 'loaded:' + key;
    }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/CacheHostTest.cls"), `
@isTest
private class CacheHostTest {
  @isTest static void nestedCacheBuilderLoadsValue() {
    String code = 'USD';
    System.assertEquals('loaded:USD', (String) CacheHost.load(code));
    System.assertEquals('loaded:USD', (String) CacheHost.load(code));
    System.assertEquals(null, CacheHost.loadFromAccount(new Account(Name = 'Local')));
    System.assertEquals(null, new CacheHost().loadField());
    System.assertEquals(null, CacheHost.loadRaw(null));
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func TestRunNamespacedStaticFieldMapSharesStateWithStaticMethod(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "glade.yml"), `org:
  features: [MultiCurrency]
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/FieldTokenRegistry.cls"), `
public class FieldTokenRegistry {
  public static Set<String> loadedKeys = new Set<String>();
  public static Map<String, SObjectField> tokens = new Map<String, SObjectField>();
  private static Boolean loaded;

  public static Boolean load() {
    if (loaded == null) {
      loadedKeys.add('Contact');
      tokens.put('Contact', Schema.sObjectType.Contact.fields.getMap().get('CurrencyIsoCode'));
      loaded = tokens.get('Contact') != null;
    }
    return loaded;
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/FieldTokenBatch.cls"), `
public class FieldTokenBatch implements Database.Batchable<Integer> {
  public List<Integer> start(Database.BatchableContext bc) {
    return new List<Integer>{1};
  }
  public void execute(Database.BatchableContext bc, List<Integer> scope) {
    FieldTokenConsumer.touch();
  }
  public void finish(Database.BatchableContext bc) {}
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/FieldTokenConsumer.cls"), `
public class FieldTokenConsumer {
  public static void touch() {
    if (FieldTokenRegistry.load()) {
      Contact contactRecord = new Contact(LastName = 'Local');
      contactRecord.put(FieldTokenRegistry.tokens.get('Contact'), 'USD');
    }
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/triggers/OpportunityProbeTrigger.trigger"), `
trigger OpportunityProbeTrigger on Opportunity (before insert) {
  FieldTokenConsumer.touch();
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/FieldTokenRegistryTest.cls"), `
@isTest
private class FieldTokenRegistryTest {
  @isTest static void staticFieldMapUsesMethodState() {
    System.assert(FieldTokenRegistry.load());
    Contact contactRecord = new Contact(LastName = 'Local');
    contactRecord.put(FieldTokenRegistry.tokens.get('Contact'), 'USD');
    System.assertEquals('USD', contactRecord.get('CurrencyIsoCode'));
    Test.startTest();
    insert new Opportunity(Name = 'Local', StageName = 'Prospecting', CloseDate = Date.today());
    Database.executeBatch(new FieldTokenBatch(), 200);
    Test.stopTest();
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v case=%#v problem=%#v", got, run.Suites[0].Cases[0], run.Suites[0].Cases[0].Problem)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeCapturedBillingArtifact(t *testing.T, path string) {
	t.Helper()
	artifact, err := packageartifact.BuildCaptured(packageartifact.BuildCapturedOptions{
		Namespace: "pkg",
		Version:   "1.0",
		Capture:   packageartifact.CaptureProvenance{Source: "org"},
		ApexTypes: []packageartifact.ApexType{{
			Kind:      apexast.DeclarationClass,
			Name:      "BillingGateway",
			Namespace: "pkg",
			Modifiers: []string{"global"},
			Members: []packageartifact.ApexMember{{
				Kind:      apexast.DeclarationMethod,
				Name:      "authorize",
				Type:      "Boolean",
				Modifiers: []string{"global", "static"},
				Parameters: []apexast.Parameter{{
					Name: "amount",
					Type: "Decimal",
				}},
			}, {
				Kind:      apexast.DeclarationConstructor,
				Name:      "BillingGateway",
				Modifiers: []string{"global"},
				Parameters: []apexast.Parameter{{
					Name: "amount",
					Type: "Decimal",
				}},
			}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := packageartifact.WriteJSON(path, artifact); err != nil {
		t.Fatal(err)
	}
}
