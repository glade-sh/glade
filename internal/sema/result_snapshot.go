package sema

import (
	"encoding/json"
	"unsafe"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

// EstimateResultRetainedBytes conservatively accounts for the container and
// object memory retained beside Result's encoded bytes. SnapshotResult trims
// slice capacities to their lengths, so the retained diagnostic capacity is
// exactly the result length. String payload bytes are already counted by the
// encoded-size component used by the cache manager.
func EstimateResultRetainedBytes(result Result) int64 {
	size := int64(unsafe.Sizeof(result))
	size += int64(len(result.Diagnostics)) * int64(unsafe.Sizeof(diagnostic.Diagnostic{}))
	for i := range result.Diagnostics {
		if result.Diagnostics[i].Range != nil {
			size += int64(unsafe.Sizeof(diagnostic.Range{}))
		}
	}
	// Go does not expose map capacity. Two entry-widths plus a small bucket
	// allowance per live entry conservatively covers keys, values, and buckets
	// at the runtime's supported load factor.
	entryWidth := int64(unsafe.Sizeof("")) + int64(unsafe.Sizeof(TypeReference{}))
	size += int64(len(result.Types)) * (2*entryWidth + 16)
	return size
}

// ResultSnapshot is an immutable, complete serialization view of Result.
// Its representation is intentionally opaque so callers cannot mutate shared
// maps or slices. Result returns a deep copy suitable for request-local use.
//
// ResultSnapshot has its own JSON representation because Result.MarshalJSON
// intentionally omits very large exported-type maps from normal CLI output.
type ResultSnapshot struct {
	result Result
}

type resultSnapshotJSON struct {
	Project     typesys.ProjectInfo      `json:"project"`
	Summary     Summary                  `json:"summary"`
	Diagnostics []diagnostic.Diagnostic  `json:"diagnostics"`
	Types       map[string]TypeReference `json:"types"`
}

// SnapshotResult captures all serializable semantic output without retaining
// aliases to request-local result state.
func SnapshotResult(result Result) ResultSnapshot {
	return ResultSnapshot{result: cloneResult(result)}
}

// Result returns a request-local deep copy of the captured semantic output.
func (snapshot ResultSnapshot) Result() Result {
	return cloneResult(snapshot.result)
}

func (snapshot ResultSnapshot) MarshalJSON() ([]byte, error) {
	result := snapshot.result
	return json.Marshal(resultSnapshotJSON{
		Project:     result.Project,
		Summary:     result.Summary,
		Diagnostics: result.Diagnostics,
		Types:       result.Types,
	})
}

func (snapshot *ResultSnapshot) UnmarshalJSON(data []byte) error {
	var decoded resultSnapshotJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	snapshot.result = cloneResult(Result{
		Project:     decoded.Project,
		Summary:     decoded.Summary,
		Diagnostics: decoded.Diagnostics,
		Types:       decoded.Types,
	})
	return nil
}

func cloneResult(result Result) Result {
	cloned := Result{
		Project: result.Project,
		Summary: result.Summary,
	}
	if result.Diagnostics != nil {
		cloned.Diagnostics = make([]diagnostic.Diagnostic, len(result.Diagnostics))
		copy(cloned.Diagnostics, result.Diagnostics)
		for i := range cloned.Diagnostics {
			if result.Diagnostics[i].Range == nil {
				continue
			}
			copiedRange := *result.Diagnostics[i].Range
			cloned.Diagnostics[i].Range = &copiedRange
		}
	}
	if result.Types != nil {
		cloned.Types = make(map[string]TypeReference, len(result.Types))
		for name, reference := range result.Types {
			cloned.Types[name] = reference
		}
	}
	return cloned
}
