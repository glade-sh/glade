package dataweave

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

//go:embed artifacts.json
var artifactManifest []byte

//go:embed THIRD-PARTY-NOTICES.md
var distributionNotice []byte

type artifact struct {
	Path      string `json:"path"`
	URL       string `json:"url"`
	SHA256    string `json:"sha256"`
	Bytes     int64  `json:"bytes"`
	ClassPath bool   `json:"classpath"`
}

func artifacts() []artifact {
	var out []artifact
	if err := json.Unmarshal(artifactManifest, &out); err != nil {
		panic(err)
	}
	return out
}

// verifyClassPath admits only the exact locked runtime archives, before Java can
// load classes, service providers or DWL resources from them.
func verifyClassPath(paths []string) error {
	expected := map[string]artifact{}
	for _, a := range artifacts() {
		if a.ClassPath {
			expected[filepath.Base(a.Path)] = a
		}
	}
	if len(paths) != len(expected) {
		return fmt.Errorf("expected %d locked DataWeave jars", len(expected))
	}
	for _, path := range paths {
		name := filepath.Base(path)
		a, ok := expected[name]
		if !ok {
			return fmt.Errorf("unexpected or duplicate DataWeave jar: %s", name)
		}
		if err := verifyArtifact(path, a); err != nil {
			return err
		}
		delete(expected, name)
	}
	return nil
}
func verifyArtifact(path string, a artifact) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, a.Bytes+1))
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if n != a.Bytes || hex.EncodeToString(h.Sum(nil)) != a.SHA256 {
		return fmt.Errorf("DataWeave artifact verification failed: %s", a.Path)
	}
	return nil
}

// VerifyEngine checks every engine and corresponding source archive against the
// embedded lock. It returns only the runtime classpath, excluding source jars.
func VerifyEngine(directory string) ([]string, error) {
	var paths []string
	for _, a := range artifacts() {
		path := filepath.Join(directory, filepath.FromSlash(a.Path))
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		h := sha256.New()
		n, err := io.Copy(h, io.LimitReader(f, a.Bytes+1))
		closeErr := f.Close()
		if err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if n != a.Bytes || hex.EncodeToString(h.Sum(nil)) != a.SHA256 {
			return nil, fmt.Errorf("DataWeave artifact verification failed: %s", a.Path)
		}
		if a.ClassPath {
			paths = append(paths, path)
		}
	}
	for _, item := range []struct {
		name  string
		value []byte
	}{{"THIRD-PARTY-NOTICES.md", distributionNotice}, {"artifacts.json", artifactManifest}} {
		got, err := os.ReadFile(filepath.Join(directory, item.name))
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(got, item.value) {
			return nil, fmt.Errorf("DataWeave distribution metadata changed: %s", item.name)
		}
	}
	for _, path := range paths {
		if err := processNotices(path, filepath.Join(directory, "notices", filepath.Base(path)), true); err != nil {
			return nil, err
		}
	}
	return paths, nil
}

// InstallEngine explicitly downloads the pinned engine and its corresponding
// source offers. It never runs during Execute. Existing incomplete directories
// are left untouched; a caller may choose a new versioned installation path.
func InstallEngine(ctx context.Context, directory string) ([]string, error) {
	if _, err := os.Stat(directory); err == nil {
		return VerifyEngine(directory)
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	parent := filepath.Dir(directory)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return nil, err
	}
	stage, err := os.MkdirTemp(parent, ".dataweave-install-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)
	for _, a := range artifacts() {
		path := filepath.Join(stage, filepath.FromSlash(a.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return nil, err
		}
		if err := downloadArtifact(ctx, a, path); err != nil {
			return nil, err
		}
		if a.ClassPath {
			if err := extractNotices(path, filepath.Join(stage, "notices", filepath.Base(path))); err != nil {
				return nil, err
			}
		}
	}
	if err := os.WriteFile(filepath.Join(stage, "THIRD-PARTY-NOTICES.md"), distributionNotice, 0600); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(stage, "artifacts.json"), artifactManifest, 0600); err != nil {
		return nil, err
	}
	if _, err := VerifyEngine(stage); err != nil {
		return nil, err
	}
	if err := os.Rename(stage, directory); err != nil {
		return nil, err
	}
	return VerifyEngine(directory)
}

func downloadArtifact(ctx context.Context, a artifact, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("DataWeave artifact %s: %s", a.Path, response.Status)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	h := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(f, h), io.LimitReader(response.Body, a.Bytes+1))
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if n != a.Bytes || hex.EncodeToString(h.Sum(nil)) != a.SHA256 {
		return fmt.Errorf("DataWeave artifact checksum mismatch: %s", a.Path)
	}
	return nil
}
func extractNotices(archive, directory string) error {
	return processNotices(archive, directory, false)
}
func processNotices(archive, directory string, verify bool) error {
	z, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer z.Close()
	for _, entry := range z.File {
		name := strings.ToUpper(filepath.Base(entry.Name))
		if entry.FileInfo().IsDir() || (!strings.Contains(name, "LICENSE") && !strings.Contains(name, "NOTICE") && !strings.Contains(name, "COPYING")) {
			continue
		}
		// Flatten only notice entries; archive paths never become filesystem paths.
		safe := strings.NewReplacer("/", "_", "\\", "_").Replace(entry.Name)
		r, err := entry.Open()
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(io.LimitReader(r, 1<<20))
		closeErr := r.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		if len(data) >= 1<<20 {
			return fmt.Errorf("oversized DataWeave notice: %s", entry.Name)
		}
		if verify {
			actual, err := os.ReadFile(filepath.Join(directory, safe))
			if err != nil {
				return err
			}
			if !bytes.Equal(actual, data) {
				return fmt.Errorf("DataWeave notice changed: %s", entry.Name)
			}
			continue
		}
		if err := os.MkdirAll(directory, 0700); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(directory, safe), data, 0600); err != nil {
			return err
		}
	}
	return nil
}
