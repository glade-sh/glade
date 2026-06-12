package enterprise

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadServiceConfigValidatesFixturePath(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "fixtures", "pricing.json")
	if err := os.MkdirAll(filepath.Dir(fixture), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixture, []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "services.yml")
	if err := os.WriteFile(path, []byte(`version: 0
mode: strict
calloutFixtures: [fixtures/pricing.json]
asyncDrain: true
asyncMaxDepth: 5
platformEventsOut: reports/platform-events.jsonl
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadServiceConfig(path)
	if err != nil {
		t.Fatalf("LoadServiceConfig: %v", err)
	}
	if cfg.Mode != "strict" || len(cfg.CalloutFixtures) != 1 || cfg.AsyncMaxDepth != 5 {
		t.Fatalf("config = %#v", cfg)
	}
}

func TestLoadServiceConfigRejectsMissingFixture(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "services.yml")
	if err := os.WriteFile(path, []byte(`version: 0
mode: strict
calloutFixtures: [fixtures/missing.json]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadServiceConfig(path)
	if err == nil || !strings.Contains(err.Error(), "fixtures/missing.json") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadServiceConfigRejectsAsyncDrainWithoutDepth(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "services.yml")
	if err := os.WriteFile(path, []byte(`version: 0
mode: strict
asyncDrain: true
asyncMaxDepth: 0
`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadServiceConfig(path)
	if err == nil || !strings.Contains(err.Error(), "asyncMaxDepth") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadServiceConfigRejectsUnknownKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "services.yml")
	if err := os.WriteFile(path, []byte(`version: 0
potato: true
`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadServiceConfig(path)
	if err == nil || !strings.Contains(err.Error(), "unsupported service config key") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadServiceConfigRejectsUnsupportedVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "services.yml")
	if err := os.WriteFile(path, []byte(`version: 2
mode: strict
`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadServiceConfig(path)
	if err == nil || !strings.Contains(err.Error(), "unsupported services.yml version 2") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadServiceConfigRejectsEmptyInlineListValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "services.yml")
	if err := os.WriteFile(path, []byte(`version: 0
calloutFixtures: [fixtures/one.json, ]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadServiceConfig(path)
	if err == nil || !strings.Contains(err.Error(), "empty inline list value") {
		t.Fatalf("error = %v", err)
	}
}
