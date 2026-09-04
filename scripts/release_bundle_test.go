package scripts

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestReleaseBundleResumesWithoutChangingUploadedBytes(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "dist")
	writeReleaseFixtureFile(t, filepath.Join(source, "index.json"), "release index\n")
	bundle := filepath.Join(root, "release.tar.gz")
	f, err := os.Create(bundle)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	gz.Header.ModTime = time.Unix(100, 0)
	tw := tar.NewWriter(gz)
	body := []byte("release index\n")
	if err := tw.WriteHeader(&tar.Header{Name: "./index.json", Mode: 0o644, Size: int64(len(body)), ModTime: time.Unix(100, 0)}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	original := string(mustReadFile(t, bundle))
	cmd := exec.Command("python3", "release-bundle.py", source, bundle)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("resume: %v\n%s", err, out)
	}
	if string(mustReadFile(t, bundle)) != original {
		t.Fatal("resume changed uploaded bundle bytes")
	}
	writeReleaseFixtureFile(t, filepath.Join(source, "index.json"), "different release\n")
	cmd = exec.Command("python3", "release-bundle.py", source, bundle)
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("accepted changed payload: %s", out)
	}
	if string(mustReadFile(t, bundle)) != original {
		t.Fatal("rejection changed uploaded bundle bytes")
	}
}

func TestReleaseBundleIgnoresSourceTimestamps(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "dist")
	file := filepath.Join(source, "v1.2.3", "release-manifest.json")
	writeReleaseFixtureFile(t, file, "{}\n")
	first, second := filepath.Join(root, "first.tar.gz"), filepath.Join(root, "second.tar.gz")
	for i, output := range []string{first, second} {
		stamp := time.Unix(int64(i+1), 0)
		if err := os.Chtimes(file, stamp, stamp); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("python3", "release-bundle.py", source, output)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("bundle: %v\n%s", err, out)
		}
	}
	if string(mustReadFile(t, first)) != string(mustReadFile(t, second)) {
		t.Fatal("bundle depends on source timestamps")
	}
}
