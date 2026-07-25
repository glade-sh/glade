package server

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lwc/compile"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestVFPageBootstrapsLightningOut(t *testing.T) {
	if _, err := os.Stat(filepath.Join("..", "..", "third_party", "lwc", "node_modules")); err != nil {
		t.Skip("npm install required in third_party/lwc")
	}
	root, err := lightningTestRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(root, "testdata", "local-tests", "lightning-out-vf")
	p, err := project.Load(fixture)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	handler := NewWithSource(&org, source)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/apex/WidgetHost", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"/lightning/glade.out.js",
		"glade-lightning-config",
		"window.$Lightning",
		"c:lightningOut",
		`id="host"`,
		"@salesforce/apex/",
		"lightning/uiRecordApi",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in body:\n%s", want, body)
		}
	}
}

func TestVFPageBootstrapsMultiWidgetLightningOut(t *testing.T) {
	if _, err := os.Stat(filepath.Join("..", "..", "third_party", "lwc", "node_modules")); err != nil {
		t.Skip("npm install required in third_party/lwc")
	}
	root, err := lightningTestRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(root, "testdata", "local-tests", "lightning-out-vf")
	p, err := project.Load(fixture)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	handler := NewWithSource(&org, source)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/apex/MultiWidgetHost", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		`id="apexHost"`,
		`id="recordHost"`,
		`id="labelResourceHost"`,
		`id="eventHost"`,
		"c:apexWireHost",
		"c:recordWireHost",
		"c:labelResourceHost",
		"c:eventChild",
		`"c:labelresourcehost"`,
		"@salesforce/resourceUrl/",
		"GLADELWC080",
		"GLADELWC081",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in body:\n%s", want, body)
		}
	}
}

func TestLightningModulesServesCompiledJS(t *testing.T) {
	if _, err := os.Stat(filepath.Join("..", "..", "third_party", "lwc", "node_modules")); err != nil {
		t.Skip("npm install required in third_party/lwc")
	}
	root, err := lightningTestRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(root, "testdata", "local-tests", "lightning-out-vf")
	p, err := project.Load(fixture)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	handler := NewWithSource(&org, source)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lightning/modules/c/counter/counter.js", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "javascript") {
		t.Fatalf("content-type = %q", rec.Header().Get("Content-Type"))
	}
}

func TestLightningDevAssetsUseNoStore(t *testing.T) {
	org := testOrg()
	org.Metadata.StaticResources = []storage.StaticResourceMetadata{{Name: "WidgetAssets", Content: "widget-assets", ContentType: "text/plain"}}
	handler := New(&org)

	for _, path := range []string{
		"/lightning/glade.out.js",
		"/lightning/shims/i18n/lang.js",
		"/resource/WidgetAssets",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d body=%s", path, rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Cache-Control"); got != devNoStoreCacheControl {
			t.Fatalf("%s Cache-Control = %q, want %q", path, got, devNoStoreCacheControl)
		}
	}
}

func TestLightningCacheRootIncludesProjectIdentity(t *testing.T) {
	parent := t.TempDir()
	first := filepath.Join(parent, "a", "same")
	second := filepath.Join(parent, "b", "same")

	firstRoot := lightningCacheRoot(project.Project{Root: first})
	secondRoot := lightningCacheRoot(project.Project{Root: second})

	if firstRoot == secondRoot {
		t.Fatalf("cache roots collide for same basename: %s", firstRoot)
	}
	if !strings.HasPrefix(firstRoot, filepath.Join(os.TempDir(), "glade-lwc-cache")) {
		t.Fatalf("cache root = %s", firstRoot)
	}
}

func TestLightningModulesRejectsEscapingPaths(t *testing.T) {
	org := storage.NewOrgState()
	handler := New(&org)
	cacheRoot := filepath.Join(t.TempDir(), "cache")
	cacheDir := filepath.Join(cacheRoot, "lwc")
	writeLightningFixtureFile(t, filepath.Join(cacheDir, "c", "widget", "widget.js"), "export default 1;")
	handler.lightning = lightningState{
		cacheRoot: cacheRoot,
		cacheDir:  cacheDir,
		manifest:  lwcbrowser.Manifest{Modules: map[string]lwcbrowser.ModuleEntry{"c:widget": {URL: "/lightning/modules/c/widget/widget.js"}}},
	}

	for _, path := range []string{
		"/lightning/modules/%2Fetc%2Fpasswd",
		"/lightning/modules/c/widget/%2e%2e/secret.js",
		"/lightning/modules/%5Cwindows%5Csystem.ini",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		assertSalesforceError(t, rec, http.StatusNotFound, "NOT_FOUND", "invalid module path")
	}
}

func TestResetLightningCacheRemovesCompiledOutput(t *testing.T) {
	org := storage.NewOrgState()
	handler := New(&org)
	cacheRoot := filepath.Join(t.TempDir(), "cache")
	writeLightningFixtureFile(t, filepath.Join(cacheRoot, "lwc", "c", "widget", "widget.js"), "stale")
	handler.lightning = lightningState{
		cacheRoot: cacheRoot,
		cacheDir:  filepath.Join(cacheRoot, "lwc"),
		manifest:  lwcbrowser.Manifest{Modules: map[string]lwcbrowser.ModuleEntry{"c:widget": {URL: "/lightning/modules/c/widget/widget.js"}}},
	}

	handler.ResetLightningCache()

	if _, err := os.Stat(cacheRoot); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cache root still exists: %v", err)
	}
	if handler.lightning.cacheDir != "" || len(handler.lightning.manifest.Modules) != 0 {
		t.Fatalf("lightning state not cleared: %#v", handler.lightning)
	}
}

func TestReloadProjectStateUpdatesRuntimeAndClearsLightningCache(t *testing.T) {
	org := storage.NewOrgState()
	oldRoot := filepath.Join(t.TempDir(), "old")
	newRoot := filepath.Join(t.TempDir(), "new")
	handler := NewWithSource(&org, SourceMetadata{Project: project.Project{Root: oldRoot}})
	cacheRoot := filepath.Join(t.TempDir(), "cache")
	writeLightningFixtureFile(t, filepath.Join(cacheRoot, "lwc", "c", "widget", "widget.js"), "stale")
	handler.lightning = lightningState{
		cacheRoot: cacheRoot,
		cacheDir:  filepath.Join(cacheRoot, "lwc"),
		manifest:  lwcbrowser.Manifest{Modules: map[string]lwcbrowser.ModuleEntry{"c:widget": {URL: "/lightning/modules/c/widget/widget.js"}}},
	}
	runtimeErr := errors.New("compiled runtime failed")
	index := typesys.Index{Project: typesys.ProjectInfo{Root: newRoot, Namespace: "pkg"}}
	runtime := vm.New(nil)

	handler.ReloadProjectState(SourceMetadata{Project: project.Project{Root: newRoot, Namespace: "pkg"}}, index, runtime, runtimeErr)

	if handler.Source.Project.Root != newRoot {
		t.Fatalf("source root = %q, want %q", handler.Source.Project.Root, newRoot)
	}
	if handler.Index == nil || handler.Index.Project.Root != newRoot {
		t.Fatalf("index = %#v", handler.Index)
	}
	if handler.runtime != runtime {
		t.Fatalf("runtime pointer was not installed")
	}
	if !errors.Is(handler.runtimeErr, runtimeErr) {
		t.Fatalf("runtimeErr = %v", handler.runtimeErr)
	}
	if _, err := os.Stat(cacheRoot); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cache root still exists: %v", err)
	}
}

func TestVisualforceIncludeLightningWithoutToolchainShowsLocalNotice(t *testing.T) {
	previous := ensureLightningRoot
	ensureLightningRoot = func() (string, error) {
		return "", errors.New("toolchain missing")
	}
	t.Cleanup(func() { ensureLightningRoot = previous })

	root := t.TempDir()
	writeLightningFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"61.0"}`)
	writeLightningFixtureFile(t, filepath.Join(root, "force-app/main/default/pages/WidgetHost.page"), `<apex:page><apex:includeLightning/><div id="probe">page body</div></apex:page>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	handler := NewWithSource(&org, source)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/apex/WidgetHost", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Lightning Out is not available in local Visualforce preview") {
		t.Fatalf("missing Lightning warning in body:\n%s", body)
	}
	if !strings.Contains(body, `id="probe"`) {
		t.Fatalf("missing page body in:\n%s", body)
	}
}

