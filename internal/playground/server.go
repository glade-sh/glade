package playground

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/glade-sh/glade/internal/vm"
)

const (
	defaultPublicRunTimeout        = 5 * time.Second
	defaultPublicBodyTimeout       = 5 * time.Second
	defaultPublicWriteTimeout      = 5 * time.Second
	defaultPublicOutputBytes       = 16 * 1024
	maxPublicResponseBytes         = 128 * 1024
	maxPublicDatabaseBytes         = 512 * 1024 // the seeded org alone is about 100 KiB of snapshot JSON
	maxPublicRequestBytes          = 1 << 20
	defaultPublicRatePerMinute     = 30
	defaultPublicMaxWorkspaceFiles = 64
	defaultPublicMaxWorkspaceBytes = 2 * 1024 * 1024
)

type Server struct {
	workspace           *Workspace
	runner              *Runner
	defaultLimitMode    vm.LimitMode
	dbPath              string
	projectRefs         []ProjectReference
	showExamples        bool
	public              bool
	runTimeout          time.Duration
	publicLimitCaps     vm.LimitCaps
	maxWorkspaceFiles   int
	maxWorkspaceBytes   int64
	ratePerMinute       int
	rateLimiter         *fixedWindowRateLimiter
	trustedProxy        string
	digitalOceanIngress bool
	version             string
	workspaceMutation   sync.Mutex
	sessions            *publicSessions
	requests            chan struct{}
}

func NewServer(workspace *Workspace, opts ServerOptions) *Server {
	serverOpts := opts
	if opts.Public {
		serverOpts.DBPath = ""
	}
	s := newServer(workspace, serverOpts)
	if opts.Public {
		s.requests = make(chan struct{}, 16)
		s.sessions = &publicSessions{opts: opts, items: make(map[string]*publicSession), creationLimiter: newFixedWindowRateLimiter(s.rateLimiter.limit)}
	}
	return s
}

