package sema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Named API 62/67 publishing observations come from a Knowledge-enabled org.
// API 52/68 retain the existing compatibility controls for the same org shape.
func TestPublishingServiceObservedStaticCall(t *testing.T) {
	for _, api := range []string{"52.0", "62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			for _, test := range []struct {
				name, mode string
				wantError  bool
			}{
				{"publish", "true", false},
				{"wrong-mode", "'invalid'", true},
			} {
				t.Run(test.name, func(t *testing.T) {
					source := "public class ArticlePublishProbe { public static void observe(String articleId) { KbManagement.PublishingService.publishArticle(articleId, " + test.mode + "); } }"
					root := t.TempDir()
					path := filepath.Join(root, "ArticlePublishProbe.cls")
					writeSemaFile(t, path, source)
					if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
						t.Fatal(err)
					}
					writeSemaFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer","features":["Knowledge"],"settings":{"knowledgeSettings":{"enableKnowledge":true}}}`)
					index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
					result := Analyze(index)
					if result.HasErrors() != test.wantError {
						t.Fatalf("errors = %v, want %v: %#v", result.HasErrors(), test.wantError, result.Diagnostics)
					}
					for _, d := range result.Diagnostics {
						if strings.Contains(d.Message, "Type is not visible") {
							t.Fatalf("static receiver was treated as a variable type: %#v", result.Diagnostics)
						}
					}
				})
			}
		})
	}
}