func TestLightningModulesServesSiblingModuleWithoutJSExtension(t *testing.T) {
	if _, err := os.Stat(filepath.Join("..", "..", "third_party", "lwc", "node_modules")); err != nil {
		t.Skip("npm install required in third_party/lwc")
	}
	fixtureDir := filepath.Join(t.TempDir(), "sibling-fixture")
	bundleDir := filepath.Join(fixtureDir, "force-app", "main", "default", "lwc", "widget")
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeLightningFixtureFile(t, filepath.Join(bundleDir, "widget.html"), `<template><p>hi</p></template>`)
	writeLightningFixtureFile(t, filepath.Join(bundleDir, "widget.js"), `import { LightningElement } from 'lwc';
import { labels } from './labels';
export default class Widget extends LightningElement { label = labels; }`)
	writeLightningFixtureFile(t, filepath.Join(bundleDir, "labels.js"), `export const labels = { title: "Hello" };`)
	writeLightningFixtureFile(t, filepath.Join(bundleDir, "widget.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><isExposed>true</isExposed></LightningComponentBundle>`)

	p, err := project.Load(fixtureDir)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	handler := NewWithSource(&org, source)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lightning/modules/c/widget/labels", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Hello") {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func writeLightningFixtureFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestStaticResourceServesDirectorySubpathFromPackageRoot(t *testing.T) {
	root := t.TempDir()
	writeLightningFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeLightningFixtureFile(t, filepath.Join(root, "force-app/fixture-app/main/staticresources/Fixture_Assets.resource-meta.xml"), "<StaticResource/>")
	writeLightningFixtureFile(t, filepath.Join(root, "force-app/fixture-app/main/staticresources/Fixture_Assets/css/main.css"), ".fixture{}")

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	handler := NewWithSource(&org, source)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/resource/Fixture_Assets/css/main.css", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != ".fixture{}" {
		t.Fatalf("status/body = %d %q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/css") {
		t.Fatalf("content type = %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != devNoStoreCacheControl {
		t.Fatalf("Cache-Control = %q", got)
	}
}

func TestStaticResourceServesZipSubpathFromPackageRoot(t *testing.T) {
	root := t.TempDir()
	writeLightningFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"src","default":true}]}`)
	writeLightningFixtureFile(t, filepath.Join(root, "src/staticresources/Bundle.resource-meta.xml"), "<StaticResource/>")
	writeLightningStaticResourceZip(t, filepath.Join(root, "src/staticresources/Bundle.resource"), map[string]string{
		"css/site.css": "body { color: steelblue; }",
	})

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	handler := NewWithSource(&org, source)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/resource/Bundle/css/site.css", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "body { color: steelblue; }" {
		t.Fatalf("status/body = %d %q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/css") {
		t.Fatalf("content type = %q", got)
	}
}

func TestStaticResourceRejectsEncodedResourceNameTraversal(t *testing.T) {
	root := t.TempDir()
	writeLightningFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeLightningFixtureFile(t, filepath.Join(root, "force-app/main/default/private.resource"), "private sentinel")
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	handler := NewWithSource(&org, source)

	for _, requestPath := range []string{"/resource/..%2fprivate", "/resource/Bundle/..%2fprivate"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, requestPath, nil))
		if rec.Code == http.StatusOK || !strings.Contains(rec.Body.String(), "unknown static resource") {
			t.Fatalf("%s status/body = %d %q", requestPath, rec.Code, rec.Body.String())
		}
	}
}

func TestStaticResourceRejectsEncodedFilesMapTraversal(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "outside.css")
	if err := os.WriteFile(outside, []byte("outside sentinel"), 0o644); err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Metadata.StaticResources = []storage.StaticResourceMetadata{{
		Name:  "Bundle",
		Files: map[string]string{"../outside": outside},
	}}
	handler := New(&org)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/resource/Bundle/%2e%2e%2foutside", nil))
	if rec.Code == http.StatusOK || strings.Contains(rec.Body.String(), "outside sentinel") || !strings.Contains(rec.Body.String(), "unknown static resource") {
		t.Fatalf("status/body = %d %q", rec.Code, rec.Body.String())
	}
}

func TestStaticResourceServesInlineContentWithoutSharedTempFile(t *testing.T) {
	const name = "InlineNoTemp"
	legacyPath := filepath.Join(os.TempDir(), "glade-static-resource", name+".resource")
	if err := os.Remove(legacyPath); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(legacyPath) })

	org := storage.NewOrgState()
	org.Metadata.StaticResources = []storage.StaticResourceMetadata{{Name: name, Content: "inline bytes", ContentType: "text/plain"}}
	handler := New(&org)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/resource/"+name, nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "inline bytes" {
		t.Fatalf("status/body = %d %q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
		t.Fatalf("content type = %q", got)
	}
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("legacy inline static-resource file exists: %v", err)
	}
}

func TestStaticResourceServesFlatContentPathBytesAndMIME(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flat.css")
	if err := os.WriteFile(path, []byte(".flat{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Metadata.StaticResources = []storage.StaticResourceMetadata{{Name: "Flat", ContentPath: path}}
	handler := New(&org)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/resource/Flat", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != ".flat{}" {
		t.Fatalf("status/body = %d %q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/css") {
		t.Fatalf("content type = %q", got)
	}
}

func TestStaticResourceReportsDiscoveredUnsafeBundleAsUnreadable(t *testing.T) {
	resourcePath := filepath.Join(t.TempDir(), "staticresources", "Bundle.resource")
	if err := os.MkdirAll(filepath.Dir(resourcePath), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.resource")
	if err := os.WriteFile(outside, []byte("outside sentinel"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, resourcePath); err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	source := SourceMetadata{Project: project.Project{StaticResourceFiles: []string{resourcePath}}}
	handler := NewWithSource(&org, source)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/resource/Bundle/css/site.css", nil))
	if rec.Code == http.StatusOK || !strings.Contains(rec.Body.String(), "static resource not readable") {
		t.Fatalf("status/body = %d %q", rec.Code, rec.Body.String())
	}
}

func writeLightningStaticResourceZip(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	writer := zip.NewWriter(file)
	for name, content := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
}

func lightningFixtureRoot(t *testing.T) string {
	t.Helper()
	if cwd, err := os.Getwd(); err == nil {
		for _, dir := range walkTestAncestors(cwd) {
			fixture := filepath.Join(dir, "testdata", "local-tests", "lightning-out-vf")
			if _, err := os.Stat(filepath.Join(fixture, "sfdx-project.json")); err == nil {
				return dir
			}
		}
	}
	root, err := compile.FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func walkTestAncestors(start string) []string {
	var out []string
	dir := start
	for {
		out = append(out, dir)
		parent := filepath.Dir(dir)
		if parent == dir {
			return out
		}
		dir = parent
	}
}

func TestLightningLabelShimResolvesPackageCPrefix(t *testing.T) {
	root := lightningFixtureRoot(t)
	fixture := filepath.Join(root, "testdata", "local-tests", "lightning-out-vf")
	p, err := project.Load(fixture)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Namespace = "c"
	if err := resource.ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&org, source)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lightning/shims/label/c.Greeting.js", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `export default "Hello from Glade"`) {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestLightningLabelShim(t *testing.T) {
	root := lightningFixtureRoot(t)
	fixture := filepath.Join(root, "testdata", "local-tests", "lightning-out-vf")
	p, err := project.Load(fixture)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	if err := resource.ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&org, source)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lightning/shims/label/c.Greeting.js", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `export default "Hello from Glade"`) {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestLightningLabelShimFallsBackForUnknownPlatformLabel(t *testing.T) {
	org := storage.NewOrgState()
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lightning/shims/label/c.lightning_LightningRecordForm_save", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `export default "lightning_LightningRecordForm_save"`) {
		t.Fatalf("body = %q", body)
	}
	if strings.Contains(body, "NOT_FOUND") {
		t.Fatalf("served non-module response: %q", body)
	}
}

func TestLightningLabelShimUsesSourceLabelsWhenOrgMissingLabel(t *testing.T) {
	root := t.TempDir()
	writeLightningFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"62.0"}`)
	writeLightningFixtureFile(t, filepath.Join(root, "force-app/main/default/translations/en_US.translation-meta.xml"), `<Translations xmlns="http://soap.sforce.com/2006/04/metadata">
  <customLabels>
    <name>lightning_LightningRecordForm_save</name>
    <label>Save</label>
  </customLabels>
</Translations>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	handler := NewWithSource(&org, source)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lightning/shims/label/c.lightning_LightningRecordForm_save", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `export default "Save"`) {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestLightningUserShimResolvesCurrentUser(t *testing.T) {
	org := testOrg()
	addUser(&org, "005000000000123", "ada@example.test", "ada.alias@example.test", "Ada Trail")
	handler := New(&org)

	req := httptest.NewRequest(http.MethodGet, "/lightning/shims/user/Id.js", nil)
	req.Header.Set("X-GLADE-User-Id", "005000000000123")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `export default "005000000000123"`) {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestLightningI18nShim(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lightning/shims/i18n/lang.js", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `export default "en-US"`) {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestLightningResourceAndContentAssetShimsResolveMetadataURLs(t *testing.T) {
	org := testOrg()
	org.Metadata.StaticResources = []storage.StaticResourceMetadata{{Name: "WidgetAssets", URL: "/resource/WidgetAssets"}}
	org.Metadata.ContentAssets = []storage.ContentAssetMetadata{{Name: "Logo", URL: "/sfc/servlet.shepherd/version/download/Logo"}}
	handler := New(&org)

	resourceRec := httptest.NewRecorder()
	handler.ServeHTTP(resourceRec, httptest.NewRequest(http.MethodGet, "/lightning/shims/resourceUrl/WidgetAssets.js", nil))
	if resourceRec.Code != http.StatusOK {
		t.Fatalf("resource status = %d body=%s", resourceRec.Code, resourceRec.Body.String())
	}
	if !strings.Contains(resourceRec.Body.String(), `export default "/resource/WidgetAssets"`) {
		t.Fatalf("resource body = %q", resourceRec.Body.String())
	}

	assetRec := httptest.NewRecorder()
	handler.ServeHTTP(assetRec, httptest.NewRequest(http.MethodGet, "/lightning/shims/contentAssetUrl/Logo.js", nil))
	if assetRec.Code != http.StatusOK {
		t.Fatalf("asset status = %d body=%s", assetRec.Code, assetRec.Body.String())
	}
	if !strings.Contains(assetRec.Body.String(), `export default "/sfc/servlet.shepherd/version/download/Logo"`) {
		t.Fatalf("asset body = %q", assetRec.Body.String())
	}
}

func TestLightningMessageChannelShimServesToken(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lightning/shims/messageChannel/LwcProbe__c.js", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`name: "LwcProbe__c"`, `messageChannelName: "LwcProbe__c"`, "export default channel"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("missing %q in %q", want, rec.Body.String())
		}
	}
}

func TestLightningPackageCorpusShimsServeLocalContracts(t *testing.T) {
	org := testOrg()
	handler := New(&org)
	cases := []struct {
		path string
		want []string
	}{
		{"/lightning/shims/client/formFactor.js", []string{"readFormFactor", "export default"}},
		{"/lightning/shims/customPermission/LocalAuditLogs.js", []string{"LocalAuditLogs", "export default true"}},
		{"/lightning/shims/lightning/configProvider.js", []string{"getPathPrefix", "getToken", "getIconSvgTemplates", "getLocalizationService", "getOneConfig"}},
		{"/lightning/shims/lightning/pageReferenceUtils.js", []string{"encodeDefaultFieldValues", "decodeDefaultFieldValues"}},
		{"/lightning/shims/lightning/alert.js", []string{"LightningAlert", "static open", "gladealert"}},
		{"/lightning/shims/lightning/confirm.js", []string{"LightningConfirm", "Promise.resolve(true)"}},
		{"/lightning/shims/lightning/prompt.js", []string{"LightningPrompt", "static open", "gladeprompt"}},
		{"/lightning/shims/lightning/showToastEvent.js", []string{"SHOW_TOAST_EVENT_NAME", "ShowToastEvent", "lightning__showtoast"}},
		{"/lightning/shims/lightning/toast.js", []string{"LightningToast", "static show", "lightning__showtoast"}},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d body=%s", tc.path, rec.Code, rec.Body.String())
		}
		for _, want := range tc.want {
			if !strings.Contains(rec.Body.String(), want) {
				t.Fatalf("%s missing %q in %q", tc.path, want, rec.Body.String())
			}
		}
	}
}

func TestLightningResourceURLShimFetchesStaticResourceBytes(t *testing.T) {
	root := lightningFixtureRoot(t)
	fixture := filepath.Join(root, "testdata", "local-tests", "lightning-out-vf")
	p, err := project.Load(fixture)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	if err := resource.ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&org, source)

	shimRec := httptest.NewRecorder()
	handler.ServeHTTP(shimRec, httptest.NewRequest(http.MethodGet, "/lightning/shims/resourceUrl/WidgetAssets.js", nil))
	if shimRec.Code != http.StatusOK {
		t.Fatalf("shim status = %d body=%s", shimRec.Code, shimRec.Body.String())
	}
	if !strings.Contains(shimRec.Body.String(), `export default "/resource/WidgetAssets"`) {
		t.Fatalf("shim body = %q", shimRec.Body.String())
	}

	resourceRec := httptest.NewRecorder()
	handler.ServeHTTP(resourceRec, httptest.NewRequest(http.MethodGet, "/resource/WidgetAssets", nil))
	if resourceRec.Code != http.StatusOK {
		t.Fatalf("resource status = %d body=%s", resourceRec.Code, resourceRec.Body.String())
	}
	if got := strings.TrimSpace(resourceRec.Body.String()); got != "widget-assets" {
		t.Fatalf("resource body = %q", got)
	}
	if contentType := resourceRec.Header().Get("Content-Type"); contentType != "text/plain" {
		t.Fatalf("content-type = %q", contentType)
	}
}

func TestLightningNavigationShimServesLocalPageReferenceSupport(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lightning/shims/lightning/navigation.js", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{"CurrentPageReference", "NavigationMixin", "generateUrl", "navigate", "standard__recordPage", "GLADELWC040"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("missing %q in body = %q", want, rec.Body.String())
		}
	}
	if strings.Contains(rec.Body.String(), "lightning/navigation is not implemented") {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestLightningCoreShimServesLDSCache(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lightning/shims/core/lds-cache.mjs", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{"notifyRecordUpdateAvailable", "getRecordNotifyChange", "refreshApex", "registerLDSAdapter"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("missing %q in body = %q", want, rec.Body.String())
		}
	}
}

func TestLightningCoreShimPrefersSourceWireAdapter(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lightning/shims/core/wire-adapter.js", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{"registerLDSAdapter", "emitEmptyFetchWireValue", "createFetchWireAdapter"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("missing %q in body = %q", want, rec.Body.String())
		}
	}
}

func TestLightningBaseComponentShimServesNoopComponent(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lightning/shims/lightning/combobox", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{"LightningElement", "registerComponent", "lightning-combobox", "export default"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("missing %q in body = %q", want, rec.Body.String())
		}
	}
}

func TestLightningBaseComponentShimPrefersSourceBackedRuntimeWrapper(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lightning/shims/lightning/badge.js", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`export { default }`, `/lightning/runtime/lightning/source/badge/badge.js`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("missing %q in body = %q", want, rec.Body.String())
		}
	}
	if strings.Contains(rec.Body.String(), "createBaseComponent") {
		t.Fatalf("badge should use source-backed wrapper, got body = %q", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lightning/shims/lightning/recordPicker.js", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`export { default }`, `/lightning/runtime/lightning/recordPicker.js`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("missing %q in body = %q", want, rec.Body.String())
		}
	}
	if strings.Contains(rec.Body.String(), "createBaseComponent") {
		t.Fatalf("recordPicker should use source-backed wrapper, got body = %q", rec.Body.String())
	}
}

func TestLightningWireApexReturnsData(t *testing.T) {
	root := lightningFixtureRoot(t)
	fixture := filepath.Join(root, "testdata", "local-tests", "lightning-out-vf")
	p, err := project.Load(fixture)
	if err != nil {
		t.Fatal(err)
	}
	schema, err := gladeschema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	handler := NewWithSource(&org, source)
	handler.SetProjectIndex(typesys.Build(p, schema))

	body := `{"className":"ItemCtrl","method":"getItems","params":{"recordId":"001XX0000000001"}}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/lightning/wire/apex", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var out lwcbrowser.WireResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Error != nil {
		t.Fatalf("error = %#v", out.Error)
	}
	rows, ok := out.Data.([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("data = %#v", out.Data)
	}
	row, ok := rows[0].(map[string]any)
	if !ok {
		t.Fatalf("row = %#v", rows[0])
	}
	if row["Id"] != "001XX0000000001" || row["Name"] != "Local Widget" {
		t.Fatalf("row = %#v", row)
	}
}

func TestLightningApexRouteInvokesImperativeController(t *testing.T) {
	root := lightningFixtureRoot(t)
	fixture := filepath.Join(root, "testdata", "local-tests", "lightning-out-vf")
	p, err := project.Load(fixture)
	if err != nil {
		t.Fatal(err)
	}
	schema, err := gladeschema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	handler := NewWithSource(&org, source)
	handler.SetProjectIndex(typesys.Build(p, schema))

	body := `{"recordId":"001XX0000000001"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/lightning/apex/ItemCtrl/getItems", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var out lwcbrowser.WireResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Error != nil {
		t.Fatalf("error = %#v", out.Error)
	}
	rows, ok := out.Data.([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("data = %#v", out.Data)
	}
	row, ok := rows[0].(map[string]any)
	if !ok || row["Id"] != "001XX0000000001" || row["Name"] != "Local Widget" {
		t.Fatalf("row = %#v", rows[0])
	}
}

func TestLightningWireApexUsesRequestCurrentUser(t *testing.T) {
	root := lightningFixtureRoot(t)
	fixture := filepath.Join(root, "testdata", "local-tests", "lightning-out-vf")
	p, err := project.Load(fixture)
	if err != nil {
		t.Fatal(err)
	}
	schema, err := gladeschema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	addUser(&org, "005000000000777AAA", "lwc@example.test", "lwc@example.test", "LWC User")
	handler := NewWithSource(&org, source)
	handler.SetProjectIndex(typesys.Build(p, schema))

	body := `{"className":"ItemCtrl","method":"currentUserId","params":{}}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/lightning/wire/apex", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GLADE-User-Id", "005000000000777AAA")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var out lwcbrowser.WireResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Error != nil {
		t.Fatalf("error = %#v", out.Error)
	}
	if out.Data != "005000000000777AAA" {
		t.Fatalf("data = %#v", out.Data)
	}
}

