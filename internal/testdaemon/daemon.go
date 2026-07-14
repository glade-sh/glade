package testdaemon

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/watch"
)

type Daemon struct {
	root string
	mu   sync.RWMutex
	// updateMu serializes every index/graph writer. A writer snapshots after
	// acquiring it, so no other writer can publish before that snapshot is
	// replaced and no generation retry is necessary.
	updateMu         sync.Mutex
	project          project.Project
	index            typesys.Index
	graph            *watch.RefGraph
	tryUpdateIndexFn func(typesys.Index, []string, []string, project.Project) (typesys.Index, bool, error)
	loadProjectFn    func(string) (project.Project, error)
	buildIndexFn     func(project.Project) (typesys.Index, error)
	captureScopeFn   func(watch.Scope) (watch.Snapshot, error)
	refreshGraphFn   func(*watch.RefGraph, typesys.Index, []watch.Change) (*watch.RefGraph, error)
}

// WatchStateDriftError reports an ordinary input change while a watcher and
// its authoritative project state are being prepared.
type WatchStateDriftError struct {
	Path string
}

func (e *WatchStateDriftError) Error() string {
	if e.Path == "" {
		return "watch project scope changed while preparing stable state"
	}
	return fmt.Sprintf("watch inputs changed while preparing stable state: %s", e.Path)
}

func New(root string) (*Daemon, error) {
	p, index, err := loadProjectState(root)
	if err != nil {
		return nil, err
	}
	return &Daemon{
		root:             root,
		project:          p,
		index:            index,
		graph:            watch.BuildReferenceGraph(index),
		tryUpdateIndexFn: typesys.TryUpdateApexFilesCheckedWithLoadedProject,
		loadProjectFn:    loadProject,
		buildIndexFn:     buildProjectIndex,
		captureScopeFn:   watch.CaptureScope,
		refreshGraphFn: func(graph *watch.RefGraph, index typesys.Index, changes []watch.Change) (*watch.RefGraph, error) {
			return graph.Refreshed(index, changes), nil
		},
	}, nil
}

func (d *Daemon) Root() string {
	return d.root
}

// IndexSnapshot returns the index that is currently published by the daemon.
// Published indices are immutable; updates replace the complete value.
func (d *Daemon) IndexSnapshot() typesys.Index {
	d.mu.RLock()
	index := d.index
	d.mu.RUnlock()
	return index
}

// WatchScopeSnapshot returns an owned scope derived from the project paired
// with the currently published index and graph.
func (d *Daemon) WatchScopeSnapshot(requestedRoot string) watch.Scope {
	d.mu.RLock()
	scope := watch.ProjectScope(requestedRoot, d.project)
	d.mu.RUnlock()
	return scope
}

func (d *Daemon) Warm() {
	d.mu.RLock()
	index := d.index
	d.mu.RUnlock()
	apextest.WarmRuntime(index)
}

func (d *Daemon) RunFilter(filter string) testreport.Run {
	return d.RunOptions(apextest.Options{Filter: filter})
}

func (d *Daemon) RunOptions(opts apextest.Options) testreport.Run {
	d.mu.RLock()
	index := d.index
	d.mu.RUnlock()
	return apextest.Run(index, opts)
}

func (d *Daemon) RunSelectionContext(ctx context.Context, opts apextest.Options, selection watch.TestSelection) testreport.Run {
	d.mu.RLock()
	index := d.index
	d.mu.RUnlock()
	selectedOpts, ok := watch.ApplyTestSelection(opts, selection)
	if !ok {
		return testreport.Run{Name: "glade test"}
	}
	return apextest.RunContext(ctx, index, selectedOpts)
}

func (d *Daemon) RunChangedSince(ref string) (testreport.Run, watch.TestSelection, error) {
	return d.RunChangedSinceOptions(ref, apextest.Options{})
}

func (d *Daemon) RunChangedSinceOptions(ref string, opts apextest.Options) (testreport.Run, watch.TestSelection, error) {
	changes, err := watch.GitChangesSince(d.root, ref)
	if err != nil {
		return testreport.Run{}, watch.TestSelection{}, err
	}
	d.mu.RLock()
	index := d.index
	graph := d.graph
	d.mu.RUnlock()
	selection := watch.SelectAffectedTestsWithRefGraph(index, changes, graph)
	selectedOpts, ok := watch.ApplyTestSelection(opts, selection)
	if !ok {
		return testreport.Run{Name: "glade test"}, selection, nil
	}
	return apextest.Run(index, selectedOpts), selection, nil
}

