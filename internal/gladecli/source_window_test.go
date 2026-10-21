package gladecli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunConfigShowSourceAPI68Window(t *testing.T) {
	for _, version := range []string{"68.0", "69.0"} {
		t.Run(version, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+version+`"}`)
			var stdout, stderr bytes.Buffer
			args := []string{"config", "show", "--project", root}
			if version == "68.0" {
				args = append(args, "--json")
			}
			code := Run(context.Background(), args, &stdout, &stderr)
			if version == "69.0" {
				want := `unsupported source API version "69.0" for Apex; expected a whole version no later than 68.0`
				if code == 0 || !strings.Contains(stderr.String(), want) {
					t.Fatalf("code=%d stdout=%s stderr=%s; want %q", code, stdout.String(), stderr.String(), want)
				}
				return
			}
			if code != 0 {
				t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
			}
			var got struct {
				SourceAPIVersion string `json:"sourceApiVersion"`
			}
			decodeCLIEnvelopeData(t, stdout.Bytes(), "config show", &got)
			if got.SourceAPIVersion != "68.0" {
				t.Fatalf("sourceApiVersion = %q; want 68.0", got.SourceAPIVersion)
			}
		})
	}
}
