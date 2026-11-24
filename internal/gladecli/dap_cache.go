package gladecli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/frontendcache"
	"github.com/glade-sh/glade/internal/inputview"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// Version 21 records full directory listings where loaders consume hidden
// metadata. Earlier entries do not prove those inputs.
const dapCacheVersion = 23

// dapLoadAttempts bounds the cold builds one startup makes while project
// inputs keep changing.
const dapLoadAttempts = 3

// dapRetryBackoff is the wait before attempts 2 and 3. Tests set it to zero.
var dapRetryBackoff = []time.Duration{100 * time.Millisecond, 400 * time.Millisecond}

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

// dapLoadSeams are test hooks inside one cold build. Each receives the
// 1-based attempt number.
type dapLoadSeams struct {
	beforeLoad     func(attempt int)
	beforeFrontend func(attempt int)
	afterFrontend  func(attempt int)
	afterIndex     func(attempt int)
	beforeRecheck  func(attempt int)
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

// DAPInputsChangingError reports that project inputs kept changing through
// every cold build attempt. It unwraps to the last change seen.
type DAPInputsChangingError struct {
	Root     string
	Attempts int
	Last     error
}

func (e *DAPInputsChangingError) Error() string {
	path, what := "", "file content"
	var changed *inputview.ChangedError
	var mismatch *apextest.SourceSnapshotMismatchError
	switch {
	case errors.As(e.Last, &changed):
		path, what = changed.Path, changed.What
	case errors.As(e.Last, &mismatch):
		path = mismatch.File
	}
	if rel, err := filepath.Rel(e.Root, path); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		path = filepath.ToSlash(rel)
	}
	return fmt.Sprintf("project files changed while the debugger was starting: %s (%s).\nGlade retried %d times and the project was still changing. Wait for the save, checkout or sync to finish, then start debugging again.", path, what, e.Attempts)
}

func (e *DAPInputsChangingError) Unwrap() error { return e.Last }

func loadDAPStartupStateWithIndexAndSeams(ctx context.Context, projectRoot string, seams dapLoadSeams) (storage.OrgState, apextest.CompiledProjectRuntime, string, *typesys.Index, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return storage.OrgState{}, apextest.CompiledProjectRuntime{}, "", nil, err
	}
	root = filepath.Clean(root)
	header, digest, err := startupcache.ReadDAPManifest(root)
	if err != nil {
		return storage.OrgState{}, apextest.CompiledProjectRuntime{}, "", nil, err
	}
	if header != nil && startupcache.Fresh(header, root, dapCacheVersion) && dapObservationsHold(root, header) {
		cache, err := startupcache.ReadDAPVerified(root, digest)
		if err != nil {
			return storage.OrgState{}, apextest.CompiledProjectRuntime{}, "", nil, err
		}
		if cache != nil && startupcache.Fresh(cache, root, dapCacheVersion) && dapObservationsHold(root, cache) {
			return cache.Org, compiledRuntimeFromCache(cache.Runtime), cache.Manifest.SourceAPIVersion, nil, nil
		}
	}
	var last error
	for attempt := 1; attempt <= dapLoadAttempts; attempt++ {
		if attempt > 1 {
			if err := dapRetryWait(ctx, attempt); err != nil {
				return storage.OrgState{}, apextest.CompiledProjectRuntime{}, "", nil, err
			}
		}
		built, changed, err := buildDAPStateOnce(ctx, root, seams, attempt)
		if changed != nil {
			last = changed
			continue
		}
		if err != nil {
			return storage.OrgState{}, apextest.CompiledProjectRuntime{}, "", nil, err
		}
		lockRoot, _ := normalizedDAPRoot(root)
		publish := dapPublishLock(lockRoot)
		publish.Lock()
		if err := ctx.Err(); err != nil {
			publish.Unlock()
			return storage.OrgState{}, apextest.CompiledProjectRuntime{}, "", nil, err
		}
		// Publication is best effort; the verified state is served either way.
		_ = startupcache.Write(&built.entry, startupcache.SubdirDAP)
		publish.Unlock()
		return built.org, built.runtime, built.apiVersion, &built.index, nil
	}
	return storage.OrgState{}, apextest.CompiledProjectRuntime{}, "", nil, &DAPInputsChangingError{Root: root, Attempts: dapLoadAttempts, Last: last}
}

