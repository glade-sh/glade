package watch

import "time"

const DefaultDebounce = 200 * time.Millisecond

type Config struct {
	Root     string        `json:"root"`
	Debounce time.Duration `json:"-"`
}

type ConfigSnapshot struct {
	Root           string `json:"root"`
	DebounceMillis int64  `json:"debounceMillis"`
}

func (c Config) Normalized() Config {
	if c.Debounce <= 0 {
		c.Debounce = DefaultDebounce
	}
	return c
}

func (c Config) Snapshot() ConfigSnapshot {
	normalized := c.Normalized()
	return ConfigSnapshot{
		Root:           normalized.Root,
		DebounceMillis: normalized.Debounce.Milliseconds(),
	}
}
