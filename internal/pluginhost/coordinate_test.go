package pluginhost

import "testing"

func TestParsePluginRefAcceptsScopedNamesAndVersions(t *testing.T) {
	tests := []struct {
		input       string
		wantName    string
		wantVersion string
	}{
		{input: "@glade/compat", wantName: "@glade/compat"},
		{input: "@glade/compat@0.1.0", wantName: "@glade/compat", wantVersion: "0.1.0"},
		{input: "@acme/quality-tools@1.2.3", wantName: "@acme/quality-tools", wantVersion: "1.2.3"},
		{input: "compat", wantName: "@glade/compat"},
		{input: "performance", wantName: "@glade/performance"},
	}
	for _, tt := range tests {
		got, err := ParsePluginRef(tt.input)
		if err != nil {
			t.Fatalf("ParsePluginRef(%q) error: %v", tt.input, err)
		}
		if got.Name != tt.wantName || got.Version != tt.wantVersion {
			t.Fatalf("ParsePluginRef(%q)=%#v, want name=%q version=%q", tt.input, got, tt.wantName, tt.wantVersion)
		}
	}
}

func TestParsePluginRefRejectsUnsafeNames(t *testing.T) {
	for _, input := range []string{
		"",
		"@",
		"@glade",
		"@glade/",
		"@glade/../compat",
		"@glade/com/pat",
		"@glade/compat@../1.0.0",
		"../compat",
		"https://example.com/plugin.tar.gz",
	} {
		if _, err := ParsePluginRef(input); err == nil {
			t.Fatalf("ParsePluginRef(%q) succeeded, want error", input)
		}
	}
}

func TestPluginRefManifestNameAndStorageKey(t *testing.T) {
	ref, err := ParsePluginRef("@acme/quality@1.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if got := ref.ManifestName(); got != "quality" {
		t.Fatalf("ManifestName=%q, want quality", got)
	}
	if got := ref.StorageName(); got != "acme__quality" {
		t.Fatalf("StorageName=%q, want acme__quality", got)
	}
}
