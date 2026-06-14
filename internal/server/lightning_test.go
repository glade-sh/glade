package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
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
		"Lightning Out app not found",
		"Lightning component not found",
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
	for _, want := range []string{"CurrentPageReference", "NavigationMixin", "window.location.assign", "/lwc/preview/record/"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("missing %q in body = %q", want, rec.Body.String())
		}
	}
	if strings.Contains(rec.Body.String(), "lightning/navigation is not implemented") {
		t.Fatalf("body = %q", rec.Body.String())
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
	name, ok := fields["Name"].(map[string]any)
	if !ok || name["label"] != "Account Name" || name["createable"] != true || name["updateable"] != true {
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
