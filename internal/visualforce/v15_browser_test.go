package visualforce

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/vm"
)

// Render the owned source through the product, then observe it in Chromium.
// The browser receives input controls only, never any native expected answers.
func v15BrowserObservations(t *testing.T, table v15Table, api string, controllers []vm.Class) map[string]map[string]any {
	t.Helper()
	pages := map[string]string{}
	specs := []map[string]any{}
	resourceRoot := ""
	for _, c := range table.Cases {
		if c.Kind != "runtime" {
			continue
		}
		root, p, index, err := v15LoadCase(t, table, c, api)
		if resourceRoot == "" {
			resourceRoot = root
		}
		content := ""
		if err == nil {
			machine := testRunner(t)
			for _, controller := range controllers {
				if err := machine.RegisterClass(controller); err != nil {
					t.Fatal(err)
				}
			}
			result, renderErr := RenderPage(PageRenderRequest{Project: p, VFIndex: index, Machine: machine, PageName: c.Inputs[api].Name})
			content, err = result.HTML, renderErr
		}
		if err != nil {
			// Serve the renderer's actual error text as a local error response, so the
			// exception observer measures visible text and actual root absence.
			content = "<!doctype html><html><body><pre>" + html.EscapeString(err.Error()) + "</pre></body></html>"
		}
		route := "/apex/" + c.Inputs[api].Name
		pages[route] = content
		specs = append(specs, map[string]any{"id": c.ID, "path": route, "click": c.Click, "category": c.Category, "chart": strings.Contains(c.ID, "chart_") && !strings.HasSuffix(c.ID, "_hidden")})
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if content, ok := pages[r.URL.Path]; ok {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, content)
			return
		}
		if content, contentType, ok := VisualforcePresentationAsset(r.URL.Path); ok {
			w.Header().Set("Content-Type", contentType)
			w.Write(content)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/resource/") {
			parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/resource/"), "/", 2)
			name, subpath := parts[0], ""
			if len(parts) == 2 {
				subpath = parts[1]
			}
			content, filename, err := ReadStaticResource(resourceRoot, name, subpath)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			contentType := mime.TypeByExtension(filepath.Ext(filename))
			if contentType == "" || filepath.Ext(filename) == ".resource" {
				metadata, err := os.ReadFile(filepath.Join(resourceRoot, "force-app/main/default/staticresources", name+".resource-meta.xml"))
				if err != nil {
					t.Errorf("resource metadata %s: %v", name, err)
					http.NotFound(w, r)
					return
				}
				var meta struct {
					ContentType string `xml:"contentType"`
				}
				if err := xml.Unmarshal(metadata, &meta); err != nil {
					t.Errorf("resource metadata %s: %v", name, err)
					http.NotFound(w, r)
					return
				}
				contentType = meta.ContentType
			}
			w.Header().Set("Content-Type", contentType)
			w.Write(content)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	playwright := os.Getenv("GLADE_LWC_PLAYWRIGHT_MODULE")
	if playwright == "" {
		playwright = filepath.Join(repo, "lwcruntime/node_modules/playwright")
	}
	observer, err := filepath.Abs("testdata/v15_browser.mjs")
	if err != nil {
		t.Fatal(err)
	}
	config, err := json.Marshal(map[string]any{"url": server.URL, "specs": specs, "playwrightModule": playwright, "executablePath": os.Getenv("GLADE_LWC_BROWSER_EXECUTABLE")})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", observer)
	cmd.Stdin = bytes.NewReader(config)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("V15 API %s local browser: %v: %s", api, err, stderr.String())
	}
	var observed map[string]map[string]any
	if err := json.Unmarshal(output, &observed); err != nil {
		t.Fatalf("V15 browser JSON: %v", err)
	}
	if len(observed) != len(specs) {
		t.Fatalf("V15 browser returned %d/%d rows at API %s", len(observed), len(specs), api)
	}
	for _, spec := range specs {
		if observed[spec["id"].(string)] == nil {
			t.Fatalf("V15 browser omitted %s at API %s", spec["id"], api)
		}
	}
	return observed
}

func v15LoadCase(t *testing.T, table v15Table, c v15Case, api string) (string, project.Project, Index, error) {
	t.Helper()
	root := t.TempDir()
	for path, encoded := range table.Support[api] {
		payload, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatalf("API %s fixture %s: %v", api, path, err)
		}
		writeFile(t, filepath.Join(root, filepath.FromSlash(path)), string(payload))
	}
	input := c.Inputs[api]
	path := filepath.Join(root, "force-app/main/default/pages", input.Name+".page")
	writeFile(t, path, input.Markup)
	writeFile(t, path+"-meta.xml", input.Metadata)
	p, err := project.Load(root)
	var index Index
	if err == nil {
		index, err = LoadProject(p)
	}
	return root, p, index, err
}
