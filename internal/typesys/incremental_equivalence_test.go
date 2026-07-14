package typesys

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/packageartifact"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

func TestUpdateApexFilesPreservesDormantStateOnSamePathChanges(t *testing.T) {
	tests := []struct {
		name          string
		changedPath   func(*incrementalEquivalenceFixture) string
		changedSource string
		compareSymbol func(t *testing.T, path string, got, want Index)
	}{
		{
			name:        "source-backed dependency class",
			changedPath: func(f *incrementalEquivalenceFixture) string { return f.dependencyClass },
			changedSource: `global class StageService {
  global static %%%NAMESPACE_DOT%%%StageService modifiedValue(BasePkg.StageService input) { return input; }
}`,
			compareSymbol: compareIncrementalTypeSymbolAtPath,
		},
		{
			name:          "source-backed dependency trigger",
			changedPath:   func(f *incrementalEquivalenceFixture) string { return f.dependencyTrigger },
			changedSource: "trigger StageTrigger on BasePkg__Ledger__c (before insert, after update) { BasePkg.StageService.modifiedValue(null); }",
			compareSymbol: compareIncrementalTriggerSymbolAtPath,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newIncrementalEquivalenceFixture(t)
			path := test.changedPath(fixture)
			previous := fixture.buildFresh(t)
			previous.Diagnostics = append(previous.Diagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "stale_changed_path",
				Message:  "stale diagnostic from the prior source",
				File:     path,
			})

			writeFile(t, path, test.changedSource)
			updated := UpdateApexFiles(previous, []string{path}, nil)
			fresh := fixture.buildFresh(t)

			if mismatches := incrementalDormantStateMismatches(updated, fresh); len(mismatches) > 0 {
				t.Errorf("dormant index fields differ from full Build: %v", mismatches)
			}
			test.compareSymbol(t, path, updated, fresh)
			for _, diag := range updated.Diagnostics {
				if diag.Code == "stale_changed_path" {
					t.Errorf("changed-path diagnostic was retained: %#v", diag)
				}
			}
		})
	}
}

func TestUpdateApexFilesCleanRichSamePathUsesFastPath(t *testing.T) {
	tests := []struct {
		name          string
		changedPath   func(*incrementalEquivalenceFixture) string
		changedSource string
		compare       func(*testing.T, string, Index, Index)
	}{
		{
			name:          "dependency trigger",
			changedPath:   func(f *incrementalEquivalenceFixture) string { return f.dependencyTrigger },
			changedSource: "trigger StageTrigger on BasePkg__Ledger__c (before insert, after update) {}",
			compare:       compareIncrementalTriggerSymbolAtPath,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newCleanIncrementalEquivalenceFixture(t)
			previous := fixture.buildFresh(t)
			path := test.changedPath(fixture)
			writeFile(t, path, test.changedSource)
			candidate, fastPath := updateApexFilesIncremental(previous, []string{path}, nil)
			if !fastPath {
				t.Fatal("clean rich same-path edit did not use fast path")
			}
			fresh := fixture.buildFresh(t)
			if mismatches := incrementalDormantStateMismatches(candidate, fresh); len(mismatches) > 0 {
				t.Errorf("fast-path dormant fields differ: %v", mismatches)
			}
			test.compare(t, path, candidate, fresh)
		})
	}
}

func TestUpdateApexFilesDependencyClassCountChangesFallBack(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*testing.T, *incrementalEquivalenceFixture) (changed, deleted []string)
		wantCount int
	}{
		{
			name: "same-path edit adds nested type",
			mutate: func(t *testing.T, fixture *incrementalEquivalenceFixture) ([]string, []string) {
				writeFile(t, fixture.dependencyClass, "global class StageService { global class Nested {} global static String changed() { return 'changed'; } }")
				return []string{fixture.dependencyClass}, nil
			},
			wantCount: 2,
		},
		{
			name: "deletion removes dependency type",
			mutate: func(t *testing.T, fixture *incrementalEquivalenceFixture) ([]string, []string) {
				fixture.deleteFile(t, &fixture.dependencyApexFiles, fixture.dependencyClass)
				return nil, []string{fixture.dependencyClass}
			},
			wantCount: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newCleanIncrementalEquivalenceFixture(t)
			previous := fixture.buildFresh(t)
			changed, deleted := test.mutate(t, fixture)
			if candidate, fastPath := updateApexFilesIncremental(previous, changed, deleted); fastPath {
				t.Fatalf("dependency class count change unexpectedly used fast path: dependencies=%#v types=%#v", candidate.Dependencies, candidate.Types)
			}

			updated := UpdateApexFiles(previous, changed, deleted)
			fresh := fixture.buildFresh(t)
			if !reflect.DeepEqual(updated, fresh) {
				t.Errorf("dependency class count fallback differs from full Build:\nincremental: %#v\nfull: %#v", updated, fresh)
			}
			if got := dependencyApexTypeCount(t, updated, "stagepkg"); got != test.wantCount {
				t.Errorf("stagepkg dependency ApexTypes = %d, want %d", got, test.wantCount)
			}
		})
	}
}

func dependencyApexTypeCount(t *testing.T, idx Index, namespace string) int {
	t.Helper()
	for _, dependency := range idx.Dependencies {
		if dependency.Namespace == namespace {
			return dependency.ApexTypes
		}
	}
	t.Fatalf("dependency %q not found: %#v", namespace, idx.Dependencies)
	return 0
}

func TestUpdateApexFilesCanonicalizesAndDeduplicatesChangedPaths(t *testing.T) {
	tests := []struct {
		name          string
		changedPath   func(*incrementalEquivalenceFixture) string
		changedSource string
		countSymbols  func(Index, string) int
	}{
		{
			name:          "class",
			changedPath:   func(fixture *incrementalEquivalenceFixture) string { return fixture.localClass },
			changedSource: "public class LocalService { public static String changed() { return 'changed'; } }",
			countSymbols: func(idx Index, path string) int {
				count := 0
				for _, typ := range idx.Types {
					if typ.Name == "LocalService" && typ.File == path {
						count++
					}
				}
				return count
			},
		},
		{
			name:          "trigger",
			changedPath:   func(fixture *incrementalEquivalenceFixture) string { return fixture.localTrigger },
			changedSource: "trigger LocalTrigger on Account (before insert, after update) {}",
			countSymbols: func(idx Index, path string) int {
				count := 0
				for _, trigger := range idx.Triggers {
					if trigger.Name == "LocalTrigger" && trigger.File == path {
						count++
					}
				}
				return count
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newCleanIncrementalEquivalenceFixture(t)
			previous := fixture.buildFresh(t)
			path := test.changedPath(fixture)
			equivalentPath := filepath.Dir(path) + string(os.PathSeparator) + ".." + string(os.PathSeparator) + filepath.Base(filepath.Dir(path)) + string(os.PathSeparator) + filepath.Base(path)
			if equivalentPath == path || cleanFilePath(equivalentPath) != cleanFilePath(path) {
				t.Fatalf("dot-segment path is not equivalent: canonical=%q equivalent=%q", path, equivalentPath)
			}
			writeFile(t, path, test.changedSource)

			candidate, fastPath := updateApexFilesIncremental(previous, []string{equivalentPath, path, equivalentPath}, nil)
			if !fastPath {
				t.Fatal("normalized equivalent notifications did not use fast path")
			}
			fresh := fixture.buildFresh(t)
			if !reflect.DeepEqual(candidate, fresh) {
				t.Errorf("canonicalized fast path differs from full Build:\nincremental: %#v\nfull: %#v", candidate, fresh)
			}
			if got := test.countSymbols(candidate, path); got != 1 {
				t.Errorf("canonical symbol count = %d, want 1", got)
			}
			if got := test.countSymbols(candidate, equivalentPath); got != 0 {
				t.Errorf("raw equivalent-path symbol count = %d, want 0", got)
			}
		})
	}
}

