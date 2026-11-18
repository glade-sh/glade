package server

import (
	"encoding/csv"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// The captured capability cases exercise the server's cold compiler, its
// reused Lightning manifest, and rebuilding after cache invalidation.
func TestDynamicImportNativeLightningCache(t *testing.T) {
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GLADE_HOME", repo)
	fixture := filepath.Join(repo, "internal", "lwc", "compile", "testdata", "pr163_dynamic")
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil || len(rows) != 5 {
		t.Fatalf("native rows = %d, error %v", len(rows), err)
	}
	js, err := os.ReadFile(filepath.Join(fixture, "probe.js"))
	if err != nil {
		t.Fatal(err)
	}
	html, err := os.ReadFile(filepath.Join(fixture, "probe.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0]+"_"+row[1], func(t *testing.T) {
			root := t.TempDir()
			writeLightningFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+row[1]+`"}`)
			bundle := filepath.Join(root, "force-app", "main", "default", "lwc", "probe")
			writeLightningFixtureFile(t, filepath.Join(bundle, "probe.js"), string(js))
			writeLightningFixtureFile(t, filepath.Join(bundle, "probe.html"), string(html))
			capability := ""
			if row[0] == "dynamic_cap" {
				capability = `<capabilities><capability>lightning__dynamicComponent</capability></capabilities>`
			}
			writeLightningFixtureFile(t, filepath.Join(bundle, "probe.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+row[1]+`</apiVersion><isExposed>false</isExposed>`+capability+`</LightningComponentBundle>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			handler := newLightningTestServer(t, &org, SourceMetadata{Project: p})
			t.Cleanup(func() { _ = handler.Close() })
			err = handler.ensureLightningLocked()
			if row[3] == "rejected" {
				if err == nil || !strings.Contains(err.Error(), "LWC1503") {
					t.Fatalf("native rejection = %q, cold preparation error = %v", row[4], err)
				}
				if handler.lightning.cacheDir != "" || len(handler.lightning.manifest.Modules) != 0 {
					t.Fatal("failed preparation left a reusable Lightning cache")
				}
				return
			}
			if err != nil {
				t.Fatalf("cold preparation: %v", err)
			}
			entry, ok := handler.lightning.manifest.Modules["c:probe"]
			if !ok || entry.URL == "" {
				t.Fatalf("missing compiled component: %#v", handler.lightning.manifest.Modules)
			}
			coldRoot := handler.lightning.cacheRoot
			request := func() (int, string) {
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, entry.URL, nil))
				return rec.Code, rec.Body.String()
			}
			status, cold := request()
			if status != http.StatusOK || !strings.Contains(cold, "import(") {
				t.Fatalf("cold compiled module: status %d, body %q", status, cold)
			}
			previous := ensureLightningRoot
			t.Cleanup(func() { ensureLightningRoot = previous })
			ensureLightningRoot = func() (string, error) { return "", errors.New("unexpected warm compiler check") }
			status, warm := request()
			ensureLightningRoot = previous
			if status != http.StatusOK || warm != cold || handler.lightning.cacheRoot != coldRoot {
				t.Fatalf("warm module differs or rebuilt: status %d, same body %t, same root %t", status, warm == cold, handler.lightning.cacheRoot == coldRoot)
			}
			handler.ResetLightningCache()
			if _, err := os.Stat(coldRoot); !os.IsNotExist(err) {
				t.Fatalf("reset did not remove cold cache: %v", err)
			}
			if err := handler.ensureLightningLocked(); err != nil {
				t.Fatalf("rebuild after reset: %v", err)
			}
			status, rebuilt := request()
			if status != http.StatusOK || rebuilt != cold || handler.lightning.cacheRoot == coldRoot {
				t.Fatalf("rebuilt module differs or reused root: status %d, same body %t, same root %t", status, rebuilt == cold, handler.lightning.cacheRoot == coldRoot)
			}
		})
	}
}
