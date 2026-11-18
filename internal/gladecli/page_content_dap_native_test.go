package gladecli

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/dap"
	"github.com/glade-sh/glade/internal/visualforce"
)

// page_error_host_routes/org.tsv:2,4,8,10 captures anonymous page errors at
// APIs 62 and 67. Debugger execution must retain the same project context.
func TestDAPPageContentNative(t *testing.T) {
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
			t.Run("abandoned-before-start", func(t *testing.T) {
				baseline := visualforce.VMRenderEnvironmentCountForTest()
				handler := dap.NewHandler(dap.Snapshot{})
				source := "PageErrorHostV" + version + ".html();"
				if err := handleDAPLaunch(context.Background(), handler, dap.LaunchRequest{ProjectRoot: root, AnonymousBody: source}, root, "", true); err != nil {
					t.Fatal(err)
				}
				handler.Handle(dap.Request{Command: dap.CommandDisconnect})
				if got := visualforce.VMRenderEnvironmentCountForTest(); got != baseline {
					t.Fatalf("unstarted debugger leaked render context: got %d want %d", got, baseline)
				}
			})
			for _, kind := range []string{"html", "pdf"} {
				t.Run(kind, func(t *testing.T) {
					baseline := visualforce.VMRenderEnvironmentCountForTest()
					inR, inW := io.Pipe()
					outR, outW := io.Pipe()
					done := make(chan error, 1)
					go func() { done <- runDAP(context.Background(), nil, inR, outW) }()
					messages := make(chan map[string]any, 16)
					go readDAPMessages(t, outR, messages)
					source := "PageErrorHostV" + version + ".pdf();"
					if kind == "html" {
						source = "System.assertEquals('caught|System.VisualforceException|Page MissingHostPage does not exist', PageErrorHostV" + version + ".html());"
					}
					writeDAPRequest(t, inW, dap.CommandInitialize, 1, nil)
					writeDAPRequest(t, inW, dap.CommandLaunch, 2, map[string]any{"projectRoot": root, "anonymousBody": source})
					writeDAPRequest(t, inW, dap.CommandConfigurationDone, 3, nil)
					output := waitForDAPTerminatedAndStderr(t, messages)
					writeDAPRequest(t, inW, dap.CommandDisconnect, 4, nil)
					_ = inW.Close()
					_ = outW.Close()
					select {
					case err := <-done:
						if err != nil {
							t.Fatal(err)
						}
					case <-time.After(dapTestTimeout):
						t.Fatal("DAP did not stop")
					}
					if kind == "html" && output != "" {
						t.Fatalf("unexpected HTML failure: %s", output)
					}
					if kind == "pdf" && !strings.Contains(output, "Page MissingHostPage does not exist") {
						t.Fatalf("missing native PDF fault: %s", output)
					}
					if got := visualforce.VMRenderEnvironmentCountForTest(); got != baseline {
						t.Fatalf("render context leak: got %d want %d", got, baseline)
					}
				})
			}
		})
	}
}
