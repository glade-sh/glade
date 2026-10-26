package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorPDFRendererConfiguration(t *testing.T) {
	for _, command := range []string{"", "/opt/tools/render-pdf"} {
		t.Run(command, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "glade.yml"), []byte("project:\n  root: .\npdf:\n  command: '"+command+"'\n"), 0600); err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if _, err := runDoctor(context.Background(), []string{"--project", root, "--json"}, &out); err != nil {
				t.Fatal(err)
			}
			var got map[string]any
			if err := json.Unmarshal(out.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			want := "local text-only PDF writer"
			if command != "" {
				want = command
			}
			if got["pdfRenderer"] != want {
				t.Fatalf("pdfRenderer=%v want=%s", got["pdfRenderer"], want)
			}
			out.Reset()
			if _, err := runDoctor(context.Background(), []string{"--project", root}, &out); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), "PDF") || !strings.Contains(out.String(), want) {
				t.Fatalf("missing PDF line: %s", out.String())
			}
		})
	}
}
