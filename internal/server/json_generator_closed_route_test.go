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

// Local server routing of the fatal category captured by
// json_generator_closed/org.tsv; this does not assert hosted response shaping.
func TestLightningClosedGeneratorAndExceptionContextRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, tc := range []struct{ name, source, wantType string }{
			{"closed", "JSONGenerator g=JSON.createGenerator(false); g.close(); g.writeNull();", "System.UnexpectedException"},
			{"handled", "throw new AuraHandledException('blocked');", "AuraHandledException"},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				program, err := vm.CompileAnonymousWithOptions(tc.source, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := vm.New(nil)
				if err := machine.RegisterMethod(vm.Method{Name: "ClosedRoute.observe", ClassName: "ClosedRoute", ReturnType: "void", IsStatic: true, Modifiers: []string{"AuraEnabled"}, APIVersion: api, Program: program}); err != nil {
					t.Fatal(err)
				}
				org := storage.NewOrgState()
				handler := New(&org)
				handler.SetProjectRuntime(typesys.Index{}, machine, nil)
				for i := 0; i < 2; i++ {
					rec := httptest.NewRecorder()
					req := httptest.NewRequest(http.MethodPost, "/lightning/wire/apex", strings.NewReader(`{"className":"ClosedRoute","method":"observe","params":{}}`))
					req.Header.Set("Content-Type", "application/json")
					handler.ServeHTTP(rec, req)
					var out lwcbrowser.WireResponse
					if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
						t.Fatal(err)
					}
					if rec.Code != http.StatusOK || out.Error == nil || out.Error.Type != tc.wantType {
						t.Fatalf("invocation %d: status=%d response=%+v", i, rec.Code, out)
					}
				}
			})
		}
	}
}