func (d *Daemon) SelectAffected(changes []watch.Change) watch.TestSelection {
	_, selection := d.SnapshotSelection(changes)
	return selection
}

// SnapshotSelection captures the published index and computes the selection
// from the graph paired with that exact index. Watch callers retain the index
// until the selected run starts so a later update cannot change its meaning.
func (d *Daemon) SnapshotSelection(changes []watch.Change) (typesys.Index, watch.TestSelection) {
	d.mu.RLock()
	index := d.index
	graph := d.graph
	d.mu.RUnlock()
	return index, watch.SelectAffectedTestsWithRefGraph(index, changes, graph)
}

func optionsForSelection(opts apextest.Options, selection watch.TestSelection) (apextest.Options, bool) {
	return watch.ApplyTestSelection(opts, selection)
}

func (d *Daemon) UpdateChanges(changes []watch.Change) error {
	d.updateMu.Lock()
	defer d.updateMu.Unlock()
	exact, err := d.tryUpdateChangesLocked(changes, nil, true)
	if err != nil {
		return err
	}
	if !exact {
		return d.reloadLocked(nil)
	}
	return nil
}

// TryUpdateChanges publishes an exact incremental update or one authoritative
// rebuild that preserves currentScope. False means the watch owner must
// prepare a replacement watcher because the project scope changed.
func (d *Daemon) TryUpdateChanges(ctx context.Context, changes []watch.Change, currentScope watch.Scope) (bool, error) {
	d.updateMu.Lock()
	defer d.updateMu.Unlock()
	allowAuthoritativeGraphRefresh := true
	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		reusable, err := d.tryUpdateChangesLocked(changes, &currentScope, allowAuthoritativeGraphRefresh)
		var drift *WatchStateDriftError
		if errors.As(err, &drift) {
			allowAuthoritativeGraphRefresh = false
			continue
		}
		return reusable, err
	}
}

func (d *Daemon) tryUpdateChangesLocked(changes []watch.Change, currentScope *watch.Scope, allowAuthoritativeGraphRefresh bool) (bool, error) {
	if !canIncrementalIndex(changes) {
		return false, nil
	}
	var changed []string
	var deleted []string
	for _, change := range changes {
		switch change.Op {
		case watch.ChangeDeleted:
			deleted = append(deleted, change.Path)
		default:
			changed = append(changed, change.Path)
		}
	}
	d.mu.RLock()
	previous := d.index
	previousGraph := d.graph
	d.mu.RUnlock()
	var baseline watch.Snapshot
	baselineCaptured := false
	if currentScope != nil && typesys.RequiresAuthoritativeApexRebuild(previous, changed, deleted) {
		var err error
		baseline, err = d.captureScopeFn(*currentScope)
		if err != nil {
			return false, err
		}
		baselineCaptured = true
	}
	p, err := d.loadProjectFn(d.root)
	if err != nil {
		return false, err
	}
	index, exact, err := d.tryUpdateIndexFn(previous, changed, deleted, p)
	if err != nil {
		return false, err
	}
	var graph *watch.RefGraph
	if !exact {
		if currentScope != nil {
			if !baselineCaptured {
				baseline, err = d.captureScopeFn(*currentScope)
				if err != nil {
					return false, err
				}
				p, err = d.loadProjectFn(d.root)
				if err != nil {
					return false, err
				}
			}
		}
		if !typesys.MatchesProjectIdentity(previous, p) {
			return false, nil
		}
		if currentScope != nil {
			candidateScope := watch.ProjectScopeWithPrevious(d.root, p, *currentScope)
			if !reflect.DeepEqual(candidateScope, *currentScope) {
				return false, nil
			}
		}
		index, err = d.buildIndexFn(p)
		if err != nil {
			return false, err
		}
		graphChanges, digestCoverage := watch.AuthoritativeApexGraphChanges(previous, index)
		if allowAuthoritativeGraphRefresh && digestCoverage && watch.CanRefreshAuthoritativeFallbackGraph(previous, index, graphChanges) {
			graph, err = d.refreshGraphFn(previousGraph, index, graphChanges)
			if err != nil {
				return false, err
			}
		} else {
			graph = watch.BuildReferenceGraph(index)
		}
		if currentScope != nil {
			proof, captureErr := d.captureScopeFn(*currentScope)
			if captureErr != nil {
				return false, captureErr
			}
			if drift := watch.DiffSnapshots(baseline, proof); len(drift) != 0 {
				return false, &WatchStateDriftError{Path: drift[0].Path}
			}
		}
	}
	if exact {
		graph = watch.BuildReferenceGraph(index)
	}
	d.mu.Lock()
	d.project = p
	d.index = index
	d.graph = graph
	d.mu.Unlock()
	return true, nil
}

