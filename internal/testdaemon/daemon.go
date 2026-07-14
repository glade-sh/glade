package testdaemon

import (
	"context"
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
	updateMu      sync.Mutex
	index         typesys.Index
	graph         *watch.RefGraph
	updateIndexFn func(typesys.Index, []string, []string) (typesys.Index, error)
	loadIndexFn   func(string) (typesys.Index, error)
}

func New(root string) (*Daemon, error) {
	index, err := loadIndex(root)
	if err != nil {
		return nil, err
	}
	return &Daemon{
		root:          root,
		index:         index,
		graph:         watch.BuildReferenceGraph(index),
		updateIndexFn: typesys.UpdateApexFilesChecked,
		loadIndexFn:   loadIndex,
	}, nil
}

func (d *Daemon) Root() string {
	return d.root
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
	selectedOpts, ok := optionsForSelection(opts, selection)
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
	selectedOpts, ok := optionsForSelection(opts, selection)
	if !ok {
		return testreport.Run{Name: "glade test"}, selection, nil
	}
	return apextest.Run(index, selectedOpts), selection, nil
}

func (d *Daemon) SelectAffected(changes []watch.Change) watch.TestSelection {
	d.mu.RLock()
	index := d.index
	graph := d.graph
	d.mu.RUnlock()
	return watch.SelectAffectedTestsWithRefGraph(index, changes, graph)
}

func optionsForSelection(opts apextest.Options, selection watch.TestSelection) (apextest.Options, bool) {
	switch selection.Mode {
	case watch.SelectionNone:
		return opts, false
	case watch.SelectionAll:
		return opts, true
	case watch.SelectionDirect:
		selectedClasses := selectedClassesForDirectSelection(opts.SelectedClasses, selection.TestClasses)
		if len(selectedClasses) == 0 {
			return opts, false
		}
		opts.SelectedClasses = selectedClasses
		return opts, true
	default:
		return opts, true
	}
}

func selectedClassesForDirectSelection(existing, affected []string) []string {
	affectedSet := make(map[string]string, len(affected))
	for _, className := range affected {
		className = strings.TrimSpace(className)
		if className == "" {
			continue
		}
		affectedSet[strings.ToLower(className)] = className
	}
	if len(affectedSet) == 0 {
		return nil
	}
	if len(existing) == 0 {
		out := make([]string, 0, len(affectedSet))
		for _, className := range affected {
			className = strings.TrimSpace(className)
			if className != "" {
				out = append(out, className)
			}
		}
		return out
	}
	out := make([]string, 0, len(existing))
	for _, className := range existing {
		className = strings.TrimSpace(className)
		if className == "" {
			continue
		}
		if _, ok := affectedSet[strings.ToLower(className)]; ok {
			out = append(out, className)
		}
	}
	return out
}

func (d *Daemon) UpdateChanges(changes []watch.Change) error {
	d.updateMu.Lock()
	defer d.updateMu.Unlock()
	if !canIncrementalIndex(changes) {
		return d.reloadLocked()
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
	d.mu.RUnlock()
	index, err := d.updateIndexFn(previous, changed, deleted)
	if err != nil {
		return err
	}
	graph := watch.BuildReferenceGraph(index)
	d.mu.Lock()
	d.index = index
	d.graph = graph
	d.mu.Unlock()
	return nil
}

func (d *Daemon) Reload() error {
	d.updateMu.Lock()
	defer d.updateMu.Unlock()
	return d.reloadLocked()
}

func (d *Daemon) reloadLocked() error {
	index, err := d.loadIndexFn(d.root)
	if err != nil {
		return err
	}
	graph := watch.BuildReferenceGraph(index)
	d.mu.Lock()
	d.index = index
	d.graph = graph
	d.mu.Unlock()
	return nil
}

func loadIndex(root string) (typesys.Index, error) {
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	p, err := project.Load(root)
	if err != nil {
		return typesys.Index{}, err
	}
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
