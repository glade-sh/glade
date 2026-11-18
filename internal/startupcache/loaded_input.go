package startupcache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/glade-sh/glade/internal/filestamp"
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
func fingerprintInputForScope(inputs *inputview.View, projectRoot, path string, includeHash bool, digest [sha256.Size]byte, useDigest bool, readFile func(string) ([]byte, error), eligible func(string) bool, lookups ...func(string) ([sha256.Size]byte, bool)) (File, error) {
	if eligible != nil && len(lookups) != 0 && lookups[0] != nil {
		return fingerprintInputWithScopedDigest(inputs, projectRoot, path, includeHash, digest, useDigest, readFile, eligible, lookups[0])
	}
	return fingerprintInput(inputs, projectRoot, path, includeHash, digest, useDigest, readFile, lookups...)
}

func fingerprintInputWithScopedDigest(inputs *inputview.View, projectRoot, path string, includeHash bool, digest [sha256.Size]byte, useDigest bool, readFile func(string) ([]byte, error), eligible func(string) bool, lookup func(string) ([sha256.Size]byte, bool)) (File, error) {
	if inputs == nil && includeHash && !useDigest && readFile == nil && eligible != nil && lookup != nil && eligible(path) {
		info, before, beforeOK := filestamp.StatWithInfo(path)
		cached, hit := lookup(path)
		after, afterOK := filestamp.Stat(path)
		if beforeOK && afterOK && before == after && hit {
			fp, err := fileFingerprintFromInfo(projectRoot, path, info)
			if err != nil {
				return File{}, err
			}
			fp.SHA256 = hex.EncodeToString(cached[:])
			return fp, nil
		}
	}
	return fingerprintInput(inputs, projectRoot, path, includeHash, digest, useDigest, readFile)
}

func fingerprintInput(inputs *inputview.View, projectRoot, path string, includeHash bool, digest [sha256.Size]byte, useDigest bool, readFile func(string) ([]byte, error), lookups ...func(string) ([sha256.Size]byte, bool)) (File, error) {
	if inputs == nil {
		if includeHash && !useDigest && readFile == nil && len(lookups) != 0 && lookups[0] != nil {
			lookup := lookups[0]
			if _, hit := lookup(path); hit {
				before, beforeOK := filestamp.Stat(path)
				info, infoErr := os.Stat(path)
				cached, stillHit := lookup(path)
				after, afterOK := filestamp.Stat(path)
				if beforeOK && afterOK && before == after && infoErr == nil && info.Size() == before.Size && info.ModTime().UnixNano() == before.Mtime && stillHit {
					fp, err := fileFingerprintFromInfo(projectRoot, path, info)
					if err != nil {
						return File{}, err
					}
					fp.SHA256 = hex.EncodeToString(cached[:])
					return fp, nil
				}
			}
		}
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

func fingerprintOptionalInputForScope(inputs *inputview.View, projectRoot, path string, includeHash bool, readFile func(string) ([]byte, error), eligible func(string) bool, lookups ...func(string) ([sha256.Size]byte, bool)) (File, bool, error) {
	if eligible == nil || len(lookups) == 0 || lookups[0] == nil {
		return fingerprintOptionalInput(inputs, projectRoot, path, includeHash, readFile, lookups...)
	}
	fp, err := fingerprintInputWithScopedDigest(inputs, projectRoot, path, includeHash, [sha256.Size]byte{}, false, readFile, eligible, lookups[0])
	if errors.Is(err, os.ErrNotExist) {
		return File{}, false, nil
	}
	if err != nil {
		return File{}, false, err
	}
	return fp, true, nil
}

func fingerprintOptionalInput(inputs *inputview.View, projectRoot, path string, includeHash bool, readFile func(string) ([]byte, error), lookups ...func(string) ([sha256.Size]byte, bool)) (File, bool, error) {
	if inputs == nil && len(lookups) == 0 {
		return fingerprintOptionalFileWithReader(projectRoot, path, includeHash, readFile)
	}
	fp, err := fingerprintInput(inputs, projectRoot, path, includeHash, [sha256.Size]byte{}, false, readFile, lookups...)
	if errors.Is(err, os.ErrNotExist) {
		return File{}, false, nil
	}
	if err != nil {
		return File{}, false, err
	}
	return fp, true, nil
}