func TestLightningWireApexUsesLocalLWCContextHeader(t *testing.T) {
	program, err := vm.CompileAnonymous(`
PageReference page = ApexPages.currentPage();
return page.getUrl() + ':' + page.getParameters().get('id') + ':' + page.getParameters().get('recordId');
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := vm.New(nil)
	if err := machine.RegisterMethod(vm.Method{
		Name:       "ItemCtrl.currentRecord",
		ClassName:  "ItemCtrl",
		ReturnType: "String",
		IsStatic:   true,
		Modifiers:  []string{"AuraEnabled"},
		Program:    program,
	}); err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	handler := New(&org)
	handler.SetProjectRuntime(typesys.Index{}, machine, nil)

	body := `{"className":"ItemCtrl","method":"currentRecord","params":{}}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/lightning/wire/apex", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Glade-LWC-Context", url.QueryEscape(`{"url":"/lwc/preview/record/Account/001LOCAL0000001AAA?id=001LOCAL0000001AAA&recordId=001LOCAL0000001AAA","context":{"kind":"recordPage","recordId":"001LOCAL0000001AAA","objectApiName":"Account"}}`))
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var out lwcbrowser.WireResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Error != nil {
		t.Fatalf("error = %#v", out.Error)
	}
	if got := out.Data; got != "/lwc/preview/record/Account/001LOCAL0000001AAA?id=001LOCAL0000001AAA&recordId=001LOCAL0000001AAA:001LOCAL0000001AAA:001LOCAL0000001AAA" {
		t.Fatalf("data = %#v", got)
	}
}

