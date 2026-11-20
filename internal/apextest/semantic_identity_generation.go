package apextest

import (
	"context"
	"encoding/json"
	"sort"
	"sync"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/semanticcache"
	"github.com/glade-sh/glade/internal/typesys"
)

// SemanticIdentityGeneration binds memoized identities to one immutable daemon
// index and its matching artifacts. Publication of a new generation creates a
// new owner. The last subset is retained separately from the complete view.
type SemanticIdentityGeneration struct {
	index        typesys.Index
	artifacts    typesys.BuildArtifacts
	mu           sync.Mutex
	full, subset semanticIdentityEntry
}

type semanticIdentityEntry struct {
	key      string
	identity semanticcache.Identity
	valid    bool
}

type semanticIdentityGenerationKey struct{}

func NewSemanticIdentityGeneration(index typesys.Index, artifacts typesys.BuildArtifacts) *SemanticIdentityGeneration {
	return &SemanticIdentityGeneration{index: index, artifacts: artifacts}
}

func WithSemanticIdentityGeneration(ctx context.Context, generation *SemanticIdentityGeneration) context.Context {
	return context.WithValue(ctx, semanticIdentityGenerationKey{}, generation)
}

func (generation *SemanticIdentityGeneration) identity(options sema.AnalyzeOptions, subset bool, excluded map[string]bool) (semanticcache.Identity, error) {
	keys := make([]string, 0, len(excluded))
	if subset {
		for key, yes := range excluded {
			if yes {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)
	}
	keyBytes, _ := json.Marshal(struct {
		Options  string
		Excluded []string
	}{sema.AnalyzeOptionsFingerprint(options), keys})
	key := string(keyBytes)
	generation.mu.Lock()
	defer generation.mu.Unlock()
	entry := &generation.full
	if subset {
		entry = &generation.subset
	}
	if entry.valid && entry.key == key {
		return entry.identity, nil
	}
	index := generation.index
	if subset {
		index = filterCompileGateIndex(index, excluded)
	}
	identity, err := semanticcache.IdentityForBuild(index, &generation.artifacts, options)
	if err == nil {
		*entry = semanticIdentityEntry{key: key, identity: identity, valid: true}
	}
	return identity, err
}
