package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestReadVisualforceRecordFieldsAPI67(t *testing.T) {
	const (
		accountID = storage.ID("001000000000001")
		userID    = storage.ID("005000000000002")
		otherID   = storage.ID("005000000000999")
	)

	t.Run("owner receives only requested scalar field and sanitized identity", func(t *testing.T) {
		machine, org := visualforceRecordAccessFixture()
		setVisualforceAccountOwner(org, userID)
		setVisualforceSecretReadPermission(org, true)

		got, found, err := machine.ReadVisualforceRecordFields("Account", accountID, []string{"Secret__c"})
		if err != nil || !found {
			t.Fatalf("ReadVisualforceRecordFields found=%t err=%v; want authorized record", found, err)
		}
		if got.ID != accountID || got.Object != "Account" {
			t.Fatalf("projected identity = (%q, %q); want (Account, %q)", got.Object, got.ID, accountID)
		}
		if len(got.Fields) != 1 || got.Fields["Secret__c"].Kind != storage.ValueString || got.Fields["Secret__c"].String != "Hidden" {
			t.Fatalf("projected fields = %#v; want only Secret__c=Hidden", got.Fields)
		}
		if got.System != (storage.SystemFields{}) || len(got.Children) != 0 || len(got.ParentRelationships) != 0 ||
			len(got.ExplicitNulls) != 0 || len(got.LoadedReferences) != 0 {
			t.Fatalf("raw record metadata leaked into projection: %#v", got)
		}
	})

	t.Run("authorized null field remains in projection", func(t *testing.T) {
		machine, org := visualforceRecordAccessFixture()
		setVisualforceAccountOwner(org, userID)
		setVisualforceSecretReadPermission(org, true)
		account := org.Objects["Account"]
		row := account.Records[accountID]
		row.Fields["Secret__c"] = storage.NullValue()
		account.Records[accountID] = row
		org.Objects["Account"] = account

		got, found, err := machine.ReadVisualforceRecordFields("Account", accountID, []string{"Secret__c"})
		if err != nil || !found {
			t.Fatalf("ReadVisualforceRecordFields found=%t err=%v; want authorized record", found, err)
		}
		value, ok := got.GetField("Secret__c")
		if len(got.Fields) != 1 || !ok || value.Kind != storage.ValueNull {
			t.Fatalf("projected fields = %#v; want Secret__c present with null value", got.Fields)
		}
	})

	t.Run("authorized OwnerId retains reference ID type", func(t *testing.T) {
		machine, org := visualforceRecordAccessFixture()
		setVisualforceAccountOwner(org, userID)
		account := org.Objects["Account"]
		account.Definition.Fields["OwnerId"] = storage.Field{
			APIName: "OwnerId", Type: storage.FieldReference, ReferenceTo: []string{"User"},
		}
		row := account.Records[accountID]
		row.Fields["OwnerId"] = storage.IDValue(userID)
		account.Records[accountID] = row
		org.Objects["Account"] = account

		got, found, err := machine.ReadVisualforceRecordFields("Account", accountID, []string{"OwnerId"})
		if err != nil || !found {
			t.Fatalf("OwnerId read found=%t err=%v", found, err)
		}
		owner, present := got.GetField("OwnerId")
		if !present || owner.Kind != storage.ValueID || owner.ID != userID || len(got.Fields) != 1 {
			t.Fatalf("OwnerId projection=%#v; want only typed ID %q", got.Fields, userID)
		}
		if got.System != (storage.SystemFields{}) || len(got.Children) != 0 || len(got.ParentRelationships) != 0 {
			t.Fatalf("OwnerId projection leaked raw record metadata: %#v", got)
		}
	})

	t.Run("forged execution user cannot read OwnerId", func(t *testing.T) {
		machine, org := visualforceRecordAccessFixture()
		setVisualforceAccountOwner(org, userID)
		account := org.Objects["Account"]
		account.Definition.Fields["OwnerId"] = storage.Field{
			APIName: "OwnerId", Type: storage.FieldReference, ReferenceTo: []string{"User"},
		}
		row := account.Records[accountID]
		row.Fields["OwnerId"] = storage.IDValue(userID)
		account.Records[accountID] = row
		org.Objects["Account"] = account
		machine.SetCurrentUser(storage.Record{ID: otherID, Object: "User"})

		got, found, err := machine.ReadVisualforceRecordFields("Account", accountID, []string{"OwnerId"})
		if err == nil || found {
			t.Fatalf("forged OwnerId read found=%t err=%v; want denial", found, err)
		}
		assertNoVisualforceRecordFieldLeak(t, got)
	})

	t.Run("private row is hidden without returning its stored field", func(t *testing.T) {
		machine, org := visualforceRecordAccessFixture()
		setVisualforceSecretReadPermission(org, true)

		got, found, err := machine.ReadVisualforceRecordFields("Account", accountID, []string{"Secret__c"})
		if err != nil || found {
			t.Fatalf("private read found=%t err=%v; want no visible row", found, err)
		}
		assertNoVisualforceRecordFieldLeak(t, got)
	})

	t.Run("object read denial returns no record", func(t *testing.T) {
		machine, org := visualforceRecordAccessFixture()
		setVisualforceAccountOwner(org, userID)
		setVisualforceSecretReadPermission(org, true)
		permissions := org.Objects["ObjectPermissions"]
		permission := permissions.Records["110000000000001"]
		permission.Fields["PermissionsRead"] = storage.BooleanValue(false)
		permissions.Records["110000000000001"] = permission
		org.Objects["ObjectPermissions"] = permissions

		got, found, err := machine.ReadVisualforceRecordFields("Account", accountID, []string{"Secret__c"})
		if err == nil || found {
			t.Fatalf("object-denied read found=%t err=%v; want denial", found, err)
		}
		assertNoVisualforceRecordFieldLeak(t, got)
	})

	t.Run("field read denial returns no record", func(t *testing.T) {
		machine, org := visualforceRecordAccessFixture()
		setVisualforceAccountOwner(org, userID)
		setVisualforceSecretReadPermission(org, false)

		got, found, err := machine.ReadVisualforceRecordFields("Account", accountID, []string{"Secret__c"})
		if err == nil || found {
			t.Fatalf("FLS-denied read found=%t err=%v; want denial", found, err)
		}
		assertNoVisualforceRecordFieldLeak(t, got)
	})

	t.Run("missing execution user fails closed", func(t *testing.T) {
		machine, _ := visualforceRecordAccessFixture()
		machine.SetCurrentUser(storage.Record{})

		got, found, err := machine.ReadVisualforceRecordFields("Account", accountID, []string{"Secret__c"})
		if err == nil || found {
			t.Fatalf("missing-user read found=%t err=%v; want denial", found, err)
		}
		assertNoVisualforceRecordFieldLeak(t, got)
	})

	t.Run("unknown forged execution user fails closed", func(t *testing.T) {
		machine, _ := visualforceRecordAccessFixture()
		machine.SetCurrentUser(storage.Record{
			ID: otherID, Object: "User",
			Fields: map[string]storage.Value{"ProfileId": storage.IDValue("00e000000000002")},
		})

		got, found, err := machine.ReadVisualforceRecordFields("Account", accountID, []string{"Secret__c"})
		if err == nil || found {
			t.Fatalf("forged-user read found=%t err=%v; want denial", found, err)
		}
		assertNoVisualforceRecordFieldLeak(t, got)
	})

	t.Run("caller-forged privileges cannot reveal a private row", func(t *testing.T) {
		machine, org := visualforceRecordAccessFixture()
		setVisualforceAccountOwner(org, otherID)
		setVisualforceSecretReadPermission(org, true)
		machine.SetCurrentUser(storage.Record{
			ID: userID, Object: "User",
			Fields: map[string]storage.Value{
				"ProfileId":              storage.IDValue("00e000000000099"),
				"PermissionsViewAllData": storage.BooleanValue(true),
			},
		})

		got, found, err := machine.ReadVisualforceRecordFields("Account", accountID, []string{"Secret__c"})
		if err != nil || found {
			t.Fatalf("forged-privilege read found=%t err=%v; want no visible row", found, err)
		}
		assertNoVisualforceRecordFieldLeak(t, got)
	})

	for name, requested := range map[string][]string{
		"dotted field":  []string{"Name.value"},
		"unknown field": []string{"Missing__c"},
	} {
		t.Run(name+" is rejected without a record", func(t *testing.T) {
			machine, org := visualforceRecordAccessFixture()
			setVisualforceAccountOwner(org, userID)
			setVisualforceSecretReadPermission(org, true)

			got, found, err := machine.ReadVisualforceRecordFields("Account", accountID, requested)
			if err == nil || found {
				t.Fatalf("invalid-field read found=%t err=%v; want rejection", found, err)
			}
			assertNoVisualforceRecordFieldLeak(t, got)
		})
	}
}

