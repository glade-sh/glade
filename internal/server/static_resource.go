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
	setDevNoStore(w)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (s *Server) lookupStaticResource(name, subpath string) (path string, contentType string, ok bool) {
	if s.Org != nil {
		for _, resource := range s.Org.Metadata.StaticResources {
			if strings.EqualFold(resource.Name, name) {
				if subpath != "" {
					if resolved, ok := staticResourceSubpath(resource, subpath); ok {
						return resolved, resource.ContentType, true
					}
				}
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
			if subpath != "" {
				if resolved, ok := staticResourceSubpath(resource, subpath); ok {
					return resolved, resource.ContentType, true
				}
			}
			if subpath == "" && resource.ContentPath != "" {
				return resource.ContentPath, resource.ContentType, true
			}
		}
	}
	for _, path := range s.Source.Project.StaticResourceFiles {
		resourceName, resourceSubpath, ok := projectStaticResourceNameAndSubpath(path)
		if ok && strings.EqualFold(resourceName, name) && subpath != "" && cleanResourceSubpath(resourceSubpath) == cleanResourceSubpath(subpath) {
			return path, "", true
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

func projectStaticResourceNameAndSubpath(file string) (name string, subpath string, ok bool) {
	parts := strings.Split(filepath.ToSlash(file), "/")
	for i, part := range parts {
		if part == "staticresources" && i+1 < len(parts) {
			name = strings.TrimSuffix(parts[i+1], ".resource")
			if i+2 < len(parts) {
				subpath = strings.Join(parts[i+2:], "/")
			}
			return name, subpath, name != ""
		}
	}
	return "", "", false
}

func cleanResourceSubpath(subpath string) string {
	return strings.Trim(strings.ReplaceAll(filepath.ToSlash(subpath), "\\", "/"), "/")
}

func staticResourceSubpath(resource storage.StaticResourceMetadata, subpath string) (string, bool) {
	subpath = strings.Trim(strings.TrimSpace(subpath), "/")
	if subpath == "" {
		return "", false
	}
	if resource.Files != nil {
		if path := resource.Files[subpath]; path != "" {
			return path, true
		}
	}
	if resource.ContentPath == "" {
		return "", false
	}
	root := filepath.Clean(resource.ContentPath)
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return "", false
	}
	candidate := filepath.Clean(filepath.Join(root, filepath.FromSlash(subpath)))
	if candidate != root && strings.HasPrefix(candidate, root+string(filepath.Separator)) {
		return candidate, true
	}
	return "", false
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
