package gladecli

import (
	"path/filepath"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// Require person-family metadata and the captured business-default fallback in the restored org.
const dapCacheVersion = 18

func loadDAPStartupState(projectRoot string) (storage.OrgState, apextest.CompiledProjectRuntime, string, error) {
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
	runtime := apextest.CompileProjectRuntimeForRequest(index)
	if dapBeforePublishHook != nil {
		dapBeforePublishHook()
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

// dapBeforePublishHook runs after the build and before the freshness re-check.
// Tests use it to edit inputs mid-build.
var dapBeforePublishHook func()

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