func TestLightningWireApexRejectsNonObjectParamsWithSalesforceError(t *testing.T) {
	root := lightningFixtureRoot(t)
	fixture := filepath.Join(root, "testdata", "local-tests", "lightning-out-vf")
	p, err := project.Load(fixture)
	if err != nil {
		t.Fatal(err)
	}
	schema, err := gladeschema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	handler := NewWithSource(&org, source)
	handler.SetProjectIndex(typesys.Build(p, schema))

	body := `{"className":"ItemCtrl","method":"getItems","params":["bad"]}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/lightning/wire/apex", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var out lwcbrowser.WireResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Error == nil || out.Error.Status != http.StatusBadRequest {
		t.Fatalf("error = %#v", out.Error)
	}
	if out.Error.Body == nil || out.Error.Body.ExceptionType != "InvalidParameterValueException" || !strings.Contains(out.Error.Body.Message, "Apex params must be an object") {
		t.Fatalf("error body = %#v", out.Error.Body)
	}
}

func TestLightningWireApexReturnsOverloadDiagnosticCode(t *testing.T) {
	firstProgram, err := vm.CompileAnonymous(`return 'first';`)
	if err != nil {
		t.Fatal(err)
	}
	secondProgram, err := vm.CompileAnonymous(`return 'second';`)
	if err != nil {
		t.Fatal(err)
	}
	machine := vm.New(nil)
	for _, method := range []vm.Method{
		{
			Name:       "WidgetController.load",
			ClassName:  "WidgetController",
			ReturnType: "String",
			IsStatic:   true,
			Modifiers:  []string{"AuraEnabled"},
			Params:     []vm.Param{{Name: "value", Type: "String"}},
			Program:    firstProgram,
		},
		{
			Name:       "WidgetController.load",
			ClassName:  "WidgetController",
			ReturnType: "String",
			IsStatic:   true,
			Modifiers:  []string{"AuraEnabled"},
			Params:     []vm.Param{{Name: "value", Type: "Object"}},
			Program:    secondProgram,
		},
	} {
		if err := machine.RegisterMethod(method); err != nil {
			t.Fatal(err)
		}
	}
	org := storage.NewOrgState()
	handler := New(&org)
	handler.SetProjectRuntime(typesys.Index{}, machine, nil)

	body := `{"className":"WidgetController","method":"load","params":{"value":"Acme"}}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/lightning/wire/apex", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var out lwcbrowser.WireResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Error == nil {
		t.Fatalf("error missing: %#v", out)
	}
	if out.Error.Code != "GLADELWC013" || out.Error.Body == nil || out.Error.Body.Code != "GLADELWC013" {
		t.Fatalf("error = %#v body=%#v", out.Error, out.Error.Body)
	}
	if out.Error.Body.ExceptionType != "UnsupportedFeature" || !strings.Contains(out.Error.Body.Message, "WidgetController.load") || !strings.Contains(out.Error.Body.Message, "overloaded AuraEnabled method unsupported") {
		t.Fatalf("error body = %#v", out.Error.Body)
	}
}

func TestLightningApexIncludesInvocationDetailsInErrorBody(t *testing.T) {
	org := storage.NewOrgState()
	handler := New(&org)
	handler.SetProjectRuntime(typesys.Index{}, vm.New(nil), nil)

	body := `{"className":"MissingController","method":"load","params":{"recordId":"001XX0000000001"}}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/lightning/wire/apex", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var out lwcbrowser.WireResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Error == nil || out.Error.Body == nil {
		t.Fatalf("error missing: %#v", out)
	}
	bodyErr := out.Error.Body
	for _, want := range []string{"MissingController.load", "recordId", "001XX0000000001"} {
		if !strings.Contains(bodyErr.Message, want) {
			t.Fatalf("message %q missing %q", bodyErr.Message, want)
		}
	}
	if bodyErr.ExceptionType == "" || bodyErr.StackTrace == "" {
		t.Fatalf("error body = %#v", bodyErr)
	}
}

func lightningTestRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := wd; dir != "" && dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
	}
	return "", os.ErrNotExist
}

func TestLightningWireGetRecordReturnsStoredName(t *testing.T) {
	org := testOrg()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:     "Account",
			Label:       "Account",
			PluralLabel: "Accounts",
			KeyPrefix:   "001",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Label: "Account Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"001XX0000000001": {
				ID:     "001XX0000000001",
				Object: "Account",
				Fields: map[string]storage.Value{
					"Name": {Kind: storage.ValueString, String: "Acme"},
				},
			},
		},
	}
	handler := New(&org)

	body := `{"recordId":"001XX0000000001","fields":["Account.Name"]}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/lightning/wire/getRecord", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var out lwcbrowser.WireResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Error != nil {
		t.Fatalf("error = %#v", out.Error)
	}
	payload, ok := out.Data.(map[string]any)
	if !ok {
		t.Fatalf("data = %#v", out.Data)
	}
	fields, ok := payload["fields"].(map[string]any)
	if !ok {
		t.Fatalf("fields = %#v", payload["fields"])
	}
	nameField, ok := fields["Name"].(map[string]any)
	if !ok || nameField["value"] != "Acme" {
		t.Fatalf("Name field = %#v", fields["Name"])
	}
	if nameField["label"] != "Account Name" {
		t.Fatalf("Name label = %#v", nameField)
	}
}

