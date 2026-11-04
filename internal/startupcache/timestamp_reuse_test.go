package startupcache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTimestampOnlyReuseAndContentInvalidation(t *testing.T) {
	for _, name := range []string{"source", "config"} {
		t.Run(name, func(t *testing.T) {
			root, source, entry := newExactInvalidationFixture(t, nil)
			target := source
			if name == "config" {
				target = filepath.Join(root, "sfdx-project.json")
			}
			info, err := os.Stat(target)
			if err != nil {
				t.Fatal(err)
			}
			changed := info.ModTime().Add(2 * time.Second)
			if err := os.Chtimes(target, changed, changed); err != nil {
				t.Fatal(err)
			}
			proof, err := ValidateInputWithSourceDigests(root, nil)
			if err != nil {
				t.Fatal(err)
			}
			if proof.Digest() == "" {
				t.Fatal("missing generation proof")
			}
			if !Fresh(&entry, root, Version) {
				t.Error("timestamp-only change invalidated fresh content")
			}
			if !freshManifestWithValidatedInput(entry.Version, entry.ProjectRoot, entry.PlatformABI, entry.Manifest, root, Version, proof) {
				t.Error("timestamp-only change invalidated validated content")
			}
			data, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			data[len(data)-2] ^= 1
			if err := os.WriteFile(target, data, 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(target, changed, changed); err != nil {
				t.Fatal(err)
			}
			if Fresh(&entry, root, Version) {
				t.Error("same-size content change accepted")
			}
			if freshManifestWithValidatedInput(entry.Version, entry.ProjectRoot, entry.PlatformABI, entry.Manifest, root, Version, proof) {
				t.Error("stale validated proof accepted changed live content")
			}
		})
	}
}
