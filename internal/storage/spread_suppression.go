package storage

import (
	"encoding/hex"
	"encoding/json"
	"sync"
)

// SpreadSuppression is shared by value copies of an org, just as Objects is.
// Runtime clones receive their own registry. Entries are independent snapshots.
type SpreadSuppression struct {
	mu      sync.RWMutex
	entries map[string]string
}

func newSpreadSuppression() *SpreadSuppression {
	return &SpreadSuppression{entries: make(map[string]string)}
}

func (s *SpreadSuppression) snapshot() map[string]string {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]string, len(s.entries))
	for name, fingerprint := range s.entries {
		result[name] = fingerprint
	}
	return result
}

func (s *SpreadSuppression) lookup(name string) (string, bool) {
	if s == nil {
		return "", false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	fingerprint, ok := s.entries[name]
	return fingerprint, ok
}

func (s *SpreadSuppression) put(name, fingerprint string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.entries == nil {
		s.entries = make(map[string]string)
	}
	s.entries[name] = fingerprint
}

func (s *SpreadSuppression) replace(entries map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = entries
}

func (s *SpreadSuppression) MarshalJSON() ([]byte, error) { return json.Marshal(s.snapshot()) }

func (s *SpreadSuppression) UnmarshalJSON(data []byte) error {
	var entries map[string]string
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}
	s.replace(entries)
	return nil
}

func cloneSpreadSuppressed(source *SpreadSuppression) *SpreadSuppression {
	if source == nil {
		return nil
	}
	clone := newSpreadSuppression()
	clone.replace(source.snapshot())
	return clone
}

func (o *OrgState) ensureSpreadSuppression() {
	if o != nil && o.StandardSpreadCapturedPlain && o.StandardSpreadSuppressed == nil {
		o.StandardSpreadSuppressed = newSpreadSuppression()
	}
}

// MarshalJSON rebases untouched fingerprints to the representation JSON will
// restore. Empty collections omitted by JSON otherwise change the fingerprint.
// Edited definitions lose their marker and remain visible after restoration.
func (o OrgState) MarshalJSON() ([]byte, error) {
	type orgStateJSON OrgState
	encoded := orgStateJSON(o)
	if o.StandardSpreadSuppressed != nil {
		entries := o.StandardSpreadSuppressed.snapshot()
		for name, saved := range entries {
			object, exists := o.Objects[name]
			if !exists {
				continue
			}
			current := spreadDefinitionFingerprint(object.Definition)
			if hex.EncodeToString(current[:]) != saved {
				delete(entries, name)
				continue
			}
			data, err := json.Marshal(object.Definition)
			if err != nil {
				return nil, err
			}
			var restored ObjectDefinition
			if err := json.Unmarshal(data, &restored); err != nil {
				return nil, err
			}
			fingerprint := spreadDefinitionFingerprint(restored)
			entries[name] = hex.EncodeToString(fingerprint[:])
		}
		encoded.StandardSpreadSuppressed = newSpreadSuppression()
		encoded.StandardSpreadSuppressed.replace(entries)
	}
	return json.Marshal(encoded)
}

// UnmarshalJSON keeps decoding behavior unchanged and restores the shared
// registry for source-backed states written before the registry existed.
func (o *OrgState) UnmarshalJSON(data []byte) error {
	type orgStateJSON OrgState
	if err := json.Unmarshal(data, (*orgStateJSON)(o)); err != nil {
		if typeErr, ok := err.(*json.UnmarshalTypeError); ok && typeErr.Struct == "orgStateJSON" {
			typeErr.Struct = "OrgState"
		}
		return err
	}
	if o.StandardSpreadCapturedPlain && o.StandardSpread != "" {
		o.ensureSpreadSuppression()
	}
	if o.StandardSpread != "" {
		// Older source states do not persist install provenance. Conservatively
		// retain existing candidates when that provenance is unavailable.
		for name := range o.Objects {
			if plainDeveloperPrunable(name) {
				o.MarkExplicitSpreadObject(name)
			}
		}
	}
	return nil
}
