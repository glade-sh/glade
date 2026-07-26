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

// RawString returns the immutable source text exactly as read from disk.
func (s WorkspaceSource) RawString() string {
	return s.raw
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
	Sources       *WorkspaceSources
	SourceDigests *SourceDigestSet
}

// SourceDigestSet is an immutable, compact snapshot of the exact raw source
// digests used by one type-index build.
type SourceDigestSet struct {
	physical  map[string][sha256.Size]byte
	requested map[string]string
	absolute  map[string]string
}

// Digest returns the raw-byte SHA-256 digest captured for path. Lookup uses
// only aliases recorded by the build and never reevaluates symlinks or reads
// the filesystem.
func (s *SourceDigestSet) Digest(path string) ([sha256.Size]byte, bool) {
	if s == nil {
		return [sha256.Size]byte{}, false
	}
	physicalPath, ok := s.requested[path]
	if !ok {
		if !filepath.IsAbs(path) {
			return [sha256.Size]byte{}, false
		}
		physicalPath, ok = s.absolute[filepath.Clean(path)]
		if !ok {
			return [sha256.Size]byte{}, false
		}
	}
	digest, ok := s.physical[physicalPath]
	return digest, ok
}

// PhysicalCount returns the number of distinct successfully read physical
// sources represented by the snapshot.
func (s *SourceDigestSet) PhysicalCount() int {
	if s == nil {
		return 0
	}
	return len(s.physical)
}

func (s *SourceDigestSet) withSourceDigest(path string, raw []byte) *SourceDigestSet {
	copySet := &SourceDigestSet{
		physical:  make(map[string][sha256.Size]byte),
		requested: make(map[string]string),
		absolute:  make(map[string]string),
	}
	if s != nil {
		for key, value := range s.physical {
			copySet.physical[key] = value
		}
		for key, value := range s.requested {
			copySet.requested[key] = value
		}
		for key, value := range s.absolute {
			copySet.absolute[key] = value
		}
	}
	physical := copySet.requested[path]
	if physical == "" {
		physical = copySet.absolute[cleanedAbsolutePath(path)]
	}
	if physical == "" {
		physical = canonicalPhysicalPath(path)
	}
	copySet.physical[physical] = sha256.Sum256(raw)
	copySet.requested[path] = physical
	copySet.absolute[cleanedAbsolutePath(path)] = physical
	copySet.absolute[cleanedAbsolutePath(physical)] = physical
	return copySet
}

func (s *SourceDigestSet) withoutSource(path string) *SourceDigestSet {
	if s == nil {
		return nil
	}
	copySet := &SourceDigestSet{
		physical:  make(map[string][sha256.Size]byte, len(s.physical)),
		requested: make(map[string]string, len(s.requested)),
		absolute:  make(map[string]string, len(s.absolute)),
	}
	for key, value := range s.physical {
		copySet.physical[key] = value
	}
	for key, value := range s.requested {
		copySet.requested[key] = value
	}
	for key, value := range s.absolute {
		copySet.absolute[key] = value
	}
	cleaned := cleanedAbsolutePath(path)
	physical := copySet.requested[path]
	if physical == "" {
		physical = copySet.absolute[cleaned]
	}
	for requested := range copySet.requested {
		if requested == path || cleanedAbsolutePath(requested) == cleaned {
			delete(copySet.requested, requested)
		}
	}
	delete(copySet.absolute, cleaned)
	retained := false
	for _, candidate := range copySet.requested {
		if candidate == physical {
			retained = true
			break
		}
	}
	if physical != "" && !retained {
		delete(copySet.physical, physical)
		for alias, candidate := range copySet.absolute {
			if candidate == physical {
				delete(copySet.absolute, alias)
			}
		}
	} else if physical != "" {
		copySet.absolute[cleanedAbsolutePath(physical)] = physical
	}
	return copySet
}

// SourceForType returns the exact logical source occurrence used to build typ.
// It never reads the filesystem.
func (a BuildArtifacts) SourceForType(typ TypeSymbol) (WorkspaceSource, bool) {
	if a.Sources == nil {
		return WorkspaceSource{}, false
	}
	return a.Sources.sourceForMetadata(SourceMetadata{
		RequestedPath:   typ.File,
		Root:            typ.SourceRoot,
		Namespace:       typ.Namespace,
		Version:         typ.Version,
		Dependency:      typ.Dependency,
		NamespaceRemaps: typ.SourceNamespaceRemaps,
	})
}

