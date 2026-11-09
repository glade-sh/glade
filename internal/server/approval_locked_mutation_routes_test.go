package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func TestApprovalLockedMutationServerRoutes(t *testing.T) {
	for _, role := range []string{"admin", "standard"} {
		for _, op := range []string{"update", "delete"} {
			t.Run(role+"/"+op, func(t *testing.T) {
				org := testOrg()
				storage.EnsureStandardObject(&org, "User")
				storage.EnsureStandardObject(&org, "Profile")
				uid := storage.ID("005000000000991AAA")
				pid := storage.ID("00e000000000991AAA")
				users := org.Objects["User"]
				users.Records[uid] = storage.Record{ID: uid, Object: "User", Fields: map[string]storage.Value{"ProfileId": storage.IDValue(pid)}}
				org.Objects["User"] = users
				profiles := org.Objects["Profile"]
				profiles.Records[pid] = storage.Record{ID: pid, Object: "Profile", Fields: map[string]storage.Value{"PermissionsModifyAllData": storage.BooleanValue(role == "admin")}}
				org.Objects["Profile"] = profiles
				engine := dml.NewEngine(&org)
				created := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Probe")}}})
				if !created[0].Success {
					t.Fatal(created)
				}
				id := created[0].ID
				if locked := engine.Lock([]storage.Record{{Object: "Account", ID: id}}); !locked[0].Success {
					t.Fatal(locked)
				}
				method, body := http.MethodPatch, `{"Name":"Changed"}`
				if op == "delete" {
					method, body = http.MethodDelete, ""
				}
				request := httptest.NewRequest(method, serverTestDataPath+"/sobjects/Account/"+string(id), strings.NewReader(body))
				request.Header.Set("X-GLADE-User-Id", string(uid))
				rec := httptest.NewRecorder()
				New(&org).ServeHTTP(rec, request)
				if role == "standard" {
					if rec.Code < 400 || !strings.Contains(rec.Body.String(), "ENTITY_IS_LOCKED") {
						t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
					}
				} else if rec.Code != http.StatusNoContent {
					t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
				}
			})
		}
	}
}
