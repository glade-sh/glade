package typesys

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
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
	Sources            *WorkspaceSources
	SourceDigests      *SourceDigestSet
	ApexMetadataInputs map[string]ApexMetadataInput
}

// ApexMetadataInput records the presence and raw digest of the companion
// metadata file that determined an Apex source's effective API version.
// Present=false is meaningful: adding a sidecar after the build changes the
// generation just as editing one does.
type ApexMetadataInput struct {
	Present bool
	Digest  [sha256.Size]byte
}

// SourceDigestSet is an immutable, compact snapshot of the exact raw source
// digests used by one type-index build.
type SourceDigestSet struct {
	physical  map[string][sha256.Size]byte
	requested map[string]string
	absolute  map[string]string
}

// SourceSnapshotMismatchError reports that a source generation changed after
// an index captured its source and companion metadata inputs.
type SourceSnapshotMismatchError struct {
	File           string
	ExpectedSHA256 string
	ActualSHA256   string
	Cause          error
}

func (e *SourceSnapshotMismatchError) Error() string {
	if e == nil {
		return "source snapshot mismatch"
	}
	message := fmt.Sprintf("source snapshot mismatch for %s: expected sha256 %s", e.File, e.ExpectedSHA256)
	if e.ActualSHA256 != "" {
		return message + ", got " + e.ActualSHA256
	}
	if e.Cause != nil {
		return message + ": " + e.Cause.Error()
	}
	return message
}

