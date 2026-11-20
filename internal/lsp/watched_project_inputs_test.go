package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestRegisteredProjectInputChangesMatchFreshWorkspaceSymbols(t *testing.T) {
	for _, tc := range []struct {
		name, path, original, edited, query, originalSymbol, editedSymbol string
	}{
		{"ignore", ".forceignore", "", "classes/First.cls\n", "First", "First", ""},
		{"object", "src/objects/Thing__c.object", `<CustomObject><fields><fullName>Before__c</fullName><type>Text</type></fields></CustomObject>`, `<CustomObject><fields><fullName>After__c</fullName><type>Text</type></fields></CustomObject>`, "__c", "Before__c", "After__c"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			write := func(name, content string) {
				t.Helper()
				path := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}
			write("sfdx-project.json", `{"packageDirectories":[{"path":".","default":true}],"sourceApiVersion":"67.0"}`)
			write("classes/First.cls", "public class First {}")
			write(tc.path, tc.original)
			load := func() (Base, error) {
				p, err := project.Load(root)
				if err != nil {
					return Base{}, err
				}
				s, err := schema.LoadProject(p)
				if err != nil {
					return Base{}, err
				}
				index, artifacts := typesys.BuildWithArtifacts(p, s)
				return Base{Index: index, Project: p, Artifacts: &artifacts}, nil
			}
			base, err := load()
			if err != nil {
				t.Fatal(err)
			}
			h := NewPendingHandler()
			h.SetBase(base)
			h.SetRefresher(func(context.Context, RefreshRequest) (Base, error) { return load() })
			c := startServe(t, h)
			c.send(map[string]any{"id": 1, "method": "initialize", "params": map[string]any{"capabilities": map[string]any{"workspace": map[string]any{"didChangeWatchedFiles": map[string]any{"dynamicRegistration": true}}}}})
			c.mustNext(5 * time.Second)
			c.send(map[string]any{"method": "initialized", "params": map[string]any{}})
			registration := c.mustNext(5 * time.Second)
			var params struct {
				Registrations []struct {
					RegisterOptions struct {
						Watchers []struct {
							GlobPattern string `json:"globPattern"`
						} `json:"watchers"`
					} `json:"registerOptions"`
				} `json:"registrations"`
			}
			if err := json.Unmarshal(registration["params"], &params); err != nil {
				t.Fatal(err)
			}
			if len(params.Registrations) != 1 {
				t.Fatalf("registrations = %s", registration["params"])
			}
			matches := func(path string) bool {
				for _, watcher := range params.Registrations[0].RegisterOptions.Watchers {
					pattern := strings.TrimPrefix(watcher.GlobPattern, "**/")
					matched, err := filepath.Match(pattern, filepath.Base(path))
					if err != nil {
						t.Fatal(err)
					}
					if matched {
						return true
					}
				}
				return false
			}
			if matches("src/objects/Thing__c.objectx") || matches("src/.forceignore-backup") {
				t.Fatal("neighbor file matched registration")
			}
			query := func(client *serveClient) []byte {
				client.send(map[string]any{"id": 2, "method": "workspace/symbol", "params": map[string]any{"query": tc.query}})
				response := client.mustNext(10 * time.Second)
				raw, err := json.Marshal(response)
				if err != nil {
					t.Fatal(err)
				}
				return raw
			}
			initial := query(c)
			if !bytes.Contains(initial, []byte(tc.originalSymbol)) {
				t.Fatalf("initial symbols = %s", initial)
			}
			for i, content := range []string{tc.edited, tc.original} {
				write(tc.path, content)
				seq := h.baseGen.Load().seq
				if matches(tc.path) {
					c.send(watchedMessage(FileEvent{URI: uriFromPath(filepath.Join(root, tc.path)), Type: fileChanged}))
					deadline := time.Now().Add(10 * time.Second)
					for h.baseGen.Load().seq == seq && time.Now().Before(deadline) {
						time.Sleep(10 * time.Millisecond)
					}
					if h.baseGen.Load().seq == seq {
						t.Fatal("watched event did not refresh")
					}
				}
				fresh, err := load()
				if err != nil {
					t.Fatal(err)
				}
				freshHandler := NewPendingHandler()
				freshHandler.SetBase(fresh)
				freshClient := startServe(t, freshHandler)
				want, got := query(freshClient), query(c)
				if i == 0 && bytes.Equal(want, initial) {
					t.Fatalf("edit did not change fresh symbols: %s", want)
				}
				if i == 0 && tc.editedSymbol != "" && !bytes.Contains(want, []byte(tc.editedSymbol)) {
					t.Fatalf("edited symbols = %s", want)
				}
				if !bytes.Equal(got, want) {
					t.Errorf("edit %d: live symbols %s, fresh %s (registered=%v)", i, got, want, matches(tc.path))
				}
			}
		})
	}
}
