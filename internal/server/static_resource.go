package server

import (
	"errors"
	"mime"
	"net/http"
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
	if err := visualforce.ValidateStaticResourceName(name); err != nil {
		writeSalesforceError(w, errUnknownEndpoint, "unknown static resource")
		return
	}
	subpath := strings.Join(parts[1:], "/")

	content, filename, contentType, ok, err := s.lookupStaticResource(name, subpath)
	if err != nil {
		writeSalesforceError(w, errUnknownEndpoint, "static resource not readable")
		return
	}
	if !ok {
		writeSalesforceError(w, errUnknownEndpoint, "unknown static resource")
		return
	}
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	} else if ext := filepath.Ext(filename); ext != "" {
		if guessed := mime.TypeByExtension(ext); guessed != "" {
			w.Header().Set("Content-Type", guessed)
		}
	}
	setDevNoStore(w)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (s *Server) lookupStaticResource(name, subpath string) (content []byte, filename string, contentType string, ok bool, readErr error) {
	if s.Org != nil {
		for _, resource := range s.Org.Metadata.StaticResources {
			if strings.EqualFold(resource.Name, name) {
				if subpath != "" {
					if content, filename, ok, err := staticResourceSubpath(resource, subpath); err != nil {
						return nil, "", "", false, err
					} else if ok {
						return content, filename, resource.ContentType, true, nil
					}
				}
				if subpath == "" && resource.ContentPath != "" {
					if content, filename, err := visualforce.ReadStaticResourceContentPath(resource.ContentPath, resource.Name, ""); err == nil {
						return content, filename, resource.ContentType, true, nil
					} else {
						return nil, "", "", false, err
					}
				}
				if subpath == "" && resource.Content != "" {
					return []byte(resource.Content), resource.Name + ".resource", resource.ContentType, true, nil
				}
			}
		}
	}
	for _, resource := range s.Source.ToolingOrg.Metadata.StaticResources {
		if strings.EqualFold(resource.Name, name) {
			if subpath != "" {
				if content, filename, ok, err := staticResourceSubpath(resource, subpath); err != nil {
					return nil, "", "", false, err
				} else if ok {
					return content, filename, resource.ContentType, true, nil
				}
			}
			if subpath == "" && resource.ContentPath != "" {
				if content, filename, err := visualforce.ReadStaticResourceContentPath(resource.ContentPath, resource.Name, ""); err == nil {
					return content, filename, resource.ContentType, true, nil
				} else {
					return nil, "", "", false, err
				}
			}
		}
	}
	for _, path := range s.Source.Project.StaticResourceFiles {
		resourceName, resourceSubpath, ok := projectStaticResourceNameAndSubpath(path)
		if ok && strings.EqualFold(resourceName, name) && subpath != "" && cleanResourceSubpath(resourceSubpath) == cleanResourceSubpath(subpath) {
			if content, filename, err := visualforce.ReadStaticResourceContentPath(path, resourceName, ""); err == nil {
				return content, filename, "", true, nil
			} else {
				return nil, "", "", false, err
			}
		}
		if ok && strings.EqualFold(resourceName, name) && resourceSubpath == "" && subpath != "" {
			if content, filename, err := visualforce.ReadStaticResourceContentPath(path, resourceName, subpath); err == nil {
				return content, filename, "", true, nil
			} else if !errors.Is(err, visualforce.ErrStaticResourceNotFound) {
				return nil, "", "", false, err
			}
		}
	}
	if s.Source.Project.Root != "" {
		if content, filename, err := visualforce.ReadStaticResource(s.Source.Project.Root, name, subpath); err == nil {
			return content, filename, "", true, nil
		} else if !errors.Is(err, visualforce.ErrStaticResourceNotFound) {
			return nil, "", "", false, err
		}
	}
	for _, path := range s.Source.Project.StaticResourceFiles {
		base := strings.TrimSuffix(filepath.Base(path), ".resource")
		if strings.EqualFold(base, name) && subpath == "" {
			if content, filename, err := visualforce.ReadStaticResourceContentPath(path, base, ""); err == nil {
				return content, filename, "", true, nil
			} else {
				return nil, "", "", false, err
			}
		}
	}
	return nil, "", "", false, nil
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

func staticResourceSubpath(resource storage.StaticResourceMetadata, subpath string) ([]byte, string, bool, error) {
	subpath = strings.Trim(strings.TrimSpace(subpath), "/")
	normalizedSubpath, err := visualforce.NormalizeStaticResourceSubpath(subpath)
	if err != nil || normalizedSubpath == "" {
		return nil, "", false, nil
	}
	if resource.Files != nil {
		if path := resource.Files[normalizedSubpath]; path != "" {
			content, filename, err := visualforce.ReadStaticResourceContentPath(path, resource.Name, "")
			if err == nil {
				return content, filename, true, nil
			}
			return nil, "", false, err
		}
	}
	if resource.ContentPath == "" {
		return nil, "", false, nil
	}
	if content, filename, err := visualforce.ReadStaticResourceContentPath(resource.ContentPath, resource.Name, normalizedSubpath); err == nil {
		return content, filename, true, nil
	} else if !errors.Is(err, visualforce.ErrStaticResourceNotFound) {
		return nil, "", false, err
	}
	return nil, "", false, nil
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
