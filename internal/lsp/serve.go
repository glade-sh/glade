package lsp

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"

	"sync"
	"time"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
)

// Serve reads LSP messages from r and writes responses and notifications to w
// until EOF or shutdown.
//
// The read loop never runs index-dependent work. initialize and document sync
// are handled in order as they arrive. Other requests run in their own
// goroutines, wait for the project index if it is still loading, and end early
// with a RequestCancelled error when the client sends $/cancelRequest.
// Diagnostics are published off the read loop; didChange diagnostics are
// debounced and always reflect the latest document text.
func Serve(r io.Reader, w io.Writer, handler *Handler) error {
	s := &server{
		h:        handler,
		w:        w,
		inflight: make(map[string]context.CancelFunc),
		pending:  make(map[DocumentURI]*diagJob),
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()
	reader := NewProtocolReader(r)
	for {
		data, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				s.drain()
				return s.firstWriteErr()
			}
			s.shutdownNow()
			return err
		}
		s.dispatch(data)
		if handler.Shutdown() {
			s.shutdownNow()
			return s.firstWriteErr()
		}
	}
}

type diagJob struct {
	timer *time.Timer
}

type server struct {
	h      *Handler
	w      io.Writer
	ctx    context.Context
	cancel context.CancelFunc

	writeMu  sync.Mutex
	writeErr error

	reqWG sync.WaitGroup
	mu    sync.Mutex // guards inflight and pending
	// inflight maps a request ID's JSON text to its cancel function.
	inflight map[string]context.CancelFunc

	diagWG     sync.WaitGroup
	pubMu      sync.Mutex // orders diagnostic publishes against pending checks
	pending    map[DocumentURI]*diagJob
	dirty      map[string]int // path to LSP FileChangeType
	refreshing bool
	refreshWG  sync.WaitGroup

	// Overlay sync state, guarded by mu. overlayCancel stops an overlay
	// sync in flight; lastChange and overlayCost drive the adaptive wait.
	overlayPending bool
	overlayCancel  context.CancelFunc
	lastChange     time.Time
	overlayCost    time.Duration

	// Watched-file events waiting to settle, guarded by mu.
	watchPending map[string]int
	watchFirst   time.Time
	watchTimer   *time.Timer
	closed       bool // no new background work after drain or shutdown
}

func (s *server) write(message any) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if s.writeErr != nil {
		return
	}
	if err := WriteMessage(s.w, message); err != nil {
		s.writeErr = err
	}
}

func (s *server) firstWriteErr() error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.writeErr
}

func (s *server) dispatch(data []byte) {
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		s.write(Response{JSONRPC: "2.0", Error: &ResponseError{Code: errorCodeParseError, Message: err.Error()}})
		return
	}
	if req.Method == "" && len(req.ID) != 0 {
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(data, &fields)
		if _, ok := fields["result"]; ok {
			return
		}
		if _, ok := fields["error"]; ok {
			return
		}
	}
	if len(req.ID) == 0 {
		s.notification(req)
		return
	}
	switch req.Method {
	case "initialize":
		s.write(s.h.HandleRequest(req))
		return
	case "shutdown":
		// Stop in-flight diagnostics and requests before the handler closes
		// the native parser.
		s.shutdownNow()
		s.write(s.h.HandleRequest(req))
		return
	}
	ctx, cancel := context.WithCancel(s.ctx)
	key := string(req.ID)
	s.mu.Lock()
	s.inflight[key] = cancel
	s.mu.Unlock()
	s.reqWG.Add(1)
	go func() {
		defer s.reqWG.Done()
		defer func() {
			s.mu.Lock()
			delete(s.inflight, key)
			s.mu.Unlock()
			cancel()
		}()
		result, rpcErr := s.h.handleContext(ctx, req.Method, req.Params)
		if rpcErr == nil && ctx.Err() != nil {
			rpcErr = indexError(ctx.Err())
			result = nil
		}
		s.write(Response{JSONRPC: "2.0", ID: req.ID, Result: result, Error: rpcErr})
	}()
}

