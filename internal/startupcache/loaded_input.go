package startupcache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/glade-sh/glade/internal/inputview"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/typesys"
)

// ValidateLoadedInput builds the input proof from a project that was already
// loaded, without loading it again. When p carries an input view, every
// non-Apex fingerprint is an observation of that view, so the proof and the
// loaded state come from the same reads. Apex bodies use digests, as in
// ValidateInputWithSourceDigests.
func ValidateLoadedInput(projectRoot string, p project.Project, digests *typesys.SourceDigestSet) (*ValidatedInput, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, err
	}
	root = filepath.Clean(root)
	manifest, err := buildManifestWithSourceDigests(root, p, true, digests, nil)
	if err != nil {
		return nil, err
	}
	if !manifest.Complete {
		return nil, errors.New("startup cache validated input manifest is incomplete")
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(encoded)
	return &ValidatedInput{
		projectRoot: root,
		manifest:    cloneManifest(manifest),
		digest:      hex.EncodeToString(sum[:]),
	}, nil
}

// fingerprintInput fingerprints path through inputs when the manifest is built
// from a viewed project; otherwise it keeps the existing disk path.
func fingerprintInput(inputs *inputview.View, projectRoot, path string, includeHash bool, digest [sha256.Size]byte, useDigest bool, readFile func(string) ([]byte, error)) (File, error) {
	if inputs == nil {
		return fingerprintFileWithSourceDigest(projectRoot, path, includeHash, digest, useDigest, readFile)
	}
	if useDigest {
		info, err := inputs.Stat(path)
		if err != nil {
			return File{}, err
		}
		fp, err := fileFingerprintFromInfo(projectRoot, path, info)
		if err != nil {
			return File{}, err
		}
		if includeHash {
			fp.SHA256 = hex.EncodeToString(digest[:])
		}
		return fp, nil
	}
	pinned, err := inputs.Fingerprint(path)
	if err != nil {
		return File{}, err
	}
	rel, err := filepath.Rel(projectRoot, path)
	if err != nil {
		return File{}, err
	}
	fp := File{Path: filepath.ToSlash(filepath.Clean(rel)), Size: pinned.Size, ModTime: pinned.ModTime.UnixNano()}
	if includeHash {
		fp.SHA256 = hex.EncodeToString(pinned.SHA256[:])
	}
	return fp, nil
}

func fingerprintOptionalInput(inputs *inputview.View, projectRoot, path string, includeHash bool, readFile func(string) ([]byte, error)) (File, bool, error) {
	if inputs == nil {
		return fingerprintOptionalFileWithReader(projectRoot, path, includeHash, readFile)
	}
	fp, err := fingerprintInput(inputs, projectRoot, path, includeHash, [sha256.Size]byte{}, false, readFile)
	if errors.Is(err, os.ErrNotExist) {
		return File{}, false, nil
	}
	if err != nil {
		return File{}, false, err
	}
	return fp, true, nil
}
