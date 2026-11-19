package project

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverLooseLWCCompanionsNativeDeploy(t *testing.T) {
	const fixture = "testdata/pr163_loose_companions"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil || len(rows) != 13 {
		t.Fatalf("native rows: %d, %v", len(rows), err)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0]+"_"+row[1], func(t *testing.T) {
			parts := strings.SplitN(row[0], "_", 2)
			if len(parts) != 2 || row[2] != "deploy" {
				t.Fatalf("invalid native row: %v", row)
			}
			ext, variant := parts[0], parts[1]
			if variant != "loose" && variant != "bundle_invalid" {
				t.Fatalf("invalid variant: %v", row)
			}
			if variant == "loose" && row[3] != "accepted" || variant == "bundle_invalid" && row[3] != "rejected" {
				t.Fatalf("invalid outcome: %v", row)
			}
			invalid, err := os.ReadFile(filepath.Join(fixture, "invalid."+ext))
			if err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+row[1]+`"}`)
			lwc := filepath.Join(root, "force-app", "main", "default", "lwc")
			bundle := filepath.Join(lwc, "probe")
			writeFile(t, filepath.Join(lwc, "stray."+ext), string(invalid))
			files := map[string]string{
				"js":          `import { LightningElement } from 'lwc'; export default class Probe extends LightningElement {}`,
				"html":        `<template><p>Test</p></template>`,
				"css":         `.example { color: red; }`,
				"js-meta.xml": `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + row[1] + `</apiVersion><isExposed>false</isExposed></LightningComponentBundle>`,
			}
			if variant == "bundle_invalid" {
				files[ext] = string(invalid)
			}
			for name, content := range files {
				writeFile(t, filepath.Join(bundle, "probe."+name), content)
			}
			p, err := Load(root)
			if err != nil {
				t.Fatal(err)
			}
			checks := []struct {
				name   string
				files  []string
				suffix string
			}{
				{"JS", p.LWCFiles, "/lwc/probe/probe.js"},
				{"HTML", p.LWCHTMLFiles, "/lwc/probe/probe.html"},
				{"CSS", p.LWCCSSFiles, "/lwc/probe/probe.css"},
				{"metadata", p.LWCMetaFiles, "/lwc/probe/probe.js-meta.xml"},
			}
			for _, check := range checks {
				if len(check.files) != 1 || !strings.HasSuffix(filepath.ToSlash(check.files[0]), check.suffix) {
					t.Errorf("%s files = %#v; want only %s", check.name, check.files, check.suffix)
				}
			}
		})
	}
}