func TestUpdateApexFilesBulkChangedSourcesFallBack(t *testing.T) {
	fixture := newCleanIncrementalEquivalenceFixture(t)
	previous := fixture.buildFresh(t)
	writeFile(t, fixture.localClass, "public class LocalService { public static String changed() { return 'changed'; } }")
	writeFile(t, fixture.localTrigger, "trigger LocalTrigger on Account (before insert, after update) {}")

	changed := []string{fixture.localClass, fixture.localTrigger}
	if candidate, fastPath := updateApexFilesIncremental(previous, changed, nil); fastPath {
		t.Fatalf("multiple distinct changed source keys unexpectedly used fast path: %#v", candidate)
	}
	updated := UpdateApexFiles(previous, changed, nil)
	fresh := fixture.buildFresh(t)
	if !reflect.DeepEqual(updated, fresh) {
		t.Errorf("bulk changed-source fallback differs from full Build:\nincremental: %#v\nfull: %#v", updated, fresh)
	}
}

func TestUpdateApexFilesIdentityWorkIsBounded(t *testing.T) {
	newCountingOps := func(statCalls, sameFileCalls *int) incrementalFileIdentityOps {
		return incrementalFileIdentityOps{
			stat: func(path string) (os.FileInfo, error) {
				(*statCalls)++
				return os.Stat(path)
			},
			sameFile: func(left, right os.FileInfo) bool {
				(*sameFileCalls)++
				return os.SameFile(left, right)
			},
		}
	}

	t.Run("bulk changes do no identity work", func(t *testing.T) {
		fixture := newCleanIncrementalEquivalenceFixture(t)
		previous := fixture.buildFresh(t)
		statCalls := 0
		sameFileCalls := 0
		candidate, fastPath := updateApexFilesIncrementalWithIdentityOps(
			previous,
			[]string{fixture.localClass, fixture.localTrigger},
			nil,
			newCountingOps(&statCalls, &sameFileCalls),
		)
		if fastPath {
			t.Fatalf("bulk changes unexpectedly used fast path: %#v", candidate)
		}
		if statCalls != 0 || sameFileCalls != 0 {
			t.Errorf("bulk identity work: stat=%d sameFile=%d, want zero", statCalls, sameFileCalls)
		}
	})

	t.Run("delete-only does no identity work", func(t *testing.T) {
		fixture := newCleanIncrementalEquivalenceFixture(t)
		previous := fixture.buildFresh(t)
		fixture.deleteFile(t, &fixture.localApexFiles, fixture.localClass)
		statCalls := 0
		sameFileCalls := 0
		candidate, fastPath := updateApexFilesIncrementalWithIdentityOps(
			previous,
			nil,
			[]string{fixture.localClass},
			newCountingOps(&statCalls, &sameFileCalls),
		)
		if fastPath {
			t.Fatalf("delete-only change unexpectedly used fast path: %#v", candidate)
		}
		if statCalls != 0 || sameFileCalls != 0 {
			t.Errorf("delete-only identity work: stat=%d sameFile=%d, want zero", statCalls, sameFileCalls)
		}
		updated := UpdateApexFiles(previous, nil, []string{fixture.localClass})
		fresh := fixture.buildFresh(t)
		if !reflect.DeepEqual(updated, fresh) {
			t.Errorf("delete-only fallback differs from full Build:\nincremental: %#v\nfull: %#v", updated, fresh)
		}
	})

	t.Run("changed plus deleted does no identity work", func(t *testing.T) {
		fixture := newCleanIncrementalEquivalenceFixture(t)
		previous := fixture.buildFresh(t)
		writeFile(t, fixture.localClass, "public class LocalService { public static String changed() { return 'changed'; } }")
		fixture.deleteFile(t, &fixture.localApexFiles, fixture.localTrigger)
		statCalls := 0
		sameFileCalls := 0
		candidate, fastPath := updateApexFilesIncrementalWithIdentityOps(
			previous,
			[]string{fixture.localClass},
			[]string{fixture.localTrigger},
			newCountingOps(&statCalls, &sameFileCalls),
		)
		if fastPath {
			t.Fatalf("changed-plus-deleted sources unexpectedly used fast path: %#v", candidate)
		}
		if statCalls != 0 || sameFileCalls != 0 {
			t.Errorf("changed-plus-deleted identity work: stat=%d sameFile=%d, want zero", statCalls, sameFileCalls)
		}
		updated := UpdateApexFiles(previous, []string{fixture.localClass}, []string{fixture.localTrigger})
		fresh := fixture.buildFresh(t)
		if !reflect.DeepEqual(updated, fresh) {
			t.Errorf("changed-plus-deleted fallback differs from full Build:\nincremental: %#v\nfull: %#v", updated, fresh)
		}
	})

	t.Run("nested declarations stat once per requested path", func(t *testing.T) {
		fixture := newCleanIncrementalEquivalenceFixture(t)
		writeFile(t, fixture.localClass, "public class LocalService { public class NestedOne {} public class NestedTwo {} public static String beforeEdit() { return 'before'; } }")
		previous := fixture.buildFresh(t)
		distinctSourcePaths := make(map[string]bool)
		for _, typ := range previous.Types {
			if !typ.Artifact && strings.HasSuffix(strings.ToLower(typ.File), ".cls") {
				distinctSourcePaths[cleanFilePath(typ.File)] = true
			}
		}
		for _, trigger := range previous.Triggers {
			if strings.HasSuffix(strings.ToLower(trigger.File), ".trigger") {
				distinctSourcePaths[cleanFilePath(trigger.File)] = true
			}
		}
		if len(distinctSourcePaths) < 2 {
			t.Fatalf("fixture has too few distinct Apex source paths: %#v", distinctSourcePaths)
		}

		writeFile(t, fixture.localClass, "public class LocalService { public class NestedOne {} public class NestedTwo {} public static String afterEdit() { return 'after'; } }")
		statCalls := 0
		sameFileCalls := 0
		candidate, fastPath := updateApexFilesIncrementalWithIdentityOps(
			previous,
			[]string{fixture.localClass},
			nil,
			newCountingOps(&statCalls, &sameFileCalls),
		)
		if !fastPath {
			t.Fatal("single nested-class edit did not use fast path")
		}
		fresh := fixture.buildFresh(t)
		if !reflect.DeepEqual(candidate, fresh) {
			t.Errorf("bounded identity candidate differs from full Build:\nincremental: %#v\nfull: %#v", candidate, fresh)
		}
		if statCalls != len(distinctSourcePaths) {
			t.Errorf("stat calls = %d, want one per %d distinct requested paths", statCalls, len(distinctSourcePaths))
		}
		if sameFileCalls != len(distinctSourcePaths)-1 {
			t.Errorf("SameFile calls = %d, want one changed path against %d peers", sameFileCalls, len(distinctSourcePaths)-1)
		}
	})
}

