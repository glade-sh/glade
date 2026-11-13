package typesys

import (
	"crypto/sha256"
	"errors"
	"os"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

// ErrVolatileGeneration identifies a generation containing editor content.
var ErrVolatileGeneration = errors.New("generation contains unsaved editor content")

// SourceOverlay is an immutable collection of in-memory source files.
type SourceOverlay struct {
	texts             map[string]string
	digests           map[string][sha256.Size]byte
	physical          map[string]string
	requestedPhysical map[string]string
	ambiguous         map[string]bool
}

func NewSourceOverlay(texts map[string]string) *SourceOverlay {
	o := &SourceOverlay{texts: make(map[string]string, len(texts)), digests: make(map[string][sha256.Size]byte, len(texts)), physical: make(map[string]string, len(texts)), requestedPhysical: make(map[string]string, len(texts)), ambiguous: make(map[string]bool)}
	for path, content := range texts {
		key := cleanedAbsolutePath(path)
		o.texts[key] = content
		physical := cleanedAbsolutePath(canonicalPhysicalPath(path))
		o.requestedPhysical[key] = physical
		if previous, ok := o.physical[physical]; ok && previous != content {
			o.ambiguous[physical] = true
		}
		o.physical[physical] = content
		o.digests[key] = sha256.Sum256([]byte(content))
	}
	return o
}
func (o *SourceOverlay) ReadFile(path string) ([]byte, error) {
	if o != nil {
		key := cleanedAbsolutePath(path)
		physical := o.requestedPhysical[key]
		if physical == "" {
			physical = key
		}
		if o.ambiguous[physical] {
			return nil, errors.New("conflicting overlay aliases")
		}
		if value, ok := o.texts[key]; ok {
			return []byte(value), nil
		}
		if value, ok := o.physical[cleanedAbsolutePath(path)]; ok {
			return []byte(value), nil
		}
	}
	return os.ReadFile(path)
}
func (o *SourceOverlay) Has(path string) bool {
	if o == nil {
		return false
	}
	_, ok := o.texts[cleanedAbsolutePath(path)]
	return ok
}
func (o *SourceOverlay) Digest(path string) ([sha256.Size]byte, bool) {
	if o == nil {
		return [sha256.Size]byte{}, false
	}
	value, ok := o.digests[cleanedAbsolutePath(path)]
	return value, ok
}
func (o *SourceOverlay) Len() int {
	if o == nil {
		return 0
	}
	return len(o.texts)
}
func (idx Index) Volatile() bool        { return idx.sourceDigests != nil && idx.sourceDigests.volatile }
func (a BuildArtifacts) Volatile() bool { return a.SourceDigests != nil && a.SourceDigests.volatile }

func TryUpdateApexFileWithOverlay(previous Index, changedPath string, p project.Project, overlay *SourceOverlay) (Index, bool) {
	ops := incrementalIdentityOpsWithDefaults(incrementalFileIdentityOps{readSource: overlay.ReadFile})
	idx, ok := updateApexFilesIncrementalWithLoadedProject(previous, []string{changedPath}, nil, ops, p)
	if ok && overlay.Has(changedPath) {
		idx.sourceDigests.volatile = true
	}
	return idx, ok
}

func BuildWithArtifactsOverlay(p project.Project, s schema.Schema, overlay *SourceOverlay) (Index, BuildArtifacts) {
	sources := newWorkspaceSources(overlay.ReadFile)
	sources.volatile = overlay.Len() > 0
	return buildWithWorkspaceSources(p, s, sources)
}
