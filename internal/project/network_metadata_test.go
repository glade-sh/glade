package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNetworkExtraMetadataDiscoveryRoutes(t *testing.T) {
	for _, route := range []string{"package", "extra", "external_extra"} {
		t.Run(route, func(t *testing.T) {
			workspace := t.TempDir()
			root := filepath.Join(workspace, "project")
			put := func(path, body string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			put(filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
			dir := "force-app"
			if route == "extra" {
				dir = "supplemental"
			}
			if route == "external_extra" {
				dir = "../shared"
			}
			if route != "package" {
				put(filepath.Join(root, "glade.yml"), "project:\n  extraMetadataDirs: ["+dir+", "+dir+"]\n")
			}
			network := filepath.Join(root, dir, "networks/Synthetic Portal.network-meta.xml")
			put(network, `<Network><status>UnderConstruction</status><urlPathPrefix>synthetic</urlPathPrefix></Network>`)
			put(filepath.Join(root, dir, "profiles/Synthetic.profile-meta.xml"), `<Profile><custom>true</custom></Profile>`)
			p, err := Load(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(p.ProfileFiles) != 1 {
				t.Fatalf("extra profile discovery/deduplication: %v", p.ProfileFiles)
			}
			raw, err := json.Marshal(p)
			if err != nil {
				t.Fatal(err)
			}
			var result struct {
				NetworkFiles []string `json:"networkFiles"`
			}
			if err := json.Unmarshal(raw, &result); err != nil {
				t.Fatal(err)
			}
			if len(result.NetworkFiles) != 1 || result.NetworkFiles[0] != network {
				t.Fatalf("Network discovery/deduplication: %s", raw)
			}
			if len(p.PackageDirectories) != 1 || p.PackageDirectories[0].Path != "force-app" {
				t.Fatalf("extra dirs replaced packages: %v", p.PackageDirectories)
			}
		})
	}
}

func TestExtraMetadataMissingDirectoryRejected(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "glade.yml"), []byte("project:\n  extraMetadataDirs: [missing]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil {
		t.Fatal("missing extra metadata directory accepted")
	}
}
