package frontendcache

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestSnapshotReadPinsPrivateComponents(t *testing.T) {
	for _, component := range []string{".glade", "semantic", name} {
		t.Run(component, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, ".glade", "semantic", name)
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			content := []byte("owned snapshot bytes")
			if err := os.WriteFile(path, content, 0600); err != nil {
				t.Fatal(err)
			}
			if got, err := read(root); err != nil || !bytes.Equal(got, content) {
				t.Fatalf("regular snapshot: %q, %v", got, err)
			}
			link := path
			if component == ".glade" {
				link = filepath.Join(root, ".glade")
			} else if component == "semantic" {
				link = filepath.Dir(path)
			}
			if err := os.Rename(link, link+"-target"); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(link+"-target", link); err != nil {
				t.Fatal(err)
			}
			if _, err := read(root); err == nil {
				t.Fatal("accepted a symlink in the private cache path")
			}
		})
	}
}

func TestSnapshotReadRejectsNonregularAndOversizedFiles(t *testing.T) {
	for _, mode := range []string{"directory", "oversized"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, ".glade", "semantic", name)
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if mode == "directory" {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			} else {
				file, err := os.Create(path)
				if err != nil {
					t.Fatal(err)
				}
				err = file.Truncate(maxSnapshotBytes + 1)
				file.Close()
				if err != nil {
					t.Fatal(err)
				}
			}
			if _, err := read(root); err == nil {
				t.Fatal("accepted an invalid snapshot file")
			}
		})
	}
}

func TestValidatedHitDoesNotConstructFrontend(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Owned.cls")
	if err := os.WriteFile(path, []byte("public class Owned {}"), 0600); err != nil {
		t.Fatal(err)
	}
	p := project.Project{Root: root, ApexFiles: []string{path}, SourceAPIVersion: "67.0"}
	builds := 0
	builder := func() (typesys.Index, typesys.BuildArtifacts) {
		builds++
		return typesys.BuildWithArtifacts(p, schema.Schema{})
	}
	first := LoadOrBuild(p, schema.Schema{}, builder)
	second := LoadOrBuild(p, schema.Schema{}, builder)
	if first.Hit || !second.Hit || builds != 1 {
		t.Fatalf("expected one build and one parser-free hit: first=%v second=%v builds=%d", first.Hit, second.Hit, builds)
	}
	if err := typesys.ValidateBuildGeneration(second.Index, &second.Artifacts); err != nil {
		t.Fatal(err)
	}
	second.Index.Types[0].Name = "Mutated"
	third := LoadOrBuild(p, schema.Schema{}, builder)
	if !third.Hit || third.Index.Types[0].Name != "Owned" || builds != 1 {
		t.Fatal("restored frontend escaped its ownership boundary")
	}
}

func TestConcurrentCacheInvalidation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Owned.cls")
	if err := os.WriteFile(path, []byte("public class Owned {}"), 0600); err != nil {
		t.Fatal(err)
	}
	p := project.Project{Root: root, ApexFiles: []string{path}}
	errors := make(chan bool, 4)
	for worker := 0; worker < 4; worker++ {
		go func() {
			valid := true
			for i := 0; i < 5; i++ {
				result := LoadOrBuild(p, schema.Schema{}, func() (typesys.Index, typesys.BuildArtifacts) { return typesys.BuildWithArtifacts(p, schema.Schema{}) })
				valid = valid && len(result.Index.Types) == 1 && result.Index.Types[0].Name == "Owned"
				Invalidate()
			}
			errors <- valid
		}()
	}
	for i := 0; i < 4; i++ {
		if !<-errors {
			t.Fatal("concurrent invalidation changed the frontend")
		}
	}
}

func TestPublicationKeepsCapturedMembership(t *testing.T) {
	for _, change := range []string{"add", "remove", "rename"} {
		t.Run(change, func(t *testing.T) {
			root := t.TempDir()
			first := filepath.Join(root, "First.cls")
			other := filepath.Join(root, "Other.cls")
			if err := os.WriteFile(first, []byte("public class First {}"), 0600); err != nil {
				t.Fatal(err)
			}
			p := project.Project{Root: root, ApexFiles: []string{first}}
			LoadOrBuild(p, schema.Schema{}, func() (typesys.Index, typesys.BuildArtifacts) {
				index, artifacts := typesys.BuildWithArtifacts(p, schema.Schema{})
				switch change {
				case "add":
					if err := os.WriteFile(other, []byte("public class Other {}"), 0600); err != nil {
						t.Fatal(err)
					}
				case "remove":
					if err := os.Remove(first); err != nil {
						t.Fatal(err)
					}
				case "rename":
					if err := os.Rename(first, other); err != nil {
						t.Fatal(err)
					}
				}
				return index, artifacts
			})
			switch change {
			case "add":
				p.ApexFiles = append(p.ApexFiles, other)
			case "remove":
				p.ApexFiles = nil
			case "rename":
				p.ApexFiles = []string{other}
			}
			builds := 0
			result := LoadOrBuild(p, schema.Schema{}, func() (typesys.Index, typesys.BuildArtifacts) {
				builds++
				return typesys.BuildWithArtifacts(p, schema.Schema{})
			})
			if result.Hit || builds != 1 {
				t.Fatal("publication accepted membership absent from its captured generation")
			}
			expected, _ := typesys.BuildWithArtifacts(p, schema.Schema{})
			gotJSON, _ := json.Marshal(result.Index)
			wantJSON, _ := json.Marshal(expected)
			if string(gotJSON) != string(wantJSON) {
				t.Fatal("membership fallback changed the complete index")
			}
		})
	}
}

