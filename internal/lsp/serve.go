package lsp

import (
	"context"
	"encoding/json"
	"io"
	"reflect"
	"sort"

	"github.com/glade-sh/glade/internal/diagnostic"
	"sync"
	"time"
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
	dirty      map[string]struct{}
	refreshing bool
	refreshWG  sync.WaitGroup
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
	case "textDocument/didChange":
		var p DidChangeTextDocumentParams
		if decodeParams(req.Params, &p) != nil {
			return
		}
		if _, err := s.h.applyChange(p); err != nil {
			return
		}
		s.schedule(p.TextDocument.URI, s.h.debounce)
	case "textDocument/didSave":
		var p DidSaveTextDocumentParams
		if decodeParams(req.Params, &p) != nil {
			return
		}
		s.enqueueRefresh(pathFromURI(p.TextDocument.URI))
	case "textDocument/didClose":
		var p DidCloseTextDocumentParams
		if decodeParams(req.Params, &p) != nil {
			return
		}
		s.h.applyClose(p.TextDocument.URI)
		s.schedule(p.TextDocument.URI, 0)
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
	if !current || s.ctx.Err() != nil || (generation != nil && generation != s.h.gen.Load()) {
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
	s.reqWG.Wait()
	s.diagWG.Wait()
	s.refreshWG.Wait()
}

// shutdownNow cancels outstanding work and waits for it to stop.
func (s *server) shutdownNow() {
	s.cancel()
	s.mu.Lock()
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
		s.dirty = make(map[string]struct{})
	}
	s.dirty[path] = struct{}{}
	if !s.refreshing {
		s.refreshing = true
		s.refreshWG.Add(1)
		go s.refreshWorker()
	}
}

func (s *server) refreshWorker() {
	defer s.refreshWG.Done()
	for {
		s.mu.Lock()
		if len(s.dirty) == 0 || s.ctx.Err() != nil {
			s.refreshing = false
			s.mu.Unlock()
			return
		}
		paths := make([]string, 0, len(s.dirty))
		for path := range s.dirty {
			paths = append(paths, path)
		}
		s.dirty = make(map[string]struct{})
		s.mu.Unlock()
		sort.Strings(paths)
		if s.h.waitReady(s.ctx) != nil {
			return
		}
		before := s.h.gen.Load()
		oldResult := make(chan []diagnostic.Diagnostic, 1)
		go func() { oldResult <- before.analysisResult().Diagnostics }()
		var old []diagnostic.Diagnostic
		select {
		case old = <-oldResult:
		case <-s.ctx.Done():
			return
		}
		var after *generation
		var next []diagnostic.Diagnostic
		valid := false
		for attempt := 0; attempt < 2; attempt++ {
			err := s.h.refresh(s.ctx, paths, func(g *generation) {
				s.pubMu.Lock()
				s.h.gen.Store(g)
				s.pubMu.Unlock()
			})
			after = s.h.gen.Load()
			if err != nil || after == before {
				break
			}
			result := make(chan []diagnostic.Diagnostic, 1)
			go func(g *generation) { result <- g.analysisResult().Diagnostics }(after)
			select {
			case next = <-result:
			case <-s.ctx.Done():
				return
			}
			if s.h.validateGeneration == nil || s.h.validateGeneration(after.index) {
				valid = true
				break
			}
		}
		if !valid {
			continue
		}
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
			s.h.docMu.RUnlock()
			if open {
				s.schedule(uri, 0)
				continue
			}
			s.pubMu.Lock()
			if s.ctx.Err() == nil && s.h.gen.Load() == after {
				s.write(Notification{JSONRPC: "2.0", Method: "textDocument/publishDiagnostics", Params: BuildPublishDiagnostics(uri, newByFile[file])})
			}
			s.pubMu.Unlock()
		}
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
