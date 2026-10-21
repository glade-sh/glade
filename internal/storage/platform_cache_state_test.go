package storage

import (
	"bytes"
	"encoding/gob"
	"testing"
	"time"
)

func TestOrgStateGobRoundTripDropsPlatformCacheSnapshots(t *testing.T) {
	entry := PlatformCacheString{Text: "value", ExpireAt: time.Unix(1, 0).UTC()}
	org := OrgState{
		OrgID:                  "00D000000000mH2",
		Objects:                map[string]ObjectState{},
		PlatformCache:          (*PlatformCacheState)(nil).WithStringEntry("cache.orgpartition:local.P", "k", entry),
		PlatformCacheTestSetup: (*PlatformCacheState)(nil).WithStringEntry("cache.sessionpartition:local.P", "k", entry),
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(org); err != nil {
		t.Fatalf("gob encode OrgState: %v", err)
	}
	var decoded OrgState
	if err := gob.NewDecoder(&buf).Decode(&decoded); err != nil {
		t.Fatalf("gob decode OrgState: %v", err)
	}
	if decoded.OrgID != org.OrgID {
		t.Fatalf("decoded OrgID = %q, want %q", decoded.OrgID, org.OrgID)
	}
	if _, ok := decoded.PlatformCache.StringEntry("cache.orgpartition:local.P", "k"); ok {
		t.Fatal("decoded org restored a request-scoped platform cache entry")
	}
	if _, ok := decoded.PlatformCacheTestSetup.StringEntry("cache.sessionpartition:local.P", "k"); ok {
		t.Fatal("decoded org restored a test-setup platform cache entry")
	}
}
