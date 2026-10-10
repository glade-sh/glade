package storage

import "testing"

func TestSitesGuestProfilesKeepFeatureAndStandardUserBoundaries(t *testing.T) {
	org := NewOrgState()
	EnsureDeterministicPlatformData(&org)
	find := func(name string) (Record, bool) {
		for _, r := range org.Objects["Profile"].Records {
			if r.Fields["Name"].String == name {
				return r, true
			}
		}
		return Record{}, false
	}
	for _, name := range []string{"Standard Guest", "Guest License User"} {
		if _, ok := find(name); ok {
			t.Fatalf("Sites-only seed appeared without feature: %s", name)
		}
	}
	ApplyOrgShape(&org, []string{"Communities"})
	for _, name := range []string{"Standard Guest", "Guest License User"} {
		if _, ok := find(name); ok {
			t.Fatalf("Sites-only seed changed Communities baseline: %s", name)
		}
	}
	ApplyOrgShape(&org, []string{"Sites"})
	for _, name := range []string{"Standard Guest", "Guest License User"} {
		profile, ok := find(name)
		if !ok || profile.Fields["UserType"].String != "Guest" {
			t.Fatalf("guest profile missing or wrong type: %s %+v", name, profile)
		}
		licenseID := profile.Fields["UserLicenseId"].ID
		if _, ok := org.Objects["UserLicense"].Records[licenseID]; !ok {
			t.Fatalf("missing guest license %s", licenseID)
		}
	}
	profile, _ := find("Standard User")
	if profile.Fields["UserType"].String != "Standard" {
		t.Fatal("ordinary user type changed")
	}
	count := len(org.Objects["Profile"].Records)
	ApplyOrgShape(&org, []string{"Sites"})
	if len(org.Objects["Profile"].Records) != count {
		t.Fatal("repeated Sites application duplicated profiles")
	}
}
