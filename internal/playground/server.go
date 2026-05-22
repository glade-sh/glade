package playground

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/open-aer/oaer/internal/vm"
)

type Server struct {
	workspace        *Workspace
	runner           *Runner
	defaultLimitMode vm.LimitMode
	projectRefs      []ProjectReference
	showExamples     bool
}

func NewServer(workspace *Workspace, opts ServerOptions) *Server {
	mode := opts.DefaultLimitMode
	if mode == "" {
		mode = vm.LimitModePermissive
	}
	return &Server{
		workspace:        workspace,
		runner:           NewRunner(workspace, RunnerOptions{Version: opts.Version, DBPath: opts.DBPath}),
		defaultLimitMode: mode,
		projectRefs:      normalizeProjectReferences(opts.ProjectReferences),
		showExamples:     opts.ShowExamples,
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
		s.runner.Reset()
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
	s.runner.Reset()
	if s.runner.cache != nil {
		_ = s.runner.cache.ClearLatest()
	}
	meta.LimitMode = s.defaultLimitMode
	writeJSON(w, http.StatusOK, meta)
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
	meta.LimitMode = s.defaultLimitMode
	writeJSON(w, http.StatusOK, meta)
}

func (s *Server) handleSaveFile(w http.ResponseWriter, r *http.Request) {
	var req FileSaveRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
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
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
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
	result, err := s.runner.Run(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleSeed(w http.ResponseWriter, r *http.Request) {
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
