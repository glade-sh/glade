package project

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Source deployment ignores metadata files outside LWC bundles, while the
// same malformed files remain discoverable in their proper source directories.
func TestLooseLWCMetadataNativeDiscovery(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "pr163_loose_metadata", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 13 {
		t.Fatalf("native rows %d: %v", len(rows), err)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0]+"_"+row[1], func(t *testing.T) {
			root := t.TempDir()
			base := filepath.Join(root, "force-app", "main", "default")
			lwc := filepath.Join(base, "lwc")
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+row[1]+`"}`)
			bundle := filepath.Join(lwc, "probe")
			writeFile(t, filepath.Join(bundle, "probe.js"), `import { LightningElement } from 'lwc'; export default class Probe extends LightningElement {}`)
			writeFile(t, filepath.Join(bundle, "probe.html"), `<template><p>Test</p></template>`)
			writeFile(t, filepath.Join(bundle, "probe.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+row[1]+`</apiVersion><isExposed>false</isExposed></LightningComponentBundle>`)
			var target, content string
			switch row[0] {
			case "loose_cls":
				target, content = filepath.Join(lwc, "Stray.cls"), "not valid Apex class @@@"
			case "inside_cls":
				target, content = filepath.Join(base, "classes", "Stray.cls"), "not valid Apex class @@@"
			case "loose_trigger":
				target, content = filepath.Join(lwc, "Stray.trigger"), "not valid Apex trigger @@@"
			case "inside_trigger":
				target, content = filepath.Join(base, "triggers", "Stray.trigger"), "not valid Apex trigger @@@"
			case "loose_object":
				target, content = filepath.Join(lwc, "Stray__c.object-meta.xml"), "<notValid/>"
			case "inside_object":
				target, content = filepath.Join(base, "objects", "Stray__c", "Stray__c.object-meta.xml"), "<notValid/>"
			default:
				t.Fatalf("unknown case %q", row[0])
			}
			writeFile(t, target, content)
			p, err := Load(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(p.LWCFiles) != 1 || len(p.LWCHTMLFiles) != 1 || len(p.LWCMetaFiles) != 1 {
				t.Fatalf("bundle discovery: js %v html %v meta %v", p.LWCFiles, p.LWCHTMLFiles, p.LWCMetaFiles)
			}
			files := p.ApexFiles
			if strings.HasSuffix(row[0], "object") {
				files = p.ObjectFiles
			}
			want := 0
			if strings.HasPrefix(row[0], "inside_") {
				want = 1
			}
			if len(files) != want || want == 1 && files[0] != target {
				t.Fatalf("discovery %v, want %d target %s", files, want, target)
			}
		})
	}
}
