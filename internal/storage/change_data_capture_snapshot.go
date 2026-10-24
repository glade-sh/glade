package storage

// ChangeDataCaptureEntry preserves the mutation and its submitting user across
// the test runner's setup-to-method boundary.
type ChangeDataCaptureEntry struct {
	Operation     string
	Before, After Record
	User          string
}

// ChangeDataCaptureSnapshot is immutable. Runtime org clones may share it;
// each test method receives its own journal and record maps from Entries.
type ChangeDataCaptureSnapshot struct {
	entries []ChangeDataCaptureEntry
}

func NewChangeDataCaptureSnapshot(entries []ChangeDataCaptureEntry) *ChangeDataCaptureSnapshot {
	if len(entries) == 0 {
		return nil
	}
	return &ChangeDataCaptureSnapshot{entries: cloneChangeDataCaptureEntries(entries)}
}

func (s *ChangeDataCaptureSnapshot) Entries() []ChangeDataCaptureEntry {
	if s == nil {
		return nil
	}
	return cloneChangeDataCaptureEntries(s.entries)
}

func cloneChangeDataCaptureEntries(entries []ChangeDataCaptureEntry) []ChangeDataCaptureEntry {
	out := append([]ChangeDataCaptureEntry(nil), entries...)
	for i := range out {
		out[i].Before = out[i].Before.Clone()
		out[i].After = out[i].After.Clone()
	}
	return out
}

// Setup journals belong to one run and never enter the persisted runtime cache.
func (ChangeDataCaptureSnapshot) GobEncode() ([]byte, error) { return []byte{}, nil }

func (s *ChangeDataCaptureSnapshot) GobDecode([]byte) error {
	s.entries = nil
	return nil
}