func (s *server) notification(req Request) {
	switch req.Method {
	case "$/cancelRequest":
		var p struct {
			ID json.RawMessage `json:"id"`
		}
		if decodeParams(req.Params, &p) != nil {
			return
		}
		s.mu.Lock()
		cancel := s.inflight[string(p.ID)]
		s.mu.Unlock()
		if cancel != nil {
			cancel()
		}
	case "textDocument/didOpen":
		var p DidOpenTextDocumentParams
		if decodeParams(req.Params, &p) != nil {
			return
		}
		s.h.applyOpen(p)
		s.schedule(p.TextDocument.URI, 0)
		s.enqueueOverlay()
	case "textDocument/didChange":
		var p DidChangeTextDocumentParams
		if decodeParams(req.Params, &p) != nil {
			return
		}
		if _, err := s.h.applyChange(p); err != nil {
			return
		}
		s.noteChange()
		s.schedule(p.TextDocument.URI, s.h.debounce)
	case "textDocument/didSave":
		var p DidSaveTextDocumentParams
		if decodeParams(req.Params, &p) != nil {
			return
		}
		s.enqueueRefresh(pathFromURI(p.TextDocument.URI))
	case "initialized":
		if s.h.watchFiles.Load() {
			s.write(registerWatchersRequest())
		}
	case "workspace/didChangeWatchedFiles":
		var p DidChangeWatchedFilesParams
		if decodeParams(req.Params, &p) != nil {
			return
		}
		s.enqueueWatched(s.h.watchedFileEvents(p))
	case "textDocument/didClose":
		var p DidCloseTextDocumentParams
		if decodeParams(req.Params, &p) != nil {
			return
		}
		s.h.applyClose(p.TextDocument.URI)
		s.schedule(p.TextDocument.URI, 0)
		s.enqueueOverlay()
	default:
		_, _ = s.h.handleContext(s.ctx, req.Method, req.Params)
	}
}

// schedule queues a diagnostics publish for uri after delay. A later call for
// the same uri replaces an earlier pending one, so a burst of edits publishes
// once, from the document text as it stands when the timer fires.
func (s *server) schedule(uri DocumentURI, delay time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job := &diagJob{}
	if old := s.pending[uri]; old != nil && old.timer != nil && old.timer.Stop() {
		s.diagWG.Done()
	}
	s.pending[uri] = job
	s.diagWG.Add(1)
	job.timer = time.AfterFunc(delay, func() { s.runDiagnostics(uri, job) })
}

func (s *server) runDiagnostics(uri DocumentURI, job *diagJob) {
	defer s.diagWG.Done()
	s.mu.Lock()
	superseded := s.pending[uri] != job
	s.mu.Unlock()
	if superseded {
		return
	}
	generation := s.h.gen.Load()
	// Parse errors need no index; the semantic pass waits for it inside
	// currentDiagnostics. Abandon the wait if the server shuts down first.
	result := make(chan []Notification, 1)
	go func() { result <- s.h.currentDiagnostics(uri) }()
	var notifications []Notification
	select {
	case notifications = <-result:
	case <-s.ctx.Done():
		return
	}
	s.pubMu.Lock()
	defer s.pubMu.Unlock()
	s.mu.Lock()
	current := s.pending[uri] == job
	if current {
		delete(s.pending, uri)
	}
	s.mu.Unlock()
	if !current || s.ctx.Err() != nil {
		return
	}
	if generation != nil && generation != s.h.gen.Load() {
		s.schedule(uri, 0)
		return
	}
	if notifications == nil {
		// A clean buffer the current generation does not reflect: publish
		// nothing stale and let the worker build its overlay.
		s.enqueueOverlay()
		return
	}
	for _, n := range notifications {
		s.write(n)
	}
}

type flushJob struct {
	uri DocumentURI
	job *diagJob
}

// drain runs at EOF. It fires pending debounce timers now so the final
// document state is published, then waits for requests and diagnostics.
func (s *server) drain() {
	s.mu.Lock()
	var jobs []flushJob
	for uri, job := range s.pending {
		if job.timer != nil && job.timer.Stop() {
			jobs = append(jobs, flushJob{uri, job})
		}
	}
	s.mu.Unlock()
	for _, j := range jobs {
		s.runDiagnostics(j.uri, j.job)
	}
	s.mu.Lock()
	if s.watchTimer != nil {
		s.watchTimer.Stop()
	}
	s.flushWatchedLocked()
	s.mu.Unlock()
	s.reqWG.Wait()
	// Diagnostics and the worker can start each other; wait until both rest.
	for {
		s.diagWG.Wait()
		s.refreshWG.Wait()
		s.mu.Lock()
		idle := len(s.pending) == 0 && !s.refreshing
		if idle {
			s.closed = true
		}
		s.mu.Unlock()
		if idle {
			return
		}
	}
}

// shutdownNow cancels outstanding work and waits for it to stop.
func (s *server) shutdownNow() {
	s.cancel()
	s.mu.Lock()
	s.closed = true
	if s.watchTimer != nil {
		s.watchTimer.Stop()
	}
	for _, job := range s.pending {
		if job.timer != nil && job.timer.Stop() {
			s.diagWG.Done()
		}
	}
	s.mu.Unlock()
	s.reqWG.Wait()
	s.diagWG.Wait()
	s.refreshWG.Wait()
}

