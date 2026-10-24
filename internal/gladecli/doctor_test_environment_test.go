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

func TestDoctorConfiguredTestEnvironment(t *testing.T) {
	root := t.TempDir()
	err := os.WriteFile(filepath.Join(root, "glade.yml"), []byte("test:\n  user:\n    timeZone: Europe/Berlin\n    locale: en_GB\n  clock:\n    start: '2026-05-04T22:30:00Z'\n"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"text", "json"} {
		args := []string{"--project", root}
		if format == "json" {
			args = append(args, "--json")
		}
		var out bytes.Buffer
		_, err := runDoctor(context.Background(), args, &out)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"Europe/Berlin", "en_GB", "en_US", "USD", "2026-05-04T22:30:00Z"} {
			if !strings.Contains(out.String(), want) {
				t.Errorf("doctor %s lacks %s: %s", format, want, out.String())
			}
		}
		if format == "json" {
			var data map[string]json.RawMessage
			if err := json.Unmarshal(out.Bytes(), &data); err != nil {
				t.Fatal(err)
			}
			if len(data["testEnvironment"]) == 0 {
				t.Fatal("doctor lacks testEnvironment")
			}
		}
	}
}
