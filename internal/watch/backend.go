package watch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/fsnotify/fsnotify"
)

type BackendWatcher interface {
	Changes() <-chan []Change
	Errors() <-chan error
	Close() error
}

func NewBackendWatcher(ctx context.Context, cfg Config, initial Snapshot) (BackendWatcher, Backend, error) {
	cfg = cfg.Normalized()
	switch cfg.Backend {
	case BackendPoll:
		return NewPollingWatcher(ctx, cfg, initial), BackendPoll, nil
	case BackendNative, BackendAuto:
		watcher, err := NewNativeWatcher(ctx, cfg, initial)
		if err == nil {
			return watcher, BackendNative, nil
		}
		if cfg.Backend == BackendNative {
			return nil, "", err
		}
		return NewPollingWatcher(ctx, cfg, initial), BackendPoll, nil
	default:
		return nil, "", fmt.Errorf("unknown watch backend %q", cfg.Backend)
	}
}

type PollingWatcher struct {
	cancel  context.CancelFunc
	changes chan []Change
	errors  chan error
}

func NewPollingWatcher(ctx context.Context, cfg Config, initial Snapshot) *PollingWatcher {
	ctx, cancel := context.WithCancel(ctx)
	w := &PollingWatcher{
		cancel:  cancel,
		changes: make(chan []Change, 1),
		errors:  make(chan error, 1),
	}
	go w.run(ctx, cfg.Normalized(), initial)
	return w
}

func (w *PollingWatcher) Changes() <-chan []Change { return w.changes }
func (w *PollingWatcher) Errors() <-chan error     { return w.errors }

func (w *PollingWatcher) Close() error {
	w.cancel()
	return nil
}

func (w *PollingWatcher) run(ctx context.Context, cfg Config, previous Snapshot) {
	ticker := newTicker(cfg.Debounce)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			close(w.changes)
			close(w.errors)
			return
		case <-ticker.C:
			current, err := CaptureScope(cfg.Scope)
			if err != nil {
				sendWatchError(ctx, w.errors, err)
				continue
			}
			changes := DiffSnapshots(previous, current)
			previous = current
			if len(changes) > 0 {
				sendWatchChanges(ctx, w.changes, changes)
			}
		}
	}
}

type NativeWatcher struct {
	cancel  context.CancelFunc
	inner   *fsnotify.Watcher
	changes chan []Change
	errors  chan error
}

func NewNativeWatcher(ctx context.Context, cfg Config, initial Snapshot) (*NativeWatcher, error) {
	inner, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	cfg = cfg.Normalized()
	if err := addScopeWatchDirs(inner, cfg.Scope); err != nil {
		_ = inner.Close()
		return nil, err
	}
	reconciled, err := CaptureScope(cfg.Scope)
	if err != nil {
		_ = inner.Close()
		return nil, err
	}
	reconciliationChanges := DiffSnapshots(initial, reconciled)
	ctx, cancel := context.WithCancel(ctx)
	w := &NativeWatcher{
		cancel:  cancel,
		inner:   inner,
		changes: make(chan []Change, 1),
		errors:  make(chan error, 1),
	}
	if len(reconciliationChanges) > 0 {
		w.changes <- reconciliationChanges
	}
	go w.run(ctx, cfg, reconciled)
	return w, nil
}

func (w *NativeWatcher) Changes() <-chan []Change { return w.changes }
func (w *NativeWatcher) Errors() <-chan error     { return w.errors }

func (w *NativeWatcher) Close() error {
	w.cancel()
	return w.inner.Close()
}

