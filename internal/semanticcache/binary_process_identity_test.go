package semanticcache

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestSemanticCacheExecutableProcessRoutes(t *testing.T) {
	if os.Getenv("GLADE_TEST_BINARY_IDENTITY_CHILD") == "1" {
		fmt.Fprintln(os.Stdout, "ready")
		if _, err := bufio.NewReader(os.Stdin).ReadString('\n'); err != nil {
			t.Fatal(err)
		}
		root := t.TempDir()
		path := filepath.Join(root, "BinaryProcess.cls")
		if err := os.WriteFile(path, []byte(`public class BinaryProcess {}`), 0o600); err != nil {
			t.Fatal(err)
		}
		index, artifacts := typesys.BuildWithArtifacts(project.Project{Root: root, ApexFiles: []string{path}}, schema.Schema{})
		identity, err := IdentityForBuild(index, &artifacts, sema.AnalyzeOptions{})
		if err != nil {
			t.Fatal(err)
		}
		again, err := IdentityForBuild(index, &artifacts, sema.AnalyzeOptions{})
		if err != nil || identity != again {
			t.Fatalf("same process identity changed: %v", err)
		}
		if err := json.NewEncoder(os.Stdout).Encode(identity); err != nil {
			t.Fatal(err)
		}
		return
	}
	path, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	want := hex.EncodeToString(digest[:])
	for _, route := range []string{"replace_before_first_identity", "unlink_before_first_identity", "unreadable_at_startup"} {
		t.Run(route, func(t *testing.T) {
			var previous string
			attempts := 1
			if route == "unreadable_at_startup" {
				attempts = 2
			}
			for range attempts {
				childPath := filepath.Join(t.TempDir(), "identity-test")
				if err := os.WriteFile(childPath, data, 0o700); err != nil {
					t.Fatal(err)
				}
				if route == "unreadable_at_startup" {
					if err := os.Chmod(childPath, 0o111); err != nil {
						t.Fatal(err)
					}
					if file, err := os.Open(childPath); err == nil {
						file.Close()
						t.Skip("host permits reads of execute-only files")
					}
				}
				cmd := exec.Command(childPath, "-test.run=^TestSemanticCacheExecutableProcessRoutes$")
				cmd.Env = append(os.Environ(), "GLADE_TEST_BINARY_IDENTITY_CHILD=1")
				var stderr bytes.Buffer
				cmd.Stderr = &stderr
				stdout, err := cmd.StdoutPipe()
				if err != nil {
					t.Fatal(err)
				}
				stdin, err := cmd.StdinPipe()
				if err != nil {
					t.Fatal(err)
				}
				if err := cmd.Start(); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					_ = cmd.Process.Kill()
					_ = cmd.Wait()
				})
				reader := bufio.NewReader(stdout)
				if ready, err := reader.ReadString('\n'); err != nil || ready != "ready\n" {
					t.Fatalf("child startup = %q, %v; stderr=%s", ready, err, &stderr)
				}
				switch route {
				case "replace_before_first_identity":
					replacement := childPath + ".new"
					if err := os.WriteFile(replacement, []byte("replacement executable"), 0o700); err != nil {
						t.Fatal(err)
					}
					if err := os.Rename(replacement, childPath); err != nil {
						t.Fatal(err)
					}
				case "unlink_before_first_identity":
					if err := os.Remove(childPath); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := io.WriteString(stdin, "continue\n"); err != nil {
					t.Fatal(err)
				}
				stdin.Close()
				output, err := io.ReadAll(reader)
				if err != nil {
					t.Fatal(err)
				}
				if err := cmd.Wait(); err != nil {
					t.Fatalf("child failed: %v; stdout=%s stderr=%s", err, output, &stderr)
				}
				var fields map[string]string
				if err := json.NewDecoder(bytes.NewReader(output)).Decode(&fields); err != nil {
					t.Fatalf("child identity: %v; stdout=%s", err, output)
				}
				got := fields["buildFingerprint"]
				if route == "unreadable_at_startup" {
					if !strings.HasPrefix(got, "process:") || got == previous {
						t.Fatalf("unreadable binary fingerprint = %q, previous = %q; want distinct process identity", got, previous)
					}
				} else if got != want {
					t.Fatalf("running binary fingerprint = %q, want original executable digest %q", got, want)
				}
				previous = got
			}
		})
	}
}
