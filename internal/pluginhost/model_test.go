package pluginhost

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestManifestRoundTrip(t *testing.T) {
	input := Manifest{
		APIVersion: "glade.plugin.v1",
		Name:       "compat",
		Version:    "0.1.0",
		Summary:    "Compatibility fixtures.",
		Commands: []CommandManifest{
			{Path: []string{"compat"}, Summary: "Run compatibility commands."},
			{Path: []string{"surface"}, Summary: "Run surface ledger commands."},
		},
		MinimumGladeVersion: "0.1.0",
		Source:              "github.com/glade-sh/glade/tools",
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var got Manifest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "compat" || len(got.Commands) != 2 || got.Commands[1].Path[0] != "surface" {
		t.Fatalf("manifest round trip lost data: %#v", got)
	}
}

func TestManifestValidateRejectsBadRoot(t *testing.T) {
	for _, root := range []string{
		"version", "completion", "doctor", "config", "init", "parse",
		"inspect", "schema", "check", "exec", "test", "dev", "report",
		"lsp", "profile", "debug", "editor", "dap", "package", "server",
		"playground", "db", "plugins", "help",
	} {
		t.Run(root, func(t *testing.T) {
			manifest := Manifest{
				APIVersion: "glade.plugin.v1",
				Name:       "bad",
				Version:    "0.1.0",
				Commands:   []CommandManifest{{Path: []string{root}, Summary: "Override core command."}},
			}

			err := manifest.Validate()
			if err == nil {
				t.Fatal("expected core command collision to fail")
			}
			if !strings.Contains(err.Error(), "core glade command") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestManifestValidateRequiresUsableFields(t *testing.T) {
	tests := []struct {
		name     string
		manifest Manifest
		want     string
	}{
		{
			name: "api version",
			manifest: Manifest{
				APIVersion: "v0",
				Name:       "compat",
				Version:    "0.1.0",
				Commands:   []CommandManifest{{Path: []string{"compat"}}},
			},
			want: "unsupported",
		},
		{
			name: "name",
			manifest: Manifest{
				APIVersion: APIVersion,
				Version:    "0.1.0",
				Commands:   []CommandManifest{{Path: []string{"compat"}}},
			},
			want: "name",
		},
		{
			name: "version",
			manifest: Manifest{
				APIVersion: APIVersion,
				Name:       "compat",
				Commands:   []CommandManifest{{Path: []string{"compat"}}},
			},
			want: "version",
		},
		{
			name: "commands",
			manifest: Manifest{
				APIVersion: APIVersion,
				Name:       "compat",
				Version:    "0.1.0",
			},
			want: "command",
		},
		{
			name: "empty command root",
			manifest: Manifest{
				APIVersion: APIVersion,
				Name:       "compat",
				Version:    "0.1.0",
				Commands:   []CommandManifest{{Path: []string{""}}},
			},
			want: "root",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.Validate()
			if err == nil {
				t.Fatal("expected validation failure")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}
}

func TestManifestValidateRejectsUnsafeNameAndVersion(t *testing.T) {
	tests := []struct {
		name     string
		manifest Manifest
	}{
		{
			name:     "relative name",
			manifest: Manifest{APIVersion: APIVersion, Name: "../../owned", Version: "0.1.0", Commands: []CommandManifest{{Path: []string{"compat"}}}},
		},
		{
			name:     "absolute name",
			manifest: Manifest{APIVersion: APIVersion, Name: "/tmp/owned", Version: "0.1.0", Commands: []CommandManifest{{Path: []string{"compat"}}}},
		},
		{
			name:     "relative version",
			manifest: Manifest{APIVersion: APIVersion, Name: "compat", Version: "../0.1.0", Commands: []CommandManifest{{Path: []string{"compat"}}}},
		},
		{
			name:     "odd bytes",
			manifest: Manifest{APIVersion: APIVersion, Name: "compat", Version: "0.1.0 beta", Commands: []CommandManifest{{Path: []string{"compat"}}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.Validate()
			if err == nil {
				t.Fatal("expected unsafe manifest token to fail")
			}
			if !strings.Contains(err.Error(), "safe") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestManifestValidateRejectsUnsafeCommandPath(t *testing.T) {
	tests := []struct {
		name string
		path []string
	}{
		{name: "relative root", path: []string{"../compat"}},
		{name: "absolute root", path: []string{"/compat"}},
		{name: "space in root", path: []string{"bad root"}},
		{name: "unsafe subcommand", path: []string{"compat", "../local-tests"}},
		{name: "empty subcommand", path: []string{"compat", ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := Manifest{
				APIVersion: APIVersion,
				Name:       "compat",
				Version:    "0.1.0",
				Commands:   []CommandManifest{{Path: tt.path}},
			}

			err := manifest.Validate()
			if err == nil {
				t.Fatal("expected unsafe command path to fail")
			}
			if !strings.Contains(err.Error(), "safe") && !strings.Contains(err.Error(), "required") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestManifestCommandRootsDeduplicatesInOrder(t *testing.T) {
	manifest := Manifest{Commands: []CommandManifest{
		{Path: []string{"compat", "local-tests"}},
		{Path: []string{"surface"}},
		{Path: []string{"compat", "mvp"}},
		{Path: nil},
	}}

	got := manifest.CommandRoots()
	if strings.Join(got, ",") != "compat,surface" {
		t.Fatalf("unexpected command roots: %#v", got)
	}
}
