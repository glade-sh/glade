package storage

import (
	"fmt"
	"strings"
	"testing"
)

func TestSeededUserProfileIDsAvoidCounterStyleIDs(t *testing.T) {
	org := NewOrgState()
	EnsureDeterministicPlatformData(&org)
	for _, object := range []string{"User", "Profile"} {
		t.Run(object, func(t *testing.T) {
			name := "System Administrator"
			if object == "User" {
				name = "System User"
			}
			record, ok := findRecordByStringField(org.Objects[object].Records, "Name", name)
			if !ok {
				t.Fatalf("missing seeded %s", object)
			}
			for i := 1; i <= 100; i++ {
				fake := ID(fmt.Sprintf("%s%012d", org.Objects[object].Definition.KeyPrefix, i))
				if IDsEqual(fake, record.ID) {
					t.Errorf("counter-style %s Id %s collides with seeded identity", object, fake)
				}
			}
			id := string(record.ID)
			if len(id) != 18 || displayOrgID(id[:15]) != id || !strings.ContainsAny(id[:15], "ABCDEFGHIJKLMNOPQRSTUVWXYZ") || !strings.ContainsAny(id[:15], "abcdefghijklmnopqrstuvwxyz") {
				t.Errorf("seeded %s Id %q must have mixed case and a valid 18-character checksum", object, id)
			}
		})
	}
}
