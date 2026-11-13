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

// Require person-family metadata and the captured business-default fallback in the restored org.
const dapCacheVersion = 18

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

// loadDAPStartupStateWithIndex also returns the project index when this call
// had to build it (a startup cache miss). On a cache hit the index is nil.
func loadDAPStartupStateWithIndex(ctx context.Context, projectRoot string) (storage.OrgState, apextest.CompiledProjectRuntime, string, *typesys.Index, error) {
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
	p, index, err := loadProjectIndex(root)
	if err != nil {
		return storage.OrgState{}, apextest.CompiledProjectRuntime{}, "", nil, err
	}
	org, err := orgStateFromIndex(root, p, index)
	if err != nil {
		return storage.OrgState{}, apextest.CompiledProjectRuntime{}, "", nil, err
	}
	if err := ctx.Err(); err != nil {
		return storage.OrgState{}, apextest.CompiledProjectRuntime{}, "", nil, err
	}
	runtime := apextest.CompileProjectRuntimeForRequest(index)
	built := startupcache.NewEntry(root, p, index, org, compiledRuntimeToCache(runtime))
	built.Version = dapCacheVersion
	// A load that was cancelled before it got here skips the write, so an
	// abandoned build does not refresh the cache. This only narrows the old
	// window: NewEntry still fingerprints the files when it runs, so a file
	// edited between indexing and fingerprinting can pair old compiled state
	// with a new hash. That predates cancellation and is not fixed here; it needs
	// the fingerprint taken before the build and compared after it.
	lockRoot, _ := normalizedDAPRoot(root)
	publish := dapPublishLock(lockRoot)
	publish.Lock()
	defer publish.Unlock()
	if err := ctx.Err(); err != nil {
		return storage.OrgState{}, apextest.CompiledProjectRuntime{}, "", nil, err
	}
	if err := startupcache.Write(&built, startupcache.SubdirDAP); err != nil {
		// Caching is an optimization; keep serving from rebuilt state even if
		// cache persistence fails.
	}
	return built.Org, runtime, p.SourceAPIVersion, &index, nil
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
