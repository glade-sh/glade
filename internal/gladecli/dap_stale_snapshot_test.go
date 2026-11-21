package gladecli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/startupcache"
)

func TestDAPStaleSnapshotDoesNotReadPayload(t *testing.T) {
	root := t.TempDir()
	writeTestProject(t, root)
	if _, _, _, err := loadDAPStartupState(root); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "force-app", "main", "classes", "MathUtil.cls")
	info, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	changed := bytes.Replace(data, []byte("return a + b;"), []byte("return a - b;"), 1)
	if bytes.Equal(data, changed) {
		t.Fatal("source fixture did not contain the expected expression")
	}
	if err := os.WriteFile(source, changed, info.Mode()); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(source, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(source)
	if err != nil || after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
		t.Fatalf("fixture did not preserve size and mtime: %v", err)
	}
	payload := filepath.Join(root, ".glade", "dap", "startup.json")
	if err := os.Remove(payload); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(payload, 0700); err != nil {
		t.Fatal(err)
	}
	_, runtime, _, index, err := loadDAPStartupStateWithIndex(context.Background(), root)
	if err != nil || index == nil || len(runtime.Methods) == 0 {
		t.Fatalf("stale snapshot should rebuild without reading payload: index=%v methods=%d err=%v", index != nil, len(runtime.Methods), err)
	}
}

