//go:build darwin && cgo

package watch

/*
#cgo LDFLAGS: -framework CoreServices -framework CoreFoundation
#include <CoreServices/CoreServices.h>
#include <errno.h>
#include <pthread.h>
#include <stdatomic.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mount.h>
#include <unistd.h>

typedef struct {
    pthread_t thread;
    pthread_mutex_t mu;
    pthread_cond_t ready;
    int started;
    int accepting;
    int invalid;
    atomic_int needs_poll;
    int fd;
    char *root;
    char *parent;
    CFRunLoopRef loop;
    CFRunLoopSourceRef stop;
} glade_stream;

static int glade_contains(const char *parent, const char *child) {
    size_t n = strlen(parent);
    if (strncmp(parent, child, n) == 0 &&
        (n == 1 || child[n] == '/' || child[n] == '\0')) return 1;

    // macOS paths can use different case or Unicode normalization for the
    // same directory. A false positive only requests another reconciliation.
    CFStringRef a = CFStringCreateWithFileSystemRepresentation(NULL, parent);
    CFStringRef b = CFStringCreateWithFileSystemRepresentation(NULL, child);
    if (!a || !b) {
        if (a) CFRelease(a);
        if (b) CFRelease(b);
        return 1;
    }
    CFIndex length = CFStringGetLength(b);
    CFRange match;
    Boolean found = CFStringFindWithOptions(b, a, CFRangeMake(0, length),
        kCFCompareAnchored | kCFCompareCaseInsensitive | kCFCompareNonliteral, &match);
    int result = found && (match.length == length ||
        CFStringGetCharacterAtIndex(b, match.length) == '/');
    CFRelease(a);
    CFRelease(b);
    return result;
}

static void glade_events(ConstFSEventStreamRef stream, void *info, size_t count,
                         void *paths, const FSEventStreamEventFlags flags[],
                         const FSEventStreamEventId ids[]) {
    (void)stream; (void)ids;
    glade_stream *s = info;
    char **names = paths;
    char signal = 0;
    for (size_t i = 0; i < count; i++) {
        int relevant = glade_contains(s->root, names[i]) || glade_contains(names[i], s->root);
        if ((flags[i] & kFSEventStreamEventFlagRootChanged) ||
            ((flags[i] & (kFSEventStreamEventFlagUnmount | kFSEventStreamEventFlagMount)) && relevant)) {
            if (!s->accepting) s->invalid = 1;
            signal = 'R';
            break;
        }
        if ((flags[i] & (kFSEventStreamEventFlagMustScanSubDirs |
                         kFSEventStreamEventFlagUserDropped |
                         kFSEventStreamEventFlagKernelDropped)) || relevant) {
            signal = 'E';
        }
    }
    if (!s->accepting) return;
    // Preserve a recovery request even if the nonblocking pipe is full.
    if (signal == 'R') atomic_store(&s->needs_poll, 1);
    if (signal) {
        while (write(s->fd, &signal, 1) < 0 && errno == EINTR) {}
    }
}

static int glade_needs_poll(glade_stream *s) {
    return atomic_exchange(&s->needs_poll, 0);
}

static void glade_stop(void *info) { CFRunLoopStop((CFRunLoopRef)info); }

static void *glade_stream_thread(void *arg) {
    glade_stream *s = arg;
    CFStringRef path = CFStringCreateWithCString(NULL, s->parent, kCFStringEncodingUTF8);
    CFArrayRef paths = path ? CFArrayCreate(NULL, (const void **)&path, 1, &kCFTypeArrayCallBacks) : NULL;
    FSEventStreamContext context = {0, s, NULL, NULL, NULL};
    FSEventStreamRef stream = paths ? FSEventStreamCreate(NULL, glade_events, &context, paths,
        kFSEventStreamEventIdSinceNow, 0.05,
        kFSEventStreamCreateFlagFileEvents | kFSEventStreamCreateFlagWatchRoot |
        kFSEventStreamCreateFlagNoDefer) : NULL;
    CFRunLoopRef loop = CFRunLoopGetCurrent();
    CFRunLoopSourceContext stopContext = {0};
    stopContext.info = loop;
    stopContext.perform = glade_stop;
    CFRunLoopSourceRef stop = CFRunLoopSourceCreate(NULL, 0, &stopContext);
    int started = 0;
    if (stream && stop) {
        CFRunLoopAddSource(loop, stop, kCFRunLoopDefaultMode);
        FSEventStreamScheduleWithRunLoop(stream, loop, kCFRunLoopDefaultMode);
        started = FSEventStreamStart(stream);
        if (started) {
            // The following snapshot covers events already delivered here.
            FSEventStreamFlushSync(stream);
            if (s->invalid) { FSEventStreamStop(stream); started = 0; }
            s->accepting = 1;
        }
    }
    pthread_mutex_lock(&s->mu);
    s->loop = loop;
    s->stop = stop;
    s->started = started ? 1 : -1;
    pthread_cond_signal(&s->ready);
    pthread_mutex_unlock(&s->mu);
    if (started) CFRunLoopRun();
    if (stream) {
        if (started) FSEventStreamStop(stream);
        FSEventStreamInvalidate(stream);
        FSEventStreamRelease(stream);
    }
    if (stop) { CFRunLoopRemoveSource(loop, stop, kCFRunLoopDefaultMode); CFRelease(stop); }
    if (paths) CFRelease(paths);
    if (path) CFRelease(path);
    return NULL;
}

static glade_stream *glade_start(const char *root, const char *parent, int fd) {
    glade_stream *s = calloc(1, sizeof(*s));
    if (!s) return NULL;
    atomic_init(&s->needs_poll, 0);
    s->root = strdup(root);
    s->parent = strdup(parent);
    s->fd = fd;
    if (!s->root || !s->parent) { free(s->root); free(s->parent); free(s); return NULL; }
    if (pthread_mutex_init(&s->mu, NULL) != 0) {
        free(s->root); free(s->parent); free(s); return NULL;
    }
    if (pthread_cond_init(&s->ready, NULL) != 0) {
        pthread_mutex_destroy(&s->mu);
        free(s->root); free(s->parent); free(s); return NULL;
    }
    if (pthread_create(&s->thread, NULL, glade_stream_thread, s) != 0) {
        pthread_cond_destroy(&s->ready); pthread_mutex_destroy(&s->mu);
        free(s->root); free(s->parent); free(s); return NULL;
    }
    pthread_mutex_lock(&s->mu);
    while (!s->started) pthread_cond_wait(&s->ready, &s->mu);
    int started = s->started;
    pthread_mutex_unlock(&s->mu);
    if (started < 0) {
        pthread_join(s->thread, NULL);
        pthread_cond_destroy(&s->ready); pthread_mutex_destroy(&s->mu);
        free(s->root); free(s->parent); free(s); return NULL;
    }
    return s;
}

static void glade_stream_close(glade_stream *s) {
    CFRunLoopSourceSignal(s->stop);
    CFRunLoopWakeUp(s->loop);
    pthread_join(s->thread, NULL);
    pthread_cond_destroy(&s->ready); pthread_mutex_destroy(&s->mu);
    free(s->root); free(s->parent); free(s);
}
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

type recursiveWatcher struct {
	cancel      context.CancelFunc
	stream      *C.glade_stream
	read, write *os.File
	changes     chan []Change
	errors      chan error
	done        chan struct{}
}

func newRecursiveWatcher(ctx context.Context, cfg Config, initial Snapshot) (BackendWatcher, bool) {
	// Topology and multiple roots retain the existing native implementation.
	if len(cfg.Scope.Roots) != 1 || len(cfg.Scope.Topology) != 0 {
		return nil, false
	}
	root := cfg.Scope.Roots[0]
	parent, err := filepath.EvalSymlinks(filepath.Dir(root))
	if err != nil {
		return nil, false
	}
	root = filepath.Join(parent, filepath.Base(root))
	var fs syscall.Statfs_t
	if syscall.Statfs(parent, &fs) != nil || fs.Flags&C.MNT_LOCAL == 0 {
		return nil, false
	}
	parentInfo, err := os.Stat(parent)
	if err != nil {
		return nil, false
	}
	read, write, err := os.Pipe()
	if err != nil {
		return nil, false
	}
	writeFD := int(write.Fd())
	if err := syscall.SetNonblock(writeFD, true); err != nil {
		read.Close()
		write.Close()
		return nil, false
	}
	croot, cparent := C.CString(root), C.CString(parent)
	stream := C.glade_start(croot, cparent, C.int(writeFD))
	C.free(unsafe.Pointer(croot))
	C.free(unsafe.Pointer(cparent))
	if stream == nil {
		read.Close()
		write.Close()
		return nil, false
	}
	if nested, err := recursiveScopeHasNestedMount(root); err != nil || nested {
		C.glade_stream_close(stream)
		read.Close()
		write.Close()
		return nil, false
	}
	linked, err := recursiveScopeHasLinkedFile(cfg.Scope)
	if err != nil || linked {
		C.glade_stream_close(stream)
		read.Close()
		write.Close()
		return nil, false
	}
	current, err := CaptureScope(cfg.Scope)
	if err != nil {
		C.glade_stream_close(stream)
		read.Close()
		write.Close()
		return nil, false
	}
	ctx, cancel := context.WithCancel(ctx)
	w := &recursiveWatcher{cancel: cancel, stream: stream, read: read, write: write,
		changes: make(chan []Change, 1), errors: make(chan error, 1), done: make(chan struct{})}
	if changes := DiffSnapshots(initial, current); len(changes) > 0 {
		w.changes <- changes
	}
	go w.run(ctx, cfg, current, parent, parentInfo)
	return w, true
}

func recursiveScopeHasNestedMount(root string) (bool, error) {
	n, err := syscall.Getfsstat(nil, C.MNT_NOWAIT)
	if err != nil {
		return false, err
	}
	mounts := make([]syscall.Statfs_t, n+1)
	n, err = syscall.Getfsstat(mounts, C.MNT_NOWAIT)
	if err != nil {
		return false, err
	}
	if n >= len(mounts) {
		return true, nil
	}
	for _, mount := range mounts[:n] {
		var name []byte
		for _, c := range mount.Mntonname {
			if c == 0 {
				break
			}
			name = append(name, byte(c))
		}
		path := string(name)
		if recursivePathContains(root, path) {
			return true, nil
		}
	}
	return false, nil
}

func recursivePathContains(parent, child string) bool {
	a, b := C.CString(parent), C.CString(child)
	result := C.glade_contains(a, b) != 0
	C.free(unsafe.Pointer(a))
	C.free(unsafe.Pointer(b))
	return result
}

func recursiveScopeHasLinkedFile(scope Scope) (bool, error) {
	linked := false
	err := filepath.WalkDir(scope.Roots[0], func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if scopeExcludesPath(scope, path) {
			if entry.IsDir() && !scopeNeedsExcludedTraversal(scope, path) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() || !scopeCapturesPath(scope, path, classifyScopePath(scope, path)) {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			linked = true
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			stat, ok := info.Sys().(*syscall.Stat_t)
			if !ok {
				return fmt.Errorf("file identity unavailable for %q", path)
			}
			if stat.Nlink > 1 {
				linked = true
			}
		}
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		return linked, nil
	}
	return linked, err
}

func (w *recursiveWatcher) Changes() <-chan []Change { return w.changes }
func (w *recursiveWatcher) Errors() <-chan error     { return w.errors }
func (w *recursiveWatcher) Close() error {
	w.cancel()
	<-w.done
	return nil
}

func (w *recursiveWatcher) run(ctx context.Context, cfg Config, previous Snapshot, parent string, parentInfo os.FileInfo) {
	defer close(w.done)
	defer func() { C.glade_stream_close(w.stream); w.write.Close(); w.read.Close() }()
	defer close(w.changes)
	defer close(w.errors)
	// Closing the write end wakes this reader even when no events arrive.
	signals := make(chan byte, 1)
	go func() {
		defer close(signals)
		var buf [256]byte
		for {
			n, err := w.read.Read(buf[:])
			for _, b := range buf[:n] {
				select {
				case signals <- b:
				case <-ctx.Done():
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()
	timer := newTimer()
	defer timer.Stop()
	var poll *time.Ticker
	var pollC <-chan time.Time
	if len(cfg.Scope.Files) > 0 {
		poll = newTicker(auxiliaryPollInterval(cfg.Debounce))
		pollC = poll.C
		defer poll.Stop()
	}
	pending := false
	streamLost := false
	for {
		select {
		case <-ctx.Done():
			return
		case signal, ok := <-signals:
			if !ok {
				signals = nil
				streamLost = true
				sendWatchError(ctx, w.errors, fmt.Errorf("recursive watcher event stream closed"))
				pending = true
				resetTimer(timer, cfg.Debounce)
				continue
			}
			if signal == 'R' || C.glade_needs_poll(w.stream) != 0 {
				streamLost = true
			}
			pending = true
			resetTimer(timer, cfg.Debounce)
		case <-timer.C:
			if !pending {
				continue
			}
			pending = false
			current, err := CaptureScope(cfg.Scope)
			if err != nil {
				sendWatchError(ctx, w.errors, err)
				pending = true
				resetTimer(timer, auxiliaryPollInterval(cfg.Debounce))
				continue
			}
			changes := DiffSnapshots(previous, current)
			previous = current
			if len(changes) > 0 {
				sendWatchChanges(ctx, w.changes, changes)
			}
			if info, err := os.Stat(parent); err != nil || !os.SameFile(parentInfo, info) {
				streamLost = true
			}
			if !streamLost {
				linked, err := recursiveScopeHasLinkedFile(cfg.Scope)
				if err != nil || linked {
					streamLost = true
				}
			}
			if poll == nil {
				if streamLost {
					poll = newTicker(auxiliaryPollInterval(cfg.Debounce))
					pollC = poll.C
					defer poll.Stop()
				}
			}
		case <-pollC:
			// Poll the whole scope after the stream can no longer cover it.
			// Otherwise only exact external files need periodic checks.
			if streamLost {
				pending = true
				resetTimer(timer, cfg.Debounce)
				continue
			}
			current, err := captureScopeAuxiliary(cfg.Scope)
			if err != nil {
				sendWatchError(ctx, w.errors, err)
				continue
			}
			changes := DiffSnapshots(scopeAuxiliarySnapshot(cfg.Scope, previous), current)
			for _, path := range cfg.Scope.Files {
				delete(previous.Files, path)
			}
			for path, state := range current.Files {
				previous.Files[path] = state
			}
			previous.Topology = current.Topology
			if len(changes) > 0 {
				sendWatchChanges(ctx, w.changes, changes)
			}
		}
	}
}
