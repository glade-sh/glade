package testdaemon

import (
	"context"
	"strings"
	"sync"

	"github.com/open-aer/oaer/internal/apextest"
	"github.com/open-aer/oaer/internal/project"
	"github.com/open-aer/oaer/internal/schema"
	"github.com/open-aer/oaer/internal/testreport"
	"github.com/open-aer/oaer/internal/typesys"
	"github.com/open-aer/oaer/internal/watch"
)

type Daemon struct {
	root  string
	mu    sync.RWMutex
	index typesys.Index
}

func New(root string) (*Daemon, error) {
	index, err := loadIndex(root)
	if err != nil {
		return nil, err
	}
	return &Daemon{root: root, index: index}, nil
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
	if selection.Mode == watch.SelectionDirect && len(selection.TestClasses) == 1 {
		opts.Filter = selection.TestClasses[0]
	}
	if selection.Mode == watch.SelectionNone {
		return testreport.Run{Name: "oaer test"}
	}
	return apextest.RunContext(ctx, index, opts)
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
	d.mu.RUnlock()
	selection := watch.SelectAffectedTests(index, changes)
	if selection.Mode == watch.SelectionDirect && len(selection.TestClasses) == 1 {
		opts.Filter = selection.TestClasses[0]
	}
	if selection.Mode == watch.SelectionNone {
		return testreport.Run{Name: "oaer test"}, selection, nil
	}
	return apextest.Run(index, opts), selection, nil
}

func (d *Daemon) SelectAffected(changes []watch.Change) watch.TestSelection {
	d.mu.RLock()
	index := d.index
	d.mu.RUnlock()
	return watch.SelectAffectedTests(index, changes)
}

func (d *Daemon) UpdateChanges(changes []watch.Change) error {
	if !canIncrementalIndex(changes) {
		return d.Reload()
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
	d.mu.Lock()
	d.index = typesys.UpdateApexFiles(d.index, changed, deleted)
	d.mu.Unlock()
	return nil
}

func (d *Daemon) Reload() error {
	index, err := loadIndex(d.root)
	if err != nil {
		return err
	}
	d.mu.Lock()
	d.index = index
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
