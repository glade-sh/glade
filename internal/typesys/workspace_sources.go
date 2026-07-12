package typesys

import (
	"crypto/sha256"
	"encoding/binary"
	"os"
	"path/filepath"
	"sync"

	"github.com/glade-sh/glade/internal/namespaceremap"
	"github.com/glade-sh/glade/internal/project"
)

// SourceMetadata describes one logical use of an Apex source file.
type SourceMetadata struct {
	RequestedPath   string
	PhysicalPath    string
	Root            string
	Namespace       string
	Version         string
	Dependency      bool
	NamespaceRemaps []namespaceremap.Rule
}

// WorkspaceSource is an immutable view of one logical Apex source file.
type WorkspaceSource struct {
	raw        string
	normalized string
	digest     [sha256.Size]byte
	metadata   SourceMetadata
}

// Raw returns a copy of the exact bytes read from disk.
func (s WorkspaceSource) Raw() []byte {
	return []byte(s.raw)
}

// Normalized returns a copy of the source after namespace token normalization
// and namespace remapping.
func (s WorkspaceSource) Normalized() []byte {
	return []byte(s.normalized)
}

// NormalizedString returns the immutable normalized source text.
func (s WorkspaceSource) NormalizedString() string {
	return s.normalized
}

// Digest returns the SHA-256 digest of the exact raw bytes.
func (s WorkspaceSource) Digest() [sha256.Size]byte {
	return s.digest
}

// Metadata returns a copy of the logical source metadata.
func (s WorkspaceSource) Metadata() SourceMetadata {
	metadata := s.metadata
	metadata.NamespaceRemaps = append([]namespaceremap.Rule(nil), s.metadata.NamespaceRemaps...)
	return metadata
}

// BuildArtifacts contains reusable artifacts produced while building a type
// index. Consumers should treat the contained source arena as read-only.
type BuildArtifacts struct {
	Sources *WorkspaceSources
}

// WorkspaceSources owns physical reads and normalized logical source views for
// one type-index build.
type WorkspaceSources struct {
	mu       sync.Mutex
	readFile func(string) ([]byte, error)
	physical map[string]*physicalSource
	logical  map[logicalSourceKey]*logicalSource
	all      []WorkspaceSource
}

type physicalSource struct {
	ready  chan struct{}
	raw    string
	digest [sha256.Size]byte
	err    error
}

type logicalSourceKey struct {
	physicalPath     string
	namespace        string
	remapFingerprint [sha256.Size]byte
}

type logicalSource struct {
	ready      chan struct{}
	normalized string
}

func newWorkspaceSources(readFile func(string) ([]byte, error)) *WorkspaceSources {
	if readFile == nil {
		readFile = os.ReadFile
	}
	return &WorkspaceSources{
		readFile: readFile,
		physical: make(map[string]*physicalSource),
		logical:  make(map[logicalSourceKey]*logicalSource),
	}
}

// NewWorkspaceSources creates an empty source arena backed by os.ReadFile.
func NewWorkspaceSources() *WorkspaceSources {
	return newWorkspaceSources(os.ReadFile)
}

// All returns the successful logical source occurrences in build order.
// Physical aliases and repeated logical uses remain distinct entries.
func (s *WorkspaceSources) All() []WorkspaceSource {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]WorkspaceSource(nil), s.all...)
}

func (s *WorkspaceSources) record(source WorkspaceSource) {
	s.mu.Lock()
	s.all = append(s.all, source)
	s.mu.Unlock()
}

func (s *WorkspaceSources) load(metadata SourceMetadata) (WorkspaceSource, error) {
	physicalPath := canonicalPhysicalPath(metadata.RequestedPath)
	metadata.PhysicalPath = physicalPath
	metadata.NamespaceRemaps = append([]namespaceremap.Rule(nil), metadata.NamespaceRemaps...)

	physical, err := s.loadPhysical(physicalPath, metadata.RequestedPath)
	if err != nil {
		return WorkspaceSource{}, err
	}
	key := logicalSourceKey{
		physicalPath:     physicalPath,
		namespace:        metadata.Namespace,
		remapFingerprint: sourceRemapFingerprint(metadata.NamespaceRemaps),
	}
	logical := s.loadLogical(key, physical.raw, metadata.Namespace, metadata.NamespaceRemaps)
	return WorkspaceSource{
		raw:        physical.raw,
		normalized: logical.normalized,
		digest:     physical.digest,
		metadata:   metadata,
	}, nil
}

func sourceRemapFingerprint(remaps []namespaceremap.Rule) [sha256.Size]byte {
	hash := sha256.New()
	var length [8]byte
	for _, remap := range remaps {
		binary.BigEndian.PutUint64(length[:], uint64(len(remap.From)))
		hash.Write(length[:])
		hash.Write([]byte(remap.From))
		binary.BigEndian.PutUint64(length[:], uint64(len(remap.To)))
		hash.Write(length[:])
		hash.Write([]byte(remap.To))
	}
	var fingerprint [sha256.Size]byte
	copy(fingerprint[:], hash.Sum(nil))
	return fingerprint
}

func (s *WorkspaceSources) loadPhysical(physicalPath, requestedPath string) (*physicalSource, error) {
	s.mu.Lock()
	physical, ok := s.physical[physicalPath]
	if ok {
		s.mu.Unlock()
		<-physical.ready
		return physical, sourceReadError(physical.err, requestedPath)
	}
	physical = &physicalSource{ready: make(chan struct{})}
	s.physical[physicalPath] = physical
	s.mu.Unlock()

	data, err := s.readFile(physicalPath)
	if err == nil {
		physical.raw = string(data)
		physical.digest = sha256.Sum256(data)
	}
	physical.err = err
	close(physical.ready)
	return physical, sourceReadError(err, requestedPath)
}

func (s *WorkspaceSources) loadLogical(key logicalSourceKey, raw, namespace string, remaps []namespaceremap.Rule) *logicalSource {
	s.mu.Lock()
	logical, ok := s.logical[key]
	if ok {
		s.mu.Unlock()
		<-logical.ready
		return logical
	}
	logical = &logicalSource{ready: make(chan struct{})}
	s.logical[key] = logical
	s.mu.Unlock()

	normalized := project.NormalizeApexNamespaceTokens(raw, namespace)
	logical.normalized = namespaceremap.ApplySource(remaps, normalized)
	close(logical.ready)
	return logical
}

func canonicalPhysicalPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	abs = filepath.Clean(abs)
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return abs
	}
	resolvedAbs, err := filepath.Abs(resolved)
	if err != nil {
		return filepath.Clean(resolved)
	}
	return filepath.Clean(resolvedAbs)
}

func sourceReadError(err error, requestedPath string) error {
	if err == nil {
		return nil
	}
	if pathErr, ok := err.(*os.PathError); ok {
		copy := *pathErr
		copy.Path = requestedPath
		return &copy
	}
	return err
}