func (w *NativeWatcher) run(ctx context.Context, cfg Config, previous Snapshot) {
	defer close(w.changes)
	defer close(w.errors)
	pending := false
	pendingFull := false
	pendingPaths := make(map[string]struct{})
	timer := newTimer()
	defer timer.Stop()
	var auxiliaryTicker *time.Ticker
	var auxiliaryC <-chan time.Time
	if len(cfg.Scope.Topology) > 0 || len(cfg.Scope.Files) > 0 {
		auxiliaryTicker = newTicker(auxiliaryPollInterval(cfg.Debounce))
		auxiliaryC = auxiliaryTicker.C
		defer auxiliaryTicker.Stop()
	}
	for {
		select {
		case <-ctx.Done():
			return
		case err, ok := <-w.inner.Errors:
			if !ok {
				return
			}
			sendWatchError(ctx, w.errors, err)
		case event, ok := <-w.inner.Events:
			if !ok {
				return
			}
			eventPath, err := filepath.Abs(event.Name)
			if err != nil {
				continue
			}
			eventPath = filepath.Clean(eventPath)
			topologyRelevant := scopeTopologyRelevant(cfg.Scope, eventPath)
			if event.Op&(fsnotify.Create|fsnotify.Rename) != 0 && topologyRelevant {
				addCreatedDir(w.inner, event.Name, cfg.Scope)
			}
			classification := classifyScopePath(cfg.Scope, eventPath)
			if !scopeCapturesPath(cfg.Scope, eventPath, classification) {
				if topologyRelevant && event.Op&(fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 {
					pending = true
					pendingFull = true
					resetTimer(timer, cfg.Debounce)
				}
				continue
			}
			if nativeEventNeedsFullSnapshot(event, classification) {
				pending = true
				pendingFull = true
				resetTimer(timer, cfg.Debounce)
				continue
			}
			if classification.Watchable {
				pending = true
				if abs, err := filepath.Abs(event.Name); err == nil {
					pendingPaths[filepath.Clean(abs)] = struct{}{}
				}
				resetTimer(timer, cfg.Debounce)
				continue
			}
		case <-timer.C:
			if !pending {
				continue
			}
			pending = false
			paths := sortedPendingPaths(pendingPaths)
			pendingPaths = make(map[string]struct{})
			currentFull := pendingFull
			pendingFull = false
			changes, current, err := snapshotPendingScope(cfg.Scope, previous, paths, currentFull)
			if err != nil {
				sendWatchError(ctx, w.errors, err)
				continue
			}
			previous = current
			if len(changes) > 0 {
				sendWatchChanges(ctx, w.changes, changes)
			}
		case <-auxiliaryC:
			currentAuxiliary, err := captureScopeAuxiliary(cfg.Scope)
			if err != nil {
				sendWatchError(ctx, w.errors, err)
				continue
			}
			previousAuxiliary := scopeAuxiliarySnapshot(cfg.Scope, previous)
			changes := DiffSnapshots(previousAuxiliary, currentAuxiliary)
			for _, path := range cfg.Scope.Files {
				delete(previous.Files, path)
			}
			for path, state := range currentAuxiliary.Files {
				previous.Files[path] = state
			}
			previous.Topology = currentAuxiliary.Topology
			if len(changes) > 0 {
				sendWatchChanges(ctx, w.changes, changes)
			}
		}
	}
}

func auxiliaryPollInterval(debounce time.Duration) time.Duration {
	const minimum = 500 * time.Millisecond
	if debounce < minimum {
		return minimum
	}
	return debounce
}

func captureScopeAuxiliary(scope Scope) (Snapshot, error) {
	files, err := captureScopePaths(scope, scope.Files)
	if err != nil {
		return Snapshot{}, err
	}
	topology, err := captureScopeTopology(scope)
	if err != nil {
		return Snapshot{}, err
	}
	files.Topology = topology
	return files, nil
}

func scopeAuxiliarySnapshot(scope Scope, snapshot Snapshot) Snapshot {
	files := make(map[string]FileState)
	for _, path := range scope.Files {
		if state, ok := snapshot.Files[path]; ok {
			files[path] = state
		}
	}
	return Snapshot{Files: files, Topology: snapshot.Topology}
}

func nativeEventNeedsFullSnapshot(event fsnotify.Event, classification FileClassification) bool {
	if classification.Kind == FileKindTopology {
		return true
	}
	if !classification.Watchable {
		return event.Op&(fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0
	}
	if event.Op&(fsnotify.Create|fsnotify.Rename) != 0 {
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			return true
		}
	}
	return event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 && filepath.Ext(filepath.Base(event.Name)) == ""
}

func snapshotPendingPaths(root string, previous Snapshot, paths []string, full bool) ([]Change, Snapshot, error) {
	return snapshotPendingScope(NormalizeScope(Scope{Roots: []string{root}}), previous, paths, full)
}

func snapshotPendingScope(scope Scope, previous Snapshot, paths []string, full bool) ([]Change, Snapshot, error) {
	if full || len(paths) == 0 {
		current, err := CaptureScope(scope)
		if err != nil {
			return nil, previous, err
		}
		return DiffSnapshots(previous, current), current, nil
	}
	currentSubset, err := captureScopePaths(scope, paths)
	if err != nil {
		return nil, previous, err
	}
	previousSubset := Snapshot{Files: make(map[string]FileState, len(paths))}
	for _, path := range paths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil, previous, err
		}
		absPath = filepath.Clean(absPath)
		if state, ok := previous.Files[absPath]; ok {
			previousSubset.Files[absPath] = state
		}
	}
	changes := DiffSnapshots(previousSubset, currentSubset)
	current := Snapshot{Files: make(map[string]FileState, len(previous.Files)+len(currentSubset.Files)), Topology: previous.Topology}
	for path, state := range previous.Files {
		current.Files[path] = state
	}
	for _, path := range paths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil, previous, err
		}
		delete(current.Files, filepath.Clean(absPath))
	}
	for path, state := range currentSubset.Files {
		current.Files[path] = state
	}
	return changes, current, nil
}

