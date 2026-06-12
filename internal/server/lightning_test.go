package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
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

func TestLightningLabelShimResolvesPackageCPrefix(t *testing.T) {
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

func TestLightningWireApexReturnsData(t *testing.T) {
	root, err := lightningTestRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
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
		Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001"},
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
}
