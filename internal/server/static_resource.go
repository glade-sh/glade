package server

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/visualforce"
)

func (s *Server) handleStaticResource(w http.ResponseWriter, r *http.Request, parts []string) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	if len(parts) < 1 {
		writeSalesforceError(w, errUnknownEndpoint, "missing static resource name")
		return
	}
	name := strings.TrimSpace(parts[0])
	if name == "" {
		writeSalesforceError(w, errUnknownEndpoint, "missing static resource name")
		return
	}
	subpath := strings.Join(parts[1:], "/")

	contentPath, contentType, ok := s.lookupStaticResource(name, subpath)
	if !ok {
		writeSalesforceError(w, errUnknownEndpoint, "unknown static resource")
		return
	}
	content, err := os.ReadFile(contentPath)
	if err != nil {
		writeSalesforceError(w, errUnknownEndpoint, "static resource not readable")
		return
	}
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	} else if ext := filepath.Ext(contentPath); ext != "" {
		if guessed := mime.TypeByExtension(ext); guessed != "" {
			w.Header().Set("Content-Type", guessed)
		}
	}
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (s *Server) lookupStaticResource(name, subpath string) (path string, contentType string, ok bool) {
	if s.Org != nil {
		for _, resource := range s.Org.Metadata.StaticResources {
			if strings.EqualFold(resource.Name, name) {
				if subpath == "" && resource.ContentPath != "" {
					return resource.ContentPath, resource.ContentType, true
				}
				if subpath == "" && resource.Content != "" {
					return writeInlineStaticResource(name, resource.Content)
				}
			}
		}
	}
	for _, resource := range s.Source.ToolingOrg.Metadata.StaticResources {
		if strings.EqualFold(resource.Name, name) {
			if subpath == "" && resource.ContentPath != "" {
				return resource.ContentPath, resource.ContentType, true
			}
		}
	}
	if s.Source.Project.Root != "" {
		if resolved, err := visualforce.ResolveStaticResourceFile(s.Source.Project.Root, name, subpath); err == nil {
			return resolved, "", true
		}
	}
	for _, path := range s.Source.Project.StaticResourceFiles {
		base := strings.TrimSuffix(filepath.Base(path), ".resource")
		if strings.EqualFold(base, name) && subpath == "" {
			return path, "", true
		}
	}
	return "", "", false
}

func writeInlineStaticResource(name, content string) (string, string, bool) {
	dir := filepath.Join(os.TempDir(), "glade-static-resource")
	_ = os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, name+".resource")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", "", false
	}
	return path, "", true
}

func resourceForName(resources []storage.StaticResourceMetadata, name string) (storage.StaticResourceMetadata, bool) {
	needle := strings.ToLower(name)
	for _, resource := range resources {
		if strings.EqualFold(resource.Name, needle) {
			return resource, true
		}
	}
	return storage.StaticResourceMetadata{}, false
}
