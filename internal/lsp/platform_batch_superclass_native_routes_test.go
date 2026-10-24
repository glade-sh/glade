package lsp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// The deploy matrix in platform_batch_superclass/org.tsv:2-15 covers the
// inherited callbacks, missing hooks, and explicit generic interface routes.
func TestPlatformBatchSuperclassNamedAndCachedLSPNativeRoutes(t *testing.T) {
	const dir = "../sema/testdata/platform_batch_superclass"
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ ID, Name, Source string }
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, row := range platformMemberNativeTable(t, filepath.Join(dir, "org.tsv"))[1:] {
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			var name, source string
			for _, c := range cases {
				if c.ID == row[2] {
					name, source = c.Name, c.Source
				}
			}
			if source == "" {
				t.Fatal("missing captured source")
			}
			var messages []string
			if err := json.Unmarshal([]byte(row[4]), &messages); err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			classes := filepath.Join(root, "force-app/main/default/classes")
			if err := os.MkdirAll(classes, 0755); err != nil {
				t.Fatal(err)
			}
			write := func(path, content string) {
				t.Helper()
				if err := os.WriteFile(path, []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			write(filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+row[0]+`"}`)
			path := filepath.Join(classes, name+".cls")
			write(path, source)
			write(path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+row[0]+`</apiVersion><status>Active</status></ApexClass>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			s, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, s)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			h := NewHandler(index)
			defer h.parser.Close()
			for _, route := range []string{"open", "cached"} {
				t.Run(route, func(t *testing.T) {
					notes := h.DidOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uriFromPath(path), Text: source, Version: 1}})
					if (diagnosticsCount(t, notes) > 0) != (row[3] == "rejected") {
						t.Fatalf("native %s (%s); LSP=%+v", row[3], row[4], notes)
					}
					diagnostics := h.diagnosticsForDocument(uriFromPath(path))
					for _, message := range messages {
						found := false
						for _, d := range diagnostics {
							found = found || d.Message == message || d.NativeMessage == message
						}
						if !found {
							t.Errorf("missing native diagnostic %q; LSP=%+v", message, diagnostics)
						}
					}
				})
			}
		})
	}
}