func (d *Daemon) Reload() error {
	d.updateMu.Lock()
	defer d.updateMu.Unlock()
	return d.reloadLocked(nil)
}

// ReloadPreparedStable registers a candidate watcher from one baseline, then
// reloads and builds the authoritative state behind that watcher. It publishes
// only when the scope and post-build snapshot still match the baseline.
func (d *Daemon) ReloadPreparedStable(previous watch.Scope, capture func(watch.Scope) (watch.Snapshot, error), prepare func(project.Project, watch.Scope, watch.Snapshot) error) error {
	d.updateMu.Lock()
	defer d.updateMu.Unlock()
	if capture == nil {
		capture = watch.CaptureScope
	}
	scopeProject, err := d.loadProjectFn(d.root)
	if err != nil {
		return err
	}
	scope := watch.ProjectScopeWithPrevious(d.root, scopeProject, previous)
	baseline, err := capture(scope)
	if err != nil {
		return err
	}
	if prepare != nil {
		if err := prepare(scopeProject, scope, baseline); err != nil {
			return err
		}
	}
	p, err := d.loadProjectFn(d.root)
	if err != nil {
		return err
	}
	if authoritativeScope := watch.ProjectScopeWithPrevious(d.root, p, previous); !reflect.DeepEqual(scope, authoritativeScope) {
		return &WatchStateDriftError{}
	}
	index, err := d.buildIndexFn(p)
	if err != nil {
		return err
	}
	graph := watch.BuildReferenceGraph(index)
	proof, err := capture(scope)
	if err != nil {
		return err
	}
	if changes := watch.DiffSnapshots(baseline, proof); len(changes) != 0 {
		return &WatchStateDriftError{Path: changes[0].Path}
	}
	d.mu.Lock()
	d.project = p
	d.index = index
	d.graph = graph
	d.mu.Unlock()
	return nil
}

func (d *Daemon) reloadLocked(prepare func(project.Project) error) error {
	p, err := d.loadProjectFn(d.root)
	if err != nil {
		return err
	}
	if prepare != nil {
		if err := prepare(p); err != nil {
			return err
		}
	}
	index, err := d.buildIndexFn(p)
	if err != nil {
		return err
	}
	graph := watch.BuildReferenceGraph(index)
	d.mu.Lock()
	d.project = p
	d.index = index
	d.graph = graph
	d.mu.Unlock()
	return nil
}

func loadIndex(root string) (typesys.Index, error) {
	_, index, err := loadProjectState(root)
	return index, err
}

func loadProjectState(root string) (project.Project, typesys.Index, error) {
	p, err := loadProject(root)
	if err != nil {
		return project.Project{}, typesys.Index{}, err
	}
	index, err := buildProjectIndex(p)
	return p, index, err
}

func loadProject(root string) (project.Project, error) {
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	p, err := project.Load(root)
	if err != nil {
		return project.Project{}, err
	}
	return p, nil
}

func buildProjectIndex(p project.Project) (typesys.Index, error) {
	s, err := schema.LoadProject(p)
	if err != nil {
		return typesys.Index{}, err
	}
	return typesys.Build(p, s), nil
}

func canIncrementalIndex(changes []watch.Change) bool {
	if len(changes) == 0 {
		return false
	}
	for _, change := range changes {
		switch change.Kind {
		case watch.FileKindApexClass, watch.FileKindApexTrigger:
		default:
			return false
		}
	}
	return true
}
