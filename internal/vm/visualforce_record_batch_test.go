package vm

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestVisualforceSetBatchUserModeProjectionAPI67(t *testing.T) {
	const accountID = storage.ID("001000000000001")
	const userID = storage.ID("005000000000002")
	const otherID = storage.ID("005000000000999")

	t.Run("owner reads only Name and Id", func(t *testing.T) {
		machine, org := visualforceRecordAccessFixture()
		setVisualforceAccountOwner(org, userID)
		rows, err := machine.ReadVisualforceRecords("Account")
		if err != nil || len(rows) != 1 {
			t.Fatalf("rows=%#v err=%v; want one owner row", rows, err)
		}
		if rows[0].ID != accountID || len(rows[0].Fields) != 1 || rows[0].Fields["Name"].String != "Acme" ||
			rows[0].System != (storage.SystemFields{}) || len(rows[0].Children) != 0 {
			t.Fatalf("batch returned raw or wrong row: %#v", rows[0])
		}
	})
	t.Run("private row omitted", func(t *testing.T) {
		machine, _ := visualforceRecordAccessFixture()
		rows, err := machine.ReadVisualforceRecords("Account")
		if err != nil || len(rows) != 0 {
			t.Fatalf("private rows=%#v err=%v; want none", rows, err)
		}
	})
	t.Run("manual share allows row", func(t *testing.T) {
		machine, org := visualforceRecordAccessFixture()
		storage.EnsureStandardObject(org, "AccountShare")
		shares := org.Objects["AccountShare"]
		if shares.Records == nil {
			shares.Records = make(map[storage.ID]storage.Record)
		}
		shares.Records["00A000000000001"] = storage.Record{ID: "00A000000000001", Object: "AccountShare",
			Fields: map[string]storage.Value{"AccountId": storage.IDValue(accountID),
				"UserOrGroupId": storage.IDValue(userID), "AccountAccessLevel": storage.StringValue("Read")}}
		org.Objects["AccountShare"] = shares
		rows, err := machine.ReadVisualforceRecords("Account")
		if err != nil || len(rows) != 1 || rows[0].ID != accountID {
			t.Fatalf("shared rows=%#v err=%v; want account", rows, err)
		}
	})
	t.Run("deleted row omitted", func(t *testing.T) {
		machine, org := visualforceRecordAccessFixture()
		setVisualforceAccountOwner(org, userID)
		account := org.Objects["Account"]
		row := account.Records[accountID]
		row.System.IsDeleted = true
		account.Records[accountID] = row
		org.Objects["Account"] = account
		rows, err := machine.ReadVisualforceRecords("Account")
		if err != nil || len(rows) != 0 {
			t.Fatalf("deleted rows=%#v err=%v; want none", rows, err)
		}
	})
	for _, denied := range []string{"object", "name-field", "missing-user", "unknown-user"} {
		t.Run(denied+" denied", func(t *testing.T) {
			machine, org := visualforceRecordAccessFixture()
			setVisualforceAccountOwner(org, userID)
			switch denied {
			case "object":
				permissions := org.Objects["ObjectPermissions"]
				permission := permissions.Records["110000000000001"]
				permission.Fields["PermissionsRead"] = storage.BooleanValue(false)
				permissions.Records["110000000000001"] = permission
				org.Objects["ObjectPermissions"] = permissions
			case "name-field":
				permissions := org.Objects["FieldPermissions"]
				permission := permissions.Records["0FP000000000001"]
				permission.Fields["PermissionsRead"] = storage.BooleanValue(false)
				permissions.Records["0FP000000000001"] = permission
				org.Objects["FieldPermissions"] = permissions
			case "missing-user":
				machine.SetCurrentUser(storage.Record{})
			case "unknown-user":
				machine.SetCurrentUser(storage.Record{ID: otherID, Object: "User"})
			}
			if rows, err := machine.ReadVisualforceRecords("Account"); err == nil {
				t.Fatalf("%s returned rows=%#v without denial", denied, rows)
			}
		})
	}
}

func TestVisualforceSetBatchRejectsExcessiveRowsAPI67(t *testing.T) {
	machine, org := visualforceRecordAccessFixture()
	account := org.Objects["Account"]
	for index := 0; index <= visualforceSetRecordLimit; index++ {
		id := storage.ID(fmt.Sprintf("001%012d", index))
		account.Records[id] = storage.Record{ID: id, Object: "Account",
			System: storage.SystemFields{OwnerID: "005000000000002"}}
	}
	org.Objects["Account"] = account
	if rows, err := machine.ReadVisualforceRecords("Account"); err == nil || len(rows) != 0 {
		t.Fatalf("oversized set rows=%d err=%v; want explicit failure", len(rows), err)
	}
	for id, row := range account.Records {
		row.System.OwnerID = "005000000000999"
		account.Records[id] = row
	}
	org.Objects["Account"] = account
	if rows, err := machine.ReadVisualforceRecords("Account"); err != nil || len(rows) != 0 {
		t.Fatalf("hidden oversized set rows=%d err=%v; want empty authorized result", len(rows), err)
	}
	// One visible record in a much larger private population must not make
	// the final unindexed SOQL query allocate against the private population.
	visibleID := storage.ID("001000000000001")
	visibleRow := account.Records[visibleID]
	visibleRow.System.OwnerID = "005000000000002"
	account.Records[visibleID] = visibleRow
	org.Objects["Account"] = account
	bounded := visualforceSetCandidateOrg(org, "Account", account, []storage.ID{visibleID})
	if got := len(bounded.Objects["Account"].Records); got != 1 || len(bounded.Objects["Account"].Indexes) != 0 {
		t.Fatalf("unindexed query candidate set has %d records; want one", got)
	}
	if got := len(org.Objects["Account"].Records); got <= visualforceSetRecordLimit {
		t.Fatalf("source fixture has %d records; want more than limit", got)
	}
	if rows, err := machine.ReadVisualforceRecords("Account"); err != nil || len(rows) != 1 || rows[0].ID != visibleID {
		t.Fatalf("mixed private population rows=%#v err=%v; want one visible row", rows, err)
	}
	permissions := org.Objects["ObjectPermissions"]
	permission := permissions.Records["110000000000001"]
	permission.Fields["PermissionsRead"] = storage.BooleanValue(false)
	permissions.Records["110000000000001"] = permission
	org.Objects["ObjectPermissions"] = permissions
	if _, err := machine.ReadVisualforceRecords("Account"); err == nil ||
		strings.Contains(err.Error(), "limit exceeded") {
		t.Fatalf("unauthorized caller learned the oversized-set condition: %v", err)
	}
}
