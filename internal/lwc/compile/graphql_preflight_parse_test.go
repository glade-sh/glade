package compile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/project"
)

// Count the compiler script's own Babel parses of each synthetic source. The
// reference-only process uses a different grammar and is intentionally excluded.
func TestGraphQLPreflightReusesParsedSources(t *testing.T) {
	root := t.TempDir()
	bundle := filepath.Join(root, "force-app", "main", "default", "lwc", "sample")
	writeCompileFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"62.0"}`)
	writeCompileFixtureFile(t, filepath.Join(bundle, "sample.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><isExposed>false</isExposed></LightningComponentBundle>`)
	writeCompileFixtureFile(t, filepath.Join(bundle, "sample.js"), `// GLADE_SINGLE_PARSE_MARKER
import { LightningElement } from 'lwc';
import { value } from './helper';
export default class Sample extends LightningElement { get text() { return value; } }`)
	writeCompileFixtureFile(t, filepath.Join(bundle, "helper.js"), `// GLADE_SINGLE_PARSE_MARKER
export const value = 'synthetic';`)
	writeCompileFixtureFile(t, filepath.Join(bundle, "sample.html"), `<template><span>{text}</span></template>`)
	observer := filepath.Join(root, "parse-observer.cjs")
	writeCompileFixtureFile(t, observer, `const Module = require('node:module');
const fs = require('node:fs');
const load = Module._load;
Module._load = function(request, ...rest) {
  const value = load.call(this, request, ...rest);
  if (request === '@babel/parser' && !value.__gladeParseObserved) {
    const parse = value.parse;
    value.parse = function(source, options) {
      const caller = (new Error().stack || '').split('\n')[2] || '';
      const grammar = (options?.plugins || []).some(p => Array.isArray(p) && p[0] === 'decorators');
      if (typeof source === 'string' && source.includes('GLADE_SINGLE_PARSE_MARKER') && grammar && caller.includes('/compile.mjs:')) {
        fs.appendFileSync(process.env.GLADE_PARSE_OBSERVER, 'parse\n');
      }
      return parse.apply(this, arguments);
    };
    value.__gladeParseObserved = true;
  }
  return value;
};`)
	log := filepath.Join(root, "parse-count.txt")
	t.Setenv("GLADE_PARSE_OBSERVER", log)
	t.Setenv("NODE_OPTIONS", os.Getenv("NODE_OPTIONS")+" --require "+l03JSON(t, observer))
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
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Compile(p, Options{OutDir: filepath.Join(root, "dist"), RepoRoot: sourceRoot}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(data), "parse\n"); got != 2 {
		t.Fatalf("compiler preflight and GraphQL parsed two sources %d times; want 2", got)
	}
}

// Run the existing native fixture and output-byte comparisons against this
// checkout's compiler script when dependencies are supplied separately.
func TestGraphQLPreflightBatchEquivalence(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GLADE_ROOT", filepath.Clean(filepath.Join(cwd, "..", "..", "..")))
	TestCompileBatchEquivalence(t)
}

// Preflight failures in any file precede GraphQL validation, even if an
// earlier file contains a bad GraphQL field. With no preflight failure the
// original GraphQL diagnostic must still be returned.
func TestGraphQLPreflightDiagnosticPrecedence(t *testing.T) {
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
	observer := filepath.Join(t.TempDir(), "stack-limit.cjs")
	writeCompileFixtureFile(t, observer, `Error.stackTraceLimit = 100;`)
	t.Setenv("NODE_OPTIONS", os.Getenv("NODE_OPTIONS")+" --require "+l03JSON(t, observer))
	for _, tc := range []struct {
		name, laterSource, want string
	}{
		{"later-preflight-failure", `import invalid from 'lwc'; export { invalid };`, `LWC1503`},
		{"graphql-failure", `export const valid = 1;`, `Validation error (FieldUndefined@`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			bundle := filepath.Join(root, "force-app", "main", "default", "lwc", "sample")
			writeCompileFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"62.0"}`)
			writeCompileFixtureFile(t, filepath.Join(bundle, "sample.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><isExposed>false</isExposed></LightningComponentBundle>`)
			writeCompileFixtureFile(t, filepath.Join(bundle, "sample.js"), `import { LightningElement } from 'lwc'; export default class Sample extends LightningElement {}`)
			writeCompileFixtureFile(t, filepath.Join(bundle, "sample.html"), `<template></template>`)
			writeCompileFixtureFile(t, filepath.Join(bundle, "aGraph.js"), "import NAME from '@salesforce/schema/Account.Name';\nimport { gql } from 'lightning/uiGraphQLApi';\nexport const query = gql`query { uiapi { query { Account { edges { node { SyntheticMissing__c { value } } } } } } }`;\n")
			writeCompileFixtureFile(t, filepath.Join(bundle, "zLater.js"), tc.laterSource)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			_, err = Compile(p, Options{OutDir: filepath.Join(root, "dist"), RepoRoot: sourceRoot})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("compile error = %v; want %q", err, tc.want)
			}
			if tc.name == "graphql-failure" {
				if strings.Contains(err.Error(), "at preflightBundleImports") {
					t.Fatalf("GraphQL error gained an internal preflight stack frame: %v", err)
				}
				batch, batchErr := CompileBatch(p, Options{OutDir: filepath.Join(root, "batch-dist"), RepoRoot: sourceRoot})
				if batchErr != nil {
					t.Fatal(batchErr)
				}
				for _, result := range batch {
					if result.Err == nil || !strings.Contains(result.Err.Error(), tc.want) {
						t.Fatalf("batch GraphQL error = %v; want %q", result.Err, tc.want)
					}
					if strings.Contains(result.Err.Error(), "at preflightBundleImports") {
						t.Fatalf("batch GraphQL error gained an internal preflight stack frame: %v", result.Err)
					}
				}
			}
		})
	}
}

// Replay the complete existing expression fixture against the checkout script.
func TestLWCCheckoutExpressionNativeRows(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GLADE_ROOT", filepath.Clean(filepath.Join(cwd, "..", "..", "..")))
	TestL01SalesforceConformance(t)
}
