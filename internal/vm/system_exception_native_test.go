package vm

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSystemExceptionNativeCatchabilityRoutes(t *testing.T) {
	for _, api := range []string{"62", "67"} {
		file, err := os.Open(filepath.Join("..", "apextest", "testdata", "conformance", "system_exception_hierarchy", "native", "api"+api, "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		reader := csv.NewReader(file)
		reader.Comma, reader.LazyQuotes = '\t', true
		rows, err := reader.ReadAll()
		file.Close()
		if err != nil || len(rows) != 459 {
			t.Fatalf("incomplete native rows: %v", err)
		}
		for _, row := range rows[1:] {
			name, operation, route := row[0], row[1], row[2]
			if operation != "self" && operation != "parent" && operation != "shadowSelf" && operation != "shadowParent" && operation != "shadowSystem" {
				continue
			}
			var native struct {
				Compiled         bool
				ExceptionMessage string
			}
			if err := json.Unmarshal([]byte(row[3]), &native); err != nil {
				t.Fatal(err)
			}
			if !native.Compiled {
				continue
			}
			for _, registered := range []bool{false, true} {
				variant := "builtin"
				if registered {
					variant = "registeredPlatform"
				}
				t.Run(api+"/"+route+"/"+name+"/"+operation+"/"+variant, func(t *testing.T) {
					machine := New(nil)
					if registered {
						if err := machine.RegisterClass(Class{Name: "System." + name, SuperClass: "Exception", PlatformType: true}); err != nil {
							t.Fatal(err)
						}
					}
					thrown, target := "System."+name, "System.Exception"
					if operation == "self" {
						target = thrown
					}
					if strings.HasPrefix(operation, "shadow") {
						if err := machine.RegisterClass(Class{Name: name, SuperClass: "Exception"}); err != nil {
							t.Fatal(err)
						}
						if operation != "shadowSystem" {
							thrown = name
						}
						if operation == "shadowSelf" {
							target = name
						}
					}
					_, typeName, caught := strings.Cut(native.ExceptionMessage, "OBSERVED|")
					if got := machine.exceptionMatches(target, Object(thrown)); got != caught {
						t.Errorf("catch %s matches %s=%v; native=%v", target, thrown, got, caught)
					}
					if caught {
						value, _, _, handled, err := machine.callPlatformObjectMember(Object(thrown), "getTypeName", nil, &Result{})
						if err != nil || !handled || value.Text != typeName {
							t.Errorf("getTypeName=%v handled=%v err=%v; native=%s", value, handled, err, typeName)
						}
					} else {
						nativeType, nativeMessage, _ := strings.Cut(native.ExceptionMessage, ": ")
						fault := runtimeError(Object(thrown), nil).(*RuntimeError)
						if fault.Type != nativeType || fault.ExceptionMessage() != nativeMessage {
							t.Errorf("uncaught=%s: %s; native=%s", fault.Type, fault.ExceptionMessage(), native.ExceptionMessage)
						}
					}
				})
			}
		}
	}
}