func (e *SourceSnapshotMismatchError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// ValidateBuildGeneration proves that every source and Apex metadata input
// retained in artifacts still matches the physical project before a consumer
// uses the index for semantic analysis or cache publication.
func ValidateBuildGeneration(index Index, artifacts *BuildArtifacts) error {
	if artifacts == nil {
		return incompleteSourceSnapshotError("build artifacts are missing")
	}
	if artifacts.Sources == nil {
		return incompleteSourceSnapshotError("build artifacts are missing sources")
	}
	if artifacts.SourceDigests == nil {
		return incompleteSourceSnapshotError("build artifacts are missing digests")
	}
	if artifacts.ApexMetadataInputs == nil {
		return incompleteSourceSnapshotError("build artifacts are missing Apex metadata inputs")
	}
	seen := make(map[sourceOccurrenceKey]bool)
	validateType := func(typ TypeSymbol) error {
		if !typ.HasSourceSnapshot() {
			return nil
		}
		metadata := SourceMetadata{RequestedPath: typ.File, Root: typ.SourceRoot, Namespace: typ.Namespace, Version: typ.Version, Dependency: typ.Dependency, NamespaceRemaps: typ.SourceNamespaceRemaps}
		key := sourceOccurrenceKeyForMetadata(metadata)
		if seen[key] {
			return nil
		}
		seen[key] = true
		source, ok := artifacts.SourceForType(typ)
		if !ok {
			return incompleteSourceSnapshotError("missing type source " + typ.File)
		}
		if err := validateSourceDigest(typ.File, source.Digest(), artifacts.SourceDigests); err != nil {
			return err
		}
		input, ok := artifacts.ApexMetadataForType(typ)
		if !ok {
			return incompleteSourceSnapshotError("missing Apex metadata input " + typ.File + "-meta.xml")
		}
		return validateLiveSourceAndMetadata(typ.File, source.Digest(), input)
	}
	for _, typ := range index.Types {
		if err := validateType(typ); err != nil {
			return err
		}
	}
	for _, trigger := range index.Triggers {
		if !trigger.HasSourceSnapshot() {
			continue
		}
		typ := TypeSymbol{File: trigger.File, SourceRoot: trigger.SourceRoot, Namespace: trigger.Namespace, Version: trigger.Version, Dependency: trigger.Dependency, SourceNamespaceRemaps: trigger.SourceNamespaceRemaps, SourceBacked: true}
		if err := validateType(typ); err != nil {
			return err
		}
	}
	return nil
}

// RefreshBuildArtifacts builds a new immutable source arena for index. It
// reuses an earlier source only when the full logical occurrence, physical
// alias, and raw digest still match index. Every other occurrence is read
// again, so callers never pair an incremental index with a stale source view.
func RefreshBuildArtifacts(index Index, previous *BuildArtifacts) (BuildArtifacts, error) {
	if index.sourceDigests == nil || index.apexMetadataInputs == nil {
		return BuildArtifacts{}, incompleteSourceSnapshotError("index generation is incomplete")
	}
	artifacts := BuildArtifacts{Sources: NewWorkspaceSources()}
	seen := make(map[sourceOccurrenceKey]bool)
	refresh := func(metadata SourceMetadata) error {
		key := sourceOccurrenceKeyForMetadata(metadata)
		if seen[key] {
			return nil
		}
		seen[key] = true
		input, ok := index.apexMetadataInputs[key]
		if !ok {
			return incompleteSourceSnapshotError("missing Apex metadata input " + metadata.RequestedPath + "-meta.xml")
		}
		expected, ok := index.sourceDigests.Digest(metadata.RequestedPath)
		if !ok {
			return incompleteSourceSnapshotError("missing digest for " + metadata.RequestedPath)
		}
		currentPhysicalPath := canonicalPhysicalPath(metadata.RequestedPath)
		if previous != nil && previous.Sources != nil {
			if source, ok := previous.Sources.sourceForMetadata(metadata); ok && source.Digest() == expected && source.metadata.PhysicalPath == index.sourceDigests.requested[metadata.RequestedPath] && source.metadata.PhysicalPath == currentPhysicalPath {
				artifacts.Sources.adopt(source, input)
				return nil
			}
		}
		source, err := artifacts.Sources.load(metadata)
		if err != nil {
			return err
		}
		if source.Digest() != expected {
			actual := source.Digest()
			return sourceSnapshotMismatch(metadata.RequestedPath, expected, &actual, errors.New("source changed after index capture"))
		}
		artifacts.Sources.record(source)
		artifacts.Sources.recordApexMetadata(source, input)
		return nil
	}
	for _, typ := range index.Types {
		if !typ.HasSourceSnapshot() {
			continue
		}
		if err := refresh(SourceMetadata{RequestedPath: typ.File, Root: typ.SourceRoot, Namespace: typ.Namespace, Version: typ.Version, Dependency: typ.Dependency, NamespaceRemaps: typ.SourceNamespaceRemaps}); err != nil {
			return BuildArtifacts{}, err
		}
	}
	for _, trigger := range index.Triggers {
		if !trigger.HasSourceSnapshot() {
			continue
		}
		if err := refresh(SourceMetadata{RequestedPath: trigger.File, Root: trigger.SourceRoot, Namespace: trigger.Namespace, Version: trigger.Version, Dependency: trigger.Dependency, NamespaceRemaps: trigger.SourceNamespaceRemaps}); err != nil {
			return BuildArtifacts{}, err
		}
	}
	artifacts.SourceDigests = artifacts.Sources.sourceDigestSet()
	artifacts.ApexMetadataInputs = capturedApexMetadataInputs(index, artifacts.Sources)
	if err := ValidateBuildGeneration(index, &artifacts); err != nil {
		return BuildArtifacts{}, err
	}
	return artifacts, nil
}

// ValidateSourceGeneration validates an index against an explicit digest set.
// A nil digest set is the documented legacy live-read mode. Any nonnil set
// must contain every source-backed occurrence in index.
func ValidateSourceGeneration(index Index, digests *SourceDigestSet) error {
	if digests == nil {
		return nil
	}
	seen := make(map[sourceOccurrenceKey]bool)
	validate := func(typ TypeSymbol) error {
		if !typ.HasSourceSnapshot() {
			return nil
		}
		metadata := SourceMetadata{RequestedPath: typ.File, Root: typ.SourceRoot, Namespace: typ.Namespace, Version: typ.Version, Dependency: typ.Dependency, NamespaceRemaps: typ.SourceNamespaceRemaps}
		key := sourceOccurrenceKeyForMetadata(metadata)
		if seen[key] {
			return nil
		}
		seen[key] = true
		expected, ok := digests.Digest(typ.File)
		if !ok {
			return incompleteSourceSnapshotError("missing digest for " + typ.File)
		}
		input, ok := index.ApexMetadataForType(typ)
		if !ok {
			return incompleteSourceSnapshotError("missing Apex metadata input " + typ.File + "-meta.xml")
		}
		return validateLiveSourceAndMetadata(typ.File, expected, input)
	}
	for _, typ := range index.Types {
		if err := validate(typ); err != nil {
			return err
		}
	}
	for _, trigger := range index.Triggers {
		if !trigger.HasSourceSnapshot() {
			continue
		}
		typ := TypeSymbol{File: trigger.File, SourceRoot: trigger.SourceRoot, Namespace: trigger.Namespace, Version: trigger.Version, Dependency: trigger.Dependency, SourceNamespaceRemaps: trigger.SourceNamespaceRemaps, SourceBacked: true}
		if err := validate(typ); err != nil {
			return err
		}
	}
	return nil
}

func validateSourceDigest(file string, actual [sha256.Size]byte, digests *SourceDigestSet) error {
	expected, ok := digests.Digest(file)
	if !ok {
		return incompleteSourceSnapshotError("missing digest for " + file)
	}
	if expected == actual {
		return nil
	}
	return sourceSnapshotMismatch(file, expected, &actual, errors.New("source snapshot digest does not match its captured source"))
}

func validateLiveSourceAndMetadata(file string, expected [sha256.Size]byte, metadata ApexMetadataInput) error {
	data, err := os.ReadFile(file) // #nosec G304 -- file is bound to an immutable source occurrence.
	if err != nil {
		return sourceSnapshotMismatch(file, expected, nil, err)
	}
	actual := sha256.Sum256(data)
	if actual != expected {
		return sourceSnapshotMismatch(file, expected, &actual, nil)
	}
	return validateApexMetadataGeneration(file+"-meta.xml", metadata)
}

func validateApexMetadataGeneration(path string, expected ApexMetadataInput) error {
	data, err := os.ReadFile(path) // #nosec G304 -- fixed sidecar of an indexed Apex source.
	if err != nil {
		if os.IsNotExist(err) && !expected.Present {
			return nil
		}
		return sourceSnapshotMismatch(path, expected.Digest, nil, err)
	}
	actual := sha256.Sum256(data)
	if !expected.Present || actual != expected.Digest {
		return sourceSnapshotMismatch(path, expected.Digest, &actual, nil)
	}
	return nil
}

func sourceSnapshotMismatch(file string, expected [sha256.Size]byte, actual *[sha256.Size]byte, cause error) error {
	mismatch := &SourceSnapshotMismatchError{File: file, ExpectedSHA256: hex.EncodeToString(expected[:]), Cause: cause}
	if actual != nil {
		mismatch.ActualSHA256 = hex.EncodeToString(actual[:])
	}
	return mismatch
}

func incompleteSourceSnapshotError(reason string) error {
	return &SourceSnapshotMismatchError{File: "build artifacts", Cause: errors.New("source snapshot is incomplete: " + reason)}
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

// ApexMetadataForType returns the exact companion metadata identity captured
// with typ's logical source occurrence.
func (a BuildArtifacts) ApexMetadataForType(typ TypeSymbol) (ApexMetadataInput, bool) {
	if a.Sources == nil {
		return ApexMetadataInput{}, false
	}
	return a.Sources.apexMetadataForMetadata(SourceMetadata{
		RequestedPath: typ.File, Root: typ.SourceRoot, Namespace: typ.Namespace,
		Version: typ.Version, Dependency: typ.Dependency, NamespaceRemaps: typ.SourceNamespaceRemaps,
	})
}

// ApexMetadataForTrigger returns the exact companion metadata identity
// captured with trigger's logical source occurrence.
func (a BuildArtifacts) ApexMetadataForTrigger(trigger TriggerSymbol) (ApexMetadataInput, bool) {
	if a.Sources == nil {
		return ApexMetadataInput{}, false
	}
	return a.Sources.apexMetadataForMetadata(SourceMetadata{
		RequestedPath: trigger.File, Root: trigger.SourceRoot, Namespace: trigger.Namespace,
		Version: trigger.Version, Dependency: trigger.Dependency, NamespaceRemaps: trigger.SourceNamespaceRemaps,
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
	apexMetadata         map[sourceOccurrenceKey]ApexMetadataInput
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
		readFile:     readFile,
		physical:     make(map[string]*physicalSource),
		logical:      make(map[logicalSourceKey]*logicalSource),
		occurrence:   make(map[sourceOccurrenceKey]WorkspaceSource),
		apexMetadata: make(map[sourceOccurrenceKey]ApexMetadataInput),
	}
}

func (s *WorkspaceSources) recordApexMetadata(source WorkspaceSource, input ApexMetadataInput) {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.apexMetadata == nil {
		s.apexMetadata = make(map[sourceOccurrenceKey]ApexMetadataInput)
	}
	s.apexMetadata[sourceOccurrenceKeyForMetadata(source.metadata)] = input
	s.mu.Unlock()
}

func (s *WorkspaceSources) apexMetadataForMetadata(metadata SourceMetadata) (ApexMetadataInput, bool) {
	if s == nil {
		return ApexMetadataInput{}, false
	}
	s.mu.Lock()
	input, ok := s.apexMetadata[sourceOccurrenceKeyForMetadata(metadata)]
	s.mu.Unlock()
	return input, ok
}

func (s *WorkspaceSources) apexMetadataInputSet() map[sourceOccurrenceKey]ApexMetadataInput {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[sourceOccurrenceKey]ApexMetadataInput, len(s.apexMetadata))
	for key, value := range s.apexMetadata {
		out[key] = value
	}
	return out
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

// adopt records a previously captured immutable logical source in this fresh
// arena without reading the physical file again. Callers must have proved the
// occurrence and digest match their target generation.
func (s *WorkspaceSources) adopt(source WorkspaceSource, input ApexMetadataInput) {
	if s == nil {
		return
	}
	physicalPath := source.metadata.PhysicalPath
	if physicalPath == "" {
		physicalPath = canonicalPhysicalPath(source.metadata.RequestedPath)
	}
	logicalKey := logicalSourceKey{
		physicalPath:     physicalPath,
		namespace:        source.metadata.Namespace,
		remapFingerprint: sourceRemapFingerprint(source.metadata.NamespaceRemaps),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.physical[physicalPath]; !ok {
		ready := make(chan struct{})
		close(ready)
		s.physical[physicalPath] = &physicalSource{ready: ready, raw: source.raw, digest: source.digest}
	}
	if _, ok := s.logical[logicalKey]; !ok {
		ready := make(chan struct{})
		close(ready)
		s.logical[logicalKey] = &logicalSource{ready: ready, normalized: source.normalized}
	}
	key := sourceOccurrenceKeyForMetadata(source.metadata)
	s.occurrence[key] = source
	s.apexMetadata[key] = input
	s.all = append(s.all, source)
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
