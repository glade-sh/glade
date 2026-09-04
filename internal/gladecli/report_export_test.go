package gladecli

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestReportExportConfinesSavedRunFiles(t *testing.T) {
	for _, scenario := range []string{"regular", "file-link", "directory-link", "outside-run", "inside-output", "inside-output-alias", "output-hardlink"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			runsDir := filepath.Join(root, "runs")
			runDir := filepath.Join(runsDir, "run")
			outside := filepath.Join(root, "outside", "sentinel.txt")
			writeTestFile(t, outside, "outside sentinel must never be exported")
			writeTestFile(t, filepath.Join(runDir, "safe.txt"), "public report")
			switch scenario {
			case "file-link", "directory-link":
				target := outside
				if scenario == "directory-link" {
					target = filepath.Dir(outside)
				}
				if err := os.Symlink(target, filepath.Join(runDir, "linked")); err != nil {
					t.Fatal(err)
				}
			case "outside-run":
				runDir = filepath.Dir(outside)
			}
			latest, err := json.Marshal(map[string]string{"runDir": runDir, "runId": "run"})
			if err != nil {
				t.Fatal(err)
			}
			writeTestFile(t, filepath.Join(runsDir, "latest.json"), string(latest))
			output := filepath.Join(root, "report.zip")
			if scenario == "inside-output" {
				output = filepath.Join(runDir, "report.zip")
			}
			if scenario == "inside-output-alias" {
				alias := filepath.Join(root, "run-alias")
				if err := os.Symlink(runDir, alias); err != nil {
					t.Fatal(err)
				}
				output = filepath.Join(alias, "report.zip")
			}
			if scenario == "output-hardlink" {
				if err := os.Link(filepath.Join(runDir, "safe.txt"), output); err != nil {
					t.Fatal(err)
				}
			}
			err = runReportExportLatest(runsDir, output, "zip", io.Discard)
			valid := scenario == "regular" || scenario == "output-hardlink"
			if valid && err != nil {
				t.Fatal(err)
			}
			if !valid && err == nil {
				t.Fatal("export accepted a link or a run outside the selected runs directory")
			}
			if scenario == "output-hardlink" {
				data, err := os.ReadFile(filepath.Join(runDir, "safe.txt"))
				if err != nil || string(data) != "public report" {
					t.Fatalf("export changed source: %q, %v", data, err)
				}
			}
			archive, openErr := zip.OpenReader(output)
			if openErr != nil {
				if scenario == "regular" {
					t.Fatal(openErr)
				}
				return
			}
			defer archive.Close()
			if scenario == "regular" && (len(archive.File) != 1 || archive.File[0].Name != "safe.txt") {
				t.Fatalf("normal export entries = %#v", archive.File)
			}
			for _, file := range archive.File {
				reader, err := file.Open()
				if err != nil {
					t.Fatal(err)
				}
				data, err := io.ReadAll(reader)
				reader.Close()
				if err != nil || bytes.Contains(data, []byte("outside sentinel")) {
					t.Fatalf("unsafe exported content: entry=%s readError=%v", file.Name, err)
				}
			}
		})
	}
}
