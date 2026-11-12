package compile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/project"
)

// Both reference collection and compilation should use one Node child while
// retaining the same metadata hydration and resulting bundle.
func TestLWCCompilerSingleProcessPerRequest(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	sourceRoot := filepath.Clean(filepath.Join(cwd, "..", "..", ".."))
	if os.Getenv("GLADE_LWC_DEPS_DIR") == "" {
		dependencyRoot, err := gladehome.Root()
		if err != nil {
			t.Fatal(err)
		}
		t.Setenv("GLADE_LWC_DEPS_DIR", filepath.Join(dependencyRoot, "third_party", "lwc"))
	}
	root := t.TempDir()
	bundle := filepath.Join(root, "force-app", "main", "default", "lwc", "sample")
	writeCompileFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"62.0"}`)
	writeCompileFixtureFile(t, filepath.Join(bundle, "sample.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><isExposed>false</isExposed></LightningComponentBundle>`)
	writeCompileFixtureFile(t, filepath.Join(bundle, "sample.js"), `import { LightningElement } from 'lwc'; import NAME from '@salesforce/schema/Account.Name'; export default class Sample extends LightningElement { name = NAME; }`)
	writeCompileFixtureFile(t, filepath.Join(bundle, "sample.html"), `<template></template>`)
	observer := filepath.Join(root, "process-observer.cjs")
	writeCompileFixtureFile(t, observer, `require('node:fs').appendFileSync(process.env.GLADE_PROCESS_OBSERVER, 'start\n');`)
	trace := filepath.Join(root, "process-count.txt")
	t.Setenv("GLADE_PROCESS_OBSERVER", trace)
	t.Setenv("NODE_OPTIONS", os.Getenv("NODE_OPTIONS")+" --require "+l03JSON(t, observer))
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		run  func() error
	}{
		{"single", func() error {
			_, err := Compile(p, Options{OutDir: filepath.Join(root, "single"), RepoRoot: sourceRoot})
			return err
		}},
		{"batch", func() error {
			results, err := CompileBatch(p, Options{OutDir: filepath.Join(root, "batch"), RepoRoot: sourceRoot})
			if err != nil {
				return err
			}
			if len(results) != 1 {
				t.Fatalf("batch results = %d, want 1", len(results))
			}
			for _, result := range results {
				return result.Err
			}
			return nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(trace, nil, 0600); err != nil {
				t.Fatal(err)
			}
			if err := tc.run(); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(trace)
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Count(string(data), "start\n"); got != 1 {
				t.Fatalf("Node child starts = %d, want 1", got)
			}
		})
	}
}
