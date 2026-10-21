package apextest

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func nativeObservationRuntimeMatches(source nativeObservationRoute, expected, observed map[string]any, org *storage.OrgState) (bool, error) {
	switch source.Comparison {
	case "":
		return reflect.DeepEqual(expected, observed), nil
	case "identityHashRelation":
		if source.ID != "C126HASH_005" || !nativeIdentityHashRelation(expected) {
			return false, fmt.Errorf("identity comparison requires its captured row and native relation")
		}
		return nativeIdentityHashRelation(observed), nil
	case "recipientID":
		return nativeRecipientIDMatches(source, expected, observed, org)
	default:
		return false, fmt.Errorf("unknown native observation comparison %q", source.Comparison)
	}
}

// The native ID and the local ID differ. Bind the local error to the single
// inserted record; all other returned fields still compare literally.
func nativeRecipientIDMatches(source nativeObservationRoute, expected, observed map[string]any, org *storage.OrgState) (bool, error) {
	if !(source.ID == "C126EMAIL_004" && source.RecipientObject == "Account" ||
		source.ID == "C126EMAIL_007" && source.RecipientObject == "Contact") {
		return false, fmt.Errorf("recipient comparison requires its captured row and object")
	}
	nativeFields, ok := nativeRecipientErrorFields(expected)
	if !ok {
		return false, fmt.Errorf("missing captured recipient error")
	}
	actualFields, ok := nativeRecipientErrorFields(observed)
	if !ok || org == nil {
		return false, nil
	}
	nativeID, nativeOK := nativeFields[1].(string)
	actualID, actualOK := actualFields[1].(string)
	if !nativeOK || !actualOK || storage.ValidateID(storage.ID(nativeID)) != nil || storage.ValidateID(storage.ID(actualID)) != nil {
		return false, nil
	}
	object, ok := org.Objects[source.RecipientObject]
	if !ok || len(object.Records) != 1 || len(object.Definition.KeyPrefix) != 3 || !strings.HasPrefix(nativeID, object.Definition.KeyPrefix) {
		return false, nil
	}
	_, record, found := storage.LookupRecordByID(object.Records, storage.ID(actualID))
	if !found || record.System.IsDeleted || record.Object != source.RecipientObject {
		return false, nil
	}
	// expected is decoded anew for this leaf; the raw fixture/evidence is retained.
	nativeFields[1] = actualID
	return reflect.DeepEqual(expected, observed), nil
}

func nativeRecipientErrorFields(value map[string]any) ([]any, bool) {
	errors, ok := value["errors"].([]any)
	if !ok || len(errors) != 1 {
		return nil, false
	}
	errorValue, ok := errors[0].(map[string]any)
	if !ok || errorValue["statusCode"] != "INVALID_ID_FIELD" {
		return nil, false
	}
	fields, ok := errorValue["fields"].([]any)
	return fields, ok && len(fields) == 2 && fields[0] == "toAddresses"
}

func nativeHashInteger(value any) (float64, bool) {
	number, ok := value.(float64)
	return number, ok && !math.IsNaN(number) && !math.IsInf(number, 0) && math.Trunc(number) == number &&
		number >= -2147483648 && number <= 2147483647
}

func nativeIdentityHashRelation(value map[string]any) bool {
	if len(value) != 3 {
		return false
	}
	first, firstOK := nativeHashInteger(value["sameFirst"])
	again, againOK := nativeHashInteger(value["sameAgain"])
	other, otherOK := nativeHashInteger(value["other"])
	return firstOK && againOK && otherOK && first == again && first != other
}

func TestNativeObservationComparisonGuards(t *testing.T) {
	source := nativeObservationRoute{Comparison: "identityHashRelation"}
	source.ID = "C126HASH_005"
	expected := map[string]any{"sameFirst": float64(17), "sameAgain": float64(17), "other": float64(19)}
	for _, tc := range []struct {
		name  string
		value map[string]any
		match bool
	}{
		{"different allocation numbers", map[string]any{"sameFirst": float64(21), "sameAgain": float64(21), "other": float64(23)}, true},
		{"unstable same object", map[string]any{"sameFirst": float64(21), "sameAgain": float64(22), "other": float64(23)}, false},
		{"collapsed identities", map[string]any{"sameFirst": float64(21), "sameAgain": float64(21), "other": float64(21)}, false},
		{"fractional hash", map[string]any{"sameFirst": 21.5, "sameAgain": 21.5, "other": float64(23)}, false},
		{"extra observation", map[string]any{"sameFirst": float64(21), "sameAgain": float64(21), "other": float64(23), "extra": true}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			matched, err := nativeObservationRuntimeMatches(source, expected, tc.value, nil)
			if err != nil || matched != tc.match {
				t.Fatalf("match=%v error=%v", matched, err)
			}
		})
	}
	source.ID = "C126HASH_004"
	if _, err := nativeObservationRuntimeMatches(source, expected, expected, nil); err == nil {
		t.Fatal("foreign row accepted an identity relation")
	}
}

func TestNativeRecipientComparisonGuards(t *testing.T) {
	source := nativeObservationRoute{Comparison: "recipientID", RecipientObject: "Account"}
	source.ID = "C126EMAIL_004"
	payload := func(id, status string) map[string]any {
		return map[string]any{"success": false, "errors": []any{map[string]any{
			"statusCode": status, "message": "ID is invalid or you do not have access to the record.",
			"targetObjectId": nil, "fields": []any{"toAddresses", id},
		}}}
	}
	for _, tc := range []struct {
		name    string
		id      string
		status  string
		deleted bool
		match   bool
	}{
		{"bound inserted record", "001000000000002", "INVALID_ID_FIELD", false, true},
		{"different record with same prefix", "001000000000003", "INVALID_ID_FIELD", false, false},
		{"wrong status", "001000000000002", "INVALID_EMAIL_ADDRESS", false, false},
		{"deleted recipient", "001000000000002", "INVALID_ID_FIELD", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.OrgState{Objects: map[string]storage.ObjectState{"Account": {
				Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001"},
				Records: map[storage.ID]storage.Record{"001000000000002": {
					ID: "001000000000002", Object: "Account", System: storage.SystemFields{IsDeleted: tc.deleted},
				}},
			}}}
			matched, err := nativeObservationRuntimeMatches(source, payload("001000000000001", "INVALID_ID_FIELD"), payload(tc.id, tc.status), &org)
			if err != nil || matched != tc.match {
				t.Fatalf("match=%v error=%v", matched, err)
			}
		})
	}
}
