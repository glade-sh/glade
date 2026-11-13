package lsp

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/typesys"
)

// LSP FileChangeType values.
const (
	fileCreated = 1
	fileChanged = 2
	fileDeleted = 3
)

// FileEvent is one workspace/didChangeWatchedFiles change.
type FileEvent struct {
	URI  DocumentURI `json:"uri"`
	Type int         `json:"type"`
}

type DidChangeWatchedFilesParams struct {
	Changes []FileEvent `json:"changes"`
}

// watchRegistrationID is the id of the server's only outbound request.
const watchRegistrationID = "glade-watch-1"

// Watched-file events settle for watchQuiet, at most watchMaxDelay after the
// first, so a checkout arrives as one refresh.
const (
	watchQuiet    = 300 * time.Millisecond
	watchMaxDelay = 2 * time.Second
)

func registerWatchersRequest() map[string]any {
	watchers := []map[string]any{}
	for _, glob := range []string{"**/*.cls", "**/*.trigger", "**/*-meta.xml", "**/sfdx-project.json", "**/glade.yml"} {
		watchers = append(watchers, map[string]any{"globPattern": glob})
	}
	return map[string]any{
		"jsonrpc": "2.0",
		"id":      watchRegistrationID,
		"method":  "client/registerCapability",
		"params": map[string]any{"registrations": []map[string]any{{
			"id":              "glade-watched-files",
			"method":          "workspace/didChangeWatchedFiles",
			"registerOptions": map[string]any{"watchers": watchers},
		}}},
	}
}

func ignoredWatchedPath(path string) bool {
	for _, segment := range strings.Split(filepath.ToSlash(path), "/") {
		switch segment {
		case ".glade", ".git", "node_modules":
			return true
		}
	}
	return false
}

// watchedFileEvents classifies client events into a refresh request. Events
// whose disk state equals the base index, such as the echo of a didSave or
// a plain touch, are dropped.
func (h *Handler) watchedFileEvents(params DidChangeWatchedFilesParams) RefreshRequest {
	kinds := map[string]int{}
	for _, event := range params.Changes {
		path := pathFromURI(event.URI)
		if path == "" || ignoredWatchedPath(path) || h.matchesBase(path) {
			continue
		}
		if event.Type == fileCreated || event.Type == fileDeleted {
			kinds[path] = event.Type
		} else if _, ok := kinds[path]; !ok {
			kinds[path] = fileChanged
		}
	}
	return refreshRequestFromKinds(kinds)
}

func refreshRequestFromKinds(kinds map[string]int) RefreshRequest {
	var request RefreshRequest
	for path, kind := range kinds {
		switch kind {
		case fileCreated:
			request.Created = append(request.Created, path)
		case fileDeleted:
			request.Deleted = append(request.Deleted, path)
		default:
			request.Changed = append(request.Changed, path)
		}
	}
	sort.Strings(request.Changed)
	sort.Strings(request.Created)
	sort.Strings(request.Deleted)
	return request
}

func (r RefreshRequest) empty() bool {
	return len(r.Changed) == 0 && len(r.Created) == 0 && len(r.Deleted) == 0
}

// matchesBase reports whether path's Apex source and companion metadata on
// disk equal what the base index captured.
func (h *Handler) matchesBase(path string) bool {
	base := h.baseGen.Load()
	if base == nil {
		return false
	}
	source := path
	if strings.HasSuffix(strings.ToLower(path), "-meta.xml") {
		source = path[:len(path)-len("-meta.xml")]
	}
	if !isApexSourcePath(source) {
		return false
	}
	expected, ok := base.index.SourceDigest(source)
	if !ok {
		return false
	}
	data, err := os.ReadFile(source) // #nosec G304 -- a project source path the base index already holds.
	if err != nil || sha256.Sum256(data) != expected {
		return false
	}
	found := false
	metadataMatches := func(input typesys.ApexMetadataInput) bool {
		found = true
		meta, err := os.ReadFile(source + "-meta.xml") // #nosec G304 -- companion of an indexed Apex source.
		if err != nil {
			return !input.Present && os.IsNotExist(err)
		}
		return input.Present && sha256.Sum256(meta) == input.Digest
	}
	for _, typ := range base.index.Types {
		if typ.File != "" && sameDocument(typ.File, source) {
			input, ok := base.index.ApexMetadataForType(typ)
			if !ok || !metadataMatches(input) {
				return false
			}
		}
	}
	for _, trigger := range base.index.Triggers {
		if trigger.File != "" && sameDocument(trigger.File, source) {
			input, ok := base.index.ApexMetadataForTrigger(trigger)
			if !ok || !metadataMatches(input) {
				return false
			}
		}
	}
	return found
}

// DidChangeWatchedFiles refreshes the base from disk for the synchronous API.
func (h *Handler) DidChangeWatchedFiles(ctx context.Context, params DidChangeWatchedFilesParams) error {
	request := h.watchedFileEvents(params)
	if request.empty() {
		return nil
	}
	return h.refreshRequest(ctx, request, func(g *generation) { h.gen.Store(g) })
}

// enqueueWatched merges watched-file events into the pending set and arms
// the settle timer.
func (s *server) enqueueWatched(request RefreshRequest) {
	if request.empty() {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil || s.closed {
		return
	}
	if s.watchPending == nil {
		s.watchPending = map[string]int{}
	}
	mergeRefresh(s.watchPending, request)
	now := time.Now()
	if s.watchFirst.IsZero() {
		s.watchFirst = now
	}
	delay := min(watchQuiet, time.Until(s.watchFirst.Add(watchMaxDelay)))
	if s.watchTimer == nil {
		s.watchTimer = time.AfterFunc(delay, s.flushWatched)
	} else {
		s.watchTimer.Reset(delay)
	}
}

// flushWatched moves settled watched-file events into the refresh queue.
func (s *server) flushWatched() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flushWatchedLocked()
}

func (s *server) flushWatchedLocked() {
	if len(s.watchPending) == 0 || s.ctx.Err() != nil || s.closed {
		return
	}
	if s.dirty == nil {
		s.dirty = map[string]int{}
	}
	for path, kind := range s.watchPending {
		mergeKind(s.dirty, path, kind)
	}
	s.watchPending = nil
	s.watchFirst = time.Time{}
	s.startWorkerLocked()
}

func mergeRefresh(kinds map[string]int, request RefreshRequest) {
	for _, path := range request.Changed {
		mergeKind(kinds, path, fileChanged)
	}
	for _, path := range request.Created {
		mergeKind(kinds, path, fileCreated)
	}
	for _, path := range request.Deleted {
		mergeKind(kinds, path, fileDeleted)
	}
}

// mergeKind keeps the strongest event for a path: a creation or deletion
// needs a full reload, so it outranks a change.
func mergeKind(kinds map[string]int, path string, kind int) {
	if previous, ok := kinds[path]; ok && previous != fileChanged && kind == fileChanged {
		return
	}
	kinds[path] = kind
}