func captureScopePaths(scope Scope, paths []string) (Snapshot, error) {
	files := make(map[string]FileState)
	for _, path := range paths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return Snapshot{}, err
		}
		absPath = filepath.Clean(absPath)
		classification := ClassifyPath(absPath)
		if !scopeCapturesPath(scope, absPath, classification) {
			continue
		}
		state, ok, err := capturePath(absPath)
		if err != nil {
			return Snapshot{}, err
		}
		if ok {
			files[state.Path] = state
		}
	}
	return Snapshot{Files: files}, nil
}

func scopeCapturesPath(scope Scope, path string, classification FileClassification) bool {
	if scopeExcludesPath(scope, path) {
		return false
	}
	if _, topology := scopeTopologyEndpoint(scope, path); topology {
		return classification.Kind == FileKindTopology
	}
	for _, file := range scope.Files {
		if path == file {
			return classification.Watchable
		}
	}
	for _, root := range scope.Roots {
		if !pathWithin(path, root) {
			continue
		}
		return classification.Watchable && (classification.Kind != FileKindIgnored || isRootConfigPath(path, root))
	}
	return false
}

func classifyScopePath(scope Scope, path string) FileClassification {
	if endpoint, ok := scopeTopologyEndpoint(scope, path); ok {
		return FileClassification{Path: endpoint, Kind: FileKindTopology, Name: filepath.Base(endpoint), Watchable: true}
	}
	return ClassifyPath(path)
}

func scopeTopologyEndpoint(scope Scope, path string) (string, bool) {
	for _, endpoint := range scope.Topology {
		if path == endpoint || (filepath.Base(path) == filepath.Base(endpoint) && sameExistingFile(filepath.Dir(path), filepath.Dir(endpoint))) {
			return endpoint, true
		}
	}
	return "", false
}

func scopeTopologyRelevant(scope Scope, path string) bool {
	for _, root := range scope.Roots {
		if pathWithin(path, root) || pathWithin(root, path) {
			return true
		}
	}
	for _, file := range scope.Files {
		if path == file || pathWithin(file, path) {
			return true
		}
	}
	for _, endpoint := range scope.Topology {
		if _, ok := scopeTopologyEndpoint(scope, path); ok || pathWithin(endpoint, path) {
			return true
		}
	}
	return false
}

func sortedPendingPaths(paths map[string]struct{}) []string {
	out := make([]string, 0, len(paths))
	for path := range paths {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func addScopeWatchDirs(w *fsnotify.Watcher, scope Scope) error {
	for _, root := range scope.Roots {
		info, err := os.Stat(root)
		if err == nil && info.IsDir() {
			if err := addScopeRootWatchDirs(w, root, scope); err != nil {
				return err
			}
			// Keep the parent registered so deleting and later recreating the
			// scoped root remains observable after the root's own watch is gone.
			if err := addNearestExistingDir(w, filepath.Dir(root)); err != nil {
				return err
			}
			continue
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := addNearestExistingDir(w, filepath.Dir(root)); err != nil {
			return err
		}
	}
	for _, endpoint := range scope.Topology {
		if err := addNearestExistingDir(w, filepath.Dir(endpoint)); err != nil {
			return err
		}
	}
	return nil
}

func addScopeRootWatchDirs(w *fsnotify.Watcher, root string, scope Scope) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if scopeExcludesPath(scope, path) {
			if d.IsDir() {
				if scopeNeedsExcludedTraversal(scope, path) {
					return w.Add(path)
				}
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		return w.Add(path)
	})
}

func addNearestExistingDir(w *fsnotify.Watcher, path string) error {
	for {
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			return w.Add(path)
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		parent := filepath.Dir(path)
		if parent == path {
			return fmt.Errorf("no existing directory for watch path %q", path)
		}
		path = parent
	}
}

func addCreatedDir(w *fsnotify.Watcher, path string, scope Scope) {
	if scopeExcludesPath(scope, path) && !scopeNeedsExcludedTraversal(scope, path) {
		return
	}
	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		_ = addScopeRootWatchDirs(w, path, scope)
	}
}

func newTicker(delay time.Duration) *time.Ticker {
	if delay <= 0 {
		delay = DefaultDebounce
	}
	return time.NewTicker(delay)
}

func newTimer() *time.Timer {
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}
	return timer
}

func resetTimer(timer *time.Timer, delay time.Duration) {
	if delay <= 0 {
		delay = DefaultDebounce
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(delay)
}

func sendWatchChanges(ctx context.Context, out chan<- []Change, changes []Change) {
	select {
	case out <- changes:
	case <-ctx.Done():
	}
}

func sendWatchError(ctx context.Context, out chan<- error, err error) {
	select {
	case out <- err:
	case <-ctx.Done():
	}
}
