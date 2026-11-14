package inputview

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirectoryListingDetectsHiddenMembership(t *testing.T) {
	for _, remove := range []bool{false, true} {
		t.Run(map[bool]string{false: "added", true: "removed"}[remove], func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, ".hidden")
			if remove {
				if err := os.WriteFile(path, nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			v := New()
			if _, err := v.ReadDir(dir); err != nil {
				t.Fatal(err)
			}
			var err error
			if remove {
				err = os.Remove(path)
			} else {
				err = os.WriteFile(path, nil, 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := v.Recheck(); err == nil {
				t.Fatal("hidden membership change was not detected")
			}
		})
	}
}

func TestProjectRootWalkSkipsCacheAndPinsHiddenFolders(t *testing.T) {
	root := t.TempDir()
	hidden := filepath.Join(root, ".settings")
	if err := os.Mkdir(hidden, 0700); err != nil {
		t.Fatal(err)
	}
	v := New()
	if err := v.WalkDirProjectRoot(root, func(string, os.DirEntry, error) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".glade"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := v.Recheck(); err != nil {
		t.Fatalf("cache directory changed root observation: %v", err)
	}
	v2 := New()
	if err := v2.WalkDirProjectRoot(root, func(string, os.DirEntry, error) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hidden, ".item"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := v2.Recheck(); err == nil {
		t.Fatal("hidden folder membership was not observed")
	}
}

func TestFinderFileDoesNotChangeListingFingerprint(t *testing.T) {
	dir := t.TempDir()
	v := New()
	if _, err := v.ReadDir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".DS_Store"), []byte("churn"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := v.Recheck(); err != nil {
		t.Fatalf("Finder file changed listing: %v", err)
	}
}

func TestEditorChurnListingFingerprint(t *testing.T) {
	for _, name := range []string{"Source.cls.swp", "Source.cls~", ".#Source.cls"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			v := New()
			if _, err := v.ReadDir(dir); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, name), nil, 0600); err != nil {
				t.Fatal(err)
			}
			if err := v.Recheck(); err != nil {
				t.Fatalf("editor churn changed listing: %v", err)
			}
		})
	}
}

func TestEditorLikeDeployableInputsStillInvalidate(t *testing.T) {
	for _, tc := range []struct{ subdir, name string }{
		{"staticresources/Bundle", "asset.swp"},
		{"staticresources/Bundle", ".DS_Store"},
		{"lwc/widget", "asset.swp"},
		{"aura/widget", "asset~"},
		{"staticresources/Bundle", "asset~"},
		{"data", ".#records.json"},
		{"objects/Widget__c/sharingReasons", ".#Second__c.sharingReason-meta.xml"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), tc.subdir)
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			v := New()
			if _, err := v.ReadDir(dir); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, tc.name), nil, 0600); err != nil {
				t.Fatal(err)
			}
			if err := v.Recheck(); err == nil {
				t.Fatal("deployable input did not invalidate listing")
			}
		})
	}
}

func TestBundleChurnNamesRemainInputs(t *testing.T) {
	for _, kind := range []string{"aiAgentDefinitionVersions", "aiAuthoringBundles", "aura", "digitalExperiences", "documents", "experiences", "experiencePropertyTypeBundles", "genAiFunctions", "genAiPlannerBundles", "integArtifactDefs", "lwc", "staticresources", "waveTemplates", "uiBundles", "appTemplates", "lightningTypes", "contentTypes", "uiWidgets"} {
		t.Run(kind, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), kind, "Sample")
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			v := New()
			if _, err := v.ReadDir(dir); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "asset.swp"), nil, 0600); err != nil {
				t.Fatal(err)
			}
			if err := v.Recheck(); err == nil {
				t.Fatal("bundle content did not invalidate listing")
			}
		})
	}
}

func TestIgnoredChurnRemainsVisibleToDirectoryConsumers(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{".DS_Store", ".#Source.cls", "Source.cls.swp", "Source.cls~", ".real.xml"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := New().ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("listing length: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Name() != want[i].Name() || got[i].Type() != want[i].Type() {
			t.Fatalf("listing entry %d: got %v, want %v", i, got[i], want[i])
		}
	}
}