func dapRetryWait(ctx context.Context, attempt int) error {
	var wait time.Duration
	if i := attempt - 2; i >= 0 && i < len(dapRetryBackoff) {
		wait = dapRetryBackoff[i]
	}
	if wait <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type dapBuiltState struct {
	org        storage.OrgState
	runtime    apextest.CompiledProjectRuntime
	apiVersion string
	index      typesys.Index
	entry      startupcache.Entry
}

// buildDAPStateOnce makes one cold build from one input view. changed is set
// when the inputs moved during the attempt; the caller then discards the
// attempt, loader error included, and retries.
func buildDAPStateOnce(ctx context.Context, root string, seams dapLoadSeams, attempt int) (dapBuiltState, error, error) {
	v := inputview.New()
	defer v.Close()
	if seams.beforeLoad != nil {
		seams.beforeLoad(attempt)
	}
	built, err := buildDAPStateFromView(ctx, root, v, seams, attempt)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return dapBuiltState{}, nil, ctxErr
	}
	if seams.beforeRecheck != nil {
		seams.beforeRecheck(attempt)
	}
	changed := v.Err()
	if changed == nil {
		changed = v.Recheck()
	}
	var mismatch *apextest.SourceSnapshotMismatchError
	if changed == nil && errors.As(err, &mismatch) {
		changed = err
	}
	if changed != nil {
		return dapBuiltState{}, changed, nil
	}
	if err != nil {
		return dapBuiltState{}, nil, err
	}
	built.entry.Inputs = v.Observations()
	if changed := v.Err(); changed != nil {
		return dapBuiltState{}, changed, nil
	}
	return built, nil, nil
}

func buildDAPStateFromView(ctx context.Context, root string, v *inputview.View, seams dapLoadSeams, attempt int) (dapBuiltState, error) {
	p, err := project.LoadWithInputs(root, v)
	if err != nil {
		return dapBuiltState{}, err
	}
	var index typesys.Index
	var artifacts typesys.BuildArtifacts
	s, schemaErr := schema.LoadProject(p)
	if seams.beforeFrontend != nil {
		seams.beforeFrontend(attempt)
	}
	if schemaErr == nil && apextest.ResolveDiskRuntimeCachePolicy(apextest.Options{}).Enabled {
		result := frontendcache.LoadOrBuild(p, s, func() (typesys.Index, typesys.BuildArtifacts) { return typesys.BuildWithArtifacts(p, s) })
		index, artifacts = result.Index, result.Artifacts
	} else {
		index, artifacts = buildProjectIndexWithArtifacts(p)
	}
	if seams.afterFrontend != nil {
		seams.afterFrontend(attempt)
	}
	org, err := orgStateFromIndex(root, p, index)
	if err != nil {
		return dapBuiltState{}, err
	}
	if err := ctx.Err(); err != nil {
		return dapBuiltState{}, err
	}
	if seams.afterIndex != nil {
		seams.afterIndex(attempt)
	}
	if err := v.CheckDirectories(); err != nil {
		return dapBuiltState{}, err
	}
	runtime, err := apextest.CompileProjectRuntimeForProject(index, p, artifacts)
	if err != nil {
		return dapBuiltState{}, err
	}
	// The manifest comes from the same project and view, not a second load.
	proof, err := startupcache.ValidateLoadedInput(root, p, artifacts.SourceDigests)
	if err != nil {
		return dapBuiltState{}, err
	}
	entry, err := startupcache.NewEntryWithValidatedInput(proof, org, compiledRuntimeToCache(runtime))
	if err != nil {
		return dapBuiltState{}, err
	}
	entry.Version = dapCacheVersion
	return dapBuiltState{org: org, runtime: runtime, apiVersion: p.SourceAPIVersion, index: index, entry: entry}, nil
}

// dapObservationsHold re-observes the inputs a version 21 entry was built
// from. Fresh has already hashed the manifest files, so their file
// observations are skipped; everything else, including directory listings and
// optional files found absent, is checked again.
func dapObservationsHold(root string, entry *startupcache.Entry) bool {
	if len(entry.Inputs) == 0 {
		return false
	}
	verified := make(map[string]bool, len(entry.Manifest.Files)+len(entry.Manifest.ConfigFiles))
	for _, files := range [][]startupcache.File{entry.Manifest.Files, entry.Manifest.ConfigFiles} {
		for _, file := range files {
			verified[filepath.Clean(filepath.Join(root, filepath.FromSlash(file.Path)))] = true
		}
	}
	return inputview.Verify(entry.Inputs, func(o inputview.Observation) bool {
		return (o.Kind == "file" || o.Kind == "stat") && o.Exists && o.Err == "" && verified[o.Path]
	}) == nil
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
