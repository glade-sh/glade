package visualforce_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// Native V13 rows exercise the generated remoting runtime at the VF floor and ceiling.
func TestV13OnDemandRemotingRender(t *testing.T) {
	data, err := os.ReadFile("testdata/v13_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var table struct {
		Cases []v13Case `json:"cases"`
	}
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	for _, api := range v13APIVersions {
		for _, id := range []string{"r_invoke_string_empty", "r_invoke_string_null", "r_invoke_string_entities"} {
			t.Run(api+"/"+id, func(t *testing.T) {
				var row v13Case
				for _, c := range table.Cases {
					if c.ID == id {
						row = c
						break
					}
				}
				if row.ID == "" {
					t.Fatal("missing owned case")
				}
				root := v13WriteInput(t, row, api)
				host, err := v13RuntimeServer(root, api)
				if err != nil {
					t.Fatal(err)
				}
				response := httptest.NewRecorder()
				host.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/apex/"+row.Inputs[api].Name, nil))
				if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "V13_READY") {
					t.Fatalf("render status %d: %s", response.Code, response.Body.String())
				}
				if !strings.Contains(response.Body.String(), "Visualforce.remoting.Manager.invokeAction") {
					t.Fatal("remoting manager missing from rendered page")
				}
			})
		}
	}
}
