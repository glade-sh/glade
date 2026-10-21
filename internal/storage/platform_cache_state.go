package storage

import "time"

// PlatformCacheState is an immutable, in-memory request snapshot. Org copies
// can share it; updates return a new snapshot and never mutate a prior request.
// It is not serialized as org data or restored across process restarts.
type PlatformCacheState struct {
	strings map[string]map[string]PlatformCacheString
}

// GobEncode keeps OrgState gob-encodable for the startup runtime cache. The
// snapshot is request-scoped, so a persisted org carries no entries.
func (PlatformCacheState) GobEncode() ([]byte, error) { return []byte{}, nil }

// GobDecode restores an empty snapshot; entries never cross a process restart.
func (s *PlatformCacheState) GobDecode([]byte) error {
	s.strings = nil
	return nil
}

type PlatformCacheString struct {
	Text     string
	ExpireAt time.Time
}

func (s *PlatformCacheState) StringEntry(partition, key string) (PlatformCacheString, bool) {
	if s == nil {
		return PlatformCacheString{}, false
	}
	entry, ok := s.strings[partition][key]
	return entry, ok
}

func (s *PlatformCacheState) WithStringEntry(partition, key string, value PlatformCacheString) *PlatformCacheState {
	out := s.copyPartition(partition)
	out.strings[partition][key] = value
	return out
}

func (s *PlatformCacheState) WithoutStringEntry(partition, key string) *PlatformCacheState {
	if _, exists := s.StringEntry(partition, key); !exists {
		return s
	}
	out := s.copyPartition(partition)
	delete(out.strings[partition], key)
	if len(out.strings[partition]) == 0 {
		delete(out.strings, partition)
	}
	if len(out.strings) == 0 {
		return nil
	}
	return out
}

func (s *PlatformCacheState) copyPartition(partition string) *PlatformCacheState {
	out := &PlatformCacheState{strings: map[string]map[string]PlatformCacheString{}}
	if s != nil {
		for name, entries := range s.strings {
			out.strings[name] = entries
		}
	}
	entries := map[string]PlatformCacheString{}
	for key, value := range out.strings[partition] {
		entries[key] = value
	}
	out.strings[partition] = entries
	return out
}