func (s *server) enqueueRefresh(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil {
		return
	}
	if s.dirty == nil {
		s.dirty = make(map[string]int)
	}
	mergeKind(s.dirty, path, fileChanged)
	s.startWorkerLocked()
}

// enqueueOverlay asks the worker to bring the overlay up to date with the
// open buffers. Disk refreshes queued at the same time run first.
func (s *server) enqueueOverlay() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil || s.closed {
		return
	}
	s.overlayPending = true
	s.startWorkerLocked()
}

func (s *server) startWorkerLocked() {
	if !s.refreshing {
		s.refreshing = true
		s.refreshWG.Add(1)
		go s.refreshWorker()
	}
}

// noteChange records an edit and cancels the overlay sync in flight, which
// stores nothing once cancelled.
func (s *server) noteChange() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastChange = time.Now()
	if s.overlayCancel != nil {
		s.overlayCancel()
	}
}

// Overlay passes wait for quiet: the last pass's analysis time, clamped to
// the debounce floor and maxOverlayQuiet.
const maxOverlayQuiet = 2 * time.Second

func overlayQuiet(cost, floor time.Duration) time.Duration {
	if floor <= 0 {
		return 0
	}
	return min(max(cost, floor), maxOverlayQuiet)
}

func (s *server) refreshWorker() {
	defer s.refreshWG.Done()
	for {
		s.mu.Lock()
		if s.ctx.Err() != nil || (len(s.dirty) == 0 && !s.overlayPending) {
			s.refreshing = false
			s.mu.Unlock()
			return
		}
		if len(s.dirty) > 0 {
			request := refreshRequestFromKinds(s.dirty)
			s.dirty = make(map[string]int)
			s.mu.Unlock()
			if !s.refreshDisk(request) {
				return
			}
			continue
		}
		s.overlayPending = false
		ctx, cancel := context.WithCancel(s.ctx)
		s.overlayCancel = cancel
		s.mu.Unlock()
		s.overlayPass(ctx)
		cancel()
		s.mu.Lock()
		s.overlayCancel = nil
		s.mu.Unlock()
	}
}

// refreshDisk runs one disk refresh. It returns false when the server stops.
func (s *server) refreshDisk(request RefreshRequest) bool {
	if s.h.waitReady(s.ctx) != nil {
		return false
	}
	before := s.h.gen.Load()
	oldResult := make(chan []diagnostic.Diagnostic, 1)
	go func() { oldResult <- before.analysisResult().Diagnostics }()
	var old []diagnostic.Diagnostic
	select {
	case old = <-oldResult:
	case <-s.ctx.Done():
		return false
	}
	var after *generation
	var disk *generation
	var next []diagnostic.Diagnostic
	valid := false
	for attempt := 0; attempt < 2; attempt++ {
		// The first generation a refresh publishes is the disk base; an
		// overlay may follow. Only the base is checked against disk.
		disk = nil
		err := s.h.refreshRequest(s.ctx, request, func(g *generation) {
			s.pubMu.Lock()
			s.h.gen.Store(g)
			s.pubMu.Unlock()
			if disk == nil {
				disk = g
			}
		})
		after = s.h.gen.Load()
		if err != nil {
			var failure *ParseDiagnosticsError
			if errors.As(err, &failure) {
				s.publishDiskParseFailure(failure, before)
			}
			break
		}
		if after == before {
			break
		}
		result := make(chan []diagnostic.Diagnostic, 1)
		go func(g *generation) { result <- g.analysisResult().Diagnostics }(after)
		select {
		case next = <-result:
		case <-s.ctx.Done():
			return false
		}
		if s.h.validateGeneration == nil || disk == nil || s.h.validateGeneration(disk.index) {
			valid = true
			break
		}
	}
	if valid {
		// Reconcile only failures that this generation actually reflects.
		if disk == nil {
			disk = after
		}
		s.h.docMu.Lock()
		for path, failure := range s.h.diskParse {
			data, err := os.ReadFile(path)
			digest, indexed := disk.index.SourceDigest(path)
			if (err == nil && indexed && sha256.Sum256(data) == digest) || (os.IsNotExist(err) && !indexed) {
				old = replaceFileDiagnostics(old, path, failure.Diagnostics)
				delete(s.h.diskParse, path)
			}
		}
		s.h.docMu.Unlock()
		s.publishChanged(old, next, after)
	}
	return true
}

// replaceFileDiagnostics never writes into a generation's shared result.
func replaceFileDiagnostics(old []diagnostic.Diagnostic, path string, replacement []diagnostic.Diagnostic) []diagnostic.Diagnostic {
	result := make([]diagnostic.Diagnostic, 0, len(old)+len(replacement))
	for _, d := range old {
		if d.File != path {
			result = append(result, d)
		}
	}
	return append(result, replacement...)
}

