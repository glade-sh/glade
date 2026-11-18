package storage

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDuplicateSettingSQLiteRoundTrip(t *testing.T) {
	s, err := OpenSQLite(filepath.Join(t.TempDir(), "org.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	org := NewOrgState()
	for _, want := range []bool{true, false, true} {
		org.StandardDuplicateRules = want
		if err := s.Save(org); err != nil {
			t.Fatal(err)
		}
		loaded, err := s.Load()
		if err != nil {
			t.Fatal(err)
		}
		if loaded.StandardDuplicateRules != want {
			t.Errorf("round trip got %v, want %v", loaded.StandardDuplicateRules, want)
		}
	}
}

func TestLegacySeededDuplicateRuleSQLite(t *testing.T) {
	s, err := OpenSQLite(filepath.Join(t.TempDir(), "org.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	org := NewOrgState()
	EnsureDeterministicPlatformData(&org)
	if err := s.Save(org); err != nil {
		t.Fatal(err)
	}
	// Simulate the pre-setting encoding, which had no seeded property.
	if _, err := s.db.Exec(`update object_definitions set definition_json = replace(cast(definition_json as text), '"seeded":true,', '')`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`delete from org_meta where key = ?`, "glade.org.duplicateRuleOrigins"); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	EnsureDeterministicPlatformData(&loaded)
	for _, rule := range loaded.Objects["Account"].Definition.DuplicateRules {
		if strings.EqualFold(rule.Name, "Standard_Account_Duplicate_Rule") && !rule.Seeded {
			t.Error("legacy built-in rule lost seeded origin")
		}
	}
}

func TestDuplicateRuleSQLitePreservesExplicitProjectOrigin(t *testing.T) {
	s, err := OpenSQLite(filepath.Join(t.TempDir(), "org.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	org := NewOrgState()
	EnsureDeterministicPlatformData(&org)
	state := org.Objects["Account"]
	state.Definition.DuplicateRules[0].Seeded = false
	org.Objects["Account"] = state
	if err := s.Save(org); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Objects["Account"].Definition.DuplicateRules[0].Seeded {
		t.Fatal("explicit project rule classified as a seed")
	}
}
