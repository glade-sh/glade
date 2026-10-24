package lsp

import (
	"encoding/json"
	"github.com/glade-sh/glade/internal/diagnostic"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Replay the owned named declarations from the native inner-type case matrix
// through editor diagnostics. Anonymous declarations use the request route in
// apextest; this route covers both normal and @IsTest source files.
func TestInnerTypeScopeNativeLSPRoutes(t *testing.T) {
	for _, family := range []string{"inner_type_scope", "inner_type_shapes"} {
		fixture := filepath.Join("..", "sema", "testdata", family)
		if family == "inner_type_shapes" {
			fixture = filepath.Join("..", "apextest", "testdata", "conformance", family)
		}
		data, err := os.ReadFile(filepath.Join(fixture, "cases.json"))
		if err != nil {
			t.Fatal(err)
		}
		var cases []struct {
			ID          string            `json:"id"`
			Classes     map[string]string `json:"classes"`
			TestClasses map[string]string `json:"testClasses"`
			Captures    []struct {
				API      string `json:"api"`
				Route    string `json:"route"`
				Compiled bool   `json:"compiled"`
				Outcome  string `json:"outcome"`
			} `json:"captures"`
		}
		if err := json.Unmarshal(data, &cases); err != nil {
			t.Fatal(err)
		}
		for _, tc := range cases {
			for _, capture := range tc.Captures {
				// Anonymous-only rejections describe declaration preparation,
				// not named class diagnostics.
				if capture.Route == "anonymous" {
					continue
				}
				t.Run(family+"/"+tc.ID+"/"+capture.API+"/"+capture.Route, func(t *testing.T) {
					classes := tc.Classes
					if capture.Route == "isTest" && len(tc.TestClasses) > 0 {
						classes = tc.TestClasses
					}
					root := t.TempDir()
					var paths []string
					for name, source := range classes {
						path := filepath.Join(root, name+".cls")
						if err := os.WriteFile(path, []byte(source), 0600); err != nil {
							t.Fatal(err)
						}
						paths = append(paths, path)
					}
					index := typesys.Build(project.Project{Root: root, SourceAPIVersion: capture.API, ApexFiles: paths}, schema.Schema{})
					if index.HasErrors() {
						t.Fatal(index.Diagnostics)
					}
					handler := NewHandler(index)
					defer handler.parser.Close()
					rejected := false
					matchedNative := false
					var nativeErrors []string
					for name, source := range classes {
						notes := handler.DidOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uriFromPath(filepath.Join(root, name+".cls")), Version: 1, Text: source}})
						for _, note := range notes {
							if payload, ok := note.Params.(PublishDiagnosticsParams); ok {
								for _, d := range payload.Diagnostics {
									if d.Severity == diagnosticSeverityError {
										rejected = true
									}
								}
							}
						}
					}
					for name := range classes {
						for _, d := range handler.diagnosticsForDocument(uriFromPath(filepath.Join(root, name+".cls"))) {
							if d.Severity != diagnostic.Error {
								continue
							}
							message := d.NativeMessage
							if message == "" {
								message = d.Message
							}
							nativeErrors = append(nativeErrors, message)
							if strings.Contains(capture.Outcome, message) && d.Range != nil && d.File == filepath.Join(root, name+".cls") {
								matchedNative = true
							}
						}
					}
					if !capture.Compiled && !matchedNative {
						t.Fatalf("native rejection %q; LSP native diagnostics=%q", capture.Outcome, nativeErrors)
					}
					if rejected == capture.Compiled {
						t.Fatalf("native compiled=%v; LSP rejected=%v", capture.Compiled, rejected)
					}
				})
			}
		}
	}
}

// Native permission-set Id compilation: dispatch_followups/org.tsv:12-13.
// Platform type-token receivers: corpus_shape/native/org.tsv:6,11.
func TestPlatformConstantReceiverLSPRoutes(t *testing.T) {
	const body = `Id permissionSetId = null;
        AccessLevel bare = AccessLevel.USER_MODE.withPermissionSetId(permissionSetId);
        System.AccessLevel qualified = System.AccessLevel.USER_MODE.withPermissionSetId(permissionSetId);
        Account.SObjectType.getDescribe(); Account.Name.getDescribe();`
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"named", "isTest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				root := t.TempDir()
				path := filepath.Join(root, "ConstantRoutes.cls")
				source := "public class ConstantRoutes { public static void observe() {" + body + "} }"
				if route == "isTest" {
					source = "@IsTest private class ConstantRoutes { @IsTest static void observe() {" + body + "} }"
				}
				if err := os.WriteFile(path, []byte(source), 0600); err != nil {
					t.Fatal(err)
				}
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
				if index.HasErrors() {
					t.Fatal(index.Diagnostics)
				}
				handler := NewHandler(index)
				defer handler.parser.Close()
				notes := handler.DidOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uriFromPath(path), Version: 1, Text: source}})
				for _, note := range notes {
					if payload, ok := note.Params.(PublishDiagnosticsParams); ok {
						for _, d := range payload.Diagnostics {
							if d.Severity == diagnosticSeverityError {
								t.Fatalf("constant receiver rejected: %+v", d)
							}
						}
					}
				}
			})
		}
	}
}