func newServer(workspace *Workspace, opts ServerOptions) *Server {
	mode := opts.DefaultLimitMode
	if mode == "" {
		mode = vm.LimitModePermissive
	}
	runTimeout := opts.RunTimeout
	if opts.Public && runTimeout == 0 {
		runTimeout = defaultPublicRunTimeout
	}
	ratePerMinute := opts.RatePerMinute
	if opts.Public && ratePerMinute == 0 {
		ratePerMinute = defaultPublicRatePerMinute
	}
	maxWorkspaceFiles := opts.MaxWorkspaceFiles
	if opts.Public && maxWorkspaceFiles == 0 {
		maxWorkspaceFiles = defaultPublicMaxWorkspaceFiles
	}
	maxWorkspaceBytes := opts.MaxWorkspaceBytes
	if opts.Public && maxWorkspaceBytes == 0 {
		maxWorkspaceBytes = defaultPublicMaxWorkspaceBytes
	}
	publicLimitCaps := opts.PublicLimitCaps
	if opts.Public && publicLimitCaps == (vm.LimitCaps{}) {
		publicLimitCaps = defaultPublicLimitCaps()
	}
	return &Server{
		workspace:           workspace,
		runner:              NewRunner(workspace, RunnerOptions{Version: opts.Version, DBPath: opts.DBPath}),
		defaultLimitMode:    mode,
		dbPath:              opts.DBPath,
		projectRefs:         normalizeProjectReferences(opts.ProjectReferences),
		showExamples:        opts.ShowExamples,
		public:              opts.Public,
		runTimeout:          runTimeout,
		publicLimitCaps:     publicLimitCaps,
		maxWorkspaceFiles:   maxWorkspaceFiles,
		maxWorkspaceBytes:   maxWorkspaceBytes,
		ratePerMinute:       ratePerMinute,
		rateLimiter:         newFixedWindowRateLimiter(ratePerMinute),
		trustedProxy:        opts.TrustedProxy,
		digitalOceanIngress: opts.DigitalOceanIngress,
		version:             opts.Version,
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.public {
		w.Header().Set("Cache-Control", "no-store")
		var ok bool
		w, ok = publicResponseWriter(w)
		if !ok {
			return
		}
	}
	if r.Method == http.MethodGet && (r.URL.Path == "/healthz" || r.URL.Path == "/readyz") {
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Path == "/healthz" {
			writeJSON(w, http.StatusOK, PlaygroundHealth{Status: "ok", Version: s.version})
			return
		}
		if s.workspace == nil || s.runner == nil || s.runner.initErr != nil {
			writeJSON(w, http.StatusServiceUnavailable, PlaygroundHealth{Status: "unavailable"})
			return
		}
		if _, err := s.workspace.Metadata(); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, PlaygroundHealth{Status: "unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, PlaygroundHealth{Status: "ready", Version: s.version})
		return
	}
	if s.public && r.ContentLength > maxPublicRequestBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "public request body too large")
		return
	}
	if s.requests != nil {
		select {
		case s.requests <- struct{}{}:
			defer func() { <-s.requests }()
		default:
			writeError(w, http.StatusServiceUnavailable, "public playground is busy")
			return
		}
	}
	if s.public && r.Body != nil && r.Body != http.NoBody {
		controller := http.NewResponseController(w)
		if err := controller.SetReadDeadline(time.Now().Add(defaultPublicBodyTimeout)); err != nil {
			writeError(w, http.StatusServiceUnavailable, "public request deadlines are unavailable")
			return
		}
		defer func() { _ = controller.SetReadDeadline(time.Time{}) }()
	}
	if s.public && r.Method != http.MethodGet && r.Method != http.MethodHead && !publicSameOriginAtIngress(r, s.trustedProxy, s.digitalOceanIngress) {
		writeError(w, http.StatusForbidden, "cross-origin request denied")
		return
	}
	if s.public && isPublicLimitedEndpoint(r) && !s.rateLimiter.allow(publicIngressClientIP(r, s.trustedProxy, s.digitalOceanIngress), time.Now()) {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
		return
	}
	if s.sessions != nil && r.Method == http.MethodGet {
		if strings.HasPrefix(r.URL.Path, "/playground/assets/") {
			s.serveHTTP(w, r)
			return
		}
		needsSession := r.URL.Path == "/playground" || r.URL.Path == "/playground/"
		switch r.URL.Path {
		case "/playground/api/workspace", "/playground/api/examples", "/playground/api/runs/latest", "/playground/api/database":
			needsSession = true
		}
		if !needsSession {
			if _, err := r.Cookie(publicSessionCookie); err != nil {
				writeError(w, http.StatusNotFound, "unknown playground route")
				return
			}
		}
	}
	if s.sessions != nil {
		s.sessions.serve(w, r, s.rateLimiter)
		return
	}
	s.serveHTTP(w, r)
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/healthz":
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, PlaygroundHealth{Status: "ok", Version: s.version})
	case r.Method == http.MethodGet && r.URL.Path == "/readyz":
		s.handleReadiness(w)
	case r.Method == http.MethodGet && r.URL.Path == "/playground/api/workspace":
		s.handleWorkspace(w)
	case r.Method == http.MethodGet && r.URL.Path == "/playground/api/examples":
		writeJSON(w, http.StatusOK, map[string]any{"examples": s.listExamples(), "canLoad": s.workspace.Managed})
	case r.Method == http.MethodPost && r.URL.Path == "/playground/api/examples/load":
		s.handleLoadExample(w, r)
	case r.Method == http.MethodPut && r.URL.Path == "/playground/api/files":
		s.handleSaveFile(w, r)
	case r.Method == http.MethodDelete && r.URL.Path == "/playground/api/files":
		s.handleDeleteFile(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/playground/api/run":
		s.handleRun(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/playground/api/reset":
		if err := s.runner.Reset(); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if s.runner.cache != nil {
			if err := s.runner.cache.ClearLatest(); err != nil {
				writeError(w, http.StatusInternalServerError, "could not clear the previous run result")
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]bool{"reset": true})
	case r.Method == http.MethodPost && r.URL.Path == "/playground/api/seed":
		s.handleSeed(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/playground/api/runs/latest":
		if s.public {
			writeJSON(w, http.StatusOK, map[string]any{"found": false})
			return
		}
		latest, ok, err := s.runner.cache.Latest()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"found": ok, "result": latest})
	case r.Method == http.MethodGet && r.URL.Path == "/playground/api/database":
		if s.public {
			snapshot, ok := s.runner.publicDatabaseSnapshot()
			if !ok {
				writeError(w, http.StatusRequestEntityTooLarge, "public output limit exceeded")
				return
			}
			data, err := json.Marshal(snapshot)
			if err != nil || len(data) > maxPublicDatabaseBytes {
				writeError(w, http.StatusRequestEntityTooLarge, "public output limit exceeded")
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data)
		} else {
			writeJSON(w, http.StatusOK, databaseSnapshot(s.runner.CurrentOrg()))
		}
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/playground/assets/"):
		s.serveStaticAsset(w, r)
	case r.Method == http.MethodGet && (r.URL.Path == "/playground/" || r.URL.Path == "/playground"):
		data, err := staticFiles.ReadFile("static/index.html")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if s.public {
			w.Header().Set("Cache-Control", "no-store")
		}
		_, _ = w.Write(data)
	case r.Method == http.MethodGet:
		s.serveWorkspaceFile(w, r)
	default:
		writeError(w, http.StatusNotFound, "unknown playground route")
	}
}

