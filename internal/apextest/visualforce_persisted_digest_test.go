package apextest

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/inputview"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestPersistedVisualforceDigestInvalidation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "asset.js")
	if err := os.WriteFile(path, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	savedNow, savedHook := fileDigestNow, fileDigestReadHook
	t.Cleanup(func() { fileDigestNow, fileDigestReadHook = savedNow, savedHook; resetFileDigestTrust() })
	fileDigestNow = func() time.Time { return time.Now().Add(time.Minute) }
	reads := 0
	fileDigestReadHook = func(string) { reads++ }
	resetFileDigestTrust()
	first, err := currentFileDigest(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := saveVisualforceDigestCache(root, []string{path}); err != nil {
		t.Fatal(err)
	}
	resetFileDigestTrust()
	if err := loadVisualforceDigestCache(root, []string{path}); err != nil {
		t.Fatal(err)
	}
	got, err := currentFileDigest(path)
	if err != nil || got != first || reads != 1 {
		t.Fatalf("stable cached digest: reads=%d digest=%x err=%v", reads, got, err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("other"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	resetFileDigestTrust()
	if err := loadVisualforceDigestCache(root, []string{path}); err != nil {
		t.Fatal(err)
	}
	got, err = currentFileDigest(path)
	want := sha256.Sum256([]byte("other"))
	if err != nil || got != want || reads != 2 {
		t.Fatalf("rewritten file: reads=%d digest=%x err=%v", reads, got, err)
	}
}

func TestPersistedVisualforceDigestRejectsRecentAndCorruptData(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "asset.js")
	if err := os.WriteFile(path, []byte("fresh"), 0600); err != nil {
		t.Fatal(err)
	}
	savedNow, savedHook := fileDigestNow, fileDigestReadHook
	t.Cleanup(func() { fileDigestNow, fileDigestReadHook = savedNow, savedHook; resetFileDigestTrust() })
	fileDigestNow = time.Now
	reads := 0
	fileDigestReadHook = func(string) { reads++ }
	resetFileDigestTrust()
	if _, err := currentFileDigest(path); err != nil {
		t.Fatal(err)
	}
	if err := saveVisualforceDigestCache(root, []string{path}); err != nil {
		t.Fatal(err)
	}
	resetFileDigestTrust()
	if err := loadVisualforceDigestCache(root, []string{path}); err != nil {
		t.Fatal(err)
	}
	if _, err := currentFileDigest(path); err != nil {
		t.Fatal(err)
	}
	if reads != 2 {
		t.Fatalf("recent file was reused: reads=%d", reads)
	}

	fileDigestNow = func() time.Time { return time.Now().Add(time.Minute) }
	resetFileDigestTrust()
	if _, err := currentFileDigest(path); err != nil {
		t.Fatal(err)
	}
	if err := saveVisualforceDigestCache(root, []string{path}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(visualforceDigestCachePath(root), []byte("{invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	resetFileDigestTrust()
	_ = loadVisualforceDigestCache(root, []string{path})
	if _, err := currentFileDigest(path); err != nil {
		t.Fatal(err)
	}
	if reads != 4 {
		t.Fatalf("corrupt cache was reused: reads=%d", reads)
	}
}

func TestVisualforcePersistentDigestScope(t *testing.T) {
	root := t.TempDir()
	page, resource := filepath.Join(root, "Home.page"), filepath.Join(root, "asset.js")
	writeFile(t, page, "<apex:page/>")
	writeFile(t, resource, "asset")
	restore := EnableDiskCacheForTesting()
	defer restore()
	oldNow, oldHook := fileDigestNow, fileDigestReadHook
	fileDigestNow = func() time.Time { return time.Now().Add(time.Minute) }
	reads := 0
	fileDigestReadHook = func(path string) {
		if path == resource {
			reads++
		}
	}
	t.Cleanup(func() { fileDigestNow, fileDigestReadHook = oldNow, oldHook; resetFileDigestTrust() })
	capture := func(p project.Project) typesys.ApexMetadataInput {
		generation := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
		if err := captureVisualforceRuntimeInputs(p, &generation); err != nil {
			t.Fatal(err)
		}
		return generation.inputs[resource]
	}
	p := project.Project{Root: root, VisualforcePageFiles: []string{page}, StaticResourceFiles: []string{resource}}
	resetFileDigestTrust()
	first := capture(p)
	resetFileDigestTrust()
	if second := capture(p); first != second || reads != 1 {
		t.Fatalf("cross-process reuse: reads=%d first=%v second=%v", reads, first, second)
	}

	for _, withView := range []bool{false, true} {
		other := t.TempDir()
		page, resource := filepath.Join(other, "Home.page"), filepath.Join(other, "asset.js")
		writeFile(t, page, "<apex:page/>")
		writeFile(t, resource, "asset")
		p := project.Project{Root: other, StaticResourceFiles: []string{resource}}
		if withView {
			p.VisualforcePageFiles = []string{page}
			p = p.WithInputs(inputview.New())
		}
		resetFileDigestTrust()
		capture(p)
		if _, err := os.Stat(visualforceDigestCachePath(other)); !os.IsNotExist(err) {
			t.Fatalf("withView=%t: unexpected cache file: %v", withView, err)
		}
	}
}

func TestPersistedVisualforceDigestTracksIdentityChanges(t *testing.T) {
	for _, mutation := range []string{"replacement", "deletion", "symlink"} {
		t.Run(mutation, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "asset.js")
			if err := os.WriteFile(path, []byte("first"), 0600); err != nil {
				t.Fatal(err)
			}
			oldNow := fileDigestNow
			fileDigestNow = func() time.Time { return time.Now().Add(time.Minute) }
			t.Cleanup(func() { fileDigestNow = oldNow; resetFileDigestTrust() })
			resetFileDigestTrust()
			if _, err := currentFileDigest(path); err != nil {
				t.Fatal(err)
			}
			if err := saveVisualforceDigestCache(root, []string{path}); err != nil {
				t.Fatal(err)
			}
			switch mutation {
			case "replacement":
				replacement := filepath.Join(root, "replacement")
				if err := os.WriteFile(replacement, []byte("other"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(replacement, path); err != nil {
					t.Fatal(err)
				}
			case "deletion":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				target := filepath.Join(root, "target")
				if err := os.WriteFile(target, []byte("other"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			}
			resetFileDigestTrust()
			if err := loadVisualforceDigestCache(root, []string{path}); err != nil {
				t.Fatal(err)
			}
			got, err := currentFileDigest(path)
			if mutation == "deletion" {
				if !os.IsNotExist(err) {
					t.Fatalf("deleted input: %v", err)
				}
				return
			}
			want := sha256.Sum256([]byte("other"))
			if err != nil || got != want {
				t.Fatalf("changed identity: digest=%x err=%v", got, err)
			}
		})
	}
}

func TestPersistedVisualforceDigestConcurrentAccess(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "asset.js")
	if err := os.WriteFile(path, []byte("stable"), 0600); err != nil {
		t.Fatal(err)
	}
	oldNow := fileDigestNow
	fileDigestNow = func() time.Time { return time.Now().Add(time.Minute) }
	t.Cleanup(func() { fileDigestNow = oldNow; resetFileDigestTrust() })
	resetFileDigestTrust()
	if _, err := currentFileDigest(path); err != nil {
		t.Fatal(err)
	}
	if err := saveVisualforceDigestCache(root, []string{path}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := saveVisualforceDigestCache(root, []string{path}); err != nil {
				errs <- err
				return
			}
			if err := loadVisualforceDigestCache(root, []string{path}); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	got, err := currentFileDigest(path)
	want := sha256.Sum256([]byte("stable"))
	if err != nil || got != want {
		t.Fatalf("digest=%x err=%v", got, err)
	}
}

func TestPersistedVisualforceDigestRejectsUnsafeDirectory(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "asset.js")
	if err := os.WriteFile(path, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0777); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0700) })
	if err := saveVisualforceDigestCache(root, []string{path}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(visualforceDigestCachePath(root)); !os.IsNotExist(err) {
		t.Fatalf("wrote digest cache under an unsafe root: %v", err)
	}
}

func TestPersistedVisualforceDigestDoesNotReadUnsafeRoot(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "asset.js")
	if err := os.WriteFile(path, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	oldNow, oldHook := fileDigestNow, fileDigestReadHook
	fileDigestNow = func() time.Time { return time.Now().Add(time.Minute) }
	reads := 0
	fileDigestReadHook = func(string) { reads++ }
	t.Cleanup(func() {
		fileDigestNow, fileDigestReadHook = oldNow, oldHook
		resetFileDigestTrust()
		_ = os.Chmod(root, 0700)
	})
	resetFileDigestTrust()
	if _, err := currentFileDigest(path); err != nil {
		t.Fatal(err)
	}
	if err := saveVisualforceDigestCache(root, []string{path}); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0777); err != nil {
		t.Fatal(err)
	}
	resetFileDigestTrust()
	if err := loadVisualforceDigestCache(root, []string{path}); err != nil {
		t.Fatal(err)
	}
	if _, err := currentFileDigest(path); err != nil {
		t.Fatal(err)
	}
	if reads != 2 {
		t.Fatalf("read cached digest under unsafe root: reads=%d", reads)
	}
}

func TestPersistedVisualforceDigestCoversEveryInputGroup(t *testing.T) {
	cases := []struct {
		name string
		set  func(*project.Project, string)
	}{
		{"page", func(p *project.Project, f string) { p.VisualforcePageFiles = append(p.VisualforcePageFiles, f) }},
		{"component", func(p *project.Project, f string) { p.VisualforceComponentFiles = []string{f} }},
		{"email", func(p *project.Project, f string) { p.EmailTemplateFiles = []string{f} }},
		{"flow", func(p *project.Project, f string) { p.FlowFiles = []string{f} }},
		{"labels", func(p *project.Project, f string) { p.LabelFiles = []string{f} }},
		{"translations", func(p *project.Project, f string) { p.TranslationFiles = []string{f} }},
		{"static resource", func(p *project.Project, f string) { p.StaticResourceFiles = []string{f} }},
		{"static resource metadata", func(p *project.Project, f string) { p.StaticResourceMetas = []string{f} }},
		{"message channel", func(p *project.Project, f string) { p.MessageChannelFiles = []string{f} }},
		{"tab", func(p *project.Project, f string) { p.TabFiles = []string{f} }},
		{"profile", func(p *project.Project, f string) { p.ProfileFiles = []string{f} }},
		{"permission set", func(p *project.Project, f string) { p.PermissionSetFiles = []string{f} }},
		{"permission set group", func(p *project.Project, f string) { p.PermissionSetGroupFiles = []string{f} }},
	}
	restore := EnableDiskCacheForTesting()
	defer restore()
	oldNow, oldHook := fileDigestNow, fileDigestReadHook
	fileDigestNow = func() time.Time { return time.Now().Add(time.Minute) }
	t.Cleanup(func() { fileDigestNow, fileDigestReadHook = oldNow, oldHook; resetFileDigestTrust() })
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			page, input := filepath.Join(root, "Home.page"), filepath.Join(root, "input")
			writeFile(t, page, "<apex:page/>")
			writeFile(t, input, "before")
			p := project.Project{Root: root, VisualforcePageFiles: []string{page}}
			tc.set(&p, input)
			reads := 0
			fileDigestReadHook = func(path string) {
				if path == input {
					reads++
				}
			}
			capture := func() typesys.ApexMetadataInput {
				generation := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
				if err := captureVisualforceRuntimeInputs(p, &generation); err != nil {
					t.Fatal(err)
				}
				return generation.inputs[input]
			}
			resetFileDigestTrust()
			before := capture()
			resetFileDigestTrust()
			if got := capture(); got != before || reads != 1 {
				t.Fatalf("stable cache: reads=%d", reads)
			}
			replacement := filepath.Join(root, "replacement")
			writeFile(t, replacement, "after!")
			if err := os.Rename(replacement, input); err != nil {
				t.Fatal(err)
			}
			resetFileDigestTrust()
			if got := capture(); got == before || reads != 2 {
				t.Fatalf("changed input: reads=%d", reads)
			}
		})
	}
}

func TestPersistedVisualforceDigestRejectsOversizedCache(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "asset.js")
	if err := os.WriteFile(path, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := saveVisualforceDigestCache(root, []string{path}); err != nil {
		t.Fatal(err)
	}
	cache := visualforceDigestCachePath(root)
	if err := os.Truncate(cache, visualforceDigestCacheLimit+1); err != nil {
		t.Fatal(err)
	}
	if err := loadVisualforceDigestCache(root, []string{path}); err == nil {
		t.Fatal("oversized digest cache was accepted")
	}
}

func TestPersistedVisualforceDigestRejectsChecksumMismatch(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "asset.js")
	if err := os.WriteFile(path, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	oldNow := fileDigestNow
	fileDigestNow = func() time.Time { return time.Now().Add(time.Minute) }
	t.Cleanup(func() { fileDigestNow = oldNow; resetFileDigestTrust() })
	resetFileDigestTrust()
	if _, err := currentFileDigest(path); err != nil {
		t.Fatal(err)
	}
	if err := saveVisualforceDigestCache(root, []string{path}); err != nil {
		t.Fatal(err)
	}
	cache := visualforceDigestCachePath(root)
	data, err := os.ReadFile(cache)
	if err != nil {
		t.Fatal(err)
	}
	var envelope visualforceDigestEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatal(err)
	}
	envelope.Payload = bytes.Replace(envelope.Payload, []byte("asset.js"), []byte("other.js"), 1)
	data, err = json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cache, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := loadVisualforceDigestCache(root, []string{path}); err == nil {
		t.Fatal("modified cache was accepted")
	}
}
