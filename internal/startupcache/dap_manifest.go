package startupcache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/glade-sh/glade/internal/inputview"
)

const dapManifestFile = "startup-manifest.json"
const maxDAPManifestSize = 16 << 20

type dapManifest struct {
	Version       int                     `json:"version"`
	ProjectRoot   string                  `json:"projectRoot"`
	PlatformABI   string                  `json:"platformAbi"`
	Manifest      Manifest                `json:"manifest"`
	Inputs        []inputview.Observation `json:"inputs"`
	PayloadSHA256 string                  `json:"payloadSha256"`
}

// ReadDAPManifest reads only the bounded preflight data. A missing or invalid
// sidecar is a cache miss, including caches written before the sidecar existed.
func ReadDAPManifest(root string) (*Entry, string, error) {
	dir, err := openPrivateStartupCacheDir(root, SubdirDAP, false)
	if errors.Is(err, os.ErrNotExist) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	defer dir.Close()
	f, err := dir.Open(dapManifestFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", nil
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxDAPManifestSize+1))
	if err != nil || len(data) > maxDAPManifestSize {
		return nil, "", nil
	}
	var m dapManifest
	if json.Unmarshal(data, &m) != nil {
		return nil, "", nil
	}
	digest, decodeErr := hex.DecodeString(m.PayloadSHA256)
	if decodeErr != nil || len(digest) != sha256.Size {
		return nil, "", nil
	}
	return &Entry{Version: m.Version, ProjectRoot: m.ProjectRoot, PlatformABI: m.PlatformABI, Manifest: m.Manifest, Inputs: m.Inputs}, m.PayloadSHA256, nil
}

// ReadDAPVerified accepts a payload only when it belongs to the preflight.
func ReadDAPVerified(root, digest string) (*Entry, error) {
	dir, err := openPrivateStartupCacheDir(root, SubdirDAP, false)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	data, err := dir.ReadFile(stateFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != digest {
		return nil, nil
	}
	var entry Entry
	if json.Unmarshal(data, &entry) != nil {
		return nil, nil
	}
	return &entry, nil
}

func writeDAPManifest(entry *Entry, cacheDir, digest string) error {
	m := dapManifest{Version: entry.Version, ProjectRoot: entry.ProjectRoot, PlatformABI: entry.PlatformABI, Manifest: entry.Manifest, Inputs: entry.Inputs, PayloadSHA256: digest}
	tmp, err := os.CreateTemp(cacheDir, "manifest-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if err := json.NewEncoder(tmp).Encode(m); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(cacheDir, dapManifestFile))
}
