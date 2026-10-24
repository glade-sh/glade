package scripts

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCISharedBinaryDigestHostPython(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"binary", []byte{0, 1, 127, 128, 255, '\n'}},
		{"multiple_chunks", bytes.Repeat([]byte{0, 1, 127, 128, 255}, 300001)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "artifact.test")
			if err := os.WriteFile(path, tc.data, 0600); err != nil {
				t.Fatal(err)
			}
			// Keep the lane host's hashlib API even when CI runs a newer Python.
			out, err := exec.Command(python, "-B", "-c", `import hashlib, pathlib, runpy, sys
hashlib.__dict__.pop("file_digest", None)
helper = runpy.run_path("ci-test-binary.py")
print(helper["digest"](pathlib.Path(sys.argv[1])))
`, path).CombinedOutput()
			if err != nil {
				t.Fatalf("host Python digest: %v\n%s", err, out)
			}
			if got, want := strings.TrimSpace(string(out)), fmt.Sprintf("%x", sha256.Sum256(tc.data)); got != want {
				t.Fatalf("SHA-256 = %q, want %q", got, want)
			}
		})
	}
}
