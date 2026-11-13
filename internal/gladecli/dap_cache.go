package gladecli

import (
	"context"
	"path/filepath"
	"sync"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// Reject entries that may pair a runtime with a later fingerprint.
const dapCacheVersion = 19

// normalizedDAPRoot is the absolute, cleaned, symlink-resolved project root.
// Load slots and publish locks are keyed on it so two spellings of one project
// share them.
func normalizedDAPRoot(projectRoot string) (string, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", err
	}
	root = filepath.Clean(root)
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	return root, nil
}

var dapPublishLocks sync.Map // normalized root -> *sync.Mutex

func dapPublishLock(root string) *sync.Mutex {
	lock, _ := dapPublishLocks.LoadOrStore(root, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func loadDAPStartupState(projectRoot string) (storage.OrgState, apextest.CompiledProjectRuntime, string, error) {
	org, runtime, apiVersion, _, err := loadDAPStartupStateWithIndex(context.Background(), projectRoot)
	return org, runtime, apiVersion, err
}

type dapLoadSeams struct {
 afterIndex func()
 beforePublish func()
}

func loadDAPStartupStateWithSeams(root string, seams dapLoadSeams) (storage.OrgState, apextest.CompiledProjectRuntime, string, error) {
 org, runtime, api, _, err := loadDAPStartupStateWithIndexAndSeams(context.Background(), root, seams)
 return org, runtime, api, err
}

// loadDAPStartupStateWithIndex also returns the project index when this call
// had to build it (a startup cache miss). On a cache hit the index is nil.
func loadDAPStartupStateWithIndex(ctx context.Context, projectRoot string) (storage.OrgState, apextest.CompiledProjectRuntime, string, *typesys.Index, error) {
 return loadDAPStartupStateWithIndexAndSeams(ctx, projectRoot, dapLoadSeams{})
}

func loadDAPStartupStateWithIndexAndSeams(ctx context.Context, projectRoot string, seams dapLoadSeams) (storage.OrgState, apextest.CompiledProjectRuntime, string, *typesys.Index, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return storage.OrgState{}, apextest.CompiledProjectRuntime{}, "", nil, err
	}
	root = filepath.Clean(root)
	cache, err := startupcache.Read(root, startupcache.SubdirDAP)
	if err != nil {
		return storage.OrgState{}, apextest.CompiledProjectRuntime{}, "", nil, err
	}
	if cache != nil && startupcache.Fresh(cache, root, dapCacheVersion) {
		return cache.Org, compiledRuntimeFromCache(cache.Runtime), cache.Manifest.SourceAPIVersion, nil, nil
	}
	before, beforeErr := startupcache.ValidateInputWithSourceDigests(root, nil)
 p, err := project.Load(root)
	if err != nil {
		return storage.OrgState{}, apextest.CompiledProjectRuntime{}, "", nil, err
	}
	index, artifacts := buildProjectIndexWithArtifacts(p)
	org, err := orgStateFromIndex(root, p, index)
	if err != nil {
		return storage.OrgState{}, apextest.CompiledProjectRuntime{}, "", nil, err
	}
	if err := ctx.Err(); err != nil {
		return storage.OrgState{}, apextest.CompiledProjectRuntime{}, "", nil, err
	}
 if seams.afterIndex != nil { seams.afterIndex() }
 runtime, err := apextest.CompileProjectRuntimeForRequestWithSourceDigests(index, artifacts.SourceDigests)
 if err != nil { return storage.OrgState{}, apextest.CompiledProjectRuntime{}, "", nil, err }
 if seams.beforePublish != nil { seams.beforePublish() }
 lockRoot, _ := normalizedDAPRoot(root)
 publish := dapPublishLock(lockRoot)
 publish.Lock()
 defer publish.Unlock()
 if err := ctx.Err(); err != nil { return storage.OrgState{}, apextest.CompiledProjectRuntime{}, "", nil, err }
 if beforeErr == nil {
  if built, ok := dapPublishableEntry(root, before, artifacts.SourceDigests, org, runtime); ok {
   _ = startupcache.Write(&built, startupcache.SubdirDAP)
  }
 }
 return org, runtime, p.SourceAPIVersion, &index, nil
}

// dapPublishableEntry binds the entry to the input proof taken before the
// build. It reports false when the inputs drifted, so no entry pairs an old
// runtime with a newer fingerprint.
func dapPublishableEntry(root string, before *startupcache.ValidatedInput, digests *typesys.SourceDigestSet, org storage.OrgState, runtime apextest.CompiledProjectRuntime) (startupcache.Entry, bool) {
	after, err := startupcache.ValidateInputWithSourceDigests(root, digests)
	if err != nil || after.Digest() != before.Digest() {
		return startupcache.Entry{}, false
	}
	entry, err := startupcache.NewEntryWithValidatedInput(before, org, compiledRuntimeToCache(runtime))
	if err != nil {
		return startupcache.Entry{}, false
	}
	entry.Version = dapCacheVersion
	return entry, true
}

func compiledRuntimeFromCache(runtime startupcache.CompiledRuntime) apextest.CompiledProjectRuntime {
	return apextest.CompiledProjectRuntime{
		Methods:   runtime.Methods,
		Classes:   apextest.RestoreLazyClasses(runtime.Classes),
		Triggers:  runtime.Triggers,
		PageNames: runtime.PageNames,
	}
}

func compiledRuntimeToCache(runtime apextest.CompiledProjectRuntime) startupcache.CompiledRuntime {
	return startupcache.CompiledRuntime{
		Methods:   runtime.Methods,
		Classes:   apextest.PersistableClasses(runtime.Classes),
		Triggers:  runtime.Triggers,
		PageNames: runtime.PageNames,
	}
}

func buildDAPStartupCache(projectRoot string, p project.Project, index typesys.Index, org storage.OrgState, runtime apextest.CompiledProjectRuntime) startupcache.Entry {
	entry := startupcache.NewEntry(projectRoot, p, index, org, compiledRuntimeToCache(runtime))
	entry.Version = dapCacheVersion
	return entry
}