func TestLightningWireGetRecordMatchesFifteenAndEighteenCharacterIDs(t *testing.T) {
	org := testOrg()
	addAccountForTest(&org, "00100000000LFLT", "Imported Account")
	handler := New(&org)

	body := `{"recordId":"00100000000LFLTAA4","fields":["Account.Name"]}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/lightning/wire/getRecord", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var out lwcbrowser.WireResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Error != nil {
		t.Fatalf("error = %#v", out.Error)
	}
	payload, ok := out.Data.(map[string]any)
	if !ok {
		t.Fatalf("data = %#v", out.Data)
	}
	fields, ok := payload["fields"].(map[string]any)
	if !ok {
		t.Fatalf("fields = %#v", payload["fields"])
	}
	nameField, ok := fields["Name"].(map[string]any)
	if !ok || nameField["value"] != "Imported Account" {
		t.Fatalf("Name field = %#v", fields["Name"])
	}
}

func TestLightningWireGetRecordIncludesOptionalRelationshipAndDisplayValues(t *testing.T) {
	org := testOrg()
	org.Objects["User"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "User",
			Label:     "User",
			KeyPrefix: "005",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Label: "Full Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"005XX0000000001": {
				ID:     "005XX0000000001",
				Object: "User",
				Fields: map[string]storage.Value{
					"Name": storage.StringValue("Owner Person"),
				},
			},
		},
	}
	account := org.Objects["Account"]
	account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: "012XX0000000001", DeveloperName: "Business", Name: "Business", Active: true, Available: true, Default: true}}
	account.Definition.Fields["OwnerId"] = storage.Field{APIName: "OwnerId", Label: "Owner ID", Type: storage.FieldReference, ReferenceTo: []string{"User"}, RelationshipName: "Owner"}
	account.Definition.Fields["RecordTypeId"] = storage.Field{APIName: "RecordTypeId", Label: "Record Type ID", Type: storage.FieldReference, ReferenceTo: []string{"RecordType"}, RelationshipName: "RecordType"}
	account.Definition.Relations = append(account.Definition.Relations, storage.Relationship{Field: "AccountId", ParentObjects: []string{"Account"}, ChildRelationship: "Contacts"})
	account.Records["001XX0000000001"] = storage.Record{
		ID:     "001XX0000000001",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":         storage.StringValue("Acme"),
			"OwnerId":      storage.IDValue("005XX0000000001"),
			"RecordTypeId": storage.IDValue("012XX0000000001"),
		},
	}
	org.Objects["Account"] = account
	handler := New(&org)

	body := `{"recordId":"001XX0000000001","fields":["Account.Name"],"optionalFields":["Account.OwnerId"]}`
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/lightning/wire/getRecord", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var out lwcbrowser.WireResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Error != nil {
		t.Fatalf("error = %#v", out.Error)
	}
	payload := out.Data.(map[string]any)
	if payload["lastModifiedById"] == "" || payload["lastModifiedDate"] == "" || payload["recordTypeId"] != "012XX0000000001" {
		t.Fatalf("record header = %#v", payload)
	}
	childRelationships, ok := payload["childRelationships"].([]any)
	if !ok || len(childRelationships) == 0 {
		t.Fatalf("childRelationships = %#v", payload["childRelationships"])
	}
	fields := payload["fields"].(map[string]any)
	name := fields["Name"].(map[string]any)
	if name["displayValue"] != "Acme" || name["dataType"] != "String" {
		t.Fatalf("Name field = %#v", name)
	}
	owner := fields["OwnerId"].(map[string]any)
	if owner["relationshipName"] != "Owner" || owner["dataType"] != "Reference" {
		t.Fatalf("OwnerId field = %#v", owner)
	}
	refs, ok := owner["referenceToInfos"].([]any)
	if !ok || len(refs) != 1 || refs[0].(map[string]any)["apiName"] != "User" {
		t.Fatalf("referenceToInfos = %#v", owner["referenceToInfos"])
	}
}

func TestLightningWireGetRecordsReturnsBatchResults(t *testing.T) {
	org := testOrg()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Account",
			KeyPrefix: "001",
			Fields: map[string]storage.Field{
				"Name":  {APIName: "Name", Label: "Account Name", Type: storage.FieldString},
				"Phone": {APIName: "Phone", Label: "Phone", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"001XX0000000001": {
				ID:     "001XX0000000001",
				Object: "Account",
				Fields: map[string]storage.Value{
					"Name":  storage.StringValue("Acme"),
					"Phone": storage.StringValue("555-0100"),
				},
			},
		},
	}
	handler := New(&org)

	body := `{"records":[{"recordIds":["001XX0000000001","001XX0000000999"],"fields":["Account.Name"],"optionalFields":["Account.Phone"]}]}`
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/lightning/wire/getRecords", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var out lwcbrowser.WireResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Error != nil {
		t.Fatalf("error = %#v", out.Error)
	}
	payload, ok := out.Data.(map[string]any)
	if !ok {
		t.Fatalf("data = %#v", out.Data)
	}
	results, ok := payload["results"].([]any)
	if !ok || len(results) != 2 {
		t.Fatalf("results = %#v", payload["results"])
	}
	first, ok := results[0].(map[string]any)
	if !ok || first["statusCode"] != float64(http.StatusOK) {
		t.Fatalf("first result = %#v", results[0])
	}
	firstRecord, ok := first["result"].(map[string]any)
	if !ok {
		t.Fatalf("first record = %#v", first["result"])
	}
	fields, ok := firstRecord["fields"].(map[string]any)
	if !ok {
		t.Fatalf("fields = %#v", firstRecord["fields"])
	}
	if fields["Name"].(map[string]any)["value"] != "Acme" || fields["Phone"].(map[string]any)["value"] != "555-0100" {
		t.Fatalf("fields = %#v", fields)
	}
	second, ok := results[1].(map[string]any)
	if !ok || second["statusCode"] != float64(http.StatusNotFound) {
		t.Fatalf("second result = %#v", results[1])
	}
	errPayload, ok := second["result"].(map[string]any)
	if !ok || errPayload["errorCode"] != "NOT_FOUND" {
		t.Fatalf("second error = %#v", second["result"])
	}
}

func TestLightningWireGetRecordUIReturnsRecordObjectInfoAndLayouts(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Label = "Account"
	account.Definition.PluralLabel = "Accounts"
	account.Definition.Fields["Name"] = storage.Field{APIName: "Name", Label: "Account Name", Type: storage.FieldString, Required: true, Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(true)}
	account.Definition.Fields["Description"] = storage.Field{APIName: "Description", Label: "Description", Type: storage.FieldString, Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(true)}
	account.Records["001XX0000000001"] = storage.Record{
		ID:     "001XX0000000001",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":        storage.StringValue("Acme"),
			"Description": storage.StringValue("Local account"),
		},
	}
	org.Objects["Account"] = account
	handler := New(&org)

	body := `{"recordIds":["001XX0000000001"],"fields":["Account.Name"],"optionalFields":["Account.Description"],"layoutTypes":["Full"],"modes":["View"]}`
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/lightning/wire/getRecordUi", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var out lwcbrowser.WireResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Error != nil {
		t.Fatalf("error = %#v", out.Error)
	}
	payload, ok := out.Data.(map[string]any)
	if !ok {
		t.Fatalf("data = %#v", out.Data)
	}
	records, ok := payload["records"].(map[string]any)
	if !ok {
		t.Fatalf("records = %#v", payload["records"])
	}
	record, ok := records["001XX0000000001"].(map[string]any)
	if !ok || record["apiName"] != "Account" {
		t.Fatalf("record = %#v", records["001XX0000000001"])
	}
	fields, ok := record["fields"].(map[string]any)
	if !ok || fields["Name"].(map[string]any)["value"] != "Acme" || fields["Description"].(map[string]any)["value"] != "Local account" {
		t.Fatalf("record fields = %#v", record["fields"])
	}
	objectInfos, ok := payload["objectInfos"].(map[string]any)
	if !ok || objectInfos["Account"] == nil {
		t.Fatalf("objectInfos = %#v", payload["objectInfos"])
	}
	accountInfo, ok := objectInfos["Account"].(map[string]any)
	if !ok {
		t.Fatalf("Account object info = %#v", objectInfos["Account"])
	}
	infoFields, ok := accountInfo["fields"].(map[string]any)
	if !ok {
		t.Fatalf("Account fields = %#v", accountInfo["fields"])
	}
	assertUIAPIFieldDataTypeWithoutRESTType(t, infoFields, "Name", "String")
	layouts, ok := payload["layouts"].(map[string]any)
	if !ok {
		t.Fatalf("layouts = %#v", payload["layouts"])
	}
	accountLayouts, ok := layouts["Account"].(map[string]any)
	if !ok {
		t.Fatalf("Account layouts = %#v", layouts["Account"])
	}
	var typeLayouts map[string]any
	for _, raw := range accountLayouts {
		typeLayouts, ok = raw.(map[string]any)
		if ok {
			break
		}
	}
	if !ok || typeLayouts["Full"] == nil {
		t.Fatalf("type layouts = %#v", accountLayouts)
	}
	full, ok := typeLayouts["Full"].(map[string]any)
	if !ok || full["View"] == nil {
		t.Fatalf("Full layout = %#v", typeLayouts["Full"])
	}
}

func TestLightningWireGetObjectInfoReturnsLocalSchema(t *testing.T) {
	org := testOrg()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:     "Account",
			Label:       "Account",
			PluralLabel: "Accounts",
			KeyPrefix:   "001",
			Fields: map[string]storage.Field{
				"Name": {
					APIName:    "Name",
					Label:      "Account Name",
					Type:       storage.FieldString,
					Createable: storage.BoolFlag(true),
					Updateable: storage.BoolFlag(true),
				},
				"Rating": {
					APIName: "Rating",
					Label:   "Rating",
					Type:    storage.FieldPicklist,
					PicklistValues: []storage.PicklistValue{
						{Value: "Hot", Label: "Hot", Active: true, Default: true},
					},
				},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	handler := New(&org)

	body := `{"objectApiName":"Account"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/lightning/wire/getObjectInfo", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var out lwcbrowser.WireResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Error != nil {
		t.Fatalf("error = %#v", out.Error)
	}
	payload, ok := out.Data.(map[string]any)
	if !ok {
		t.Fatalf("data = %#v", out.Data)
	}
	if payload["apiName"] != "Account" || payload["label"] != "Account" || payload["labelPlural"] != "Accounts" {
		t.Fatalf("object info = %#v", payload)
	}
	fields, ok := payload["fields"].(map[string]any)
	if !ok {
		t.Fatalf("fields = %#v", payload["fields"])
	}
	name := assertUIAPIFieldDataTypeWithoutRESTType(t, fields, "Name", "String")
	if name["label"] != "Account Name" || name["createable"] != true || name["updateable"] != true {
		t.Fatalf("Name field = %#v", fields["Name"])
	}
	rating, ok := fields["Rating"].(map[string]any)
	if !ok {
		t.Fatalf("Rating field = %#v", fields["Rating"])
	}
	values, ok := rating["picklistValues"].([]any)
	if !ok || len(values) != 1 {
		t.Fatalf("Rating picklistValues = %#v", rating["picklistValues"])
	}
}

func TestLightningWireGetObjectInfoEnsuresKnownStandardObject(t *testing.T) {
	org := storage.NewOrgState()
	handler := New(&org)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/lightning/wire/getObjectInfo", strings.NewReader(`{"objectApiName":"Contact"}`))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var out lwcbrowser.WireResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Error != nil {
		t.Fatalf("error = %#v", out.Error)
	}
	payload, ok := out.Data.(map[string]any)
	if !ok {
		t.Fatalf("data = %#v", out.Data)
	}
	if payload["apiName"] != "Contact" {
		t.Fatalf("apiName = %#v", payload["apiName"])
	}
	fields, ok := payload["fields"].(map[string]any)
	if !ok {
		t.Fatalf("fields = %#v", payload["fields"])
	}
	for _, field := range []string{"Name", "LastName", "AccountId"} {
		if _, ok := fields[field]; !ok {
			t.Fatalf("missing field %s: %#v", field, fields)
		}
	}
}

func TestLightningWireGetObjectInfoIncludesCareProgramParentLookup(t *testing.T) {
	org := storage.NewOrgState()
	handler := New(&org)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/lightning/wire/getObjectInfo", strings.NewReader(`{"objectApiName":"CareProgram"}`))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var out lwcbrowser.WireResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Error != nil {
		t.Fatalf("error = %#v", out.Error)
	}
	payload, ok := out.Data.(map[string]any)
	if !ok {
		t.Fatalf("data = %#v", out.Data)
	}
	if payload["label"] != "Care Program" {
		t.Fatalf("CareProgram label = %#v", payload["label"])
	}
	fields, ok := payload["fields"].(map[string]any)
	if !ok {
		t.Fatalf("fields = %#v", payload["fields"])
	}
	parent := assertUIAPIFieldDataTypeWithoutRESTType(t, fields, "ParentProgramId", "Reference")
	if parent["relationshipName"] != "ParentProgram" {
		t.Fatalf("ParentProgramId = %#v", parent)
	}
	references, ok := parent["referenceToInfos"].([]any)
	if !ok || len(references) != 1 || references[0].(map[string]any)["apiName"] != "CareProgram" {
		t.Fatalf("ParentProgramId referenceToInfos = %#v", parent["referenceToInfos"])
	}
}

func TestLightningWireGetObjectInfoIncludesRecordTypesPicklistsAndFieldPermissions(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Label = "Account"
	account.Definition.PluralLabel = "Accounts"
	account.Definition.KeyPrefix = "001"
	account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: "012XX0000000001", DeveloperName: "Business", Name: "Business", Active: true, Available: true, Default: true}}
	account.Definition.Fields["OwnerId"] = storage.Field{APIName: "OwnerId", Label: "Owner ID", Type: storage.FieldReference, ReferenceTo: []string{"User"}, RelationshipName: "Owner", Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(false), Accessible: storage.BoolFlag(true)}
	account.Definition.Fields["Rating"] = storage.Field{
		APIName:    "Rating",
		Label:      "Rating",
		Type:       storage.FieldPicklist,
		Createable: storage.BoolFlag(true),
		Updateable: storage.BoolFlag(true),
		Accessible: storage.BoolFlag(true),
		PicklistValues: []storage.PicklistValue{
			{Value: "Hot", Label: "Hot", Active: true, Default: true},
		},
	}
	org.Objects["Account"] = account
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/lightning/wire/getObjectInfo", strings.NewReader(`{"objectApiName":"Account"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var out lwcbrowser.WireResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Error != nil {
		t.Fatalf("error = %#v", out.Error)
	}
	payload := out.Data.(map[string]any)
	if payload["defaultRecordTypeId"] != "012XX0000000001" || payload["themeInfo"] == nil {
		t.Fatalf("object info header = %#v", payload)
	}
	recordTypes, ok := payload["recordTypeInfos"].(map[string]any)
	if !ok || recordTypes["012XX0000000001"] == nil {
		t.Fatalf("recordTypeInfos = %#v", payload["recordTypeInfos"])
	}
	fields := payload["fields"].(map[string]any)
	owner := assertUIAPIFieldDataTypeWithoutRESTType(t, fields, "OwnerId", "Reference")
	if owner["dataType"] != "Reference" || owner["relationshipName"] != "Owner" || owner["reference"] != true || owner["createable"] != true || owner["updateable"] != false || owner["accessible"] != true {
		t.Fatalf("OwnerId field = %#v", owner)
	}
	refs, ok := owner["referenceToInfos"].([]any)
	if !ok || len(refs) != 1 || refs[0].(map[string]any)["apiName"] != "User" {
		t.Fatalf("referenceToInfos = %#v", owner["referenceToInfos"])
	}
	nameFields, ok := refs[0].(map[string]any)["nameFields"].([]any)
	if !ok || len(nameFields) == 0 || nameFields[0] != "Name" {
		t.Fatalf("referenceToInfos.nameFields = %#v", refs[0].(map[string]any)["nameFields"])
	}
	rating := fields["Rating"].(map[string]any)
	values, ok := rating["picklistValues"].([]any)
	if !ok || len(values) != 1 || values[0].(map[string]any)["value"] != "Hot" {
		t.Fatalf("Rating picklistValues = %#v", rating["picklistValues"])
	}
}

