package playground

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/glade-sh/glade/internal/vm"
)

const (
	defaultPublicRunTimeout        = 5 * time.Second
	defaultPublicRatePerMinute     = 30
	defaultPublicMaxWorkspaceFiles = 64
	defaultPublicMaxWorkspaceBytes = 2 * 1024 * 1024
)

type Server struct {
	workspace         *Workspace
	runner            *Runner
	defaultLimitMode  vm.LimitMode
	dbPath            string
	projectRefs       []ProjectReference
	showExamples      bool
	public            bool
	runTimeout        time.Duration
	publicLimitCaps   vm.LimitCaps
	maxWorkspaceFiles int
	maxWorkspaceBytes int64
	rateLimiter       *fixedWindowRateLimiter
}

func NewServer(workspace *Workspace, opts ServerOptions) *Server {
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
		workspace:         workspace,
		runner:            NewRunner(workspace, RunnerOptions{Version: opts.Version, DBPath: opts.DBPath}),
		defaultLimitMode:  mode,
		dbPath:            opts.DBPath,
		projectRefs:       normalizeProjectReferences(opts.ProjectReferences),
		showExamples:      opts.ShowExamples,
		public:            opts.Public,
		runTimeout:        runTimeout,
		publicLimitCaps:   publicLimitCaps,
		maxWorkspaceFiles: maxWorkspaceFiles,
		maxWorkspaceBytes: maxWorkspaceBytes,
		rateLimiter:       newFixedWindowRateLimiter(ratePerMinute),
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.public && isPublicLimitedEndpoint(r) && !s.rateLimiter.allow(clientIP(r), time.Now()) {
		writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
		return
	}
	switch {
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
		writeJSON(w, http.StatusOK, map[string]bool{"reset": true})
	case r.Method == http.MethodPost && r.URL.Path == "/playground/api/seed":
		s.handleSeed(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/playground/api/runs/latest":
		latest, ok, err := s.runner.cache.Latest()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"found": ok, "result": latest})
	case r.Method == http.MethodGet && r.URL.Path == "/playground/api/database":
		writeJSON(w, http.StatusOK, databaseSnapshot(s.runner.CurrentOrg()))
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/playground/assets/"):
		s.serveStaticAsset(w, r)
	case r.Method == http.MethodGet && (r.URL.Path == "/playground/" || r.URL.Path == "/playground"):
		data, err := staticFiles.ReadFile("static/index.html")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
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
		_ = s.runner.cache.ClearLatest()
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
	w.Header().Set("Cache-Control", "no-cache")
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
	if !s.public {
		meta.DBPath = s.dbPath
	}
}

func (s *Server) handleSaveFile(w http.ResponseWriter, r *http.Request) {
	var req FileSaveRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if s.public {
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
	path := r.URL.Query().Get("path")
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
		req.LimitCaps = &s.publicLimitCaps
	}
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || strings.Contains(result.ErrorMessage, context.DeadlineExceeded.Error()) {
		result.Status = RunStatusRuntimeError
		result.ErrorMessage = "execution timed out"
		result.Diagnostics = []Diagnostic{{Severity: "error", Message: "execution timed out"}}
		writeJSON(w, http.StatusServiceUnavailable, result)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleSeed(w http.ResponseWriter, r *http.Request) {
	if s.public {
		writeError(w, http.StatusForbidden, "seed is disabled in public playground mode")
		return
	}
	reader := r.Body
	if r.ContentLength == 0 {
		file, err := os.Open(filepath.Join(s.workspace.Root, "seed.json"))
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
	full, err := s.workspace.SafePath(path)
	if err != nil {
		writeError(w, http.StatusNotFound, "unknown playground route")
		return
	}
	http.ServeFile(w, r, full)
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
	case "/playground/api/run", "/playground/api/examples/load", "/playground/api/files", "/playground/api/reset", "/playground/api/seed":
		return r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete
	default:
		return false
	}
}

func clientIP(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		first, _, _ := strings.Cut(forwarded, ",")
		if first = strings.TrimSpace(first); first != "" {
			return first
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
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
	full, err := s.workspace.SafePath(req.Path)
	if err != nil {
		return err
	}
	rel := slashRel(s.workspace.Root, full)
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
	return json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20)).Decode(out)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}
