package vm

import "testing"

// group_user_default_read.json GU04 records these Developer defaults;
// GU01..GU03 exercise reads with all three class sharing declarations.
func TestGroupUserDefaultReadSharing(t *testing.T) {
	for _, object := range []string{"User", "Group", "GroupMember"} {
		for _, model := range []string{"", "Private", "Read"} {
			t.Run(object+"/"+model, func(t *testing.T) {
				machine, _ := defaultSharingWriteFixture(object, model, false)
				if got, want := machine.soqlObjectHasPublicReadSharing(object), model != "Private"; got != want {
					t.Fatalf("public read=%v, want %v", got, want)
				}
			})
		}
	}
}

// The capture measures reads only. Adding a read fallback must not extend
// omitted-model write grants or private custom-object visibility.
func TestGroupUserDefaultReadPreservesWriteAndPrivateBoundaries(t *testing.T) {
	for _, object := range []string{"User", "Group", "GroupMember", "OwnedEntry__c"} {
		t.Run(object, func(t *testing.T) {
			machine, record := defaultSharingWriteFixture(object, "", false)
			if standardObjectDefaultSharingModel(object) != "" {
				t.Fatal("read fallback changed the write model")
			}
			for _, operation := range []string{"update", "upsert", "delete", "undelete"} {
				if machine.currentUserCanWriteRecord(object, record, "005000000000002", operation) {
					t.Fatalf("read fallback granted another owner's %s", operation)
				}
			}
			if object == "OwnedEntry__c" && machine.soqlObjectHasPublicReadSharing(object) {
				t.Fatal("read fallback granted custom-object visibility")
			}
		})
	}
}
