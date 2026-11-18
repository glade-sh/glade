package storage

import "testing"

// Shape migration treats a reserved bootstrap profile as untouched only when it
// equals its recorded original. The originals must be the values the seeder
// writes, and a profile the seeder no longer creates has no original.
func TestBootstrapProfileOriginalsMatchSeeder(t *testing.T) {
	org := NewOrgState()
	EnsureDeterministicPlatformData(&org)
	profiles := org.Objects["Profile"].Records
	for _, id := range []ID{ID(DefaultProfileID), "00e000000000002", "00e000000000003", "00e000000000004", "00e000000000005", "00e000000000006", "00e000000000008"} {
		record, ok := profiles[id]
		if !ok {
			t.Errorf("%s: seeded profile missing", id)
			continue
		}
		if !untouchedBootstrapProfile(id, record) {
			t.Errorf("%s (%s): freshly seeded profile is not recognized as untouched; seeded=%v original=%v", id, record.Fields["Name"].String, record.Fields, bootstrapProfileSeedFields(id))
		}
	}
	if _, ok := profiles["00e000000000007"]; ok {
		t.Fatal("00e000000000007 is seeded")
	}
	if original := bootstrapProfileSeedFields("00e000000000007"); original != nil {
		t.Errorf("00e000000000007 has a bootstrap original although it is not seeded: %v", original)
	}
}
