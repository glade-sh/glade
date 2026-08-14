// Package semanticcache stores complete semantic results behind caller-supplied
// immutable content identities. It never reads project, schema, or dependency
// inputs itself.
package semanticcache

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/glade-sh/glade/internal/sema"
)

const EnvelopeVersion = 1

// maxEnvelopeBytes bounds data accepted from the untrusted disk cache. It
// matches the initial maximum completed semantic-result residency from the
// Phase 2 cache plan. Store rejects larger entries so it never publishes data
// that Load will refuse.
const maxEnvelopeBytes int64 = 512 << 20

const cacheSubdir = ".glade/semantic"

// Identity proves the exact semantic inputs used to produce a cached result.
type Identity struct {
	ProjectContentSHA256 string `json:"projectContentSHA256"`
	SchemaContentSHA256  string `json:"schemaContentSHA256"`
	DependencySHA256     string `json:"dependencySHA256"`
	SemanticABI          string `json:"semanticABI"`
	PlatformABI          string `json:"platformABI"`
	OptionsFingerprint   string `json:"optionsFingerprint"`
}

// Envelope is the in-memory semantic cache contract. The on-disk
// representation is private and uses sema.ResultSnapshot to preserve the
// complete result without exposing mutable shared state.
type Envelope struct {
	Version  int
	Identity Identity
	Result   sema.Result
}

// MissReason classifies fail-closed cache misses for profiling and policy.
type MissReason string

const (
	MissAbsent             MissReason = "absent"
	MissIdentityMismatch   MissReason = "identity_mismatch"
	MissOptionsMismatch    MissReason = "option_mismatch"
	MissCorrupt            MissReason = "corrupt"
	MissUnsupportedVersion MissReason = "unsupported_version"
)

var ErrMiss = errors.New("semantic cache miss")

// MissError reports why a cached value was not trusted.
type MissError struct {
	Reason MissReason
	Cause  error
}

func (err *MissError) Error() string {
	if err.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", ErrMiss, err.Reason, err.Cause)
	}
	return fmt.Sprintf("%s: %s", ErrMiss, err.Reason)
}

func (err *MissError) Is(target error) bool {
	return target == ErrMiss
}

func (err *MissError) Unwrap() error {
	return err.Cause
}

type diskEnvelope struct {
	Version        int                 `json:"version"`
	Identity       Identity            `json:"identity"`
	Result         sema.ResultSnapshot `json:"result"`
	ChecksumSHA256 string              `json:"checksumSHA256"`
}

type checksumPayload struct {
	Version  int                 `json:"version"`
	Identity Identity            `json:"identity"`
	Result   sema.ResultSnapshot `json:"result"`
}

// Store atomically replaces relativePath beneath projectRoot with a private,
// checksummed semantic result. projectRoot must be a trusted caller-owned root;
// every relative cache-directory component is verified before publication.
func Store(projectRoot, relativePath string, identity Identity, result sema.Result) error {
	if err := identity.validate(); err != nil {
		return err
	}
	dir, name, err := splitCachePath(relativePath)
	if err != nil {
		return err
	}
	payload := checksumPayload{
		Version:  EnvelopeVersion,
		Identity: identity,
		Result:   sema.SnapshotResult(result),
	}
	checksum, err := payloadChecksum(payload)
	if err != nil {
		return fmt.Errorf("encode semantic cache checksum: %w", err)
	}
	data, err := json.Marshal(diskEnvelope{
		Version:        payload.Version,
		Identity:       payload.Identity,
		Result:         payload.Result,
		ChecksumSHA256: checksum,
	})
	if err != nil {
		return fmt.Errorf("encode semantic cache envelope: %w", err)
	}
	if int64(len(data)) > maxEnvelopeBytes {
		return fmt.Errorf("semantic cache envelope is %d bytes, maximum is %d", len(data), maxEnvelopeBytes)
	}
	cacheRoot, err := openPrivateCacheDir(projectRoot, dir, true)
	if err != nil {
		return fmt.Errorf("open semantic cache directory: %w", err)
	}
	defer cacheRoot.Close()
	return writeAtomically(cacheRoot, name, data)
}

