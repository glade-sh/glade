package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/typesys"
)

const conformanceProjectCacheLimit = 64

// conformanceProjectCache owns immutable project snapshots for one family.
// Identical files reuse both the parsed index and its stable source identity,
// allowing sema's existing anonymous setup cache to hit across rows. Row VMs
// and orgs still come from conformanceRunner's private runtime clones.
// This is opt-in: live-edit/cache-invalidation tests keep fresh project roots.
type conformanceProjectCache struct {
	owner   *testing.T
	mu      sync.Mutex
	entries map[[sha256.Size]byte]typesys.Index
	order   [][sha256.Size]byte
	stats   conformanceProjectStatistics
}

type conformanceProjectStatistics struct {
	Requests, Hits, Builds, Evictions int
	BuildTime                         time.Duration
}

func (c *conformanceProjectCache) statistics() conformanceProjectStatistics {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stats
}

func newConformanceProjectCache(t *testing.T) *conformanceProjectCache {
	return &conformanceProjectCache{owner: t, entries: make(map[[sha256.Size]byte]typesys.Index)}
}

func (c *conformanceProjectCache) index(t *testing.T, api string, files map[string]string) typesys.Index {
	t.Helper()
	if len(files) == 0 {
		files = map[string]string{}
	}
	// encoding/json sorts map keys. Include every path and byte, including
	// companion metadata, not just Apex names or filesystem timestamps.
	raw, err := json.Marshal(struct {
		API   string
		Files map[string]string
	}{api, files})
	if err != nil {
		t.Fatal(err)
	}
	key := sha256.Sum256(raw)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stats.Requests++
	if index, ok := c.entries[key]; ok {
		c.stats.Hits++
		return index
	}
	// The owner outlives every row, including parallel API children. A row's
	// cleanup must never remove files that later snapshot consumers can read.
	started := time.Now()
	root := c.owner.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
	for path, source := range files {
		writeFile(t, filepath.Join(root, path), source)
	}
	index := loadTestIndex(t, root)
	c.stats.Builds++
	c.stats.BuildTime += time.Since(started)
	if len(c.order) == conformanceProjectCacheLimit {
		c.stats.Evictions++
		delete(c.entries, c.order[0])
		c.order = c.order[1:]
	}
	c.order = append(c.order, key)
	c.entries[key] = index
	// Index is an immutable value; nested source/schema data stays frozen.
	return index
}
