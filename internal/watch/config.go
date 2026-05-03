package watch

import "time"

const DefaultDebounce = 200 * time.Millisecond

type Backend string

const (
	BackendAuto   Backend = "auto"
	BackendNative Backend = "native"
	BackendPoll   Backend = "poll"
)

type Config struct {
	Root     string        `json:"root"`
	Debounce time.Duration `json:"-"`
	Backend  Backend       `json:"backend,omitempty"`
}

type ConfigSnapshot struct {
	Root           string `json:"root"`
	DebounceMillis int64  `json:"debounceMillis"`
	Backend        string `json:"backend,omitempty"`
}

func (c Config) Normalized() Config {
	if c.Debounce <= 0 {
		c.Debounce = DefaultDebounce
	}
	if c.Backend == "" {
		c.Backend = BackendAuto
	}
	return c
}

func (c Config) Snapshot() ConfigSnapshot {
	normalized := c.Normalized()
	return ConfigSnapshot{
		Root:           normalized.Root,
		DebounceMillis: normalized.Debounce.Milliseconds(),
		Backend:        string(normalized.Backend),
	}
}
