package compile

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// A constant module specifier does not remove the dynamic-component capability requirement.
func TestDynamicStaticSpecifierNativeDeploy(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "pr163_dynamic_static", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 25 {
		t.Fatalf("native rows %d: %v", len(rows), err)
	}
	repo, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0]+"_"+row[1], func(t *testing.T) {
			tokens := strings.Split(row[0], "_")
			if len(tokens) != 3 {
				t.Fatalf("invalid row: %v", row)
			}
			route, style, capability := tokens[0], tokens[1], tokens[2]
			root := t.TempDir()
			lwc := filepath.Join(root, "force-app", "main", "default", "lwc")
			writeCompileFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+row[1]+`"}`)
			meta := func(cap bool) string {
				c := ""
				if cap {
					c = `<capabilities><capability>lightning__dynamicComponent</capability></capabilities>`
				}
				return `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + row[1] + `</apiVersion><isExposed>false</isExposed>` + c + `</LightningComponentBundle>`
			}
			child := filepath.Join(lwc, "child")
			writeCompileFixtureFile(t, filepath.Join(child, "child.js"), `import { LightningElement } from 'lwc'; export default class Child extends LightningElement {}`)
			writeCompileFixtureFile(t, filepath.Join(child, "child.html"), `<template><p>Child</p></template>`)
			writeCompileFixtureFile(t, filepath.Join(child, "child.js-meta.xml"), meta(false))
			arg := "'c/child'"
			if style == "template" {
				arg = "`c/child`"
			} else if style != "literal" {
				t.Fatalf("unknown style %q", style)
			}
			bundle := filepath.Join(lwc, "probe")
			writeCompileFixtureFile(t, filepath.Join(bundle, "probe.js-meta.xml"), meta(capability == "cap"))
			module := `export async function load(name) { return await import(` + arg + `); }`
			var js string
			switch route {
			case "utility":
				js = module
			case "sibling":
				js = `import { LightningElement } from 'lwc'; import { load } from './helper'; export default class Probe extends LightningElement { connectedCallback() { load('c/child'); } }`
				writeCompileFixtureFile(t, filepath.Join(bundle, "helper.js"), module)
				writeCompileFixtureFile(t, filepath.Join(bundle, "probe.html"), `<template><p>Test</p></template>`)
			case "component":
				js = `import { LightningElement } from 'lwc'; export default class Probe extends LightningElement { async connectedCallback() { const name = 'c/child'; await import(` + arg + `); } }`
				writeCompileFixtureFile(t, filepath.Join(bundle, "probe.html"), `<template><p>Test</p></template>`)
			default:
				t.Fatalf("unknown route %q", route)
			}
			writeCompileFixtureFile(t, filepath.Join(bundle, "probe.js"), js)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			_, err = Compile(p, Options{OutDir: filepath.Join(root, "dist"), Namespace: "c", RepoRoot: repo})
			if row[3] == "accepted" && err != nil {
				t.Fatalf("native accepted; error %v", err)
			}
			if row[3] == "rejected" && (err == nil || !strings.Contains(err.Error(), "LWC1503")) {
				t.Fatalf("native rejected with %s, error %v", row[4], err)
			}
		})
	}
}
