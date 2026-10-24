package apexlang

import (
	"strings"
	"sync"
)

const (
	lowerNameLimit    = 32 * 1024
	lowerNameMaxBytes = 256
)

var lowerNames = lowerNameMemo{limit: lowerNameLimit}

// LowerName reuses lowercase lookup keys for repeated identifier spellings.
// It has exactly strings.ToLower's behavior, including for Unicode and invalid
// UTF-8; it does not implement strings.EqualFold or trim whitespace. Callers
// keep the original spelling for diagnostics, reflection and returned names.
func LowerName(name string) string {
	return lowerNames.lower(name)
}

// Only changed spellings are retained. Both entry count and string size are
// bounded, and copied keys cannot retain a source file through a substring.
// Once full, the memo still serves existing entries and lowers misses normally.
type lowerNameMemo struct {
	names sync.Map
	mu    sync.Mutex
	count int
	limit int
}

func (m *lowerNameMemo) lower(name string) string {
	if name == "" || len(name) > lowerNameMaxBytes {
		return strings.ToLower(name)
	}
	if cached, ok := m.names.Load(name); ok {
		return cached.(string)
	}
	lower := strings.ToLower(name)
	if lower == name || len(lower) > lowerNameMaxBytes {
		return lower
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if cached, ok := m.names.Load(name); ok {
		return cached.(string)
	}
	if m.count < m.limit {
		m.names.Store(strings.Clone(name), lower)
		m.count++
	}
	return lower
}