// Load returns the complete cached result only when its version, checksum, and
// exact identity match. Every untrusted file is a typed fail-closed miss.
func Load(projectRoot, relativePath string, expected Identity) (sema.Result, error) {
	if err := expected.validate(); err != nil {
		reason := MissIdentityMismatch
		if strings.TrimSpace(expected.OptionsFingerprint) == "" {
			reason = MissOptionsMismatch
		}
		return sema.Result{}, miss(reason, err)
	}
	dir, name, err := splitCachePath(relativePath)
	if err != nil {
		return sema.Result{}, miss(MissCorrupt, err)
	}
	cacheRoot, err := openPrivateCacheDir(projectRoot, dir, false)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return sema.Result{}, miss(MissAbsent, err)
		}
		return sema.Result{}, miss(MissCorrupt, err)
	}
	defer cacheRoot.Close()
	data, err := readEnvelope(cacheRoot, name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return sema.Result{}, miss(MissAbsent, err)
		}
		return sema.Result{}, miss(MissCorrupt, err)
	}

	var stored diskEnvelope
	if err := json.Unmarshal(data, &stored); err != nil {
		return sema.Result{}, miss(MissCorrupt, err)
	}
	if stored.Version != EnvelopeVersion {
		return sema.Result{}, miss(MissUnsupportedVersion, nil)
	}
	checksum, err := payloadChecksum(checksumPayload{
		Version:  stored.Version,
		Identity: stored.Identity,
		Result:   stored.Result,
	})
	if err != nil {
		return sema.Result{}, miss(MissCorrupt, err)
	}
	if checksum != stored.ChecksumSHA256 {
		return sema.Result{}, miss(MissCorrupt, errors.New("checksum mismatch"))
	}
	if err := stored.Identity.validate(); err != nil {
		return sema.Result{}, miss(MissCorrupt, err)
	}
	if !stored.Identity.equalInputs(expected) {
		return sema.Result{}, miss(MissIdentityMismatch, nil)
	}
	if stored.Identity.OptionsFingerprint != expected.OptionsFingerprint {
		return sema.Result{}, miss(MissOptionsMismatch, nil)
	}
	return stored.Result.Result(), nil
}

// Clear removes entries from the verified project-local semantic cache
// directory without following project-controlled links.
func Clear(projectRoot string) error {
	cacheRoot, err := openPrivateCacheDir(projectRoot, cacheSubdir, false)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer cacheRoot.Close()
	directory, err := cacheRoot.Open(".")
	if err != nil {
		return err
	}
	defer directory.Close()
	entries, err := directory.ReadDir(-1)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return fmt.Errorf("semantic cache contains unexpected directory %q", entry.Name())
		}
		if err := cacheRoot.Remove(entry.Name()); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (identity Identity) equalInputs(other Identity) bool {
	return identity.ProjectContentSHA256 == other.ProjectContentSHA256 &&
		identity.SchemaContentSHA256 == other.SchemaContentSHA256 &&
		identity.DependencySHA256 == other.DependencySHA256 &&
		identity.SemanticABI == other.SemanticABI &&
		identity.PlatformABI == other.PlatformABI
}

func (identity Identity) validate() error {
	fields := []struct {
		name  string
		value string
	}{
		{name: "project content SHA-256", value: identity.ProjectContentSHA256},
		{name: "schema content SHA-256", value: identity.SchemaContentSHA256},
		{name: "dependency SHA-256", value: identity.DependencySHA256},
		{name: "semantic ABI", value: identity.SemanticABI},
		{name: "platform ABI", value: identity.PlatformABI},
		{name: "options fingerprint", value: identity.OptionsFingerprint},
	}
	for _, field := range fields {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("semantic cache identity requires %s", field.name)
		}
	}
	return nil
}

func payloadChecksum(payload checksumPayload) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}

func miss(reason MissReason, cause error) error {
	return &MissError{Reason: reason, Cause: cause}
}

func splitCachePath(relativePath string) (string, string, error) {
	clean := filepath.Clean(filepath.FromSlash(relativePath))
	if filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", "", fmt.Errorf("semantic cache path %q must stay within the project root", relativePath)
	}
	name := filepath.Base(clean)
	if name == "." || name == string(os.PathSeparator) {
		return "", "", fmt.Errorf("semantic cache path %q requires a file name", relativePath)
	}
	return filepath.Dir(clean), name, nil
}

func openPrivateCacheDir(projectRoot, relativeDir string, create bool) (*os.Root, error) {
	return openPrivateCacheDirAfterLstat(projectRoot, relativeDir, create, nil)
}

