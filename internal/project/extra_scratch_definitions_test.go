package project

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func writeShapeFile(t *testing.T, root, name, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A project that names one definition (Networks off) but keeps another under
// config/ (Networks on) may deploy into either shape. The unreferenced
// definition cannot turn the runtime feature on, but it does mean the static
// object-absence gate cannot establish that Networks-only objects such as
// AccountBrand are absent.
func TestOrgShapeIncludesUnreferencedConfigDefinitions(t *testing.T) {
	root := t.TempDir()
	writeShapeFile(t, root, "sfdx-project.json", `{"packageDirectories":[{"path":"force-app","definitionFile":"./config/scratch-orgs/base-scratch-def.json","default":true}],"sourceApiVersion":"62.0"}`)
	writeShapeFile(t, root, "config/scratch-orgs/base-scratch-def.json", `{"edition":"Enterprise","features":[],"settings":{"communitiesSettings":{"enableNetworksEnabled":false}}}`)
	if got := StaticOnlyShapeFeatures(Project{Root: root}); len(got) != 0 {
		t.Fatalf("base definition alone must add no static-only features: %v", got)
	}
	writeShapeFile(t, root, "config/scratch-orgs/advanced-scratch-def.json", `{"edition":"Enterprise","features":["Communities"],"settings":{"communitiesSettings":{"enableNetworksEnabled":true}}}`)
	if got := StaticOnlyShapeFeatures(Project{Root: root}); !slices.Contains(got, StaticOnlyShapeFeaturePrefix+"NetworksEnabled") {
		t.Fatalf("unreferenced config definition must contribute a static-only Networks marker: %v", got)
	}
	if !slices.Contains(OrgShapeInputFiles(root), filepath.Join(root, "config", "scratch-orgs", "advanced-scratch-def.json")) {
		t.Fatalf("extra definition missing from shape inputs")
	}
}

// Unreferenced definitions never change the runtime feature set: only the
// named definition decides which org features the tests run with.
func TestRuntimeOrgShapeIgnoresUnreferencedConfigDefinitions(t *testing.T) {
	root := t.TempDir()
	writeShapeFile(t, root, "sfdx-project.json", `{"packageDirectories":[{"path":"force-app","definitionFile":"./config/scratch-orgs/base-scratch-def.json","default":true}],"sourceApiVersion":"62.0"}`)
	writeShapeFile(t, root, "config/scratch-orgs/base-scratch-def.json", `{"edition":"Enterprise","features":[],"settings":{"communitiesSettings":{"enableNetworksEnabled":false}}}`)
	before := OrgShapeFeatures(root)
	beforePolicy := StandardFieldPolicyFeatures(Project{Root: root})
	writeShapeFile(t, root, "config/scratch-orgs/advanced-scratch-def.json", `{"edition":"Enterprise","features":["Communities","MultiCurrency"],"settings":{"communitiesSettings":{"enableNetworksEnabled":true}}}`)
	if got := OrgShapeFeatures(root); !slices.Equal(got, before) {
		t.Fatalf("runtime shape changed: before %v after %v", before, got)
	}
	if got := StandardFieldPolicyFeatures(Project{Root: root}); !slices.Equal(got, beforePolicy) {
		t.Fatalf("policy features changed: before %v after %v", beforePolicy, got)
	}
	if slices.Contains(OrgShapeFeatures(root), "NetworksEnabled") {
		t.Fatalf("Networks must stay off at runtime")
	}
}