func dapStaleFixture(t testing.TB) (string, []byte) {
	t.Helper()
	root := makeDAPBenchmarkProject(t, dapBenchmarkSize{"stale", 3, 0, 0})
	if _, _, _, _, err := loadDAPStartupStateWithIndex(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".glade", "dap", "startup.json")
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Grow the serialized org with copies of an actual object definition, so
	// the stale payload exercises hydration as well as reading 51 MB.
	var entry, org map[string]json.RawMessage
	if err := json.Unmarshal(payload, &entry); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(entry["org"], &org); err != nil {
		t.Fatal(err)
	}
	var objects map[string]json.RawMessage
	if err := json.Unmarshal(org["objects"], &objects); err != nil {
		t.Fatal(err)
	}
	var largest json.RawMessage
	for _, object := range objects {
		if len(object) > len(largest) || (len(object) == len(largest) && bytes.Compare(object, largest) < 0) {
			largest = object
		}
	}
	if len(largest) == 0 {
		t.Fatal("no object in startup snapshot")
	}
	var compactObject bytes.Buffer
	if err := json.Compact(&compactObject, largest); err != nil {
		t.Fatal(err)
	}
	largest = compactObject.Bytes()
	compactEntry, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	copies := (51342053-len(compactEntry))/(len(largest)+25) + 1
	for i := 0; i < copies; i++ {
		objects[fmt.Sprintf("BenchmarkCopy%06d", i)] = largest
	}
	org["objects"], err = json.Marshal(objects)
	if err != nil {
		t.Fatal(err)
	}
	entry["org"], err = json.Marshal(org)
	if err != nil {
		t.Fatal(err)
	}
	payload, err = json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	if len(payload) < 51342053 {
		t.Fatalf("payload too small: %d", len(payload))
	}
	source := filepath.Join(root, "src", "classes", "BenchClass0000.cls")
	f, err := os.OpenFile(source, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("\n// edit\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return root, payload
}

func TestDAPStaleSnapshotOutcome(t *testing.T) {
	root, payload := dapStaleFixture(t)
	fmt.Printf("stale-payload-bytes %d\n", len(payload))
	if err := os.WriteFile(filepath.Join(root, ".glade", "dap", "startup.json"), payload, 0600); err != nil {
		t.Fatal(err)
	}
	org, runtime, api, index, err := loadDAPStartupStateWithIndex(context.Background(), root)
	if err != nil || index == nil {
		t.Fatalf("index=%v err=%v", index != nil, err)
	}
	body, err := json.Marshal(struct {
		Org     any
		Runtime any
		API     string
	}{org, compiledRuntimeToCache(runtime), api})
	if err != nil {
		t.Fatal(err)
	}
	body = bytes.ReplaceAll(body, []byte(root), []byte("<ROOT>"))
	// Captured from origin/main using this fixture and normalized project root.
	const baseline = "77099426e01feb2e5bd5b2c6ba0e1629efc4565ad1e5dfe2d21ce3c8121815c0"
	if got := fmt.Sprintf("%x", sha256.Sum256(body)); got != baseline {
		t.Fatalf("stale outcome SHA-256 = %s, origin/main = %s", got, baseline)
	}
}

func BenchmarkDAPStaleSnapshot(b *testing.B) {
	root, payload := dapStaleFixture(b)
	path := filepath.Join(root, ".glade", "dap", "startup.json")
	manifestPath := filepath.Join(root, ".glade", "dap", "startup-manifest.json")
	manifest, _ := os.ReadFile(manifestPath)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		if err := os.WriteFile(path, payload, 0600); err != nil {
			b.Fatal(err)
		}
		if manifest != nil {
			if err := os.WriteFile(manifestPath, manifest, 0600); err != nil {
				b.Fatal(err)
			}
		}
		b.StartTimer()
		_, _, _, index, err := loadDAPStartupStateWithIndex(context.Background(), root)
		if err != nil || index == nil {
			b.Fatalf("index=%v err=%v", index != nil, err)
		}
	}
}

func TestDAPManifestRecoveryAndPayloadBinding(t *testing.T) {
	for _, scenario := range []string{"missing", "corrupt", "legacy", "mismatch", "corrupt-payload", "clear"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			writeTestProject(t, root)
			originalOrg, original, api, _, err := loadDAPStartupStateWithIndex(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			manifest := filepath.Join(root, ".glade", "dap", "startup-manifest.json")
			payload := filepath.Join(root, ".glade", "dap", "startup.json")
			switch scenario {
			case "clear":
				if err := startupcache.Clear(root, startupcache.SubdirDAP); err != nil {
					t.Fatal(err)
				}
				if _, err := os.Stat(manifest); !os.IsNotExist(err) {
					t.Fatalf("manifest remains after clear: %v", err)
				}
			case "missing":
				if err := os.Remove(manifest); err != nil {
					t.Fatal(err)
				}
			case "corrupt":
				if err := os.WriteFile(manifest, []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
			case "legacy":
				if err := os.WriteFile(manifest, []byte(`{"manifest":{}}`), 0600); err != nil {
					t.Fatal(err)
				}
			case "mismatch":
				data, err := os.ReadFile(payload)
				if err != nil {
					t.Fatal(err)
				}
				var entry map[string]any
				if err := json.Unmarshal(data, &entry); err != nil {
					t.Fatal(err)
				}
				entry["runtime"] = map[string]any{}
				data, err = json.Marshal(entry)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(payload, data, 0600); err != nil {
					t.Fatal(err)
				}
			case "corrupt-payload":
				if err := os.WriteFile(payload, []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			rebuiltOrg, rebuilt, gotAPI, index, err := loadDAPStartupStateWithIndex(context.Background(), root)
			if err != nil || index == nil || gotAPI != api || len(rebuilt.Methods) != len(original.Methods) {
				t.Fatalf("rebuild: index=%v methods=%d api=%s err=%v", index != nil, len(rebuilt.Methods), gotAPI, err)
			}
			warmOrg, warm, warmAPI, index, err := loadDAPStartupStateWithIndex(context.Background(), root)
			if err != nil || index != nil || warmAPI != api || len(warm.Methods) != len(original.Methods) {
				t.Fatalf("warm: index=%v methods=%d api=%s err=%v", index != nil, len(warm.Methods), warmAPI, err)
			}
			snapshot := func(org any, runtime any) []byte {
				data, err := json.Marshal([]any{org, runtime})
				if err != nil {
					t.Fatal(err)
				}
				return data
			}
			want := snapshot(originalOrg, compiledRuntimeToCache(original))
			if !bytes.Equal(want, snapshot(rebuiltOrg, compiledRuntimeToCache(rebuilt))) || !bytes.Equal(want, snapshot(warmOrg, compiledRuntimeToCache(warm))) {
				t.Fatal("recovered or warm startup differs from original")
			}
		})
	}
}

func TestDAPManifestRejectsForeignRootsBeforePayload(t *testing.T) {
	for _, field := range []string{"entry", "manifest"} {
		t.Run(field, func(t *testing.T) {
			root := t.TempDir()
			writeTestProject(t, root)
			if _, _, _, err := loadDAPStartupState(root); err != nil {
				t.Fatal(err)
			}
			sidecar := filepath.Join(root, ".glade", "dap", "startup-manifest.json")
			data, err := os.ReadFile(sidecar)
			if err != nil {
				t.Fatal(err)
			}
			var header map[string]any
			if err := json.Unmarshal(data, &header); err != nil {
				t.Fatal(err)
			}
			if field == "entry" {
				header["projectRoot"] = filepath.Join(root, "other")
			} else {
				header["manifest"].(map[string]any)["projectRoot"] = filepath.Join(root, "other")
			}
			data, err = json.Marshal(header)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(sidecar, data, 0600); err != nil {
				t.Fatal(err)
			}
			payload := filepath.Join(root, ".glade", "dap", "startup.json")
			if err := os.Remove(payload); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(payload, 0700); err != nil {
				t.Fatal(err)
			}
			_, _, _, index, err := loadDAPStartupStateWithIndex(context.Background(), root)
			if err != nil || index == nil {
				t.Fatalf("foreign root should rebuild without payload: index=%v err=%v", index != nil, err)
			}
		})
	}
}

func TestDAPWarmSnapshotOutcome(t *testing.T) {
	root := makeDAPBenchmarkProject(t, dapBenchmarkSize{"warm", 3, 0, 0})
	if _, _, _, _, err := loadDAPStartupStateWithIndex(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	org, runtime, api, index, err := loadDAPStartupStateWithIndex(context.Background(), root)
	if err != nil || index != nil {
		t.Fatalf("index=%v err=%v", index != nil, err)
	}
	body, err := json.Marshal(struct {
		Org     any
		Runtime any
		API     string
	}{org, compiledRuntimeToCache(runtime), api})
	if err != nil {
		t.Fatal(err)
	}
	body = bytes.ReplaceAll(body, []byte(root), []byte("<ROOT>"))
	// Captured from origin/main using this fixture and normalized project root.
	const baseline = "77099426e01feb2e5bd5b2c6ba0e1629efc4565ad1e5dfe2d21ce3c8121815c0"
	if got := fmt.Sprintf("%x", sha256.Sum256(body)); got != baseline {
		t.Fatalf("warm outcome SHA-256 = %s, origin/main = %s", got, baseline)
	}
}