func openPrivateCacheDirAfterLstat(projectRoot, relativeDir string, create bool, afterLstat func(string) error) (*os.Root, error) {
	clean := filepath.Clean(filepath.FromSlash(relativeDir))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return nil, fmt.Errorf("semantic cache directory %q must stay within the project root", relativeDir)
	}
	current, err := os.OpenRoot(projectRoot)
	if err != nil {
		return nil, err
	}
	if clean == "." {
		return current, nil
	}
	for _, component := range strings.Split(clean, string(os.PathSeparator)) {
		if create {
			if err := current.Mkdir(component, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
				_ = current.Close()
				return nil, err
			}
		}
		beforeOpen, err := current.Lstat(component)
		if err != nil {
			_ = current.Close()
			return nil, err
		}
		if beforeOpen.Mode()&os.ModeSymlink != 0 {
			_ = current.Close()
			return nil, fmt.Errorf("semantic cache path component %q must not be a symlink", component)
		}
		if !beforeOpen.IsDir() {
			_ = current.Close()
			return nil, fmt.Errorf("semantic cache path component %q is not a directory", component)
		}
		if afterLstat != nil {
			if err := afterLstat(component); err != nil {
				_ = current.Close()
				return nil, err
			}
		}
		next, err := current.OpenRoot(component)
		if err != nil {
			_ = current.Close()
			return nil, err
		}
		afterOpen, err := next.Stat(".")
		if err != nil {
			_ = next.Close()
			_ = current.Close()
			return nil, err
		}
		if !os.SameFile(beforeOpen, afterOpen) {
			_ = next.Close()
			_ = current.Close()
			return nil, fmt.Errorf("semantic cache path component %q changed while opening", component)
		}
		_ = current.Close()
		current = next
	}
	if err := current.Chmod(".", 0o700); err != nil {
		_ = current.Close()
		return nil, fmt.Errorf("restrict semantic cache directory: %w", err)
	}
	return current, nil
}

func readEnvelope(cacheRoot *os.Root, name string) ([]byte, error) {
	beforeOpen, err := cacheRoot.Lstat(name)
	if err != nil {
		return nil, err
	}
	if beforeOpen.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("semantic cache file %q must not be a symlink", name)
	}
	if !beforeOpen.Mode().IsRegular() {
		return nil, fmt.Errorf("semantic cache file %q is not a regular file", name)
	}
	if beforeOpen.Size() > maxEnvelopeBytes {
		return nil, fmt.Errorf("semantic cache file %q is %d bytes, maximum is %d", name, beforeOpen.Size(), maxEnvelopeBytes)
	}
	file, err := cacheRoot.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	afterOpen, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !afterOpen.Mode().IsRegular() {
		return nil, fmt.Errorf("semantic cache file %q is not a regular file", name)
	}
	if afterOpen.Size() > maxEnvelopeBytes {
		return nil, fmt.Errorf("semantic cache file %q is %d bytes, maximum is %d", name, afterOpen.Size(), maxEnvelopeBytes)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxEnvelopeBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxEnvelopeBytes {
		return nil, fmt.Errorf("semantic cache file %q exceeds %d bytes", name, maxEnvelopeBytes)
	}
	return data, nil
}

func createCacheTemp(cacheRoot *os.Root, target string) (*os.File, string, error) {
	var random [16]byte
	for range 100 {
		if _, err := rand.Read(random[:]); err != nil {
			return nil, "", err
		}
		name := "." + target + "-" + hex.EncodeToString(random[:]) + ".tmp"
		file, err := cacheRoot.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			return file, name, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, "", err
		}
	}
	return nil, "", errors.New("could not allocate unique semantic cache temporary file")
}

func writeAtomically(cacheRoot *os.Root, name string, data []byte) (err error) {
	file, tempName, err := createCacheTemp(cacheRoot, name)
	if err != nil {
		return fmt.Errorf("create semantic cache temporary file: %w", err)
	}
	closed := false
	defer func() {
		if !closed {
			_ = file.Close()
		}
		_ = cacheRoot.Remove(tempName)
	}()
	if err := file.Chmod(0o600); err != nil {
		return fmt.Errorf("protect semantic cache temporary file: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("write semantic cache temporary file: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync semantic cache temporary file: %w", err)
	}
	if err := file.Close(); err != nil {
		closed = true
		return fmt.Errorf("close semantic cache temporary file: %w", err)
	}
	closed = true
	// Root.Rename is an atomic same-directory replacement on the currently
	// published Darwin and Linux targets. Keep disk-cache publication disabled
	// if a future non-Unix target cannot provide that contract.
	if err := cacheRoot.Rename(tempName, name); err != nil {
		return fmt.Errorf("publish semantic cache file: %w", err)
	}
	return nil
}
