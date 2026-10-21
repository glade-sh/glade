package soql

import (
	"errors"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestExecuteUsingScopeMineFiltersOwnedRowsBeforeWhereAndWindow(t *testing.T) {
	for _, objectName := range []string{"Account", "WorkItem__c"} {
		t.Run(objectName, func(t *testing.T) {
			org := usingScopeMineOrg(objectName)
			for _, test := range []struct {
				query string
				want  []storage.ID
			}{
				{"SELECT Id FROM " + objectName + " USING SCOPE Mine ORDER BY Name", []storage.ID{"001000000000002", "001000000000003"}},
				{"SELECT Id FROM " + objectName + " USING SCOPE Mine WHERE Name = 'Owned later'", []storage.ID{"001000000000003"}},
				{"SELECT Id FROM " + objectName + " USING SCOPE Mine ORDER BY Name LIMIT 1", []storage.ID{"001000000000002"}},
				{"SELECT Id FROM " + objectName + " USING SCOPE Mine ORDER BY Name LIMIT 1 OFFSET 1", []storage.ID{"001000000000003"}},
			} {
				query, err := Parse(test.query)
				if err != nil {
					t.Fatal(err)
				}
				result, err := ExecuteWithCacheAndContext(org, query, nil, ExecutionContext{UserID: "005000000000001AAA"})
				if err != nil {
					t.Fatal(err)
				}
				usingScopeAssertRecordIDs(t, result, test.want)
			}
		})
	}
}

func TestExecuteUsingScopeMineActorAndCountStayOutsideCaches(t *testing.T) {
	org := usingScopeMineOrg("Account")
	cache := NewExecutionCache()
	query, err := Parse("SELECT Id FROM Account USING SCOPE Mine ORDER BY Name")
	if err != nil {
		t.Fatal(err)
	}
	countQuery, err := Parse("SELECT COUNT() FROM Account USING SCOPE Mine")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		actor storage.ID
		want  []storage.ID
	}{
		{"005000000000001", []storage.ID{"001000000000002", "001000000000003"}},
		{"005000000000002", []storage.ID{"001000000000001"}},
		{"005000000000003", []storage.ID{}},
		{"005000000000001", []storage.ID{"001000000000002", "001000000000003"}},
	} {
		context := ExecutionContext{UserID: test.actor}
		result, err := ExecuteWithCacheAndContext(org, query, cache, context)
		if err != nil {
			t.Fatal(err)
		}
		usingScopeAssertRecordIDs(t, result, test.want)
		count, err := ExecuteWithCacheAndContext(org, countQuery, cache, context)
		if err != nil {
			t.Fatal(err)
		}
		if len(count.Records) != 1 || count.Records[0].Fields["expr0"].Integer != int64(len(test.want)) {
			t.Fatalf("actor %s COUNT = %#v, want %d", test.actor, count, len(test.want))
		}
	}
}

func TestExecuteUsingScopeMineOwnerlessUserAndEverythingControls(t *testing.T) {
	org := usingScopeMineOrg("Account")
	for _, objectName := range []string{"Account", "User"} {
		for _, scope := range []string{"", " USING SCOPE Everything"} {
			query, err := Parse("SELECT Id FROM " + objectName + scope)
			if err != nil {
				t.Fatal(err)
			}
			result, err := ExecuteWithCache(org, query, NewExecutionCache())
			if err != nil || result.Rows != 3 {
				t.Fatalf("%s %s control = %#v, %v", objectName, scope, result, err)
			}
		}
	}
	query, err := Parse("SELECT Id FROM User USING SCOPE Mine ORDER BY Id")
	if err != nil {
		t.Fatal(err)
	}
	for _, actor := range []storage.ID{"005000000000001", "005000000000002"} {
		result, err := ExecuteWithCacheAndContext(org, query, NewExecutionCache(), ExecutionContext{UserID: actor})
		if err != nil {
			t.Fatal(err)
		}
		usingScopeAssertRecordIDs(t, result, []storage.ID{"005000000000001", "005000000000002", "005000000000003"})
	}
}

