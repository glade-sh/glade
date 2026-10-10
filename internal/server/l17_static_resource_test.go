package server

import (
	"encoding/json"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native native API59/67 subpath rows reject escaped spaces/Unicode while plain
// nested CSS/JS members load. The served CSS must be a stylesheet, not a zip.
func TestL17StaticResourceArchivePathsAndMIME(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "Assets.resource")
	writeLightningStaticResourceZip(t, archive, map[string]string{
		"nested/ok.css":  ".owned-target { --owned-loaded: yes; }",
		"nested/ok.js":   "window.owned = true;",
		"space file.js":  "window.owned = true;",
		"space file.css": ".owned-target { --owned-loaded: yes; }",
		"café.js":        "window.owned = true;",
		"café.css":       ".owned-target { --owned-loaded: yes; }",
	})
	org := storage.NewOrgState()
	org.Metadata.StaticResources = []storage.StaticResourceMetadata{{Name: "Assets", ContentPath: archive, ContentType: "application/zip"}}
	handler := New(&org)
	for _, row := range []struct {
		id, path, contentType, destination, contentTypeOptions string
		status                                                 int
	}{
		{"dom_subpath_loadStyle_nested", "nested/ok.css", "text/css", "style", "nosniff", http.StatusOK},
		{"dom_subpath_loadScript_nested", "nested/ok.js", "application/x-javascript", "script", "nosniff", http.StatusOK},
		{"dom_subpath_loadStyle_root", "nested/", "application/octet-stream", "style", "nosniff", http.StatusOK},
		{"dom_subpath_loadScript_root", "nested/", "", "script", "", http.StatusNotFound},
		{"absent_directory", "absent/", "", "style", "", http.StatusNotFound},
		{"dom_subpath_loadScript_space", "space%20file.js", "", "script", "", http.StatusNotFound},
		{"dom_subpath_loadStyle_space", "space%20file.css", "", "style", "", http.StatusNotFound},
		{"dom_subpath_loadScript_unicode", "caf%C3%A9.js", "", "script", "", http.StatusNotFound},
		{"dom_subpath_loadStyle_unicode", "caf%C3%A9.css", "", "style", "", http.StatusNotFound},
	} {
		t.Run(row.id, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/resource/Assets/"+row.path, nil)
			request.Header.Set("Sec-Fetch-Dest", row.destination)
			handler.ServeHTTP(response, request)
			if response.Code != row.status {
				t.Fatalf("%s got HTTP %d want %d", row.id, response.Code, row.status)
			}
			if got := response.Header().Get("X-Content-Type-Options"); got != row.contentTypeOptions {
				t.Fatalf("%s got MIME policy %q want %q", row.id, got, row.contentTypeOptions)
			}
			if row.contentType != "" {
				got, _, err := mime.ParseMediaType(response.Header().Get("Content-Type"))
				if err != nil || got != row.contentType {
					t.Fatalf("%s got MIME %q want %q: %v", row.id, got, row.contentType, err)
				}
			}
		})
	}
	// The archive root retains its declared MIME type.
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/resource/Assets", nil))
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "application/zip" {
		t.Fatalf("archive root got HTTP %d MIME %q", response.Code, response.Header().Get("Content-Type"))
	}
}

// Native API59/67 response headers and member bytes are exported unchanged
// from the nested script/style controls, including the full Content-Type.
func TestL17StaticResourceCapturedMIME(t *testing.T) {
	data, err := os.ReadFile("testdata/l17_resource_mime_controls.json")
	if err != nil {
		t.Fatal(err)
	}
	var controls struct {
		Rows []struct {
			API, ID, Member, Body string
			Response              struct {
				Status                          int
				ContentType, ContentTypeOptions string
			}
		}
	}
	if err := json.Unmarshal(data, &controls); err != nil {
		t.Fatal(err)
	}
	if len(controls.Rows) != 4 {
		t.Fatalf("got %d MIME controls, want two at each API", len(controls.Rows))
	}
	for _, row := range controls.Rows {
		for _, source := range []string{"org", "tooling", "project-zip", "project-expanded", "project-root"} {
			t.Run(row.API+"/"+row.ID+"/"+source, func(t *testing.T) {
				root := t.TempDir()
				resources := filepath.Join(root, "force-app", "main", "default", "staticresources")
				if err := os.MkdirAll(resources, 0755); err != nil {
					t.Fatal(err)
				}
				archive := filepath.Join(resources, "Assets.resource")
				writeLightningStaticResourceZip(t, archive, map[string]string{row.Member: row.Body})
				org := storage.NewOrgState()
				handler := New(&org)
				metadata := []storage.StaticResourceMetadata{{Name: "Assets", ContentPath: archive, ContentType: "application/zip"}}
				switch source {
				case "org":
					org.Metadata.StaticResources = metadata
				case "tooling":
					handler.Source.ToolingOrg.Metadata.StaticResources = metadata
				case "project-zip":
					handler.Source.Project.StaticResourceFiles = []string{archive}
				case "project-expanded":
					member := filepath.Join(resources, "Assets", filepath.FromSlash(row.Member))
					if err := os.MkdirAll(filepath.Dir(member), 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(member, []byte(row.Body), 0644); err != nil {
						t.Fatal(err)
					}
					handler.Source.Project.StaticResourceFiles = []string{member}
				case "project-root":
					handler.Source.Project.Root = root
				}
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/resource/Assets/"+row.Member, nil))
				if response.Code != row.Response.Status || response.Body.String() != row.Body {
					t.Fatalf("got status %d and body %q, want %d and %q", response.Code, response.Body.String(), row.Response.Status, row.Body)
				}
				for name, want := range map[string]string{"Content-Type": row.Response.ContentType, "X-Content-Type-Options": row.Response.ContentTypeOptions} {
					if got := response.Header().Get(name); got != want {
						t.Errorf("%s got %q, want exact native %q", name, got, want)
					}
				}
			})
		}
	}
}
