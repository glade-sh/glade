package server

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/lwc/compile"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	lwcembed "github.com/glade-sh/glade/internal/lwcruntime/embed"
	"github.com/glade-sh/glade/internal/project"
)

type lightningState struct {
	cacheRoot string
	cacheDir  string
	compiled  compile.Manifest
	manifest  lwcbrowser.Manifest
	pageCfg   lwcbrowser.PageConfig
}

var ensureLightningRoot = gladehome.EnsureRoot

func (s *Server) ResetLightningCache() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resetLightningCacheLocked()
}

func (s *Server) resetLightningCacheLocked() {
	if s.lightning.cacheRoot != "" {
		_ = os.RemoveAll(s.lightning.cacheRoot)
	}
	s.lightning = lightningState{}
}

func (s *Server) ensureLightningLocked() error {
	if s == nil {
		return nil
	}
	if s.lightning.cacheDir != "" && len(s.lightning.manifest.Modules) > 0 {
		return nil
	}
	if _, err := ensureLightningRoot(); err != nil {
		return err
	}
	cacheRoot := lightningCacheRoot(s.Source.Project)
	if err := os.RemoveAll(cacheRoot); err != nil {
		return err
	}
	cfg, compiled, err := lwcbrowser.PreparePageConfig(s.Source.Project, cacheRoot)
	if err != nil {
		return err
	}
	s.lightning = lightningState{
		cacheRoot: cacheRoot,
		cacheDir:  filepath.Join(cacheRoot, "lwc"),
		compiled:  compiled,
		manifest:  cfg.Manifest,
		pageCfg:   cfg,
	}
	return nil
}

func lightningCacheRoot(p project.Project) string {
	root := strings.TrimSpace(p.Root)
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	if root == "" {
		root = "unknown-project"
	}
	sum := sha256.Sum256([]byte(filepath.Clean(root)))
	name := filepath.Base(filepath.Clean(root))
	if name == "." || name == string(filepath.Separator) || name == "" {
		name = "project"
	}
	return filepath.Join(os.TempDir(), "glade-lwc-cache", fmt.Sprintf("%s-%x", name, sum[:6]))
}

func (s *Server) lightningBootstrapConfigLocked() (*lwcbrowser.PageConfig, bool, error) {
	if err := s.ensureLightningLocked(); err != nil {
		return nil, false, err
	}
	if len(s.lightning.manifest.Modules) == 0 {
		return nil, false, nil
	}
	cfg := s.lightning.pageCfg
	return &cfg, true, nil
}

func (s *Server) handleLightning(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) == 0 {
		writeSalesforceError(w, errUnknownEndpoint, "missing lightning path")
		return
	}
	switch parts[0] {
	case "glade.out.js":
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		setDevNoStore(w)
		_, _ = w.Write(lwcembed.OutJS)
	case "vendor":
		s.handleLightningVendor(w, r, parts[1:])
	case "modules":
		s.handleLightningModules(w, r, parts[1:])
	case "shims":
		s.handleLightningShims(w, r, parts[1:])
	case "wire":
		s.handleLightningWire(w, r, parts[1:])
	case "apex":
		s.handleLightningApex(w, r, parts[1:])
	case "runtime":
		s.serveLightningShellRuntime(w, r, parts[1:])
	case "local":
		s.handleLightningLocal(w, r, parts[1:])
	default:
		writeSalesforceError(w, errUnknownEndpoint, "unknown lightning endpoint")
	}
}

func (s *Server) handleLightningVendor(w http.ResponseWriter, r *http.Request, parts []string) {
	if r.Method != http.MethodGet || len(parts) != 1 {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	toolchain, err := gladehome.LWCToolchainDir()
	if err != nil {
		writeSalesforceError(w, errUnsupportedFeature, err.Error())
		return
	}
	var rel string
	switch parts[0] {
	case "lwc.js":
		rel = filepath.Join("@lwc", "engine-dom", "dist", "index.js")
	case "synthetic-shadow.js":
		rel = filepath.Join("@lwc", "synthetic-shadow", "dist", "index.js")
	default:
		writeSalesforceError(w, errUnknownEndpoint, "unknown lightning vendor")
		return
	}
	path := filepath.Join(toolchain, "node_modules", rel)
	content, err := os.ReadFile(path)
	if err != nil {
		writeSalesforceError(w, errUnknownEndpoint, "lwc vendor missing")
		return
	}
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	setDevNoStore(w)
	_, _ = w.Write(content)
}

func (s *Server) handleLightningModules(w http.ResponseWriter, r *http.Request, parts []string) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	if err := s.ensureLightningLocked(); err != nil {
		writeSalesforceError(w, errUnsupportedFeature, err.Error())
		return
	}
	rel := strings.Join(parts, "/")
	if rel == "" || strings.Contains(rel, "..") {
		writeSalesforceError(w, errUnknownEndpoint, "invalid module path")
		return
	}
	path := filepath.Join(s.lightning.cacheDir, filepath.FromSlash(rel))
	content, err := os.ReadFile(path)
	if err != nil && !strings.HasSuffix(rel, ".js") {
		path = path + ".js"
		content, err = os.ReadFile(path)
	}
	if err != nil {
		writeSalesforceError(w, errUnknownEndpoint, "unknown lightning module")
		return
	}
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	setDevNoStore(w)
	_, _ = w.Write(content)
}
