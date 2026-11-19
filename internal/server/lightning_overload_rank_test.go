package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Native observation: aura_overload_determinism_scalar/org.tsv (String over Integer).
// The wire route reaches the same resolver as the imperative VM call.
func TestLightningWireApexSelectsHistoricalRankedOverload(t *testing.T) {
	machine := vm.New(nil)
	for _, spec := range []struct{ typeName, result string }{{"Integer", "integer"}, {"String", "string"}} {
		program, err := vm.CompileAnonymous(`return '` + spec.result + `:' + String.valueOf(value);`)
		if err != nil {
			t.Fatal(err)
		}
		if err := machine.RegisterMethod(vm.Method{Name: "TieController.pick", ClassName: "TieController", ReturnType: "String", IsStatic: true, APIVersion: "54.0", Modifiers: []string{"AuraEnabled"}, Params: []vm.Param{{Name: "value", Type: spec.typeName}}, Program: program}); err != nil {
			t.Fatal(err)
		}
	}
	org := storage.NewOrgState()
	handler := New(&org)
	handler.SetProjectRuntime(typesys.Index{}, machine, nil)
	for _, tc := range []struct{ params, want string }{
		{`{}`, "string:null"},
		{`{"value":null}`, "string:null"},
		{`{"value":42}`, "string:42"},
		{`{"value":"bad"}`, "string:bad"},
	} {
		t.Run(tc.params, func(t *testing.T) {
			body := `{"className":"TieController","method":"pick","params":` + tc.params + `}`
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/lightning/wire/apex", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
			}
			var out lwcbrowser.WireResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
				t.Fatal(err)
			}
			if out.Error != nil || out.Data != tc.want {
				t.Fatalf("data=%#v error=%#v want %s", out.Data, out.Error, tc.want)
			}
		})
	}
}
