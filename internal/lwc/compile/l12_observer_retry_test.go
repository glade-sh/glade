package compile_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestL12ObserverRetriesFaultsSerially(t *testing.T) {
	for _, fault := range []string{"none", "row", "product", "browser"} {
		t.Run(fault, func(t *testing.T) {
			var calls []bool
			want := map[string]json.RawMessage{"first": json.RawMessage(`{"stage":"done"}`)}
			values, rowErrors, productErr, browserErr := l12ObserveWithFallback(func(parallel bool) (map[string]json.RawMessage, map[string]string, error, error) {
				calls = append(calls, parallel)
				if parallel {
					switch fault {
					case "row":
						return nil, map[string]string{"first": "row failure"}, nil, nil
					case "product":
						return nil, nil, errors.New("product failure"), nil
					case "browser":
						return nil, nil, nil, errors.New("browser failure")
					}
				}
				return want, nil, nil, nil
			})
			wantCalls := []bool{true}
			if fault != "none" {
				wantCalls = append(wantCalls, false)
			}
			if !reflect.DeepEqual(calls, wantCalls) || !reflect.DeepEqual(values, want) || len(rowErrors) != 0 || productErr != nil || browserErr != nil {
				t.Fatalf("calls=%v values=%v rowErrors=%v productErr=%v browserErr=%v", calls, values, rowErrors, productErr, browserErr)
			}
		})
	}
}
