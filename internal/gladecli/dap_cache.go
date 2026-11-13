package gladecli

import (
	"path/filepath"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// Entries from version 18 and earlier may pair a runtime with a fingerprint
// taken after the inputs changed. Reject them.
const dapCacheVersion = 19

func loadDAPStartupState(projectRoot string) (storage.OrgState, apextest.CompiledProjectRuntime, string, error) {
	return loadDAPStartupStateWithSeams(projectRoot, dapLoadSeams{})
}

// dapLoadSeams lets tests change inputs mid-build. afterIndex runs after the
// index is built and before the runtime compiles. beforePublish runs after the
// build and before the freshness re-check.
type dapLoadSeams struct {
	afterIndex    func()
	beforePublish func()
}

func loadDAPStartupStateWithSeams(projectRoot string, seams dapLoadSeams) (storage.OrgState, apextest.CompiledProjectRuntime, string, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return storage.OrgState{}, apextest.CompiledProjectRuntime{}, "", err
	}
	root = filepath.Clean(root)
	cache, err := startupcache.Read(root, startupcache.SubdirDAP)
	if err != nil {
		return storage.OrgState{}, apextest.CompiledProjectRuntime{}, "", err
	}
	if cache != nil && startupcache.Fresh(cache, root, dapCacheVersion) {
		return cache.Org, compiledRuntimeFromCache(cache.Runtime), cache.Manifest.SourceAPIVersion, nil
	}
	// Hash every input before the build reads any of them. The entry is only
	// published when the same inputs are still on disk after the build.
	before, beforeErr := startupcache.ValidateInputWithSourceDigests(root, nil)
	p, err := project.Load(root)
	if err != nil {
		return storage.OrgState{}, apextest.CompiledProjectRuntime{}, "", err
	}
	index, artifacts := buildProjectIndexWithArtifacts(p)
	org, err := orgStateFromIndex(root, p, index)
	if err != nil {
		return storage.OrgState{}, apextest.CompiledProjectRuntime{}, "", err
	}
	if seams.afterIndex != nil {
		seams.afterIndex()
	}
	// Compile from the source bytes the index read, and fail closed rather
	// than publishing an empty runtime after a source read error.
	runtime, err := apextest.CompileProjectRuntimeForRequestWithSourceDigests(index, artifacts.SourceDigests)
	if err != nil {
		return storage.OrgState{}, apextest.CompiledProjectRuntime{}, "", err
	}
	if seams.beforePublish != nil {
		seams.beforePublish()
	}
	if beforeErr == nil {
		if built, ok := dapPublishableEntry(root, before, artifacts.SourceDigests, org, runtime); ok {
			// Caching is an optimization; keep serving from rebuilt state even
			// if cache persistence fails.
			_ = startupcache.Write(&built, startupcache.SubdirDAP)
		}
	}
	return org, runtime, p.SourceAPIVersion, nil
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
