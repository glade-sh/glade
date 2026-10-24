package gladecli

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lsp"
	"github.com/glade-sh/glade/internal/testreport"
)

func TestContextAssignmentProjectRoutesNativeConformance(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Setenv("GLADE_ROOT", "")
	const fixtures = "../sema/testdata/context_assignment"
	raw, err := os.ReadFile(filepath.Join(fixtures, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(bytes.NewReader(raw))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 37 {
		t.Fatalf("native matrix rows=%d err=%v", len(rows), err)
	}
	for _, row := range rows[1:] {
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			root := t.TempDir()
			base := filepath.Join(fixtures, "project")
			err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() {
					return nil
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				relative, err := filepath.Rel(base, path)
				if err != nil {
					return err
				}
				writeTestFile(t, filepath.Join(root, relative), strings.ReplaceAll(string(body), "62.0", row[0]))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			body, err := os.ReadFile(filepath.Join(fixtures, row[2]+".apex"))
			if err != nil {
				t.Fatal(err)
			}
			accepted := row[3] == "true"
			var args []string
			var classPath string
			if row[1] == "anonymous" {
				args = []string{"exec", "--project", root, "--json", string(body)}
			} else {
				name := "AssignmentNamed"
				source := "public class " + name + " { public static void probe() { " + string(body) + " } }"
				if row[1] == "istest" {
					name = "AssignmentAcceptedTest"
					source = "@IsTest private class " + name + " { @IsTest static void probe() { " + string(body) + " } }"
				}
				classPath = filepath.Join(root, "force-app/main/default/classes", name+".cls")
				writeTestFile(t, classPath, source)
				writeTestFile(t, classPath+"-meta.xml", "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>"+row[0]+"</apiVersion><status>Active</status></ApexClass>")
				args = []string{"check", "--project", root, "--no-cache", "--no-progress", "--json"}
				if row[1] == "istest" {
					args = []string{"test", "--project", root, "--class", name, "--no-cache", "--no-serve", "--no-progress", "--json"}
				}
			}
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), args, &stdout, &stderr)
			if (code == 0) != accepted || code > 1 {
				t.Fatalf("native accepted=%t (%s); exit=%d stderr=%s stdout=%s", accepted, row[4], code, stderr.String(), stdout.String())
			}
			if !accepted && !strings.Contains(stdout.String()+stderr.String(), "GLADESEMA018") {
				t.Fatalf("missing assignment diagnostic: stderr=%s stdout=%s", stderr.String(), stdout.String())
			}
			if row[1] == "istest" && accepted {
				run, err := decodeTestRunJSON(stdout.Bytes())
				if err != nil {
					t.Fatal(err)
				}
				count := 0
				for _, suite := range run.Suites {
					for _, result := range suite.Cases {
						count++
						if result.Status != testreport.StatusPass {
							t.Fatalf("test result=%+v", result)
						}
					}
				}
				if count != 1 {
					t.Fatalf("executed %d tests, want 1", count)
				}
			}
			if classPath != "" {
				t.Run("lsp", func(t *testing.T) {
					index, err := loadIndex(root)
					if err != nil {
						t.Fatal(err)
					}
					handler := lsp.NewHandler(index)
					defer handler.HandleJSON([]byte(`{"jsonrpc":"2.0","id":1,"method":"shutdown"}`))
					source, err := os.ReadFile(classPath)
					if err != nil {
						t.Fatal(err)
					}
					uri := lsp.DocumentURI((&url.URL{Scheme: "file", Path: classPath}).String())
					notifications := handler.DidOpen(lsp.DidOpenTextDocumentParams{TextDocument: lsp.TextDocumentItem{URI: uri, Version: 1, Text: string(source)}})
					rejected := false
					for _, notification := range notifications {
						payload, ok := notification.Params.(lsp.PublishDiagnosticsParams)
						if !ok {
							t.Fatalf("unexpected LSP payload: %T", notification.Params)
						}
						for _, diag := range payload.Diagnostics {
							if diag.Severity == 1 {
								rejected = true
								if diag.Code != "GLADESEMA018" {
									encoded, _ := json.Marshal(diag)
									t.Fatalf("unexpected LSP diagnostic: %s", encoded)
								}
							}
						}
					}
					if rejected == accepted {
						t.Fatalf("native accepted=%t; LSP rejected=%t notifications=%+v", accepted, rejected, notifications)
					}
				})
			}
		})
	}
}
