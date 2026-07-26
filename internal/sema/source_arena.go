package sema

import (
	"os"

	"github.com/glade-sh/glade/internal/namespaceremap"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/typesys"
)

func semaSourceCacheKey(path, namespace string, remaps []namespaceremap.Rule) string {
	return path + "\x00" + namespace + "\x00" + namespaceremap.Fingerprint(remaps)
}

// readSemaSourceForType remains for focused internal benchmarks. Production
// analysis routes every phase through semaSources.
func readSemaSourceForType(typ typesys.TypeSymbol, cache map[string]string) (string, bool) {
	key := semaSourceCacheKey(typ.File, typ.Namespace, typ.SourceNamespaceRemaps)
	if source, ok := cache[key]; ok {
		return source, true
	}
	resolver := newSemaSources(nil, nil)
	text, ok := resolver.normalizedForType(typ)
	if ok {
		cache[key] = text
	}
	return text, ok
}

type semaSourceText struct {
	raw        string
	normalized string
}

type semaSourceResult struct {
	text semaSourceText
	ok   bool
}

// semaSources is one analysis-local resolver for every source-backed sema
// phase. An attached build artifact is authoritative: a miss never falls back
// to the filesystem.
type semaSources struct {
	artifacts *typesys.BuildArtifacts
	fallback  map[string]semaSourceResult
	recorder  *perfRecorder
}

func newSemaSources(artifacts *typesys.BuildArtifacts, recorder *perfRecorder) *semaSources {
	return &semaSources{
		artifacts: artifacts,
		fallback:  make(map[string]semaSourceResult),
		recorder:  recorder,
	}
}

func (s *semaSources) normalizedForType(typ typesys.TypeSymbol) (string, bool) {
	text, ok := s.forType(typ)
	return text.normalized, ok
}

func (s *semaSources) rawForType(typ typesys.TypeSymbol) (string, bool) {
	text, ok := s.forType(typ)
	return text.raw, ok
}

func (s *semaSources) normalizedForTrigger(trigger typesys.TriggerSymbol) (string, bool) {
	text, ok := s.forOccurrence(trigger.File, trigger.Namespace, trigger.SourceNamespaceRemaps, func() (typesys.WorkspaceSource, bool) {
		return s.artifacts.SourceForTrigger(trigger)
	})
	return text.normalized, ok
}

func (s *semaSources) forType(typ typesys.TypeSymbol) (semaSourceText, bool) {
	return s.forOccurrence(typ.File, typ.Namespace, typ.SourceNamespaceRemaps, func() (typesys.WorkspaceSource, bool) {
		return s.artifacts.SourceForType(typ)
	})
}

func (s *semaSources) forOccurrence(file, namespace string, remaps []namespaceremap.Rule, fromArtifacts func() (typesys.WorkspaceSource, bool)) (semaSourceText, bool) {
	if file == "" {
		s.miss()
		return semaSourceText{}, false
	}
	if s.artifacts != nil {
		source, ok := fromArtifacts()
		if !ok {
			s.miss()
			return semaSourceText{}, false
		}
		s.hit()
		return semaSourceText{raw: source.RawString(), normalized: source.NormalizedString()}, true
	}
	key := semaSourceCacheKey(file, namespace, remaps)
	if result, resolved := s.fallback[key]; resolved {
		if result.ok {
			s.hit()
		} else {
			s.miss()
		}
		return result.text, result.ok
	}
	s.miss()
	if s.recorder != nil {
		s.recorder.counters.SourceArenaFallbackReads++
	}
	data, err := os.ReadFile(file) // #nosec G304 -- file is an indexed project source path carried by the analyzed type or trigger occurrence.
	if err != nil {
		s.fallback[key] = semaSourceResult{}
		return semaSourceText{}, false
	}
	raw := string(data)
	normalized := project.NormalizeApexNamespaceTokens(raw, namespace)
	if len(remaps) > 0 {
		normalized = namespaceremap.ApplySource(remaps, normalized)
	}
	text := semaSourceText{raw: raw, normalized: normalized}
	s.fallback[key] = semaSourceResult{text: text, ok: true}
	return text, true
}

func (s *semaSources) hit() {
	if s != nil && s.recorder != nil {
		s.recorder.counters.SourceArenaHits++
	}
}

func (s *semaSources) miss() {
	if s != nil && s.recorder != nil {
		s.recorder.counters.SourceArenaMisses++
	}
}