// publishDiskParseFailure retains the semantic generation and publishes only a
// closed file whose on-disk contents still match the failed parse.
func (s *server) publishDiskParseFailure(failure *ParseDiagnosticsError, before *generation) {
	s.pubMu.Lock()
	defer s.pubMu.Unlock()
	if s.ctx.Err() != nil || s.h.gen.Load() != before {
		return
	}
	data, err := os.ReadFile(failure.Path)
	if err != nil || sha256.Sum256(data) != failure.Digest {
		return
	}
	uri := uriFromPath(failure.Path)
	s.h.docMu.Lock()
	defer s.h.docMu.Unlock()
	_, open := s.h.documentPaths[canonicalDocumentPath(failure.Path)]
	if s.h.diskParse == nil {
		s.h.diskParse = make(map[string]ParseDiagnosticsError)
	}
	s.h.diskParse[failure.Path] = *failure
	if open {
		return
	}
	s.write(Notification{JSONRPC: "2.0", Method: "textDocument/publishDiagnostics", Params: BuildPublishDiagnostics(uri, failure.Diagnostics)})
}

// overlayPass syncs the overlay, waits for its analysis, and republishes.
// A cancelled pass stores nothing; the next debounce enqueues another.
func (s *server) overlayPass(ctx context.Context) {
	if s.h.waitReady(ctx) != nil {
		return
	}
	for {
		s.mu.Lock()
		wait := overlayQuiet(s.overlayCost, s.h.debounce) - time.Since(s.lastChange)
		s.mu.Unlock()
		if wait <= 0 {
			break
		}
		timer := time.NewTimer(wait)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return
		}
	}
	before := s.h.gen.Load()
	beforeFallback := s.h.fallback.Load()
	err := s.h.syncOverlay(ctx, func(g *generation) {
		s.pubMu.Lock()
		s.h.gen.Store(g)
		s.pubMu.Unlock()
	})
	after := s.h.gen.Load()
	if err != nil {
		return
	}
	if after == before {
		// Documents the overlay could not represent may now publish their
		// fallback diagnostics.
		if s.h.fallback.Load() != beforeFallback {
			s.scheduleOpenDocuments()
		}
		return
	}
	started := time.Now()
	result := make(chan sema.Result, 1)
	go func() { result <- after.analysisResult() }()
	var next sema.Result
	select {
	case next = <-result:
	case <-ctx.Done():
		return
	}
	if after.abandoned.Load() {
		return
	}
	s.mu.Lock()
	s.overlayCost = time.Since(started)
	s.mu.Unlock()
	var old []diagnostic.Diagnostic
	if previous, ok := before.analysisIfReady(); ok && !before.abandoned.Load() {
		old = previous.Diagnostics
	}
	s.publishChanged(old, next.Diagnostics, after)
	s.scheduleOpenDocuments()
}

func (s *server) scheduleOpenDocuments() {
	s.h.docMu.RLock()
	uris := make([]DocumentURI, 0, len(s.h.documents))
	for uri := range s.h.documents {
		uris = append(uris, uri)
	}
	s.h.docMu.RUnlock()
	for _, uri := range uris {
		s.schedule(uri, 0)
	}
}

// publishChanged republishes every file whose diagnostics differ between old
// and next: open documents through schedule, closed files directly while
// after is still the current generation.
func (s *server) publishChanged(old, next []diagnostic.Diagnostic, after *generation) {
	oldByFile, newByFile := diagnosticsByFile(old), diagnosticsByFile(next)
	files := make(map[string]bool)
	for file := range oldByFile {
		files[file] = true
	}
	for file := range newByFile {
		files[file] = true
	}
	for file := range files {
		if reflect.DeepEqual(oldByFile[file], newByFile[file]) {
			continue
		}
		uri := uriFromPath(file)
		s.h.docMu.RLock()
		_, open := s.h.documents[uri]
		_, failedParse := s.h.diskParse[file]
		s.h.docMu.RUnlock()
		if open {
			s.schedule(uri, 0)
			continue
		}
		if failedParse {
			continue
		}
		s.pubMu.Lock()
		if s.ctx.Err() == nil && s.h.gen.Load() == after {
			s.write(Notification{JSONRPC: "2.0", Method: "textDocument/publishDiagnostics", Params: BuildPublishDiagnostics(uri, newByFile[file])})
		}
		s.pubMu.Unlock()
	}
}

func diagnosticsByFile(diags []diagnostic.Diagnostic) map[string][]diagnostic.Diagnostic {
	result := make(map[string][]diagnostic.Diagnostic)
	for _, d := range diags {
		if d.File != "" {
			result[d.File] = append(result[d.File], d)
		}
	}
	return result
}