func TestLightningWireGetObjectInfosReturnsOrderedResults(t *testing.T) {
	org := testOrg()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:     "Account",
			Label:       "Account",
			PluralLabel: "Accounts",
			KeyPrefix:   "001",
			Fields:      map[string]storage.Field{"Name": {APIName: "Name", Label: "Account Name", Type: storage.FieldString}},
		},
		Records: map[storage.ID]storage.Record{},
	}
	handler := New(&org)

	body := `{"objectApiNames":["Account","Missing__c"]}`
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/lightning/wire/getObjectInfos", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var out lwcbrowser.WireResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Error != nil {
		t.Fatalf("error = %#v", out.Error)
	}
	payload, ok := out.Data.(map[string]any)
	if !ok {
		t.Fatalf("data = %#v", out.Data)
	}
	results, ok := payload["results"].([]any)
	if !ok || len(results) != 2 {
		t.Fatalf("results = %#v", payload["results"])
	}
	first, ok := results[0].(map[string]any)
	if !ok || first["statusCode"] != float64(http.StatusOK) {
		t.Fatalf("first = %#v", results[0])
	}
	firstInfo, ok := first["result"].(map[string]any)
	if !ok || firstInfo["apiName"] != "Account" {
		t.Fatalf("first info = %#v", first["result"])
	}
	firstFields, ok := firstInfo["fields"].(map[string]any)
	if !ok {
		t.Fatalf("first fields = %#v", firstInfo["fields"])
	}
	assertUIAPIFieldDataTypeWithoutRESTType(t, firstFields, "Name", "String")
	second, ok := results[1].(map[string]any)
	if !ok || second["statusCode"] != float64(http.StatusNotFound) {
		t.Fatalf("second = %#v", results[1])
	}
	secondErr, ok := second["result"].(map[string]any)
	if !ok || secondErr["errorCode"] != "NOT_FOUND" {
		t.Fatalf("second err = %#v", second["result"])
	}
}

func TestLightningWireGetRecordCreateDefaultsReturnsDefaultRecord(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Label = "Account"
	account.Definition.PluralLabel = "Accounts"
	account.Definition.RecordTypes = []storage.RecordTypeInfo{{
		ID:               "012000000000123",
		DeveloperName:    "Business",
		Name:             "Business Account",
		Active:           true,
		Available:        true,
		Default:          true,
		PicklistDefaults: map[string]string{"Rating": "Warm"},
	}}
	account.Definition.Fields["RecordTypeId"] = storage.Field{APIName: "RecordTypeId", Label: "Record Type ID", Type: storage.FieldReference}
	account.Definition.Fields["Name"] = storage.Field{APIName: "Name", Label: "Account Name", Type: storage.FieldString, Required: true, Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(true)}
	account.Definition.Fields["Description"] = storage.Field{APIName: "Description", Label: "Description", Type: storage.FieldString, Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(true)}
	account.Definition.Fields["Active__c"] = storage.Field{APIName: "Active__c", Label: "Active", Type: storage.FieldBoolean, DefaultValue: "true", Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(true)}
	account.Definition.Fields["Rating"] = storage.Field{
		APIName:    "Rating",
		Label:      "Rating",
		Type:       storage.FieldPicklist,
		Createable: storage.BoolFlag(true),
		Updateable: storage.BoolFlag(true),
		PicklistValues: []storage.PicklistValue{
			{Value: "Hot", Label: "Hot", Active: true, Default: true},
			{Value: "Warm", Label: "Warm", Active: true},
		},
	}
	account.Definition.Fields["Internal__c"] = storage.Field{APIName: "Internal__c", Label: "Internal", Type: storage.FieldString, DefaultValue: "hidden", Createable: storage.BoolFlag(false), Updateable: storage.BoolFlag(false)}
	org.Objects["Account"] = account
	handler := New(&org)

	body := `{"objectApiName":"Account","optionalFields":["Account.Description","Account.Internal__c"]}`
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/lightning/wire/getRecordCreateDefaults", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var out lwcbrowser.WireResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Error != nil {
		t.Fatalf("error = %#v", out.Error)
	}
	payload, ok := out.Data.(map[string]any)
	if !ok {
		t.Fatalf("data = %#v", out.Data)
	}
	if payload["apiName"] != "Account" || payload["recordTypeId"] != "012000000000123" {
		t.Fatalf("defaults header = %#v", payload)
	}
	objectInfos, ok := payload["objectInfos"].(map[string]any)
	if !ok || objectInfos["Account"] == nil {
		t.Fatalf("objectInfos = %#v", payload["objectInfos"])
	}
	accountInfo, ok := objectInfos["Account"].(map[string]any)
	if !ok || accountInfo["defaultRecordTypeId"] != "012000000000123" {
		t.Fatalf("Account object info = %#v", objectInfos["Account"])
	}
	infoFields, ok := accountInfo["fields"].(map[string]any)
	if !ok {
		t.Fatalf("object info fields = %#v", accountInfo["fields"])
	}
	nameInfo := assertUIAPIFieldDataTypeWithoutRESTType(t, infoFields, "Name", "String")
	if nameInfo["apiName"] != "Name" || nameInfo["required"] != true || nameInfo["nameField"] != true {
		t.Fatalf("Name object info = %#v", infoFields["Name"])
	}
	record, ok := payload["record"].(map[string]any)
	if !ok || record["apiName"] != "Account" || record["recordTypeId"] != "012000000000123" {
		t.Fatalf("record = %#v", payload["record"])
	}
	fields, ok := record["fields"].(map[string]any)
	if !ok {
		t.Fatalf("record fields = %#v", record["fields"])
	}
	if fields["Active__c"].(map[string]any)["value"] != true {
		t.Fatalf("Active__c = %#v", fields["Active__c"])
	}
	if fields["Rating"].(map[string]any)["value"] != "Warm" {
		t.Fatalf("Rating = %#v", fields["Rating"])
	}
	if fields["Description"].(map[string]any)["value"] != nil {
		t.Fatalf("Description = %#v", fields["Description"])
	}
	if _, ok := fields["Internal__c"]; ok {
		t.Fatalf("non-createable field included = %#v", fields["Internal__c"])
	}
	layout, ok := payload["layout"].(map[string]any)
	if !ok {
		t.Fatalf("layout = %#v", payload["layout"])
	}
	if layout["id"] != "local-Account-create-layout" || layout["objectApiName"] != "Account" || layout["mode"] != "Create" || layout["layoutType"] != "Full" {
		t.Fatalf("layout header = %#v", layout)
	}
	sections, ok := layout["sections"].([]any)
	if !ok || len(sections) != 1 {
		t.Fatalf("layout sections = %#v", layout["sections"])
	}
	section, ok := sections[0].(map[string]any)
	if !ok || section["heading"] != "Account" || section["columns"] != float64(2) {
		t.Fatalf("layout section = %#v", sections[0])
	}
	rows, ok := section["layoutRows"].([]any)
	if !ok || len(rows) == 0 {
		t.Fatalf("layout rows = %#v", section["layoutRows"])
	}
	layoutFields := map[string]map[string]any{}
	for _, rawRow := range rows {
		row, ok := rawRow.(map[string]any)
		if !ok {
			t.Fatalf("layout row = %#v", rawRow)
		}
		items, ok := row["layoutItems"].([]any)
		if !ok {
			t.Fatalf("layout row items = %#v", row["layoutItems"])
		}
		for _, rawItem := range items {
			item, ok := rawItem.(map[string]any)
			if !ok {
				t.Fatalf("layout item = %#v", rawItem)
			}
			fieldName, _ := item["fieldApiName"].(string)
			if fieldName != "" {
				layoutFields[fieldName] = item
			}
		}
	}
	nameItem, ok := layoutFields["Name"]
	if !ok {
		t.Fatalf("Name missing from layout fields: %#v", layoutFields)
	}
	if nameItem["label"] != "Account Name" || nameItem["required"] != true || nameItem["editableForNew"] != true || nameItem["editableForUpdate"] != true || nameItem["uiBehavior"] != "Required" {
		t.Fatalf("Name layout item = %#v", nameItem)
	}
	components, ok := nameItem["layoutComponents"].([]any)
	if !ok || len(components) != 1 {
		t.Fatalf("Name layout components = %#v", nameItem["layoutComponents"])
	}
	component, ok := components[0].(map[string]any)
	if !ok || component["apiName"] != "Name" || component["componentType"] != "Field" {
		t.Fatalf("Name layout component = %#v", components[0])
	}
	if _, ok := layoutFields["Internal__c"]; ok {
		t.Fatalf("non-createable field included in layout = %#v", layoutFields["Internal__c"])
	}
}

