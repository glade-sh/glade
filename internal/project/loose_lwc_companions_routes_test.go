package project

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The same captured bundle boundary applies to all roots passed to collectFiles.
func TestDiscoverLooseLWCCompanionsSourceRoutes(t *testing.T) {
	const fixture = "testdata/pr163_loose_companions"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		row := row
		if !strings.HasSuffix(row[0], "_loose") {
			continue
		}
		ext := strings.TrimSuffix(row[0], "_loose")
		for _, route := range []string{"extra", "dependency"} {
			t.Run(route+"_"+ext+"_"+row[1], func(t *testing.T) {
				if row[2] != "deploy" || row[3] != "accepted" {
					t.Fatalf("invalid native row: %v", row)
				}
				invalid, err := os.ReadFile(filepath.Join(fixture, "invalid."+ext))
				if err != nil {
					t.Fatal(err)
				}
				workspace := t.TempDir()
				consumer := filepath.Join(workspace, "consumer")
				writeFile(t, filepath.Join(consumer, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+row[1]+`"}`)
				var src string
				if route == "extra" {
					writeFile(t, filepath.Join(consumer, "glade.yml"), "project:\n  extraMetadataDirs: [supplemental]\n")
					src = filepath.Join(consumer, "supplemental")
				} else {
					writeFile(t, filepath.Join(consumer, "glade.yml"), "project:\n  dependencies:\n    - name: Library\n      path: ../library\n")
					writeFile(t, filepath.Join(consumer, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":"Library"}]}],"sourceApiVersion":"`+row[1]+`"}`)
					src = filepath.Join(workspace, "library", "force-app")
					writeFile(t, filepath.Join(workspace, "library", "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true,"package":"Library"}],"sourceApiVersion":"`+row[1]+`"}`)
				}
				lwc := filepath.Join(src, "main", "default", "lwc")
				bundle := filepath.Join(lwc, "probe")
				writeFile(t, filepath.Join(lwc, "stray."+ext), string(invalid))
				writeFile(t, filepath.Join(bundle, "probe.js"), `import { LightningElement } from 'lwc'; export default class Probe extends LightningElement {}`)
				writeFile(t, filepath.Join(bundle, "probe.html"), `<template><p>Test</p></template>`)
				writeFile(t, filepath.Join(bundle, "probe.css"), `.example { color: red; }`)
				writeFile(t, filepath.Join(bundle, "probe.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+row[1]+`</apiVersion><isExposed>false</isExposed></LightningComponentBundle>`)
				p, err := Load(consumer)
				if err != nil {
					t.Fatal(err)
				}
				if route == "dependency" {
					if len(p.DependencyDiagnostics) != 0 || len(p.ManagedPackageDependencies) != 1 || p.ManagedPackageDependencies[0].Project == nil {
						t.Fatalf("dependency loading: %#v %#v", p.ManagedPackageDependencies, p.DependencyDiagnostics)
					}
					p = *p.ManagedPackageDependencies[0].Project
				}
				for _, check := range []struct {
					name   string
					files  []string
					suffix string
				}{
					{"JS", p.LWCFiles, "/lwc/probe/probe.js"}, {"HTML", p.LWCHTMLFiles, "/lwc/probe/probe.html"},
					{"CSS", p.LWCCSSFiles, "/lwc/probe/probe.css"}, {"metadata", p.LWCMetaFiles, "/lwc/probe/probe.js-meta.xml"},
				} {
					if len(check.files) != 1 || !strings.HasSuffix(filepath.ToSlash(check.files[0]), check.suffix) {
						t.Errorf("%s = %#v; want only %s", check.name, check.files, check.suffix)
					}
				}
			})
		}
	}
}
