package watch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
			current, err := CaptureSnapshot(cfg.Root)
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
	absRoot, err := filepath.Abs(cfg.Root)
	if err != nil {
		_ = inner.Close()
		return nil, err
	}
	if err := addWatchDirs(inner, absRoot); err != nil {
		_ = inner.Close()
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	w := &NativeWatcher{
		cancel:  cancel,
		inner:   inner,
		changes: make(chan []Change, 1),
		errors:  make(chan error, 1),
	}
	go w.run(ctx, cfg, initial)
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
	timer := newTimer()
	defer timer.Stop()
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
			if event.Op&(fsnotify.Create|fsnotify.Rename) != 0 {
				addCreatedDir(w.inner, event.Name)
			}
			if ClassifyPath(event.Name).Watchable || event.Op&(fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 {
				pending = true
				resetTimer(timer, cfg.Debounce)
			}
		case <-timer.C:
			if !pending {
				continue
			}
			pending = false
			current, err := CaptureSnapshot(cfg.Root)
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

func addWatchDirs(w *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		return w.Add(path)
	})
}

func addCreatedDir(w *fsnotify.Watcher, path string) {
	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		_ = addWatchDirs(w, path)
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