func TestLightningWireGetRecordCreateDefaultsUsesSourceLayoutFields(t *testing.T) {
	root := filepath.Join(".testdata-generated", strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()))
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	writeLightningFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"61.0"}`)
	writeLightningFixtureFile(t, filepath.Join(root, "force-app/main/default/layouts/Account-Account Layout.layout-meta.xml"), `<Layout>
		<layoutSections>
			<label>Account Information</label>
			<style>TwoColumnsTopToBottom</style>
			<layoutColumns>
				<layoutItems><field>Name</field><behavior>Required</behavior></layoutItems>
			</layoutColumns>
			<layoutColumns>
				<layoutItems><field>Description</field><behavior>Edit</behavior></layoutItems>
				<layoutItems><field>Internal__c</field><behavior>Edit</behavior></layoutItems>
			</layoutColumns>
		</layoutSections>
	</Layout>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Label = "Account"
	account.Definition.Fields["Name"] = storage.Field{APIName: "Name", Label: "Account Name", Type: storage.FieldString, Required: true, Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(true)}
	account.Definition.Fields["Description"] = storage.Field{APIName: "Description", Label: "Description", Type: storage.FieldString, Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(true)}
	account.Definition.Fields["Internal__c"] = storage.Field{APIName: "Internal__c", Label: "Internal", Type: storage.FieldString, Createable: storage.BoolFlag(false), Updateable: storage.BoolFlag(false)}
	org.Objects["Account"] = account
	handler := NewWithSource(&org, source)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/lightning/wire/getRecordCreateDefaults", strings.NewReader(`{"objectApiName":"Account"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var out lwcbrowser.WireResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Error != nil {
		t.Fatalf("error = %#v", out.Error)
	}
	payload := out.Data.(map[string]any)
	record := payload["record"].(map[string]any)
	fields := record["fields"].(map[string]any)
	if _, ok := fields["Description"]; !ok {
		t.Fatalf("layout field missing from record defaults: %#v", fields)
	}
	layout := payload["layout"].(map[string]any)
	if layout["id"] != "00h000000000001" {
		t.Fatalf("layout id = %#v", layout["id"])
	}
	section := layout["sections"].([]any)[0].(map[string]any)
	if section["heading"] != "Account Information" || section["columns"] != float64(2) || section["rows"] != float64(1) {
		t.Fatalf("section = %#v", section)
	}
	layoutFields := map[string]map[string]any{}
	for _, rawRow := range section["layoutRows"].([]any) {
		for _, rawItem := range rawRow.(map[string]any)["layoutItems"].([]any) {
			item := rawItem.(map[string]any)
			layoutFields[item["fieldApiName"].(string)] = item
		}
	}
	if layoutFields["Name"]["uiBehavior"] != "Required" || layoutFields["Name"]["required"] != true {
		t.Fatalf("Name layout item = %#v", layoutFields["Name"])
	}
	if layoutFields["Description"]["uiBehavior"] != "Edit" || layoutFields["Description"]["editableForNew"] != true {
		t.Fatalf("Description layout item = %#v", layoutFields["Description"])
	}
	if _, ok := layoutFields["Internal__c"]; ok {
		t.Fatalf("non-createable source layout field included = %#v", layoutFields["Internal__c"])
	}
}

func assertUIAPIFieldDataTypeWithoutRESTType(t *testing.T, fields map[string]any, fieldName, dataType string) map[string]any {
	t.Helper()
	field, ok := fields[fieldName].(map[string]any)
	if !ok {
		t.Fatalf("%s field = %#v", fieldName, fields[fieldName])
	}
	if field["dataType"] != dataType {
		t.Fatalf("%s dataType = %#v in %#v", fieldName, field["dataType"], field)
	}
	if _, hasRESTType := field["type"]; hasRESTType {
		t.Fatalf("UI API object info should not expose REST describe type on %s: %#v", fieldName, field)
	}
	return field
}

func TestLightningWireGetLayoutReturnsSourceLayout(t *testing.T) {
	root := filepath.Join(".testdata-generated", strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()))
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	writeLightningFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"61.0"}`)
	writeLightningFixtureFile(t, filepath.Join(root, "force-app/main/default/layouts/Account-Account Layout.layout-meta.xml"), `<Layout>
		<layoutSections>
			<label>Account Information</label>
			<style>TwoColumnsLeftToRight</style>
			<layoutColumns>
				<layoutItems><field>Name</field><behavior>Required</behavior></layoutItems>
			</layoutColumns>
			<layoutColumns>
				<layoutItems><field>Description</field><behavior>Edit</behavior></layoutItems>
			</layoutColumns>
		</layoutSections>
	</Layout>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Label = "Account"
	account.Definition.Fields["Name"] = storage.Field{APIName: "Name", Label: "Account Name", Type: storage.FieldString, Required: true, Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(true)}
	account.Definition.Fields["Description"] = storage.Field{APIName: "Description", Label: "Description", Type: storage.FieldString, Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(true)}
	org.Objects["Account"] = account
	handler := NewWithSource(&org, source)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/lightning/wire/getLayout", strings.NewReader(`{"objectApiName":"Account","layoutType":"Full","mode":"Create","formFactor":"Small"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var out lwcbrowser.WireResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Error != nil {
		t.Fatalf("error = %#v", out.Error)
	}
	layout := out.Data.(map[string]any)
	if layout["id"] != "00h000000000001" || layout["layoutType"] != "Full" || layout["mode"] != "Create" || layout["objectApiName"] != "Account" {
		t.Fatalf("layout header = %#v", layout)
	}
	section := layout["sections"].([]any)[0].(map[string]any)
	if section["heading"] != "Account Information" || section["tabOrder"] != "LeftRight" {
		t.Fatalf("section = %#v", section)
	}
	row := section["layoutRows"].([]any)[0].(map[string]any)
	items := row["layoutItems"].([]any)
	if len(items) != 2 {
		t.Fatalf("items = %#v", items)
	}
	nameItem := items[0].(map[string]any)
	if nameItem["fieldApiName"] != "Name" || nameItem["uiBehavior"] != "Required" {
		t.Fatalf("Name item = %#v", nameItem)
	}
}

func TestLightningWireGetPicklistValuesAndRecordTypeMap(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Rating"] = storage.Field{
		APIName: "Rating",
		Label:   "Rating",
		Type:    storage.FieldPicklist,
		PicklistValues: []storage.PicklistValue{
			{Value: "Hot", Label: "Hot", Active: true, Default: true},
			{Value: "Warm", Label: "Warm", Active: true},
		},
	}
	org.Objects["Account"] = account
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/lightning/wire/getPicklistValues", strings.NewReader(`{"fieldApiName":"Account.Rating","recordTypeId":"012000000000001"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var out lwcbrowser.WireResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Error != nil {
		t.Fatalf("error = %#v", out.Error)
	}
	payload, ok := out.Data.(map[string]any)
	if !ok {
		t.Fatalf("data = %#v", out.Data)
	}
	values, ok := payload["values"].([]any)
	if !ok || len(values) != 2 {
		t.Fatalf("values = %#v", payload["values"])
	}
	first, ok := values[0].(map[string]any)
	if !ok || first["value"] != "Hot" || first["defaultValue"] != true {
		t.Fatalf("first value = %#v", values[0])
	}

	byType := httptest.NewRecorder()
	handler.ServeHTTP(byType, httptest.NewRequest(http.MethodPost, "/lightning/wire/getPicklistValuesByRecordType", strings.NewReader(`{"objectApiName":"Account","recordTypeId":"012000000000001"}`)))
	if byType.Code != http.StatusOK {
		t.Fatalf("by type status = %d body=%s", byType.Code, byType.Body.String())
	}
	var byTypeOut lwcbrowser.WireResponse
	if err := json.Unmarshal(byType.Body.Bytes(), &byTypeOut); err != nil {
		t.Fatal(err)
	}
	byTypePayload, ok := byTypeOut.Data.(map[string]any)
	if !ok {
		t.Fatalf("by type data = %#v", byTypeOut.Data)
	}
	fields, ok := byTypePayload["picklistFieldValues"].(map[string]any)
	if !ok || fields["Rating"] == nil {
		t.Fatalf("picklistFieldValues = %#v", byTypePayload["picklistFieldValues"])
	}
}

func TestLightningWireGetRelatedListRecordsReturnsChildRows(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Relations = append(account.Definition.Relations, storage.Relationship{
		Field:             "AccountId",
		ParentObjects:     []string{"Account"},
		ChildRelationship: "Contacts",
	})
	account.Records["001XX0000000001"] = storage.Record{
		ID:     "001XX0000000001",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Acme"),
		},
	}
	org.Objects["Account"] = account
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Contact",
			KeyPrefix: "003",
			Fields: map[string]storage.Field{
				"LastName":  {APIName: "LastName", Type: storage.FieldString},
				"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account"},
			},
		},
		Records: map[storage.ID]storage.Record{
			"003XX0000000001": {
				ID:     "003XX0000000001",
				Object: "Contact",
				Fields: map[string]storage.Value{
					"LastName":  storage.StringValue("Smith"),
					"AccountId": storage.IDValue("001XX0000000001"),
				},
			},
		},
	}
	handler := New(&org)

	body := `{"parentRecordId":"001XX0000000001","relatedListId":"Contacts","fields":["Contact.LastName"]}`
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/lightning/wire/getRelatedListRecords", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var out lwcbrowser.WireResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Error != nil {
		t.Fatalf("error = %#v", out.Error)
	}
	payload, ok := out.Data.(map[string]any)
	if !ok || payload["count"] != float64(1) {
		t.Fatalf("data = %#v", out.Data)
	}
	records, ok := payload["records"].([]any)
	if !ok || len(records) != 1 {
		t.Fatalf("records = %#v", payload["records"])
	}
	row, ok := records[0].(map[string]any)
	if !ok || row["apiName"] != "Contact" {
		t.Fatalf("row = %#v", records[0])
	}
}

