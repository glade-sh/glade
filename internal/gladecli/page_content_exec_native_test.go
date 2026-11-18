package gladecli

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/visualforce"
)

// page_error_host_routes/org.tsv:2,4,8,10 records the anonymous HTML and PDF
// missing-page behavior at source APIs 62.0 and 67.0.
func TestExecPageContentNative(t *testing.T) {
	fixture := filepath.Join("..", "apextest", "testdata", "conformance", "page_error_host_routes", "project")
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			if err := filepath.WalkDir(fixture, func(path string, e fs.DirEntry, err error) error {
				if err != nil || e.IsDir() {
					return err
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(fixture, path)
				if err != nil {
					return err
				}
				if rel == "sfdx-project.json" {
					data = []byte(strings.ReplaceAll(string(data), "67.0", api))
				}
				writeTestFile(t, filepath.Join(root, rel), string(data))
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			version := strings.TrimSuffix(api, ".0")
			baselineEnvironments := visualforce.VMRenderEnvironmentCountForTest()
			t.Cleanup(func() {
				if got := visualforce.VMRenderEnvironmentCountForTest(); got != baselineEnvironments {
					t.Errorf("exec leaked render environments: got %d want %d", got, baselineEnvironments)
				}
			})
			var stdout, stderr bytes.Buffer
			source := "System.assertEquals('caught|System.VisualforceException|Page MissingHostPage does not exist', PageErrorHostV" + version + ".html());"
			if code := Run(context.Background(), []string{"exec", "--project", root, source}, &stdout, &stderr); code != 0 {
				t.Fatalf("HTML code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
			}
			stdout.Reset()
			stderr.Reset()
			source = "PageErrorHostV" + version + ".pdf();"
			if code := Run(context.Background(), []string{"exec", "--project", root, source}, &stdout, &stderr); code == 0 || !strings.Contains(stdout.String()+stderr.String(), "Page MissingHostPage does not exist") {
				t.Fatalf("PDF code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
			}
		})
	}
}
