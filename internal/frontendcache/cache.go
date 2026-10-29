// Package frontendcache restores complete validated frontend generations before
// index construction. A cache failure always falls back to the supplied builder.
package frontendcache

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/semanticcache"
	"github.com/glade-sh/glade/internal/typesys"
)

const maxSnapshotBytes = 64 << 20
const name = "frontend.json"

var publicationFence struct {
	sync.RWMutex
	generation uint64
}

// Invalidate fences publications that began before a cache-clear command.
func Invalidate() {
	publicationFence.Lock()
	publicationFence.generation++
	publicationFence.Unlock()
}

type Result struct {
	Index     typesys.Index
	Artifacts typesys.BuildArtifacts
	Hit       bool
	Reused    bool
}

type envelope struct {
	Version int    `json:"version"`
	Input   string `json:"input"`
	Build   string `json:"build"`

	Checksum string `json:"checksum"`
}

// LoadOrBuild requires freshly discovered membership and schema. Both the
// current content identity and the executable identity must match on a hit.
// It retains no frontend graphs or project aliases between invocations.
func LoadOrBuild(p project.Project, s schema.Schema, build func() (typesys.Index, typesys.BuildArtifacts)) Result {
	publicationFence.RLock()
	generation := publicationFence.generation
	publicationFence.RUnlock()
	var index typesys.Index
	var artifacts typesys.BuildArtifacts
	reused := false
	// A missing snapshot goes straight to the original builder. Do not read
	// every source and sidecar before the builder captures those same bytes.
	// One footer authenticates the envelope and the footerless record stream.
	if data, err := read(p.Root); err == nil && len(data) >= sha256.Size {
		content, digest := data[:len(data)-sha256.Size], data[len(data)-sha256.Size:]
		expected := sha256.Sum256(content)
		if !bytes.Equal(digest, expected[:]) {
			content = nil
		}
		data = content
		if split := bytes.IndexByte(data, '\n'); split >= 0 {
			var saved envelope
			if json.Unmarshal(data[:split], &saved) == nil && saved.Version == typesys.FrontendSnapshotVersion && saved.Checksum == checksum(saved) {
				inputs, err := typesys.CaptureFrontendInputs(p, s)
				if err == nil {
					var restored typesys.Index
					var captured typesys.BuildArtifacts
					if saved.Input == inputs.Identity() {
						restored, captured, err = typesys.RestoreFrontendSnapshotRecordsWithInputs(p, s, data[split+1:], inputs)
					} else {
						restored, captured, err = typesys.UpdateFrontendSnapshotRecords(p, s, data[split+1:], saved.Input)
					}
					if err == nil {
						fingerprint, identityErr := frontendBuildFingerprint(restored, &captured)
						if identityErr == nil && saved.Build == fingerprint {
							if saved.Input == inputs.Identity() {
								return Result{Index: restored, Artifacts: captured, Hit: true}
							}
							index, artifacts, reused = restored, captured, true
						}
					}
				}
			}
		}
	}

	if !reused {
		index, artifacts = build()
	}
	result := Result{Index: index, Artifacts: artifacts, Reused: reused}
	// Publish the captured generation. The next load always supplies fresh
	// membership/schema, so additions, removals and metadata edits cannot match
	// this identity. Rewalking here cannot extend the builder's captured arena.
	inputs, err := typesys.FrontendInputsForBuild(p, s, index, &artifacts)
	if err != nil {
		return result
	}
	fingerprint, err := frontendBuildFingerprint(index, &artifacts)
	if err != nil {
		return result
	}
	saved := envelope{Version: typesys.FrontendSnapshotVersion, Input: inputs.Identity(), Build: fingerprint}
	saved.Checksum = checksum(saved)
	publicationFence.RLock()
	if generation == publicationFence.generation {
		_ = store(p.Root, func(w io.Writer) error {
			hash := sha256.New()
			stream := io.MultiWriter(w, hash)
			if err := json.NewEncoder(stream).Encode(saved); err != nil {
				return err
			}
			if err := typesys.WriteFrontendSnapshotRecords(stream, index, &artifacts); err != nil {
				return err
			}
			_, err := w.Write(hash.Sum(nil))
			return err
		})
	}
	publicationFence.RUnlock()

	return result
}

// Only executable identity is needed here. Preserve the captured private
// project provenance while avoiding an unused full semantic-result digest.
func frontendBuildFingerprint(index typesys.Index, artifacts *typesys.BuildArtifacts) (string, error) {
	index.Types, index.Triggers, index.Objects, index.CustomMetadataRecords = nil, nil, nil, nil
	identity, err := semanticcache.IdentityForBuild(index, artifacts, sema.AnalyzeOptions{})
	return identity.BuildFingerprint, err
}

func checksum(value envelope) string {
	value.Checksum = ""
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

// Open each directory relative to a pinned parent. Reject symlinks and inode
// replacement between lstat/open; no publication traverses project links.
func directory(root string, create bool) (*os.Root, error) {
	current, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	for _, part := range []string{".glade", "semantic"} {
		if create {
			if err := current.Mkdir(part, 0700); err != nil && !errors.Is(err, os.ErrExist) {
				current.Close()
				return nil, err
			}
		}
		before, err := current.Lstat(part)
		if err != nil {
			current.Close()
			return nil, err
		}
		if !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
			current.Close()
			return nil, errors.New("frontend cache directory must be a real directory")
		}
		next, err := current.OpenRoot(part)
		if err != nil {
			current.Close()
			return nil, err
		}
		after, err := next.Stat(".")
		current.Close()
		if err != nil || !os.SameFile(before, after) {
			next.Close()
			return nil, errors.New("frontend cache directory changed")
		}
		current = next
	}
	if create {
		if err := current.Chmod(".", 0700); err != nil {
			current.Close()
			return nil, err
		}
	}
	return current, nil
}

func read(root string) ([]byte, error) {
	file, err := openSnapshot(root)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	after, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !after.Mode().IsRegular() || after.Size() > maxSnapshotBytes {
		return nil, errors.New("invalid frontend cache file")
	}
	data := make([]byte, after.Size())
	if _, err := io.ReadFull(file, data); err != nil {
		return nil, err
	}
	var extra [1]byte
	if n, err := file.Read(extra[:]); n != 0 || err != io.EOF {
		return nil, errors.New("frontend cache changed size")
	}
	return data, nil
}

func store(root string, encode func(io.Writer) error) error {
	dir, err := directory(root, true)
	if err != nil {
		return err
	}
	defer dir.Close()
	temporary := ".snapshot-" + rand.Text() + ".tmp"
	file, err := dir.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer dir.Remove(temporary)
	bounded := &snapshotWriter{writer: bufio.NewWriterSize(file, 64<<10)}
	if err := encode(bounded); err != nil {
		file.Close()
		return err
	}
	if err := bounded.writer.Flush(); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return dir.Rename(temporary, name)
}

// Reject oversized streams before publication, without buffering the payload.
type snapshotWriter struct {
	writer *bufio.Writer
	size   int
}

func (w *snapshotWriter) Write(p []byte) (int, error) {
	if len(p) > maxSnapshotBytes-w.size {
		return 0, errors.New("frontend snapshot exceeds size limit")
	}
	n, err := w.writer.Write(p)
	w.size += n
	return n, err
}