func (s *Server) handleLoadExample(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.workspaceMutation.Lock()
	defer s.workspaceMutation.Unlock()
	meta, err := s.loadSource(req.ID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.runner.Reset(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.runner.InvalidateSourceRuntime()
	if s.runner.cache != nil {
		if err := s.runner.cache.ClearLatest(); err != nil {
			writeError(w, http.StatusInternalServerError, "could not clear the previous run result")
			return
		}
	}
	s.decorateWorkspaceMetadata(&meta)
	writeJSON(w, http.StatusOK, meta)
}

func (s *Server) effectiveLimitMode() vm.LimitMode {
	if s.public {
		return vm.LimitModeStrict
	}
	return s.defaultLimitMode
}

func (s *Server) listExamples() []ExampleProject {
	examples := make([]ExampleProject, 0)
	if s.showExamples && len(s.projectRefs) == 0 {
		examples = ListExampleProjects()
	}
	for _, ref := range s.projectRefs {
		examples = append(examples, s.projectReferenceExample(ref))
	}
	sort.Slice(examples, func(i, j int) bool {
		if examples[i].Source != examples[j].Source {
			return examples[i].Source < examples[j].Source
		}
		return examples[i].Name < examples[j].Name
	})
	return examples
}

func (s *Server) projectReferenceExample(ref ProjectReference) ExampleProject {
	tags := append([]string{"local"}, ref.Tags...)
	description := ref.Description
	if description == "" {
		description = ref.Path
	}
	return ExampleProject{
		ID:          ref.ID,
		Name:        ref.Name,
		Description: description,
		Tags:        tags,
		FileCount:   0,
		Source:      "local",
		Path:        ref.Path,
	}
}

func (s *Server) loadSource(id string) (WorkspaceMetadata, error) {
	if s.showExamples && len(s.projectRefs) == 0 {
		if _, ok := exampleTemplateByID(id); ok {
			return s.workspace.LoadExample(id)
		}
	}
	if len(s.projectRefs) == 0 {
		if _, ok := exampleTemplateByID(id); ok {
			return WorkspaceMetadata{}, fmt.Errorf("built-in playground examples require --examples")
		}
	}
	for _, ref := range s.projectRefs {
		if ref.ID == id {
			if s.public {
				files, err := collectProjectReferenceFilesLimited(ref, s.maxWorkspaceFiles, s.maxWorkspaceBytes)
				if err != nil {
					return WorkspaceMetadata{}, err
				}
				if err := validatePublicManifest(files["sfdx-project.json"]); err != nil {
					return WorkspaceMetadata{}, err
				}
				return s.workspace.loadFiles(ref.ID, files, true)
			}
			return s.workspace.LoadProjectReference(ref)
		}
	}
	return WorkspaceMetadata{}, fmt.Errorf("unknown playground source %q", id)
}

func normalizeProjectReferences(refs []ProjectReference) []ProjectReference {
	out := make([]ProjectReference, 0, len(refs))
	used := make(map[string]int)
	for _, ref := range refs {
		ref.Path = strings.TrimSpace(ref.Path)
		if ref.Path == "" {
			continue
		}
		if abs, err := filepath.Abs(ref.Path); err == nil {
			ref.Path = abs
		}
		ref.Name = strings.TrimSpace(ref.Name)
		if ref.Name == "" {
			ref.Name = filepath.Base(ref.Path)
		}
		ref.ID = strings.TrimSpace(ref.ID)
		if ref.ID == "" {
			ref.ID = "local-" + slugID(ref.Name)
		}
		if n := used[ref.ID]; n > 0 {
			used[ref.ID] = n + 1
			ref.ID = fmt.Sprintf("%s-%d", ref.ID, n+1)
		} else {
			used[ref.ID] = 1
		}
		ref.Tags = append([]string(nil), ref.Tags...)
		out = append(out, ref)
	}
	return out
}

func slugID(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		if b.Len() > 0 {
			last := b.String()[b.Len()-1]
			if last != '-' {
				b.WriteByte('-')
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "project"
	}
	return out
}

func (s *Server) serveStaticAsset(w http.ResponseWriter, r *http.Request) {
	asset := strings.TrimPrefix(r.URL.Path, "/playground/")
	if asset == "" || strings.Contains(asset, "..") || strings.Contains(asset, "\\") {
		writeError(w, http.StatusNotFound, "unknown playground asset")
		return
	}
	data, err := staticFiles.ReadFile("static/" + asset)
	if err != nil {
		writeError(w, http.StatusNotFound, "unknown playground asset")
		return
	}
	if contentType := mime.TypeByExtension(filepath.Ext(asset)); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	if !s.public {
		w.Header().Set("Cache-Control", "no-cache")
	}
	_, _ = w.Write(data)
}

func (s *Server) handleWorkspace(w http.ResponseWriter) {
	meta, err := s.workspace.Metadata()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.decorateWorkspaceMetadata(&meta)
	writeJSON(w, http.StatusOK, meta)
}

func (s *Server) decorateWorkspaceMetadata(meta *WorkspaceMetadata) {
	meta.LimitMode = s.effectiveLimitMode()
	meta.Policy = PlaygroundPolicy{
		Public:            s.public,
		RunMode:           "selectable",
		LimitMode:         s.effectiveLimitMode(),
		RunTimeoutMS:      s.runTimeout.Milliseconds(),
		RatePerMinute:     s.ratePerMinute,
		MaxWorkspaceFiles: s.maxWorkspaceFiles,
		MaxWorkspaceBytes: s.maxWorkspaceBytes,
		LimitCaps:         s.publicLimitCaps,
	}
	if s.public {
		meta.Policy.RunMode = string(RunModeScratch)
	}
	if !s.public {
		meta.DBPath = s.dbPath
	}
}

func (s *Server) handleReadiness(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	if s == nil || s.workspace == nil || s.runner == nil {
		writeJSON(w, http.StatusServiceUnavailable, PlaygroundHealth{Status: "unavailable"})
		return
	}
	if err := s.runner.readinessError(); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, PlaygroundHealth{Status: "unavailable"})
		return
	}
	if _, err := s.workspace.Metadata(); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, PlaygroundHealth{Status: "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, PlaygroundHealth{Status: "ready", Version: s.version})
}

func (s *Server) handleSaveFile(w http.ResponseWriter, r *http.Request) {
	var req FileSaveRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.workspaceMutation.Lock()
	defer s.workspaceMutation.Unlock()
	if s.public {
		rel, err := normalizedWorkspacePath(req.Path)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		clean := strings.ToLower(rel)
		if clean == "glade.yml" || clean == "glade.yaml" {
			writeError(w, http.StatusForbidden, "public playground configuration is server managed")
			return
		}
		if clean == "sfdx-project.json" {
			if err := validatePublicManifest(req.Content); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
		}
		if err := s.checkPublicWorkspaceBudget(req); err != nil {
			writeError(w, http.StatusRequestEntityTooLarge, err.Error())
			return
		}
	}
	resp, err := s.workspace.SaveFile(req)
	if err != nil {
		var readOnly ErrReadOnlyFile
		if errors.As(err, &readOnly) {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		var conflict ErrVersionConflict
		if errors.As(err, &conflict) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if shouldInvalidateRuntimeForFile(resp.File.Path) {
		s.runner.InvalidateSourceRuntime()
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	s.workspaceMutation.Lock()
	defer s.workspaceMutation.Unlock()
	path := r.URL.Query().Get("path")
	if s.public {
		rel, err := normalizedWorkspacePath(path)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		clean := strings.ToLower(rel)
		if clean == "glade.yml" || clean == "glade.yaml" {
			writeError(w, http.StatusForbidden, "public playground configuration is server managed")
			return
		}
	}
	if err := s.workspace.DeleteFile(path); err != nil {
		var readOnly ErrReadOnlyFile
		if errors.As(err, &readOnly) {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if shouldInvalidateRuntimeForFile(path) {
		s.runner.InvalidateSourceRuntime()
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func shouldInvalidateRuntimeForFile(path string) bool {
	switch fileKind(path) {
	case "class", "trigger", "metadata":
		return true
	case "other":
		clean := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(path)), "./")
		return clean == "force-app" || strings.HasPrefix(clean, "force-app/")
	default:
		return false
	}
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	var req RunRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if s.public {
		s.workspaceMutation.Lock()
		defer s.workspaceMutation.Unlock()
		if err := installPublicConfig(s.workspace); err != nil {
			writeError(w, http.StatusInternalServerError, "cannot initialize session configuration")
			return
		}
	}
	if req.LimitMode == "" {
		req.LimitMode = s.defaultLimitMode
	}
	ctx := r.Context()
	var cancel context.CancelFunc
	if s.runTimeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, s.runTimeout)
		defer cancel()
	}
	if s.public {
		req.Mode = RunModeScratch
		req.LimitMode = vm.LimitModeStrict
		req.UseCache = false
		req.disableCacheWrite = true
		req.LimitCaps = &s.publicLimitCaps
		req.OutputLimit = defaultPublicOutputBytes
	}
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || strings.Contains(result.ErrorMessage, context.DeadlineExceeded.Error()) {
		message := "execution timed out"
		if s.runTimeout > 0 {
			message = fmt.Sprintf("execution timed out after %s; try a smaller example or reduce the work in the loop", s.runTimeout)
		}
		result.Status = RunStatusRuntimeError
		result.ErrorMessage = message
		result.Diagnostics = []Diagnostic{{Severity: "error", Message: message}}
		if s.public {
			writePublicRunResult(w, http.StatusServiceUnavailable, result)
		} else {
			writeJSON(w, http.StatusServiceUnavailable, result)
		}
		return
	}
	if s.public {
		writePublicRunResult(w, http.StatusOK, result)
	} else {
		writeJSON(w, http.StatusOK, result)
	}
}

func writePublicRunResult(w http.ResponseWriter, status int, result RunResult) {
	var ok bool
	w, ok = publicResponseWriter(w)
	if !ok {
		return
	}
	if !publicResultFitsBudget(result) {
		writeError(w, http.StatusRequestEntityTooLarge, "public output limit exceeded")
		return
	}
	data, err := json.Marshal(result)
	if err != nil || len(data) > maxPublicResponseBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "public output limit exceeded")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func (s *Server) handleSeed(w http.ResponseWriter, r *http.Request) {
	if s.public {
		writeError(w, http.StatusForbidden, "seed is disabled in public playground mode")
		return
	}
	reader := r.Body
	if r.ContentLength == 0 {
		file, _, err := s.workspace.openFile("seed.json")
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		defer file.Close()
		reader = file
	}
	defer r.Body.Close()
	if err := s.runner.Seed(reader); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"seeded": true})
}

func (s *Server) serveWorkspaceFile(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	file, info, err := s.workspace.openFile(path)
	if err != nil {
		writeError(w, http.StatusNotFound, "unknown playground route")
		return
	}
	defer file.Close()
	if contentType := mime.TypeByExtension(filepath.Ext(path)); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	modified := info.ModTime()
	if s.public {
		// Session files must not use a shared cache validator from another visitor.
		modified = time.Time{}
	}
	http.ServeContent(w, r, filepath.Base(path), modified, file)
}

func defaultPublicLimitCaps() vm.LimitCaps {
	return vm.LimitCaps{
		Queries:       25,
		QueryRows:     5000,
		DMLStatements: 20,
		DMLRows:       1000,
		HeapSize:      2 * 1024 * 1024,
		CPUTimeMS:     2000,
		Callouts:      0,
		AsyncJobs:     0,
		FutureCalls:   0,
		QueueableJobs: 0,
		BatchJobs:     0,
		ScheduledJobs: 0,
		EmailInvokes:  0,
	}
}

func isPublicLimitedEndpoint(r *http.Request) bool {
	switch r.URL.Path {
	case "/playground/api/database":
		return r.Method == http.MethodGet
	case "/playground/api/run", "/playground/api/examples/load", "/playground/api/files", "/playground/api/reset", "/playground/api/seed":
		return r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete
	default:
		return false
	}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

// publicClientIP trusts exactly one forwarded address only from the configured
// proxy. The proxy must overwrite, not append to, incoming forwarding headers.
func publicClientIP(r *http.Request, trustedProxy string) string {
	peer := clientIP(r)
	trusted, remote := net.ParseIP(trustedProxy), net.ParseIP(peer)
	if trusted == nil || remote == nil || !trusted.Equal(remote) {
		return peer
	}
	values := r.Header.Values("X-Forwarded-For")
	if len(values) == 1 {
		if forwarded := net.ParseIP(strings.TrimSpace(values[0])); forwarded != nil {
			return forwarded.String()
		}
	}
	return peer
}

// publicIngressClientIP only trusts the platform header when the deployment
// explicitly opts into its provider-controlled ingress boundary.
func publicIngressClientIP(r *http.Request, trustedProxy string, digitalOceanIngress bool) string {
	if digitalOceanIngress {
		values := r.Header.Values("Do-Connecting-Ip")
		if len(values) == 1 {
			if forwarded := net.ParseIP(strings.TrimSpace(values[0])); forwarded != nil {
				return forwarded.String()
			}
		}
		return clientIP(r)
	}
	return publicClientIP(r, trustedProxy)
}

func publicSameOrigin(r *http.Request) bool {
	return publicSameOriginAtIngress(r, "", false)
}

// publicSameOriginAtIngress compares the browser origin with the externally
// visible origin. Forwarded schemes are trusted only from a configured proxy.
func publicSameOriginAtIngress(r *http.Request, trustedProxy string, digitalOceanIngress bool) bool {
	if strings.EqualFold(r.Header.Get("Sec-Fetch-Site"), "cross-site") {
		return false
	}
	origins := r.Header.Values("Origin")
	if len(origins) == 0 {
		return true
	}
	if len(origins) != 1 || origins[0] == "" {
		return false
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if digitalOceanIngress {
		// App Platform upgrades external HTTP requests to HTTPS.
		scheme = "https"
	} else if trusted, peer := net.ParseIP(trustedProxy), net.ParseIP(clientIP(r)); trusted != nil && peer != nil && trusted.Equal(peer) {
		values := r.Header.Values("X-Forwarded-Proto")
		if len(values) != 1 || (values[0] != "http" && values[0] != "https") {
			return false
		}
		scheme = values[0]
	}
	parsed, err := url.Parse(origins[0])
	if err != nil || parsed.Scheme != scheme || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || parsed.Opaque != "" {
		return false
	}
	originHost, ok := publicOriginAuthority(parsed.Host, scheme)
	if !ok {
		return false
	}
	requestHost, ok := publicOriginAuthority(r.Host, scheme)
	return ok && originHost == requestHost
}

func publicOriginAuthority(host, scheme string) (string, bool) {
	parsed, err := url.Parse(scheme + "://" + host)
	if err != nil || parsed.User != nil || parsed.Host != host || parsed.Hostname() == "" || parsed.Path != "" || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return "", false
	}
	port := parsed.Port()
	if port == "" {
		if scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return net.JoinHostPort(strings.ToLower(parsed.Hostname()), port), true
}

type fixedWindowRateLimiter struct {
	mu      sync.Mutex
	limit   int
	windows map[string]rateWindow
}

type rateWindow struct {
	start time.Time
	count int
}

func newFixedWindowRateLimiter(limit int) *fixedWindowRateLimiter {
	return &fixedWindowRateLimiter{limit: limit, windows: make(map[string]rateWindow)}
}

func (l *fixedWindowRateLimiter) allow(key string, now time.Time) bool {
	if l == nil || l.limit <= 0 {
		return true
	}
	if key == "" {
		key = "unknown"
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	window := l.windows[key]
	if window.start.IsZero() || now.Sub(window.start) >= time.Minute {
		l.windows[key] = rateWindow{start: now, count: 1}
		l.pruneLocked(now)
		return true
	}
	if window.count >= l.limit {
		return false
	}
	window.count++
	l.windows[key] = window
	return true
}

func (l *fixedWindowRateLimiter) pruneLocked(now time.Time) {
	for key, window := range l.windows {
		if now.Sub(window.start) >= 2*time.Minute {
			delete(l.windows, key)
		}
	}
}

func (s *Server) checkPublicWorkspaceBudget(req FileSaveRequest) error {
	rel, err := normalizedWorkspacePath(req.Path)
	if err != nil {
		return err
	}
	for _, component := range strings.Split(rel, "/") {
		if strings.HasPrefix(component, ".") {
			return fmt.Errorf("hidden workspace paths are not allowed")
		}
	}
	meta, err := s.workspace.Metadata()
	if err != nil {
		return err
	}
	total := int64(len(req.Content))
	found := false
	for _, file := range meta.Files {
		if file.Path == rel {
			found = true
			continue
		}
		total += file.Size
	}
	if !found && s.maxWorkspaceFiles > 0 && len(meta.Files) >= s.maxWorkspaceFiles {
		return fmt.Errorf("workspace file limit exceeded: %d", s.maxWorkspaceFiles)
	}
	if s.maxWorkspaceBytes > 0 && total > s.maxWorkspaceBytes {
		return fmt.Errorf("workspace size limit exceeded: %d bytes", s.maxWorkspaceBytes)
	}
	return nil
}

func decodeJSON(r *http.Request, out any) error {
	defer r.Body.Close()
	return json.NewDecoder(http.MaxBytesReader(nil, r.Body, maxPublicRequestBytes)).Decode(out)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}

// validatePublicManifest confines package paths interpreted by the project loader.
func validatePublicManifest(content string) error {
	var manifest struct {
		PackageDirectories []struct {
			Path string `json:"path"`
		} `json:"packageDirectories"`
	}
	if err := json.Unmarshal([]byte(content), &manifest); err != nil {
		return fmt.Errorf("invalid project manifest: %w", err)
	}
	for _, dir := range manifest.PackageDirectories {
		path := filepath.FromSlash(strings.ReplaceAll(dir.Path, "\\", "/"))
		if !filepath.IsLocal(path) {
			return fmt.Errorf("public package directory must remain within the workspace")
		}
	}
	return nil
}
