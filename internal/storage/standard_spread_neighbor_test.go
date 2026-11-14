package storage

import (
	"crypto/sha256"
	"encoding/json"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestStandardSpreadUncapturedShapeSnapshot(t *testing.T) {
	org := sourceSpreadEagerOrg([]string{project.UnspecifiedStandardFieldShape})
	data, err := json.Marshal(org)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("uncaptured shape sha256: %x", sha256.Sum256(data))
}
