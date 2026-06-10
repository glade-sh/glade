package gladecli

import (
	"path/filepath"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

const dapCacheVersion = 3

func loadDAPStartupState(projectRoot string) (storage.OrgState, apextest.CompiledProjectRuntime, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return storage.OrgState{}, apextest.CompiledProjectRuntime{}, err
	}
	root = filepath.Clean(root)
	cache, err := startupcache.Read(root, startupcache.SubdirDAP)
	if err != nil {
		return storage.OrgState{}, apextest.CompiledProjectRuntime{}, err
	}
	if cache != nil && startupcache.Fresh(cache, root, dapCacheVersion) {
		return cache.Org, compiledRuntimeFromCache(cache.Runtime), nil
	}
	p, index, err := loadProjectIndex(root)
	if err != nil {
		return storage.OrgState{}, apextest.CompiledProjectRuntime{}, err
	}
	org := orgStateFromIndex(root, p, index)
	runtime := apextest.CompileProjectRuntimeForRequest(index)
	built := startupcache.NewEntry(root, p, index, org, compiledRuntimeToCache(runtime))
	built.Version = dapCacheVersion
	if err := startupcache.Write(&built, startupcache.SubdirDAP); err != nil {
		// Caching is an optimization; keep serving from rebuilt state even if
		// cache persistence fails.
	}
	return built.Org, runtime, nil
}

func compiledRuntimeFromCache(runtime startupcache.CompiledRuntime) apextest.CompiledProjectRuntime {
	return apextest.CompiledProjectRuntime{
		Methods:   runtime.Methods,
		Classes:   runtime.Classes,
		Triggers:  runtime.Triggers,
		PageNames: runtime.PageNames,
	}
}

func compiledRuntimeToCache(runtime apextest.CompiledProjectRuntime) startupcache.CompiledRuntime {
	return startupcache.CompiledRuntime{
		Methods:   runtime.Methods,
		Classes:   runtime.Classes,
		Triggers:  runtime.Triggers,
		PageNames: runtime.PageNames,
	}
}

func buildDAPStartupCache(projectRoot string, p project.Project, index typesys.Index, org storage.OrgState, runtime apextest.CompiledProjectRuntime) startupcache.Entry {
	entry := startupcache.NewEntry(projectRoot, p, index, org, compiledRuntimeToCache(runtime))
	entry.Version = dapCacheVersion
	return entry
}
