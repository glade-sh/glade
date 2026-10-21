package startupcache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestFreshRejectsPriorNamespaceRemapTransformation(t *testing.T) {
	for _, scope := range []string{"project", "dependency", "none"} {
		t.Run(scope, func(t *testing.T) {
			root := t.TempDir()
			consumer := filepath.Join(root, "consumer")
			writeStartupCacheTestFile(t, filepath.Join(consumer, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			writeStartupCacheTestFile(t, filepath.Join(consumer, "force-app/main/default/classes/Consumer.cls"), "public class Consumer {}\n")
			if scope == "project" {
				writeStartupCacheTestFile(t, filepath.Join(consumer, "glade.yml"), "project:\n  namespaceRemaps: [\"BasePkg:stagepkg\"]\n")
			}
			if scope == "dependency" {
				provider := filepath.Join(root, "provider")
				writeStartupCacheTestFile(t, filepath.Join(consumer, "glade.yml"), "project:\n  managedPackageDependencies: [\"stagepkg:../provider\"]\n")
				writeStartupCacheTestFile(t, filepath.Join(provider, "sfdx-project.json"), `{"namespace":"stagepkg","packageDirectories":[{"path":"force-app","default":true}]}`)
				writeStartupCacheTestFile(t, filepath.Join(provider, "glade.yml"), "project:\n  namespaceRemaps: [\"BasePkg:stagepkg\"]\n")
				writeStartupCacheTestFile(t, filepath.Join(provider, "force-app/main/default/classes/Prefix.cls"), "global class Prefix { global static String value = 'BasePkg'; }\n")
			}
			loaded, err := project.Load(consumer)
			if err != nil {
				t.Fatal(err)
			}
			if scope == "dependency" && (len(loaded.ManagedPackageDependencies) != 1 || loaded.ManagedPackageDependencies[0].Project == nil || len(loaded.ManagedPackageDependencies[0].Project.NamespaceRemaps) == 0) {
				t.Fatal("fixture did not load the dependency's namespace remap")
			}
			entry := NewEntry(consumer, loaded, typesys.Index{}, storage.NewOrgState(), CompiledRuntime{})
			if !Fresh(&entry, consumer, Version) {
				t.Fatal("current manifest is stale")
			}
			// Reconstruct the prior project identity: it recorded raw remap
			// rules, without identifying the source transformation itself.
			semantic := semanticManifest{Project: projectSemantics(consumer, loaded, make(map[string]bool)), Features: entry.Manifest.Features}
			data, err := json.Marshal(semantic)
			if err != nil {
				t.Fatal(err)
			}
			data = regexp.MustCompile(`,"namespaceRemapFingerprint":"[^"]*"`).ReplaceAll(data, nil)
			legacy := sha256.Sum256(data)
			entry.Manifest.ProjectDigest = hex.EncodeToString(legacy[:])
			if got, want := Fresh(&entry, consumer, Version), scope == "none"; got != want {
				t.Fatalf("prior transformation manifest Fresh() = %v, want %v", got, want)
			}
		})
	}
}