// One SHA-256 footer authenticates the envelope and the record stream. The
// record stream carries no second footer, and every corrupted region falls
// back to a complete build.
func TestSnapshotFileAuthenticatesEnvelopeAndRecordsOnce(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Owned.cls")
	if err := os.WriteFile(path, []byte("public class Owned { Integer value; }"), 0600); err != nil {
		t.Fatal(err)
	}
	p := project.Project{Root: root, ApexFiles: []string{path}, SourceAPIVersion: "67.0"}
	builds := 0
	builder := func() (typesys.Index, typesys.BuildArtifacts) {
		builds++
		return typesys.BuildWithArtifacts(p, schema.Schema{})
	}
	LoadOrBuild(p, schema.Schema{}, builder)
	file := filepath.Join(root, ".glade", "semantic", name)
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	content, footer := data[:len(data)-sha256.Size], data[len(data)-sha256.Size:]
	if digest := sha256.Sum256(content); !bytes.Equal(footer, digest[:]) {
		t.Fatal("snapshot file lacks its single authentication footer")
	}
	split := bytes.IndexByte(content, '\n')
	index, artifacts := typesys.BuildWithArtifacts(p, schema.Schema{})
	var records bytes.Buffer
	if err := typesys.WriteFrontendSnapshotRecords(&records, index, &artifacts); err != nil {
		t.Fatal(err)
	}
	if split < 0 || !bytes.Equal(content[split+1:], records.Bytes()) {
		t.Fatal("snapshot file carries a second record-stream footer")
	}
	for _, offset := range []int{split / 2, split + 1 + records.Len()/2, len(data) - 1} {
		broken := append([]byte(nil), data...)
		broken[offset] ^= 1
		if err := os.WriteFile(file, broken, 0600); err != nil {
			t.Fatal(err)
		}
		builds = 0
		result := LoadOrBuild(p, schema.Schema{}, builder)
		if result.Hit || result.Reused || builds != 1 || len(result.Index.Types) != 1 || result.Index.Types[0].Name != "Owned" {
			t.Fatalf("corruption at %d was not a complete fallback", offset)
		}
	}
}

// Publication encodes the builder's captured bytes. A later edit, even one
// made before publication, changes the current content identity and can never
// return the stale generation.
func TestPublicationAfterCapturedSourceEditNeverHitsStaleContent(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Owned.cls")
	if err := os.WriteFile(path, []byte("public class Owned { Integer left; }"), 0600); err != nil {
		t.Fatal(err)
	}
	p := project.Project{Root: root, ApexFiles: []string{path}, SourceAPIVersion: "67.0"}
	LoadOrBuild(p, schema.Schema{}, func() (typesys.Index, typesys.BuildArtifacts) {
		index, artifacts := typesys.BuildWithArtifacts(p, schema.Schema{})
		if err := os.WriteFile(path, []byte("public class Owned { Integer right; }"), 0600); err != nil {
			t.Fatal(err)
		}
		return index, artifacts
	})
	result := LoadOrBuild(p, schema.Schema{}, func() (typesys.Index, typesys.BuildArtifacts) {
		return typesys.BuildWithArtifacts(p, schema.Schema{})
	})
	if result.Hit {
		t.Fatal("hit a generation captured before the source edit")
	}
	if err := typesys.ValidateBuildGeneration(result.Index, &result.Artifacts); err != nil {
		t.Fatal(err)
	}
	expected, _ := typesys.BuildWithArtifacts(p, schema.Schema{})
	gotJSON, _ := json.Marshal(result.Index)
	wantJSON, _ := json.Marshal(expected)
	if string(gotJSON) != string(wantJSON) {
		t.Fatal("edited source did not produce the fresh index")
	}
}

func TestSidecarAPIVersionChangeInvalidatesSnapshot(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Owned.cls")
	meta := path + "-meta.xml"
	if err := os.WriteFile(path, []byte("public class Owned {}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(meta, []byte("<ApexClass><apiVersion>62.0</apiVersion></ApexClass>"), 0600); err != nil {
		t.Fatal(err)
	}
	p := project.Project{Root: root, ApexFiles: []string{path}, SourceAPIVersion: "67.0"}
	builds := 0
	builder := func() (typesys.Index, typesys.BuildArtifacts) {
		builds++
		return typesys.BuildWithArtifacts(p, schema.Schema{})
	}
	before, err := typesys.FrontendInputIdentity(p, schema.Schema{})
	if err != nil {
		t.Fatal(err)
	}
	first := LoadOrBuild(p, schema.Schema{}, builder)
	if warm := LoadOrBuild(p, schema.Schema{}, builder); first.Hit || !warm.Hit || builds != 1 {
		t.Fatalf("cache did not warm: first=%v warm=%v builds=%d", first.Hit, warm.Hit, builds)
	}
	if got := first.Index.Types[0].EffectiveAPIVersion; got != "62.0" {
		t.Fatalf("warm index API version = %q, want 62.0", got)
	}
	if err := os.WriteFile(meta, []byte("<ApexClass><apiVersion>66.0</apiVersion></ApexClass>"), 0600); err != nil {
		t.Fatal(err)
	}
	after, err := typesys.FrontendInputIdentity(p, schema.Schema{})
	if err != nil {
		t.Fatal(err)
	}
	if after == before {
		t.Fatal("sidecar-only edit left the frontend input identity unchanged")
	}
	second := LoadOrBuild(p, schema.Schema{}, builder)
	if second.Hit || builds != 2 {
		t.Fatalf("sidecar-only edit was served from the old snapshot: hit=%v builds=%d", second.Hit, builds)
	}
	if got := second.Index.Types[0].EffectiveAPIVersion; got != "66.0" {
		t.Fatalf("rebuilt index API version = %q, want 66.0", got)
	}
}
