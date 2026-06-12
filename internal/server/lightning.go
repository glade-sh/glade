package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/lwc/compile"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	lwcembed "github.com/glade-sh/glade/internal/lwcruntime/embed"
)

type lightningState struct {
	cacheDir  string
	compiled  compile.Manifest
	manifest  lwcbrowser.Manifest
	pageCfg   lwcbrowser.PageConfig
}

func (s *Server) ResetLightningCache() {
	if s == nil {
		return
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
	if _, err := gladehome.EnsureRoot(); err != nil {
		return err
	}
	cacheDir := filepath.Join(os.TempDir(), "glade-lwc-cache")
	if s.Source.Project.Root != "" {
		cacheDir = filepath.Join(cacheDir, filepath.Base(s.Source.Project.Root))
	}
	cfg, compiled, err := lwcbrowser.PreparePageConfig(s.Source.Project, cacheDir)
	if err != nil {
		return err
	}
	s.lightning = lightningState{
		cacheDir: filepath.Join(cacheDir, "lwc"),
		compiled: compiled,
		manifest: cfg.Manifest,
		pageCfg:  cfg,
	}
	return nil
}

func (s *Server) lightningBootstrapConfigLocked() (*lwcbrowser.PageConfig, bool) {
	if err := s.ensureLightningLocked(); err != nil || len(s.lightning.manifest.Modules) == 0 {
		return nil, false
	}
	cfg := s.lightning.pageCfg
	return &cfg, true
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
		w.Header().Set("Cache-Control", "public, max-age=300")
		_, _ = w.Write(lwcembed.OutJS)
	case "vendor":
		s.handleLightningVendor(w, r, parts[1:])
	case "modules":
		s.handleLightningModules(w, r, parts[1:])
	case "shims":
		s.handleLightningShims(w, r, parts[1:])
	case "wire":
		s.handleLightningWire(w, r, parts[1:])
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
	w.Header().Set("Cache-Control", "public, max-age=3600")
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
	w.Header().Set("Cache-Control", "public, max-age=60")
	_, _ = w.Write(content)
}
