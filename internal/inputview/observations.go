package inputview

import (
	"encoding/hex"
	"io/fs"
	"sort"
)

// Observation is one pinned observation in a form a cache entry can persist.
// A later process re-observes the same path with Verify.
type Observation struct {
	Path     string `json:"path"`
	Kind     string `json:"kind"`
	Exists   bool   `json:"exists,omitempty"`
	Mode     uint32 `json:"mode,omitempty"`
	Size     int64  `json:"size,omitempty"`
	ModTime  int64  `json:"modTime,omitempty"`
	SHA256   string `json:"sha256,omitempty"`
	Listing  string `json:"listing,omitempty"`
	Resolved string `json:"resolved,omitempty"`
	Err      string `json:"err,omitempty"`
}

func exportObservation(path string, o observation) Observation {
	out := Observation{Path: path, Kind: o.kind, Exists: o.exists, Mode: uint32(o.mode), Resolved: o.resolved, Err: o.errClass}
	switch o.kind {
	case "file":
		if o.exists && o.errClass == "" {
			out.Size = o.fingerprint.Size
			out.ModTime = o.fingerprint.ModTime.UnixNano()
			out.SHA256 = hex.EncodeToString(o.fingerprint.SHA256[:])
		}
	case "dir", "dir-visible", "dir-root":
		if o.exists {
			out.Listing = hex.EncodeToString(o.listing[:])
		}
	}
	return out
}

// Observations lists every pinned observation, sorted by path and kind.
func (v *View) Observations() []Observation {
	if v == nil {
		return nil
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	out := make([]Observation, 0, len(v.pins))
	for path, kinds := range v.pins {
		for _, o := range kinds {
			out = append(out, exportObservation(path, o))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

// Verify re-observes persisted observations on disk and returns a
// *ChangedError for the first one that no longer holds. File observations
// compare existence, type, size and content, not the modification time, so a
// touch alone does not invalidate. skip, when set, names observations the
// caller has already verified another way.
func Verify(observations []Observation, skip func(Observation) bool) error {
	for _, want := range observations {
		if skip != nil && skip(want) {
			continue
		}
		var o observation
		switch want.Kind {
		case "file":
			_, o, _ = fileObservation(want.Path)
		case "dir", "dir-visible", "dir-root":
			_, o, _ = dirObservation(want.Path, want.Kind)
		case "stat":
			_, o, _ = statObservation(want.Path, false)
		case "lstat":
			_, o, _ = statObservation(want.Path, true)
		case "symlink":
			_, o, _ = symlinkObservation(want.Path)
		default:
			return &ChangedError{Path: want.Path, What: "unknown observation"}
		}
		got := exportObservation(want.Path, o)
		if want.Kind == "file" {
			got.ModTime = want.ModTime
		}
		if got != want {
			old := observation{kind: want.Kind, exists: want.Exists, mode: fs.FileMode(want.Mode)}
			return &ChangedError{Path: want.Path, What: changedWhat(old, o)}
		}
	}
	return nil
}