func TestUpdateApexFilesSharedPhysicalAliasesFallBack(t *testing.T) {
	tests := []struct {
		name             string
		localPath        func(*incrementalEquivalenceFixture) string
		dependencyPath   func(*incrementalEquivalenceFixture) string
		physicalFile     string
		initialSource    string
		changedSource    string
		link             func(string, string) error
		assertFreshAlias func(*testing.T, Index, string, string)
	}{
		{
			name:           "symlink class",
			localPath:      func(fixture *incrementalEquivalenceFixture) string { return fixture.localClass },
			dependencyPath: func(fixture *incrementalEquivalenceFixture) string { return fixture.dependencyClass },
			physicalFile:   "Shared.cls",
			initialSource:  "global class SharedPhysical { global void beforeEdit() {} }",
			changedSource:  "global class RenamedPhysical { global void afterEdit() {} }",
			link:           os.Symlink,
			assertFreshAlias: func(t *testing.T, idx Index, localPath, dependencyPath string) {
				local, localOK := incrementalTypeSymbolAtPath(idx, localPath)
				dependency, dependencyOK := incrementalTypeSymbolAtPath(idx, dependencyPath)
				if !localOK || !dependencyOK || local.Name != "RenamedPhysical" || dependency.Name != "RenamedPhysical" || local.Namespace != "localpkg" || dependency.Namespace != "stagepkg" {
					t.Fatalf("fresh class aliases do not expose both updated identities: local=%#v/%t dependency=%#v/%t", local, localOK, dependency, dependencyOK)
				}
			},
		},
		{
			name:           "symlink trigger",
			localPath:      func(fixture *incrementalEquivalenceFixture) string { return fixture.localTrigger },
			dependencyPath: func(fixture *incrementalEquivalenceFixture) string { return fixture.dependencyTrigger },
			physicalFile:   "Shared.trigger",
			initialSource:  "trigger SharedTrigger on Account (before insert) {}",
			changedSource:  "trigger RenamedTrigger on Account (before insert, after update) {}",
			link:           os.Symlink,
			assertFreshAlias: func(t *testing.T, idx Index, localPath, dependencyPath string) {
				local, localOK := incrementalTriggerSymbolAtPath(idx, localPath)
				dependency, dependencyOK := incrementalTriggerSymbolAtPath(idx, dependencyPath)
				if !localOK || !dependencyOK || local.Name != "RenamedTrigger" || dependency.Name != "RenamedTrigger" || local.Namespace != "localpkg" || dependency.Namespace != "stagepkg" {
					t.Fatalf("fresh trigger aliases do not expose both updated identities: local=%#v/%t dependency=%#v/%t", local, localOK, dependency, dependencyOK)
				}
			},
		},
		{
			name:           "hard link class",
			localPath:      func(fixture *incrementalEquivalenceFixture) string { return fixture.localClass },
			dependencyPath: func(fixture *incrementalEquivalenceFixture) string { return fixture.dependencyClass },
			physicalFile:   "Shared.cls",
			initialSource:  "global class SharedPhysical { global void beforeEdit() {} }",
			changedSource:  "global class RenamedPhysical { global void afterEdit() {} }",
			link:           os.Link,
			assertFreshAlias: func(t *testing.T, idx Index, localPath, dependencyPath string) {
				local, localOK := incrementalTypeSymbolAtPath(idx, localPath)
				dependency, dependencyOK := incrementalTypeSymbolAtPath(idx, dependencyPath)
				if !localOK || !dependencyOK || local.Name != "RenamedPhysical" || dependency.Name != "RenamedPhysical" || local.Namespace != "localpkg" || dependency.Namespace != "stagepkg" {
					t.Fatalf("fresh class aliases do not expose both updated identities: local=%#v/%t dependency=%#v/%t", local, localOK, dependency, dependencyOK)
				}
			},
		},
		{
			name:           "hard link trigger",
			localPath:      func(fixture *incrementalEquivalenceFixture) string { return fixture.localTrigger },
			dependencyPath: func(fixture *incrementalEquivalenceFixture) string { return fixture.dependencyTrigger },
			physicalFile:   "Shared.trigger",
			initialSource:  "trigger SharedTrigger on Account (before insert) {}",
			changedSource:  "trigger RenamedTrigger on Account (before insert, after update) {}",
			link:           os.Link,
			assertFreshAlias: func(t *testing.T, idx Index, localPath, dependencyPath string) {
				local, localOK := incrementalTriggerSymbolAtPath(idx, localPath)
				dependency, dependencyOK := incrementalTriggerSymbolAtPath(idx, dependencyPath)
				if !localOK || !dependencyOK || local.Name != "RenamedTrigger" || dependency.Name != "RenamedTrigger" || local.Namespace != "localpkg" || dependency.Namespace != "stagepkg" {
					t.Fatalf("fresh trigger aliases do not expose both updated identities: local=%#v/%t dependency=%#v/%t", local, localOK, dependency, dependencyOK)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newCleanIncrementalEquivalenceFixture(t)
			localPath := test.localPath(fixture)
			dependencyPath := test.dependencyPath(fixture)
			physicalPath := filepath.Join(filepath.Dir(fixture.consumerRoot), "physical", test.physicalFile)
			for _, path := range []string{localPath, dependencyPath} {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			}
			writeFile(t, physicalPath, test.initialSource)
			for _, path := range []string{localPath, dependencyPath} {
				if err := test.link(physicalPath, path); err != nil {
					t.Fatal(err)
				}
			}

			previous := fixture.buildFresh(t)
			writeFile(t, localPath, test.changedSource)
			if candidate, fastPath := updateApexFilesIncremental(previous, []string{localPath}, nil); fastPath {
				t.Fatalf("shared physical alias unexpectedly used fast path: %#v", candidate)
			}

			updated := UpdateApexFiles(previous, []string{localPath}, nil)
			fresh := fixture.buildFresh(t)
			test.assertFreshAlias(t, fresh, localPath, dependencyPath)
			if !reflect.DeepEqual(updated, fresh) {
				t.Errorf("shared physical alias fallback differs from full Build:\nincremental: %#v\nfull: %#v", updated, fresh)
			}
		})
	}
}

func TestUpdateApexFilesFastPathMatchesNamespaceOrdering(t *testing.T) {
	t.Run("types", func(t *testing.T) {
		fixture := newCleanIncrementalEquivalenceFixture(t)
		previous := fixture.buildFresh(t)
		writeFile(t, fixture.localClass, "public class ZLocal { public void changed() {} }")
		candidate, fastPath := updateApexFilesIncremental(previous, []string{fixture.localClass}, nil)
		if !fastPath {
			t.Fatal("clean local same-path class edit did not use fast path")
		}
		fresh := fixture.buildFresh(t)
		if !reflect.DeepEqual(candidate.Types, fresh.Types) {
			t.Errorf("fast-path type order differs:\nincremental: %#v\nfull: %#v", candidate.Types, fresh.Types)
		}
	})

	t.Run("triggers", func(t *testing.T) {
		fixture := newCleanIncrementalEquivalenceFixture(t)
		previous := fixture.buildFresh(t)
		writeFile(t, fixture.localTrigger, "trigger ZLocalTrigger on Account (before insert) {}")
		candidate, fastPath := updateApexFilesIncremental(previous, []string{fixture.localTrigger}, nil)
		if !fastPath {
			t.Fatal("clean local same-path trigger edit did not use fast path")
		}
		fresh := fixture.buildFresh(t)
		if !reflect.DeepEqual(candidate.Triggers, fresh.Triggers) {
			t.Errorf("fast-path trigger order differs:\nincremental: %#v\nfull: %#v", candidate.Triggers, fresh.Triggers)
		}
	})
}

func TestUpdateApexFilesDeletedPhysicalAliasFallsBack(t *testing.T) {
	root := t.TempDir()
	consumerRoot := filepath.Join(root, "consumer")
	physicalPath := filepath.Join(root, "physical", "Physical.cls")
	requestedPath := filepath.Join(consumerRoot, "force-app/main/default/classes/Alias.cls")
	writeIncrementalSFDXProject(t, consumerRoot, "localpkg")
	writeFile(t, physicalPath, "public class SharedPhysical {}")
	if err := os.MkdirAll(filepath.Dir(requestedPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(physicalPath, requestedPath); err != nil {
		t.Fatal(err)
	}
	previous := buildIncrementalIndexFromRoot(t, consumerRoot)
	if err := os.Remove(physicalPath); err != nil {
		t.Fatal(err)
	}
	if candidate, fastPath := updateApexFilesIncremental(previous, nil, []string{physicalPath}); fastPath {
		t.Errorf("deleted physical alias unexpectedly used fast path: %#v", candidate.Types)
	}
	updated := UpdateApexFiles(previous, nil, []string{physicalPath})
	fresh := buildIncrementalIndexFromRoot(t, consumerRoot)
	if mismatches := incrementalIndexMismatches(updated, fresh); len(mismatches) > 0 {
		t.Errorf("deleted physical alias differs from full Build: %v", mismatches)
	}
}

func TestUpdateApexFilesFallsBackWhenSourceProvenanceIsUnsafe(t *testing.T) {
	t.Run("invalid dependency source repaired at same path", func(t *testing.T) {
		consumerRoot, dependencyPath := newIncrementalSourceDependencyProject(t, "global class StageService {")
		previous := buildIncrementalIndexFromRoot(t, consumerRoot)
		if len(previous.Diagnostics) != 0 {
			t.Fatalf("dependency parse diagnostics should be dormant: %#v", previous.Diagnostics)
		}

		writeFile(t, dependencyPath, "global class StageService { global static %%%NAMESPACE_DOT%%%StageService repaired(BasePkg.StageService value) { return value; } }")
		updated := UpdateApexFiles(previous, []string{dependencyPath}, nil)
		fresh := buildIncrementalIndexFromRoot(t, consumerRoot)
		if mismatches := incrementalIndexMismatches(updated, fresh); len(mismatches) > 0 {
			t.Errorf("repaired dependency differs from full Build: %v", mismatches)
		}
	})

	t.Run("symlink request notified by physical path", func(t *testing.T) {
		root := t.TempDir()
		consumerRoot := filepath.Join(root, "consumer")
		physicalPath := filepath.Join(root, "physical", "Physical.cls")
		requestedPath := filepath.Join(consumerRoot, "force-app/main/default/classes/Alias.cls")
		writeIncrementalSFDXProject(t, consumerRoot, "localpkg")
		writeFile(t, physicalPath, "public class SharedPhysical { public void beforeEdit() {} }")
		if err := os.MkdirAll(filepath.Dir(requestedPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(physicalPath, requestedPath); err != nil {
			t.Fatal(err)
		}
		previous := buildIncrementalIndexFromRoot(t, consumerRoot)
		if symbol, ok := incrementalTypeSymbolAtPath(previous, requestedPath); !ok || symbol.File != requestedPath {
			t.Fatalf("requested symlink identity not indexed: %#v", previous.Types)
		}

		writeFile(t, physicalPath, "public class SharedPhysical { public void afterEdit() {} }")
		updated := UpdateApexFiles(previous, []string{physicalPath}, nil)
		fresh := buildIncrementalIndexFromRoot(t, consumerRoot)
		if mismatches := incrementalIndexMismatches(updated, fresh); len(mismatches) > 0 {
			t.Errorf("physical-path notification differs from full Build: %v", mismatches)
		}
	})

	t.Run("ambiguous logical metadata never wins by iteration order", func(t *testing.T) {
		root := t.TempDir()
		writeIncrementalSFDXProject(t, root, "localpkg")
		path := filepath.Join(root, "force-app/main/default/classes/Ambiguous.cls")
		writeFile(t, path, "public class Ambiguous { public void beforeEdit() {} }")
		previous := buildIncrementalIndexFromRoot(t, root)
		if len(previous.Types) != 1 {
			t.Fatalf("types = %#v", previous.Types)
		}
		alternate := previous.Types[0]
		alternate.Namespace = "otherpkg"
		alternate.SourceRoot = "/other/source/root"
		alternate.Version = "9.9.9"
		alternate.Dependency = true
		previous.Types = append(previous.Types, alternate)

		writeFile(t, path, "public class Ambiguous { public void afterEdit() {} }")
		updated := UpdateApexFiles(previous, []string{path}, nil)
		fresh := buildIncrementalIndexFromRoot(t, root)
		if mismatches := incrementalIndexMismatches(updated, fresh); len(mismatches) > 0 {
			t.Errorf("ambiguous path differs from full Build: %v", mismatches)
		}
	})
}

func TestUpdateApexFilesFallbackRebuildsDiagnosticsInFullBuildOrder(t *testing.T) {
	t.Run("clean source introduces cross-file duplicate", func(t *testing.T) {
		root := t.TempDir()
		writeIncrementalSFDXProject(t, root, "")
		earlierPath := filepath.Join(root, "force-app/main/default/classes/AFirst.cls")
		laterPath := filepath.Join(root, "force-app/main/default/classes/BSecond.cls")
		writeFile(t, earlierPath, "public class First {}")
		writeFile(t, laterPath, "public class Shared {}")
		previous := buildIncrementalIndexFromRoot(t, root)
		if len(previous.Diagnostics) != 0 {
			t.Fatalf("unexpected baseline diagnostics: %#v", previous.Diagnostics)
		}

		writeFile(t, earlierPath, "public class Shared {}")
		updated := UpdateApexFiles(previous, []string{earlierPath}, nil)
		fresh := buildIncrementalIndexFromRoot(t, root)
		if !reflect.DeepEqual(updated.Diagnostics, fresh.Diagnostics) {
			t.Errorf("new duplicate diagnostics differ from full Build:\nincremental: %#v\nfull: %#v", updated.Diagnostics, fresh.Diagnostics)
		}
		if len(updated.Diagnostics) != 1 || updated.Diagnostics[0].Code != "GLADETYPE001" || cleanFilePath(updated.Diagnostics[0].File) != cleanFilePath(laterPath) {
			t.Errorf("new duplicate diagnostic identity = %#v", updated.Diagnostics)
		}
	})

	t.Run("valid dependency source becomes invalid", func(t *testing.T) {
		consumerRoot, dependencyPath := newIncrementalSourceDependencyProject(t, "global class StageService { global void valid() {} }")
		previous := buildIncrementalIndexFromRoot(t, consumerRoot)
		if len(previous.Diagnostics) != 0 {
			t.Fatalf("unexpected baseline diagnostics: %#v", previous.Diagnostics)
		}

		writeFile(t, dependencyPath, "global class StageService {")
		updated := UpdateApexFiles(previous, []string{dependencyPath}, nil)
		fresh := buildIncrementalIndexFromRoot(t, consumerRoot)
		if mismatches := incrementalIndexMismatches(updated, fresh); len(mismatches) > 0 {
			t.Errorf("invalid dependency differs from full Build: %v", mismatches)
		}
		if len(updated.Diagnostics) != 0 {
			t.Errorf("dependency parse diagnostics were published: %#v", updated.Diagnostics)
		}
	})

	t.Run("duplicate source changes", func(t *testing.T) {
		root, earlierPath, _ := newIncrementalDuplicateProject(t)
		previous := buildIncrementalIndexFromRoot(t, root)
		writeFile(t, earlierPath, "public class NoLongerShared {}")
		updated := UpdateApexFiles(previous, []string{earlierPath}, nil)
		fresh := buildIncrementalIndexFromRoot(t, root)
		if mismatches := incrementalIndexMismatches(updated, fresh); len(mismatches) > 0 {
			t.Errorf("changed duplicate differs from full Build: %v", mismatches)
		}
		assertNoIncrementalDiagnosticCode(t, updated, "GLADETYPE001")
	})

	t.Run("duplicate source deletes", func(t *testing.T) {
		root, earlierPath, _ := newIncrementalDuplicateProject(t)
		previous := buildIncrementalIndexFromRoot(t, root)
		if err := os.Remove(earlierPath); err != nil {
			t.Fatal(err)
		}
		updated := UpdateApexFiles(previous, nil, []string{earlierPath})
		fresh := buildIncrementalIndexFromRoot(t, root)
		if mismatches := incrementalIndexMismatches(updated, fresh); len(mismatches) > 0 {
			t.Errorf("deleted duplicate differs from full Build: %v", mismatches)
		}
		assertNoIncrementalDiagnosticCode(t, updated, "GLADETYPE001")
	})

	t.Run("early parse error precedes later missing file", func(t *testing.T) {
		root := t.TempDir()
		writeIncrementalSFDXProject(t, root, "")
		earlyPath := filepath.Join(root, "force-app/main/default/classes/AEarly.cls")
		missingPath := filepath.Join(root, "force-app/main/default/classes/ZMissing.cls")
		writeFile(t, earlyPath, "public class Early {}")
		if err := os.MkdirAll(filepath.Dir(missingPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(root, "does-not-exist.cls"), missingPath); err != nil {
			t.Fatal(err)
		}
		previous := buildIncrementalIndexFromRoot(t, root)
		writeFile(t, earlyPath, "public class Early {")
		updated := UpdateApexFiles(previous, []string{earlyPath}, nil)
		fresh := buildIncrementalIndexFromRoot(t, root)
		if !reflect.DeepEqual(updated.Diagnostics, fresh.Diagnostics) {
			t.Errorf("diagnostic order differs from full Build:\nincremental: %#v\nfull: %#v", updated.Diagnostics, fresh.Diagnostics)
		}
	})
}

func TestUpdateApexFilesDoesNotMutatePreviousSnapshot(t *testing.T) {
	root := t.TempDir()
	writeIncrementalSFDXProject(t, root, "localpkg")
	path := filepath.Join(root, "force-app/main/default/classes/Immutable.cls")
	objectPath := filepath.Join(root, "force-app/main/default/objects/Immutable__c/Immutable__c.object-meta.xml")
	writeFile(t, path, "public class Immutable { public void beforeEdit() {} }")
	writeFile(t, objectPath, `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Immutable</label><pluralLabel>Immutables</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	previous := buildIncrementalIndexFromRoot(t, root)
	before := buildIncrementalIndexFromRoot(t, root)

	writeFile(t, path, "public class Immutable { public void afterEdit() {} }")
	candidate, fastPath := updateApexFilesIncremental(previous, []string{path}, nil)
	if !fastPath {
		t.Fatal("same-path source edit unexpectedly rejected by incremental fast path")
	}
	updated := UpdateApexFiles(previous, []string{path}, nil)
	if !reflect.DeepEqual(previous, before) {
		t.Errorf("UpdateApexFiles mutated the previous snapshot:\nafter: %#v\nbefore: %#v", previous, before)
	}
	if !reflect.DeepEqual(candidate, updated) {
		t.Errorf("public UpdateApexFiles did not publish eligible incremental candidate")
	}
	if len(previous.Objects) == 0 || len(updated.Objects) == 0 || &previous.Objects[0] != &updated.Objects[0] {
		t.Errorf("immutable dormant object payload was not structurally shared")
	}
}

func TestUpdateApexFilesFallbackFailurePreservesPreviousSnapshot(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{")
	path := filepath.Join(root, "Broken.cls")
	writeFile(t, path, "public class Broken {}")
	previous := Index{
		Project: ProjectInfo{Root: root, Namespace: "localpkg", SourceAPIVersion: "63.0"},
		Types: []TypeSymbol{{
			Kind:       apexast.DeclarationClass,
			Name:       "BeforeFallback",
			File:       path,
			Namespace:  "localpkg",
			SourceRoot: root,
		}},
		Diagnostics: []diagnostic.Diagnostic{{Severity: diagnostic.Warning, Code: "prior", Message: "prior diagnostic"}},
	}

	first := UpdateApexFiles(previous, []string{path}, nil)
	second := UpdateApexFiles(previous, []string{path}, nil)
	if !reflect.DeepEqual(first, second) {
		t.Errorf("fallback failure result is not deterministic:\nfirst: %#v\nsecond: %#v", first, second)
	}
	if !reflect.DeepEqual(previous.Diagnostics, []diagnostic.Diagnostic{{Severity: diagnostic.Warning, Code: "prior", Message: "prior diagnostic"}}) {
		t.Errorf("fallback failure mutated previous diagnostics: %#v", previous.Diagnostics)
	}
	if len(first.Diagnostics) != 2 || first.Diagnostics[1].Code != "GLADETYPE000" || first.Diagnostics[1].Severity != diagnostic.Error {
		t.Fatalf("fallback diagnostic = %#v", first.Diagnostics)
	}
	want := previous
	want.Diagnostics = append(append([]diagnostic.Diagnostic(nil), previous.Diagnostics...), first.Diagnostics[1])
	if !reflect.DeepEqual(first, want) {
		t.Errorf("fallback failure published a partial candidate:\ngot: %#v\nwant: %#v", first, want)
	}
}

func TestUpdateApexFilesMatchesFullBuildAcrossEditSequence(t *testing.T) {
	fixture := newIncrementalEquivalenceFixture(t)
	incremental := fixture.buildFresh(t)

	steps := []struct {
		name   string
		mutate func(t *testing.T) (changed, deleted []string)
	}{
		{
			name: "dependency class modify",
			mutate: func(t *testing.T) ([]string, []string) {
				writeFile(t, fixture.dependencyClass, "global class StageService { global static String modifiedValue() { return 'modified'; } }")
				return []string{fixture.dependencyClass}, nil
			},
		},
		{
			name: "local class modify",
			mutate: func(t *testing.T) ([]string, []string) {
				writeFile(t, fixture.localClass, "public class LocalService { public static String modifiedValue() { return 'modified'; } }")
				return []string{fixture.localClass}, nil
			},
		},
		{
			name: "local class add",
			mutate: func(t *testing.T) ([]string, []string) {
				writeFile(t, fixture.localAddedClass, "public class LocalAdded { public void run() {} }")
				fixture.localApexFiles = append(fixture.localApexFiles, fixture.localAddedClass)
				return []string{fixture.localAddedClass}, nil
			},
		},
		{
			name: "local class rename",
			mutate: func(t *testing.T) ([]string, []string) {
				fixture.renameFile(t, &fixture.localApexFiles, fixture.localAddedClass, fixture.localRenamedClass)
				writeFile(t, fixture.localRenamedClass, "public class LocalRenamed { public void run() {} }")
				return []string{fixture.localRenamedClass}, []string{fixture.localAddedClass}
			},
		},
		{
			name: "local class delete",
			mutate: func(t *testing.T) ([]string, []string) {
				fixture.deleteFile(t, &fixture.localApexFiles, fixture.localRenamedClass)
				return nil, []string{fixture.localRenamedClass}
			},
		},
		{
			name: "local trigger modify",
			mutate: func(t *testing.T) ([]string, []string) {
				writeFile(t, fixture.localTrigger, "trigger LocalTrigger on Account (before insert, before update) { Integer changed = Trigger.new.size(); }")
				return []string{fixture.localTrigger}, nil
			},
		},
		{
			name: "local trigger add",
			mutate: func(t *testing.T) ([]string, []string) {
				writeFile(t, fixture.localAddedTrigger, "trigger LocalAddedTrigger on Contact (after insert) {}")
				fixture.localApexFiles = append(fixture.localApexFiles, fixture.localAddedTrigger)
				return []string{fixture.localAddedTrigger}, nil
			},
		},
		{
			name: "local trigger rename",
			mutate: func(t *testing.T) ([]string, []string) {
				fixture.renameFile(t, &fixture.localApexFiles, fixture.localAddedTrigger, fixture.localRenamedTrigger)
				writeFile(t, fixture.localRenamedTrigger, "trigger LocalRenamedTrigger on Contact (after insert) {}")
				return []string{fixture.localRenamedTrigger}, []string{fixture.localAddedTrigger}
			},
		},
		{
			name: "local trigger delete",
			mutate: func(t *testing.T) ([]string, []string) {
				fixture.deleteFile(t, &fixture.localApexFiles, fixture.localRenamedTrigger)
				return nil, []string{fixture.localRenamedTrigger}
			},
		},
		{
			name: "dependency class add",
			mutate: func(t *testing.T) ([]string, []string) {
				writeFile(t, fixture.dependencyAddedClass, "global class StageAdded { global void run() {} }")
				fixture.dependencyApexFiles = append(fixture.dependencyApexFiles, fixture.dependencyAddedClass)
				return []string{fixture.dependencyAddedClass}, nil
			},
		},
		{
			name: "dependency class rename",
			mutate: func(t *testing.T) ([]string, []string) {
				fixture.renameFile(t, &fixture.dependencyApexFiles, fixture.dependencyAddedClass, fixture.dependencyRenamedClass)
				writeFile(t, fixture.dependencyRenamedClass, "global class StageRenamed { global void run() {} }")
				return []string{fixture.dependencyRenamedClass}, []string{fixture.dependencyAddedClass}
			},
		},
		{
			name: "dependency class delete",
			mutate: func(t *testing.T) ([]string, []string) {
				fixture.deleteFile(t, &fixture.dependencyApexFiles, fixture.dependencyRenamedClass)
				return nil, []string{fixture.dependencyRenamedClass}
			},
		},
		{
			name: "dependency trigger modify",
			mutate: func(t *testing.T) ([]string, []string) {
				writeFile(t, fixture.dependencyTrigger, "trigger StageTrigger on BasePkg__Ledger__c (before insert, after update) { BasePkg.StageService.modifiedValue(); }")
				return []string{fixture.dependencyTrigger}, nil
			},
		},
		{
			name: "dependency trigger add",
			mutate: func(t *testing.T) ([]string, []string) {
				writeFile(t, fixture.dependencyAddedTrigger, "trigger StageAddedTrigger on BasePkg__Ledger__c (after insert) {}")
				fixture.dependencyApexFiles = append(fixture.dependencyApexFiles, fixture.dependencyAddedTrigger)
				return []string{fixture.dependencyAddedTrigger}, nil
			},
		},
		{
			name: "dependency trigger rename",
			mutate: func(t *testing.T) ([]string, []string) {
				fixture.renameFile(t, &fixture.dependencyApexFiles, fixture.dependencyAddedTrigger, fixture.dependencyRenamedTrigger)
				writeFile(t, fixture.dependencyRenamedTrigger, "trigger StageRenamedTrigger on BasePkg__Ledger__c (after insert) {}")
				return []string{fixture.dependencyRenamedTrigger}, []string{fixture.dependencyAddedTrigger}
			},
		},
		{
			name: "dependency trigger delete",
			mutate: func(t *testing.T) ([]string, []string) {
				fixture.deleteFile(t, &fixture.dependencyApexFiles, fixture.dependencyRenamedTrigger)
				return nil, []string{fixture.dependencyRenamedTrigger}
			},
		},
	}

	for _, step := range steps {
		changed, deleted := step.mutate(t)
		incremental = UpdateApexFiles(incremental, changed, deleted)
		fresh := fixture.buildFresh(t)
		if mismatches := incrementalIndexMismatches(incremental, fresh); len(mismatches) > 0 {
			t.Errorf("%s: incremental index differs from full Build in fields %v", step.name, mismatches)
		}
	}
}

type incrementalEquivalenceFixture struct {
	consumerRoot             string
	dependencyRoot           string
	localClass               string
	localTrigger             string
	missingClass             string
	localAddedClass          string
	localRenamedClass        string
	localAddedTrigger        string
	localRenamedTrigger      string
	dependencyClass          string
	dependencyTrigger        string
	dependencyAddedClass     string
	dependencyRenamedClass   string
	dependencyAddedTrigger   string
	dependencyRenamedTrigger string
	localApexFiles           []string
	dependencyApexFiles      []string
}

func newIncrementalEquivalenceFixture(t *testing.T) *incrementalEquivalenceFixture {
	return newIncrementalEquivalenceFixtureWithDiagnostics(t, true)
}

func newCleanIncrementalEquivalenceFixture(t *testing.T) *incrementalEquivalenceFixture {
	return newIncrementalEquivalenceFixtureWithDiagnostics(t, false)
}

func newIncrementalEquivalenceFixtureWithDiagnostics(t *testing.T, withDiagnostics bool) *incrementalEquivalenceFixture {
	t.Helper()
	root := t.TempDir()
	consumerRoot := filepath.Join(root, "consumer")
	dependencyRoot := filepath.Join(root, "base-source")
	artifactPath := filepath.Join(root, "packages", "artifactpkg.glade-package.json")

	fixture := &incrementalEquivalenceFixture{
		consumerRoot:             consumerRoot,
		dependencyRoot:           dependencyRoot,
		localClass:               filepath.Join(consumerRoot, "force-app/main/default/classes/LocalService.cls"),
		localTrigger:             filepath.Join(consumerRoot, "force-app/main/default/triggers/LocalTrigger.trigger"),
		missingClass:             filepath.Join(consumerRoot, "force-app/main/default/classes/Missing.cls"),
		localAddedClass:          filepath.Join(consumerRoot, "force-app/main/default/classes/LocalAdded.cls"),
		localRenamedClass:        filepath.Join(consumerRoot, "force-app/main/default/classes/LocalRenamed.cls"),
		localAddedTrigger:        filepath.Join(consumerRoot, "force-app/main/default/triggers/LocalAddedTrigger.trigger"),
		localRenamedTrigger:      filepath.Join(consumerRoot, "force-app/main/default/triggers/LocalRenamedTrigger.trigger"),
		dependencyClass:          filepath.Join(dependencyRoot, "force-app/main/default/classes/StageService.cls"),
		dependencyTrigger:        filepath.Join(dependencyRoot, "force-app/main/default/triggers/StageTrigger.trigger"),
		dependencyAddedClass:     filepath.Join(dependencyRoot, "force-app/main/default/classes/StageAdded.cls"),
		dependencyRenamedClass:   filepath.Join(dependencyRoot, "force-app/main/default/classes/StageRenamed.cls"),
		dependencyAddedTrigger:   filepath.Join(dependencyRoot, "force-app/main/default/triggers/StageAddedTrigger.trigger"),
		dependencyRenamedTrigger: filepath.Join(dependencyRoot, "force-app/main/default/triggers/StageRenamedTrigger.trigger"),
	}
	fixture.localApexFiles = []string{fixture.localClass, fixture.localTrigger}
	if withDiagnostics {
		fixture.localApexFiles = append(fixture.localApexFiles, fixture.missingClass)
	}
	fixture.dependencyApexFiles = []string{fixture.dependencyClass, fixture.dependencyTrigger}

	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{
  "namespace": "localpkg",
  "sourceApiVersion": "63.0",
  "packageDirectories": [{"path": "force-app", "default": true}]
}`)
	gladeConfig := `project:
  namespaceRemaps: ["BasePkg:stagepkg"]
  managedPackageDependencies: ["artifactpkg:artifact:../packages/artifactpkg.glade-package.json:3.1.0", "stagepkg:../base-source:2.4.0"]
`
	if withDiagnostics {
		gladeConfig = `project:
  namespaceRemaps: ["BasePkg:stagepkg"]
  managedPackageDependencies: ["artifactpkg:artifact:../packages/artifactpkg.glade-package.json:3.1.0", "stagepkg:../base-source:2.4.0", "missingpkg:../missing-dependency:9.9.9"]
`
	}
	writeFile(t, filepath.Join(consumerRoot, "glade.yml"), gladeConfig)
	writeFile(t, filepath.Join(dependencyRoot, "sfdx-project.json"), `{
  "namespace": "BasePkg",
  "sourceApiVersion": "62.0",
  "packageDirectories": [{"path": "force-app", "default": true}]
}`)
	writeFile(t, fixture.localClass, "public class LocalService { public static String value() { return 'initial'; } }")
	writeFile(t, fixture.localTrigger, "trigger LocalTrigger on Account (before insert) {}")
	writeFile(t, fixture.dependencyClass, "global class StageService { global static String value() { return 'initial'; } }")
	writeFile(t, fixture.dependencyTrigger, "trigger StageTrigger on BasePkg__Ledger__c (before insert) { BasePkg.StageService.value(); }")
	if withDiagnostics {
		if err := os.Symlink(filepath.Join(consumerRoot, "missing-source.cls"), fixture.missingClass); err != nil {
			t.Fatal(err)
		}
	}

	writeFile(t, filepath.Join(consumerRoot, "force-app/main/default/objects/LocalLedger__c/LocalLedger__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Local Ledger</label><pluralLabel>Local Ledgers</pluralLabel><nameField><label>Local Ledger Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/default/objects/LocalFeature__mdt/LocalFeature__mdt.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Local Feature</label><pluralLabel>Local Features</pluralLabel><visibility>Public</visibility></CustomObject>`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/default/customMetadata/LocalFeature.Default.md-meta.xml"), `<CustomMetadata xmlns="http://soap.sforce.com/2006/04/metadata"><label>Default Feature</label><protected>false</protected><values><field>Enabled__c</field><value>true</value></values></CustomMetadata>`)

	artifact, err := packageartifact.BuildCaptured(packageartifact.BuildCapturedOptions{
		Namespace:        "artifactpkg",
		PackageName:      "Artifact Package",
		Version:          "3.1.0",
		SourceAPIVersion: "61.0",
		Capture: packageartifact.CaptureProvenance{
			Source:     "fixture",
			CapturedAt: time.Date(2026, time.July, 14, 12, 0, 0, 0, time.UTC),
		},
		ApexTypes: []packageartifact.ApexType{{
			Kind:       apexast.DeclarationClass,
			Name:       "ArtifactContract",
			File:       "artifact/ArtifactContract.cls",
			Namespace:  "artifactpkg",
			SourceRoot: "artifact",
			Version:    "3.1.0",
			Dependency: true,
			Modifiers:  []string{"global"},
		}},
		Objects: []schema.Object{{
			Name:   "artifactpkg__ArtifactLedger__c",
			Label:  "Artifact Ledger",
			Fields: []schema.Field{{Name: "artifactpkg__State__c", Type: "Text"}},
		}},
		CustomMetadataRecords: []schema.CustomMetadataRecord{{
			FullName:      "artifactpkg__ArtifactFeature__mdt.Default",
			ObjectName:    "artifactpkg__ArtifactFeature__mdt",
			DeveloperName: "Default",
			Label:         "Artifact Feature",
			File:          "artifact/ArtifactFeature.Default.md-meta.xml",
		}},
		LabelNames:          []string{"artifactpkg__Artifact_Label"},
		StaticResourceNames: []string{"artifactpkg__ArtifactAssets"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := packageartifact.WriteJSON(artifactPath, artifact); err != nil {
		t.Fatal(err)
	}

	initial := fixture.buildFresh(t)
	if initial.Project.Namespace != "localpkg" || initial.Project.SourceAPIVersion != "63.0" {
		t.Fatalf("project identity = %#v", initial.Project)
	}
	if len(initial.CustomMetadataRecords) < 2 || len(initial.CodeIntelSymbols) == 0 || len(initial.CodeIntelUses) == 0 {
		t.Fatalf("fixture did not produce rich metadata: customMetadata=%d symbols=%d uses=%d", len(initial.CustomMetadataRecords), len(initial.CodeIntelSymbols), len(initial.CodeIntelUses))
	}
	if withDiagnostics {
		if len(initial.Dependencies) != 3 || len(initial.Diagnostics) < 2 {
			t.Fatalf("fixture did not produce dependencies and missing-file diagnostic: dependencies=%#v diagnostics=%#v", initial.Dependencies, initial.Diagnostics)
		}
		missingFileDiagnostic := false
		filelessDependencyDiagnostic := false
		for _, diag := range initial.Diagnostics {
			missingFileDiagnostic = missingFileDiagnostic || cleanFilePath(diag.File) == cleanFilePath(fixture.missingClass)
			filelessDependencyDiagnostic = filelessDependencyDiagnostic || (diag.File == "" && diag.Code == "dependency_missing")
		}
		if !missingFileDiagnostic || !filelessDependencyDiagnostic {
			t.Fatalf("fixture diagnostics missingFile=%t filelessDependency=%t: %#v", missingFileDiagnostic, filelessDependencyDiagnostic, initial.Diagnostics)
		}
	} else if len(initial.Dependencies) != 2 || len(initial.Diagnostics) != 0 {
		t.Fatalf("clean fixture dependencies/diagnostics = %#v / %#v", initial.Dependencies, initial.Diagnostics)
	}
	stageTypeFound := false
	for _, typ := range initial.Types {
		if typ.Name != "StageService" {
			continue
		}
		stageTypeFound = typ.Namespace == "stagepkg" && typ.SourceRoot == fixture.dependencyRoot && typ.Version == "2.4.0" && typ.Dependency && len(typ.SourceNamespaceRemaps) == 1 && typ.SourceNamespaceRemaps[0].From == "BasePkg" && typ.SourceNamespaceRemaps[0].To == "stagepkg"
	}
	if !stageTypeFound {
		t.Fatalf("fixture did not preserve source-backed dependency identity: %#v", initial.Types)
	}
	stageTriggerFound := false
	for _, trigger := range initial.Triggers {
		if trigger.Name != "StageTrigger" {
			continue
		}
		stageTriggerFound = trigger.Namespace == "stagepkg" && trigger.ObjectName == "stagepkg__Ledger__c" && trigger.Dependency && len(trigger.SourceNamespaceRemaps) == 1 && trigger.SourceNamespaceRemaps[0].From == "BasePkg" && trigger.SourceNamespaceRemaps[0].To == "stagepkg"
	}
	if !stageTriggerFound {
		t.Fatalf("fixture did not preserve source-backed trigger identity: %#v", initial.Triggers)
	}
	return fixture
}

func (f *incrementalEquivalenceFixture) buildFresh(t *testing.T) Index {
	t.Helper()
	p, err := project.Load(f.consumerRoot)
	if err != nil {
		t.Fatal(err)
	}
	p.ApexFiles = append([]string(nil), f.localApexFiles...)
	artifactLoaded := false
	stageLoaded := false
	for i := range p.ManagedPackageDependencies {
		dep := &p.ManagedPackageDependencies[i]
		switch dep.Namespace {
		case "artifactpkg":
			artifactLoaded = dep.Status == "loaded" && dep.ArtifactPath != ""
		case "stagepkg":
			stageLoaded = dep.Status == "loaded" && dep.Project != nil
			if dep.Project != nil {
				dep.Project.ApexFiles = append([]string(nil), f.dependencyApexFiles...)
			}
		}
	}
	if !artifactLoaded || !stageLoaded {
		t.Fatalf("project.Load dependencies artifact=%t stage=%t: %#v", artifactLoaded, stageLoaded, p.ManagedPackageDependencies)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	return Build(p, s)
}

func (f *incrementalEquivalenceFixture) renameFile(t *testing.T, files *[]string, oldPath, newPath string) {
	t.Helper()
	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatal(err)
	}
	for i, path := range *files {
		if path == oldPath {
			(*files)[i] = newPath
			return
		}
	}
	t.Fatalf("rename source %q missing from explicit Apex file list", oldPath)
}

func (f *incrementalEquivalenceFixture) deleteFile(t *testing.T, files *[]string, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	for i, candidate := range *files {
		if candidate == path {
			*files = append((*files)[:i], (*files)[i+1:]...)
			return
		}
	}
	t.Fatalf("deleted path %q missing from explicit Apex file list", path)
}

func incrementalIndexMismatches(got, want Index) []string {
	var mismatches []string
	if !reflect.DeepEqual(got.Project, want.Project) {
		mismatches = append(mismatches, "Project")
	}
	if !reflect.DeepEqual(got.Types, want.Types) {
		mismatches = append(mismatches, "Types")
	}
	if !reflect.DeepEqual(got.Triggers, want.Triggers) {
		mismatches = append(mismatches, "Triggers")
	}
	if !reflect.DeepEqual(got.Objects, want.Objects) {
		mismatches = append(mismatches, "Objects")
	}
	if !reflect.DeepEqual(got.CustomMetadataRecords, want.CustomMetadataRecords) {
		mismatches = append(mismatches, "CustomMetadataRecords")
	}
	if !reflect.DeepEqual(got.CodeIntelSymbols, want.CodeIntelSymbols) {
		mismatches = append(mismatches, "CodeIntelSymbols")
	}
	if !reflect.DeepEqual(got.CodeIntelUses, want.CodeIntelUses) {
		mismatches = append(mismatches, "CodeIntelUses")
	}
	if !reflect.DeepEqual(got.Dependencies, want.Dependencies) {
		mismatches = append(mismatches, "Dependencies")
	}
	if !reflect.DeepEqual(got.Diagnostics, want.Diagnostics) {
		mismatches = append(mismatches, "Diagnostics")
	}
	return mismatches
}

func incrementalDormantStateMismatches(got, want Index) []string {
	var mismatches []string
	if !reflect.DeepEqual(got.Project, want.Project) {
		mismatches = append(mismatches, "Project")
	}
	if !reflect.DeepEqual(got.Objects, want.Objects) {
		mismatches = append(mismatches, "Objects")
	}
	if !reflect.DeepEqual(got.CustomMetadataRecords, want.CustomMetadataRecords) {
		mismatches = append(mismatches, "CustomMetadataRecords")
	}
	if !reflect.DeepEqual(got.CodeIntelSymbols, want.CodeIntelSymbols) {
		mismatches = append(mismatches, "CodeIntelSymbols")
	}
	if !reflect.DeepEqual(got.CodeIntelUses, want.CodeIntelUses) {
		mismatches = append(mismatches, "CodeIntelUses")
	}
	if !reflect.DeepEqual(got.Dependencies, want.Dependencies) {
		mismatches = append(mismatches, "Dependencies")
	}
	if !reflect.DeepEqual(got.Diagnostics, want.Diagnostics) {
		mismatches = append(mismatches, "Diagnostics")
	}
	return mismatches
}

func compareIncrementalTypeSymbolAtPath(t *testing.T, path string, got, want Index) {
	t.Helper()
	gotSymbol, gotOK := incrementalTypeSymbolAtPath(got, path)
	wantSymbol, wantOK := incrementalTypeSymbolAtPath(want, path)
	if !gotOK || !wantOK {
		t.Errorf("changed type path %q found in incremental=%t full=%t", path, gotOK, wantOK)
		return
	}
	if !reflect.DeepEqual(gotSymbol, wantSymbol) {
		t.Errorf("changed type symbol differs from full Build:\nincremental: %#v\nfull: %#v", gotSymbol, wantSymbol)
	}
}

func incrementalTypeSymbolAtPath(idx Index, path string) (TypeSymbol, bool) {
	wantPath := cleanFilePath(path)
	for _, symbol := range idx.Types {
		if cleanFilePath(symbol.File) == wantPath {
			return symbol, true
		}
	}
	return TypeSymbol{}, false
}

func compareIncrementalTriggerSymbolAtPath(t *testing.T, path string, got, want Index) {
	t.Helper()
	gotSymbol, gotOK := incrementalTriggerSymbolAtPath(got, path)
	wantSymbol, wantOK := incrementalTriggerSymbolAtPath(want, path)
	if !gotOK || !wantOK {
		t.Errorf("changed trigger path %q found in incremental=%t full=%t", path, gotOK, wantOK)
		return
	}
	if !reflect.DeepEqual(gotSymbol, wantSymbol) {
		t.Errorf("changed trigger symbol differs from full Build:\nincremental: %#v\nfull: %#v", gotSymbol, wantSymbol)
	}
}

func incrementalTriggerSymbolAtPath(idx Index, path string) (TriggerSymbol, bool) {
	wantPath := cleanFilePath(path)
	for _, symbol := range idx.Triggers {
		if cleanFilePath(symbol.File) == wantPath {
			return symbol, true
		}
	}
	return TriggerSymbol{}, false
}

func newIncrementalSourceDependencyProject(t *testing.T, dependencySource string) (consumerRoot, dependencyPath string) {
	t.Helper()
	root := t.TempDir()
	consumerRoot = filepath.Join(root, "consumer")
	dependencyRoot := filepath.Join(root, "base-source")
	dependencyPath = filepath.Join(dependencyRoot, "force-app/main/default/classes/StageService.cls")
	writeIncrementalSFDXProject(t, consumerRoot, "localpkg")
	writeIncrementalSFDXProject(t, dependencyRoot, "BasePkg")
	writeFile(t, filepath.Join(consumerRoot, "glade.yml"), `project:
  namespaceRemaps: ["BasePkg:stagepkg"]
  managedPackageDependencies: ["stagepkg:../base-source:2.4.0"]
`)
	writeFile(t, dependencyPath, dependencySource)
	return consumerRoot, dependencyPath
}

func newIncrementalDuplicateProject(t *testing.T) (root, earlierPath, laterPath string) {
	t.Helper()
	root = t.TempDir()
	writeIncrementalSFDXProject(t, root, "")
	earlierPath = filepath.Join(root, "force-app/main/default/classes/AFirst.cls")
	laterPath = filepath.Join(root, "force-app/main/default/classes/BSecond.cls")
	writeFile(t, earlierPath, "public class Shared {}")
	writeFile(t, laterPath, "public class Shared {}")
	return root, earlierPath, laterPath
}

func writeIncrementalSFDXProject(t *testing.T, root, namespace string) {
	t.Helper()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{
  "namespace": "`+namespace+`",
  "sourceApiVersion": "63.0",
  "packageDirectories": [{"path": "force-app", "default": true}]
}`)
}

func buildIncrementalIndexFromRoot(t *testing.T, root string) Index {
	t.Helper()
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	return Build(p, s)
}

func assertNoIncrementalDiagnosticCode(t *testing.T, idx Index, code string) {
	t.Helper()
	for _, diag := range idx.Diagnostics {
		if diag.Code == code {
			t.Errorf("stale diagnostic %s retained: %#v", code, diag)
		}
	}
}
