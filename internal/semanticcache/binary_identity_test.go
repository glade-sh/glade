package semanticcache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestIdentityForBuildIncludesExecutableDigest(t *testing.T) {
	path, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		t.Fatal(err)
	}
	want := hex.EncodeToString(hash.Sum(nil))
	root := t.TempDir()
	source := filepath.Join(root, "BinaryIdentity.cls")
	if err := os.WriteFile(source, []byte(`public class BinaryIdentity {}`), 0o600); err != nil {
		t.Fatal(err)
	}
	index, artifacts := typesys.BuildWithArtifacts(project.Project{Root: root, ApexFiles: []string{source}}, schema.Schema{})
	var previous Identity
	for attempt := range 2 {
		identity, err := IdentityForBuild(index, &artifacts, sema.AnalyzeOptions{})
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(identity)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]string
		if err := json.Unmarshal(data, &fields); err != nil {
			t.Fatal(err)
		}
		if got := fields["buildFingerprint"]; got != want {
			t.Fatalf("buildFingerprint = %q, want running executable digest %q", got, want)
		}
		if attempt > 0 && identity != previous {
			t.Fatal("unchanged executable and project changed identity")
		}
		previous = identity
	}
}

func TestSemanticCacheBinaryIdentityRoutes(t *testing.T) {
	for _, route := range []string{"disk", "manager_memory", "manager_disk"} {
		t.Run(route, func(t *testing.T) {
			root := t.TempDir()
			const path = ".glade/semantic/result.json"
			first := identityWithBuildFingerprint(t, testIdentity(), strings.Repeat("1", 64))
			second := identityWithBuildFingerprint(t, first, strings.Repeat("2", 64))
			if route == "disk" {
				if err := Store(root, path, first, sema.Result{}); err != nil {
					t.Fatal(err)
				}
				if _, err := Load(root, path, first); err != nil {
					t.Fatalf("same binary missed: %v", err)
				}
				_, err := Load(root, path, second)
				var miss *MissError
				if !errors.As(err, &miss) || miss.Reason != MissIdentityMismatch {
					t.Fatalf("different binary Load = %v, want identity mismatch", err)
				}
				return
			}
			manager := NewManager(Limits{})
			t.Cleanup(manager.Wait)
			builds := 0
			compute := func() (sema.Result, error) {
				builds++
				return sema.Result{}, nil
			}
			request := Request{Identity: first, ProjectRoot: root, RelativePath: path}
			if _, _, err := manager.GetOrCompute(context.Background(), request, compute); err != nil {
				t.Fatal(err)
			}
			if _, access, err := manager.GetOrCompute(context.Background(), request, compute); err != nil || access.Source != SourceMemory {
				t.Fatalf("same binary access = %#v, %v", access, err)
			}
			if route == "manager_disk" {
				manager.Reset()
				if _, access, err := manager.GetOrCompute(context.Background(), request, compute); err != nil || access.Source != SourceDisk {
					t.Fatalf("same binary disk access = %#v, %v", access, err)
				}
				manager.Reset()
			}
			request.Identity = second
			_, access, err := manager.GetOrCompute(context.Background(), request, compute)
			if err != nil || access.Source != SourceBuild || builds != 2 {
				t.Fatalf("different binary access = %#v, builds = %d, err = %v; want rebuild", access, builds, err)
			}
		})
	}
}

func TestSemanticCacheRejectsMissingBinaryIdentityRoutes(t *testing.T) {
	identity := identityWithBuildFingerprint(t, testIdentity(), "")
	root := t.TempDir()
	const path = ".glade/semantic/result.json"
	t.Run("store", func(t *testing.T) {
		if err := Store(root, path, identity, sema.Result{}); err == nil {
			t.Fatal("Store accepted missing binary identity")
		}
	})
	t.Run("load", func(t *testing.T) {
		if err := Store(root, path, identityWithBuildFingerprint(t, identity, strings.Repeat("1", 64)), sema.Result{}); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(root, path, identity); !errors.Is(err, ErrMiss) {
			t.Fatalf("Load with missing binary identity = %v, want miss", err)
		}
	})
	t.Run("manager", func(t *testing.T) {
		manager := NewManager(Limits{})
		t.Cleanup(manager.Wait)
		_, _, err := manager.GetOrCompute(context.Background(), Request{Identity: identity}, func() (sema.Result, error) {
			return sema.Result{}, nil
		})
		if err == nil {
			t.Fatal("Manager accepted missing binary identity")
		}
	})
}

func identityWithBuildFingerprint(t *testing.T, identity Identity, digest string) Identity {
	t.Helper()
	data, err := json.Marshal(identity)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]string
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	fields["buildFingerprint"] = digest
	data, err = json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &identity); err != nil {
		t.Fatal(err)
	}
	return identity
}