func TestLightningWireRecordPickerSearchFiltersByObjectAndFields(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["BillingCity"] = storage.Field{APIName: "BillingCity", Label: "Billing City", Type: storage.FieldString}
	account.Records["001XX0000000001"] = storage.Record{
		ID:     "001XX0000000001",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":        storage.StringValue("Acme"),
			"BillingCity": storage.StringValue("Duluth"),
		},
	}
	account.Records["001XX0000000002"] = storage.Record{
		ID:     "001XX0000000002",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":        storage.StringValue("Other"),
			"BillingCity": storage.StringValue("Juneau"),
		},
	}
	org.Objects["Account"] = account
	handler := New(&org)

	body := `{"objectApiName":"Account","searchTerm":"dul","fields":["Name","BillingCity"],"matchingFields":["BillingCity"]}`
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/lightning/wire/recordPickerSearch", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var out lwcbrowser.WireResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Error != nil {
		t.Fatalf("error = %#v", out.Error)
	}
	payload := out.Data.(map[string]any)
	records, ok := payload["records"].([]any)
	if !ok || len(records) != 1 {
		t.Fatalf("records = %#v", payload["records"])
	}
	row := records[0].(map[string]any)
	if row["id"] != "001XX0000000001" || row["apiName"] != "Account" || row["title"] != "Acme" {
		t.Fatalf("row = %#v", row)
	}
	fields := row["fields"].(map[string]any)
	if fields["BillingCity"].(map[string]any)["value"] != "Duluth" {
		t.Fatalf("fields = %#v", fields)
	}
}

func TestLightningSchemaShimServesObjectToken(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lightning/shims/schema/Account.js", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`objectApiName: "Account"`, `toString() { return "Account"; }`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("missing %q in %s", want, rec.Body.String())
		}
	}
}

func TestLightningWireCreateUpdateDeleteRecordMutatesLocalStorage(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	createBody := `{"apiName":"Account","fields":{"Name":"New Local Account","Description":"First"}}`
	create := httptest.NewRecorder()
	handler.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/lightning/wire/createRecord", strings.NewReader(createBody)))
	if create.Code != http.StatusOK {
		t.Fatalf("create status = %d body=%s", create.Code, create.Body.String())
	}
	var created lwcbrowser.WireResponse
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Error != nil {
		t.Fatalf("create error = %#v", created.Error)
	}
	payload, ok := created.Data.(map[string]any)
	if !ok || payload["apiName"] != "Account" || payload["id"] == "" {
		t.Fatalf("created data = %#v", created.Data)
	}
	id := payload["id"].(string)

	updateBody := fmt.Sprintf(`{"fields":{"Id":%q,"Name":"Updated Local Account","Description":null}}`, id)
	update := httptest.NewRecorder()
	handler.ServeHTTP(update, httptest.NewRequest(http.MethodPost, "/lightning/wire/updateRecord", strings.NewReader(updateBody)))
	if update.Code != http.StatusOK {
		t.Fatalf("update status = %d body=%s", update.Code, update.Body.String())
	}
	if got := org.Objects["Account"].Records[storage.ID(id)].Fields["Name"].String; got != "Updated Local Account" {
		t.Fatalf("updated name = %q", got)
	}
	if !org.Objects["Account"].Records[storage.ID(id)].ExplicitNulls["Description"] {
		t.Fatalf("Description explicit null not recorded: %#v", org.Objects["Account"].Records[storage.ID(id)])
	}

	deleteBody := fmt.Sprintf(`{"recordId":%q}`, id)
	del := httptest.NewRecorder()
	handler.ServeHTTP(del, httptest.NewRequest(http.MethodPost, "/lightning/wire/deleteRecord", strings.NewReader(deleteBody)))
	if del.Code != http.StatusOK {
		t.Fatalf("delete status = %d body=%s", del.Code, del.Body.String())
	}
	if !org.Objects["Account"].Records[storage.ID(id)].System.IsDeleted {
		t.Fatalf("record not soft-deleted: %#v", org.Objects["Account"].Records[storage.ID(id)])
	}

	readDeleted := httptest.NewRecorder()
	readDeletedBody := fmt.Sprintf(`{"recordId":%q,"fields":["Account.Name"]}`, id)
	handler.ServeHTTP(readDeleted, httptest.NewRequest(http.MethodPost, "/lightning/wire/getRecord", strings.NewReader(readDeletedBody)))
	if readDeleted.Code != http.StatusOK {
		t.Fatalf("read deleted status = %d body=%s", readDeleted.Code, readDeleted.Body.String())
	}
	var readDeletedOut lwcbrowser.WireResponse
	if err := json.Unmarshal(readDeleted.Body.Bytes(), &readDeletedOut); err != nil {
		t.Fatal(err)
	}
	if readDeletedOut.Error == nil || !strings.Contains(readDeletedOut.Error.Message, "record not found") {
		t.Fatalf("read deleted response = %#v", readDeletedOut)
	}
}

func TestLightningWireGetRecordDistinguishesRequiredAndOptionalMissingFields(t *testing.T) {
	org := testOrg()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:     "Account",
			Label:       "Account",
			PluralLabel: "Accounts",
			KeyPrefix:   "001",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Label: "Account Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"001XX0000000001": {
				ID:     "001XX0000000001",
				Object: "Account",
				Fields: map[string]storage.Value{
					"Name": storage.StringValue("Acme"),
				},
			},
		},
	}
	handler := New(&org)

	optional := httptest.NewRecorder()
	handler.ServeHTTP(optional, httptest.NewRequest(http.MethodPost, "/lightning/wire/getRecord", strings.NewReader(`{
		"recordId":"001XX0000000001",
		"fields":["Account.Name"],
		"optionalFields":["Account.DoesNotExist__c"]
	}`)))
	if optional.Code != http.StatusOK {
		t.Fatalf("optional status = %d body=%s", optional.Code, optional.Body.String())
	}
	var optionalOut lwcbrowser.WireResponse
	if err := json.Unmarshal(optional.Body.Bytes(), &optionalOut); err != nil {
		t.Fatal(err)
	}
	if optionalOut.Error != nil {
		t.Fatalf("optional missing field error = %#v", optionalOut.Error)
	}
	payload, ok := optionalOut.Data.(map[string]any)
	if !ok {
		t.Fatalf("optional data = %#v", optionalOut.Data)
	}
	fields, ok := payload["fields"].(map[string]any)
	if !ok || fields["Name"] == nil || fields["DoesNotExist__c"] != nil {
		t.Fatalf("optional fields = %#v", payload["fields"])
	}

	required := httptest.NewRecorder()
	handler.ServeHTTP(required, httptest.NewRequest(http.MethodPost, "/lightning/wire/getRecord", strings.NewReader(`{
		"recordId":"001XX0000000001",
		"fields":["Account.DoesNotExist__c"]
	}`)))
	if required.Code != http.StatusOK {
		t.Fatalf("required status = %d body=%s", required.Code, required.Body.String())
	}
	var requiredOut lwcbrowser.WireResponse
	if err := json.Unmarshal(required.Body.Bytes(), &requiredOut); err != nil {
		t.Fatal(err)
	}
	if requiredOut.Error == nil || !strings.Contains(requiredOut.Error.Message, "DoesNotExist__c") {
		t.Fatalf("required missing field response = %#v", requiredOut)
	}
}

func TestLightningWireCreateRecordUsesDMLSequencesAndValidation(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	createAccount := func(name string) lwcbrowser.WireResponse {
		t.Helper()
		body := fmt.Sprintf(`{"apiName":"Account","fields":{"Name":%q}}`, name)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/lightning/wire/createRecord", strings.NewReader(body)))
		if rec.Code != http.StatusOK {
			t.Fatalf("create %q status = %d body=%s", name, rec.Code, rec.Body.String())
		}
		var out lwcbrowser.WireResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		if out.Error != nil {
			t.Fatalf("create %q error = %#v", name, out.Error)
		}
		return out
	}

	first := createAccount("First Local Account")
	second := createAccount("Second Local Account")
	firstPayload, ok := first.Data.(map[string]any)
	if !ok {
		t.Fatalf("first data = %#v", first.Data)
	}
	secondPayload, ok := second.Data.(map[string]any)
	if !ok {
		t.Fatalf("second data = %#v", second.Data)
	}
	firstID, _ := firstPayload["id"].(string)
	secondID, _ := secondPayload["id"].(string)
	if firstID == "" || secondID == "" || firstID == secondID {
		t.Fatalf("created ids first=%q second=%q", firstID, secondID)
	}
	if len(org.Objects["Account"].Records) != 2 {
		t.Fatalf("stored records = %#v", org.Objects["Account"].Records)
	}
	if got := org.IDSequences["Account"]; got != 2 {
		t.Fatalf("Account id sequence = %d, want 2", got)
	}
	if org.Objects["Account"].Records[storage.ID(firstID)].System.CreatedByID == "" {
		t.Fatalf("DML audit fields not stamped: %#v", org.Objects["Account"].Records[storage.ID(firstID)])
	}

	missingName := httptest.NewRecorder()
	handler.ServeHTTP(missingName, httptest.NewRequest(http.MethodPost, "/lightning/wire/createRecord", strings.NewReader(`{"apiName":"Account","fields":{"Description":"No name"}}`)))
	if missingName.Code != http.StatusOK {
		t.Fatalf("missing name status = %d body=%s", missingName.Code, missingName.Body.String())
	}
	var missingOut lwcbrowser.WireResponse
	if err := json.Unmarshal(missingName.Body.Bytes(), &missingOut); err != nil {
		t.Fatal(err)
	}
	if missingOut.Error == nil || !strings.Contains(missingOut.Error.Message, "Name") {
		t.Fatalf("missing name response = %#v", missingOut)
	}
	if len(org.Objects["Account"].Records) != 2 {
		t.Fatalf("missing-name create mutated records: %#v", org.Objects["Account"].Records)
	}
}