func TestAuthorizeVisualforceRecordFieldsWithoutVisibleRowAPI67(t *testing.T) {
	const userID = storage.ID("005000000000002")
	for _, name := range []string{"missing row", "private row"} {
		t.Run(name, func(t *testing.T) {
			machine, org := visualforceRecordAccessFixture()
			setVisualforceSecretReadPermission(org, true)
			if name == "missing row" {
				account := org.Objects["Account"]
				delete(account.Records, storage.ID("001000000000001"))
				org.Objects["Account"] = account
			} else {
				setVisualforceAccountOwner(org, storage.ID("005000000000999"))
			}
			fields, err := machine.AuthorizeVisualforceRecordFields("Account", []string{"Secret__c"})
			if err != nil || len(fields) != 1 || fields[0] != "Secret__c" {
				t.Fatalf("row-independent USER_MODE authorization fields=%#v err=%v", fields, err)
			}
		})
	}
	for _, name := range []string{"object denied", "field denied", "forged user", "unknown field"} {
		t.Run(name, func(t *testing.T) {
			machine, org := visualforceRecordAccessFixture()
			setVisualforceAccountOwner(org, userID)
			setVisualforceSecretReadPermission(org, true)
			requested := []string{"Secret__c"}
			switch name {
			case "object denied":
				permissions := org.Objects["ObjectPermissions"]
				permission := permissions.Records["110000000000001"]
				permission.Fields["PermissionsRead"] = storage.BooleanValue(false)
				permissions.Records["110000000000001"] = permission
				org.Objects["ObjectPermissions"] = permissions
			case "field denied":
				setVisualforceSecretReadPermission(org, false)
			case "forged user":
				machine.SetCurrentUser(storage.Record{ID: "005000000000999", Object: "User"})
			case "unknown field":
				requested = []string{"Missing__c"}
			}
			fields, err := machine.AuthorizeVisualforceRecordFields("Account", requested)
			if err == nil || len(fields) != 0 {
				t.Fatalf("denied USER_MODE authorization fields=%#v err=%v", fields, err)
			}
		})
	}
}

func setVisualforceSecretReadPermission(org *storage.OrgState, allowed bool) {
	permissions := org.Objects["FieldPermissions"]
	permission := permissions.Records["0FP000000000001"]
	permission.Fields["Field"] = storage.StringValue("Account.Secret__c")
	permission.Fields["PermissionsRead"] = storage.BooleanValue(allowed)
	permissions.Records["0FP000000000001"] = permission
	org.Objects["FieldPermissions"] = permissions
}

func assertNoVisualforceRecordFieldLeak(t *testing.T, record storage.Record) {
	t.Helper()
	if record.ID != "" || record.Object != "" || len(record.Fields) != 0 || record.System != (storage.SystemFields{}) ||
		len(record.Children) != 0 || len(record.ParentRelationships) != 0 || len(record.ExplicitNulls) != 0 || len(record.LoadedReferences) != 0 {
		t.Fatalf("denied Visualforce field read returned record data: %#v", record)
	}
}
