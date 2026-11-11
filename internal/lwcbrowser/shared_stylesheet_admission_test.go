package lwcbrowser

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// Exercise the server's public page-preparation path against the deployment rows.
func TestPreparePageConfigNativeStylesheetAdmission(t *testing.T) {
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GLADE_HOME", repo)
	fixture := filepath.Join("..", "lwc", "compile", "testdata", "pr163_stylesheet")
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 {
		t.Fatalf("native row count = %d", len(rows))
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			root := t.TempDir()
			writeLWCAPIVersionFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true},{"path":"installed"}],"sourceApiVersion":"`+row[1]+`"}`)
			lwc := filepath.Join(root, "force-app", "main", "default", "lwc")
			meta := `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + row[1] + `</apiVersion><isExposed>false</isExposed></LightningComponentBundle>`
			for _, name := range []string{"probe.js", "probe.html", "probe.css"} {
				b, err := os.ReadFile(filepath.Join(fixture, name))
				if err != nil {
					t.Fatal(err)
				}
				s := string(b)
				if row[0] == "css_missing" {
					s = strings.ReplaceAll(s, "sharedStyles", "absentStyles")
				}
				writeLWCAPIVersionFixtureFile(t, filepath.Join(lwc, "probe", name), s)
			}
			writeLWCAPIVersionFixtureFile(t, filepath.Join(lwc, "probe", "probe.js-meta.xml"), meta)
			if row[3] == "accepted" {
				b, err := os.ReadFile(filepath.Join(fixture, "sharedStyles.css"))
				if err != nil {
					t.Fatal(err)
				}
				overlay := filepath.Join(root, "installed", "main", "default", "lwc", "sharedStyles")
				writeLWCAPIVersionFixtureFile(t, filepath.Join(overlay, "sharedStyles.css"), string(b))
				writeLWCAPIVersionFixtureFile(t, filepath.Join(overlay, "sharedStyles.js-meta.xml"), meta)
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			_, manifest, err := PreparePageConfig(p, filepath.Join(root, "cache"))
			if row[3] == "rejected" {
				if err == nil || !strings.Contains(err.Error(), "No MODULE named") {
					t.Fatalf("native rejected; page preparation error = %v", err)
				}
			} else {
				if err != nil {
					t.Fatalf("native accepted; page preparation error = %v", err)
				}
				if _, ok := manifest.Modules["c:sharedStyles"]; !ok {
					t.Fatalf("source-backed dependency missing from manifest: files=%v modules=%v", p.LWCMetaFiles, manifest.Modules)
				}
			}
		})
	}
}

// External namespace resolution is delegated to the host; local inventory
// cannot decide whether an installed package supplies that module.
func TestPreparePageConfigForeignStylesheetDeferred(t *testing.T) {
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GLADE_HOME", repo)
	root := t.TempDir()
	writeLWCAPIVersionFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	dir := filepath.Join(root, "force-app", "main", "default", "lwc", "probe")
	writeLWCAPIVersionFixtureFile(t, filepath.Join(dir, "probe.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><isExposed>false</isExposed></LightningComponentBundle>`)
	writeLWCAPIVersionFixtureFile(t, filepath.Join(dir, "probe.js"), `import { LightningElement } from 'lwc'; export default class Probe extends LightningElement {}`)
	writeLWCAPIVersionFixtureFile(t, filepath.Join(dir, "probe.html"), `<template><p>Test</p></template>`)
	writeLWCAPIVersionFixtureFile(t, filepath.Join(dir, "probe.css"), `@import 'vendor/sharedStyles';`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = PreparePageConfig(p, filepath.Join(root, "cache"))
	if err != nil {
		t.Fatalf("foreign namespace was locally rejected: %v", err)
	}
}
