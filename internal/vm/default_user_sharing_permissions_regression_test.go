package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestDefaultOrgUserSharingPermissions(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	machine := New(nil)
	machine.SetOrg(&org)
	if !machine.currentUserBypassesRecordSharingForWrite() {
		t.Fatal("default administrator must retain its write permissions")
	}
	if !machine.currentUserBypassesRecordSharing() {
		t.Fatal("default administrator must retain its read permissions")
	}
	machine.SetCurrentUser(storage.Record{ID: "005000000000123", Object: "User"})
	if machine.currentUserBypassesRecordSharingForWrite() || machine.currentUserBypassesRecordSharing() {
		t.Fatal("explicit unprivileged user inherited default administrator permissions")
	}
}