// SourceForTrigger returns the exact logical source occurrence used to build
// trigger. It never reads the filesystem.
func (a BuildArtifacts) SourceForTrigger(trigger TriggerSymbol) (WorkspaceSource, bool) {
	if a.Sources == nil {
		return WorkspaceSource{}, false
	}
	return a.Sources.sourceForMetadata(SourceMetadata{
		RequestedPath:   trigger.File,
		Root:            trigger.SourceRoot,
		Namespace:       trigger.Namespace,
		Version:         trigger.Version,
		Dependency:      trigger.Dependency,
		NamespaceRemaps: trigger.SourceNamespaceRemaps,
	})
}

// WorkspaceSourceStats reports source-arena work for one index build.
type WorkspaceSourceStats struct {
	PhysicalReadAttempts uint64
	PhysicalSources      uint64
	LogicalViews         uint64
	Occurrences          uint64
}

// WorkspaceSources owns physical reads and normalized logical source views for
// one type-index build.
type WorkspaceSources struct {
	mu                   sync.Mutex
	readFile             func(string) ([]byte, error)
	physical             map[string]*physicalSource
	logical              map[logicalSourceKey]*logicalSource
	occurrence           map[sourceOccurrenceKey]WorkspaceSource
	all                  []WorkspaceSource
	physicalReadAttempts uint64
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

type sourceOccurrenceKey struct {
	requestedPath    string
	root             string
	namespace        string
	version          string
	dependency       bool
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
		readFile:   readFile,
		physical:   make(map[string]*physicalSource),
		logical:    make(map[logicalSourceKey]*logicalSource),
		occurrence: make(map[sourceOccurrenceKey]WorkspaceSource),
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

// Stats returns a consistent snapshot of source-arena work.
func (s *WorkspaceSources) Stats() WorkspaceSourceStats {
	if s == nil {
		return WorkspaceSourceStats{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return WorkspaceSourceStats{
		PhysicalReadAttempts: s.physicalReadAttempts,
		PhysicalSources:      uint64(len(s.physical)),
		LogicalViews:         uint64(len(s.logical)),
		Occurrences:          uint64(len(s.all)),
	}
}

func (s *WorkspaceSources) sourceDigestSet() *SourceDigestSet {
	digests := &SourceDigestSet{
		physical:  make(map[string][sha256.Size]byte),
		requested: make(map[string]string),
		absolute:  make(map[string]string),
	}
	if s == nil {
		return digests
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	digests.physical = make(map[string][sha256.Size]byte, len(s.physical))
	for path, source := range s.physical {
		if source.err != nil {
			continue
		}
		digests.physical[path] = source.digest
		digests.absolute[cleanedAbsolutePath(path)] = path
	}
	digests.requested = make(map[string]string, len(s.all))
	for _, source := range s.all {
		requestedPath := source.metadata.RequestedPath
		physicalPath := source.metadata.PhysicalPath
		digests.requested[requestedPath] = physicalPath
		digests.absolute[cleanedAbsolutePath(requestedPath)] = physicalPath
		digests.absolute[cleanedAbsolutePath(physicalPath)] = physicalPath
	}
	return digests
}

func (s *WorkspaceSources) record(source WorkspaceSource) {
	s.mu.Lock()
	s.all = append(s.all, source)
	s.occurrence[sourceOccurrenceKeyForMetadata(source.metadata)] = source
	s.mu.Unlock()
}

func (s *WorkspaceSources) sourceForMetadata(metadata SourceMetadata) (WorkspaceSource, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	source, ok := s.occurrence[sourceOccurrenceKeyForMetadata(metadata)]
	return source, ok
}

func sourceOccurrenceKeyForMetadata(metadata SourceMetadata) sourceOccurrenceKey {
	return sourceOccurrenceKey{
		requestedPath:    metadata.RequestedPath,
		root:             metadata.Root,
		namespace:        metadata.Namespace,
		version:          metadata.Version,
		dependency:       metadata.Dependency,
		remapFingerprint: sourceRemapFingerprint(metadata.NamespaceRemaps),
	}
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

	s.mu.Lock()
	s.physicalReadAttempts++
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
	abs := cleanedAbsolutePath(path)
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

func cleanedAbsolutePath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	return filepath.Clean(abs)
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
