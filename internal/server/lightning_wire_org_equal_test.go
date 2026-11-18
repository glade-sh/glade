package server

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestOrgStateEqualCloneAndWrite(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	clone := org.Clone()
	if !orgStateEqual(clone, org) {
		t.Fatalf("clone differs from its source")
	}
	if !clone.PersistedFieldsIdentical(org) {
		t.Fatalf("structural check missed an untouched clone")
	}

	changed := org.Clone()
	user := changed.Objects["User"]
	for id, record := range user.Records {
		record.Fields["LastName"] = storage.StringValue("Changed")
		user.Records[id] = record
		break
	}
	changed.Objects["User"] = user
	if orgStateEqual(changed, org) {
		t.Fatalf("record write went undetected")
	}

	renamed := org.Clone()
	renamed.OrgID += "x"
	if orgStateEqual(renamed, org) {
		t.Fatalf("org id change went undetected")
	}
}

func TestOrgStateEqualProjectOrgStructural(t *testing.T) {
	handler := lightningWireBenchmarkServer(t)
	clone := handler.Org.Clone()
	if !clone.PersistedFieldsIdentical(*handler.Org) {
		t.Fatalf("structural check missed an untouched project org clone")
	}
}
