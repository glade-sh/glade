package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestHttpUnconfiguredEndpointNativeRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, receiver := range []string{"Http", "System.Http"} {
			for _, route := range []string{"anonymous", "istest"} {
				t.Run(api+"/"+receiver+"/"+route, func(t *testing.T) {
					wantType, wantMessage := "System.CalloutException", "Unauthorized endpoint, please check Setup->Security->Remote site settings. endpoint = https://native.invalid"
					if route == "istest" {
						wantType, wantMessage = "System.TypeException", "Methods defined as TestMethod do not support Web service callouts"
					}
					source := fmt.Sprintf(`HttpRequest request = new HttpRequest();
request.setEndpoint('https://native.invalid'); request.setMethod('GET');
try {new %s().send(request); System.assert(false, 'expected callout rejection');}
catch (Exception failure) {
 System.assertEquals('%s', failure.getTypeName());
 System.assertEquals('%s', failure.getMessage());
}`, receiver, wantType, wantMessage)
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					machine := New(nil)
					org := storage.NewOrgState()
					machine.SetOrg(&org)
					if route == "istest" {
						machine.EnableTestContext()
					}
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}
