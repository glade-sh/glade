package apextest

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestRuntimeSourceUnitCallbackKeepsReceiverOrigin(t *testing.T) {
	root := t.TempDir()
	shared := filepath.Join(root, "shared", "UnitTransport.cls")
	writeFile(t, shared, `public class UnitTransport {
 public static Integer read() {
  HttpRequest request=new HttpRequest();request.setEndpoint('https://example.invalid/owned');request.setMethod('GET');
  HttpResponse response=new Http().send(request);
  Map<String,Object> value=(Map<String,Object>)JSON.deserializeUntyped(response.getBody());
  return (Integer)value.get('unit');
 }
}`)
	paths := []string{shared}
	for i, unit := range []string{"first", "second"} {
		base := filepath.Join(root, unit)
		writeFile(t, filepath.Join(base, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
		mock := fmt.Sprintf(`@IsTest public class UnitReply implements HttpCalloutMock {
 public HttpResponse respond(HttpRequest request) {
  HttpResponse response=new HttpResponse();response.setStatusCode(200);response.setBody('{"unit":%d}');return response;
 }
}`, i+1)
		testName := fmt.Sprintf("UnitCallback%dTest", i+1)
		test := fmt.Sprintf(`@IsTest private class %s {
 @IsTest static void check() {Test.setMock(HttpCalloutMock.class,new UnitReply());System.assertEquals(%d,UnitTransport.read());}
}`, testName, i+1)
		for name, body := range map[string]string{"UnitReply": mock, testName: test} {
			path := filepath.Join(base, "force-app", "classes", name+".cls")
			writeFile(t, path, body)
			paths = append(paths, path)
		}
	}
	index := typesys.Build(project.Project{Root: root, SourceAPIVersion: "67.0", ApexFiles: paths}, schema.Schema{})
	run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		t.Fatalf("summary=%#v problem=%q", got, firstRunProblem(run))
	}
}
