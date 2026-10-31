package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// This is a local shipping-behavior control, not a Salesforce capture.
func TestPersonAccountDisabledServerPreservesUpsertBaseline(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			org := testOrg()
			const userID = storage.ID("005000000000998")
			const psID = storage.ID("0PS000000000998")
			addUser(&org, userID, "boundary@example.invalid", "boundary@example.invalid", "Boundary")
			user := org.Objects["User"].Records[userID]
			user.Fields["ProfileId"] = storage.IDValue("00e000000000002")
			org.Objects["User"].Records[userID] = user
			storage.EnsureStandardObject(&org, "PermissionSetAssignment")
			org.Objects["ObjectPermissions"].Records["110000000000998"] = storage.Record{ID: "110000000000998", Object: "ObjectPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue(psID), "SObjectType": storage.StringValue("Account"), "PermissionsRead": storage.BooleanValue(true), "PermissionsCreate": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(false)}}
			org.Objects["PermissionSetAssignment"].Records["0Pa000000000998"] = storage.Record{ID: "0Pa000000000998", Object: "PermissionSetAssignment", Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(userID), "PermissionSetId": storage.IDValue(psID)}}
			source := `try { upsert as user new Account(Name='Boundary'); System.assert(false); } catch(SecurityException e) { System.assertEquals('Access to entity \'Account\' denied',e.getMessage()); }`
			payload, err := json.Marshal(map[string]string{"anonymousBody": source})
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, "/services/data/v"+api+"/tooling/executeAnonymous", strings.NewReader(string(payload)))
			req.Header.Set("X-GLADE-User-Id", string(userID))
			rec := httptest.NewRecorder()
			New(&org).ServeHTTP(rec, req)
			if api == "62.0" {
				assertSalesforceError(t, rec, http.StatusNotFound, "NOT_FOUND", "unsupported API version v62.0")
				return
			}
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"success":true`) {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}