func TestExecuteUsingScopeMinePreservesUnsupportedBoundaries(t *testing.T) {
	for _, test := range []struct {
		name       string
		objectName string
		scope      string
		actor      storage.ID
		owner      *storage.Field
	}{
		{name: "actorAbsent", objectName: "Account", scope: "Mine"},
		{name: "unknownScope", objectName: "Account", scope: "Team", actor: "005000000000001"},
		{name: "otherOwnerlessObject", objectName: "Ownerless__c", scope: "Mine", actor: "005000000000001"},
		{name: "ownerNameIsNotMetadata", objectName: "Ownerless__c", scope: "Mine", actor: "005000000000001", owner: &storage.Field{APIName: "OwnerId", Type: storage.FieldString}},
		{name: "ownerReferenceIsNotUser", objectName: "Ownerless__c", scope: "Mine", actor: "005000000000001", owner: &storage.Field{APIName: "OwnerId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			org := usingScopeMineOrg("Account")
			if test.objectName != "Account" {
				definition := storage.ObjectDefinition{APIName: test.objectName, Fields: map[string]storage.Field{}}
				if test.owner != nil {
					definition.Fields["OwnerId"] = *test.owner
				}
				org.Objects[test.objectName] = storage.ObjectState{Definition: definition, Records: map[storage.ID]storage.Record{}}
			}
			query, err := Parse("SELECT Id FROM " + test.objectName + " USING SCOPE " + test.scope)
			if err != nil {
				t.Fatal(err)
			}
			_, err = ExecuteWithCacheAndContext(org, query, nil, ExecutionContext{UserID: test.actor})
			var unsupported *UnsupportedFeatureError
			if !errors.As(err, &unsupported) {
				t.Fatalf("uncaptured boundary = %v, want unsupported", err)
			}
			if test.name == "actorAbsent" {
				if _, err := ExecuteWithCache(org, query, nil); !errors.As(err, &unsupported) {
					t.Fatalf("compatibility wrapper error = %v, want unsupported", err)
				}
			}
		})
	}
}

func usingScopeMineOrg(objectName string) storage.OrgState {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "User")
	if objectName == "Account" {
		storage.EnsureStandardObject(&org, objectName)
	} else {
		org.Objects[objectName] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: objectName, Fields: map[string]storage.Field{
			"Name":    {APIName: "Name", Type: storage.FieldString},
			"OwnerId": {APIName: "OwnerId", Type: storage.FieldReference, ReferenceTo: []string{"Group", "User"}},
		}}, Records: map[storage.ID]storage.Record{}}
	}
	object := org.Objects[objectName]
	object.Records = map[storage.ID]storage.Record{
		"001000000000001": {ID: "001000000000001", Object: objectName, System: storage.SystemFields{OwnerID: "005000000000002"}, Fields: map[string]storage.Value{"Name": storage.StringValue("Another owner's first")}},
		"001000000000002": {ID: "001000000000002", Object: objectName, System: storage.SystemFields{OwnerID: "005000000000001"}, Fields: map[string]storage.Value{"Name": storage.StringValue("Owned earlier")}},
		"001000000000003": {ID: "001000000000003", Object: objectName, Fields: map[string]storage.Value{"Name": storage.StringValue("Owned later"), "OwnerId": storage.IDValue("005000000000001")}},
	}
	org.Objects[objectName] = object
	users := org.Objects["User"]
	users.Records = map[storage.ID]storage.Record{}
	for _, id := range []storage.ID{"005000000000001", "005000000000002", "005000000000003"} {
		users.Records[id] = storage.Record{ID: id, Object: "User", Fields: map[string]storage.Value{}}
	}
	org.Objects["User"] = users
	return org
}

func usingScopeAssertRecordIDs(t *testing.T, result Result, want []storage.ID) {
	t.Helper()
	got := make([]storage.ID, 0, len(result.Records))
	for _, record := range result.Records {
		got = append(got, record.ID)
	}
	if result.Rows != len(want) || !reflect.DeepEqual(got, want) {
		t.Fatalf("record IDs = %v, rows = %d; want %v", got, result.Rows, want)
	}
}
