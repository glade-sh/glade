package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Messaging R089/R090 capture setter copying and getter aliasing, not generated
// property assignment. R095/R098-R100 capture recipient rejection through setters.
func TestEmailRecipientNativeControls(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			path := filepath.Join(root, "force-app/main/default/classes/EmailRecipientNativeControlTest.cls")
			writeFile(t, path, emailRecipientNativeControlSource)
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			run := Run(index, Options{NoDiskCache: true, Parallelism: 1, SelectedClasses: []string{"EmailRecipientNativeControlTest"}})
			summary := run.Summary()
			if summary.Total != 6 || summary.Passed != 6 {
				t.Fatalf("unexpected results: %#v: %s", summary, firstRunProblem(run))
			}
		})
	}
}

// Messaging R096/R097 were captured through anonymous execution only.
func TestEmailRecipientAnonymousNativeControls(t *testing.T) {
	type nativeCase struct {
		ID, Code, Expected string
		Compile            bool
	}
	var data struct {
		Declarations           map[string]string
		DeclarationAPIVersions map[string]string
		Cases                  []nativeCase
	}
	contents, err := os.ReadFile(filepath.Join("testdata", "conformance", "messaging.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(contents, &data); err != nil {
		t.Fatal(err)
	}
	rows := make([]nativeCase, 0, 2)
	for _, id := range []string{"R096", "R097"} {
		matches := 0
		for _, row := range data.Cases {
			if row.ID == id {
				if row.Compile || row.Code == "" || row.Expected == "" {
					t.Fatalf("invalid anonymous native row %s", id)
				}
				rows = append(rows, row)
				matches++
			}
		}
		if matches != 1 {
			t.Fatalf("expected one anonymous native row %s, got %d", id, matches)
		}
	}
	if data.Declarations["A41"] == "" || data.DeclarationAPIVersions["A41"] == "" {
		t.Fatal("missing captured A41 declaration or source API")
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			path := filepath.Join(root, "force-app/main/default/classes/A41.cls")
			writeFile(t, path, data.Declarations["A41"])
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+data.DeclarationAPIVersions["A41"]+"</apiVersion></ApexClass>")
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("helper compilation: parser=%v semantics=%v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, row := range rows {
				t.Run(row.ID+"/anonymous", func(t *testing.T) {
					source := "A41 a41=new A41(); Object r; try {" + row.Code +
						"} catch(Exception e) {r='EXC|'+e.getTypeName()+'|'+e.getMessage();}" +
						"String expected=" + conformanceApexString(row.Expected) + "; String actual=String.valueOf(r);" +
						"System.assert(expected.equals(actual),'expected <'+expected+'> actual <'+actual+'>');"
					analysis := sema.AnalyzeAnonymous(index, source, api)
					program, compileErr := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
					if analysis.HasErrors() || compileErr != nil {
						t.Fatalf("anonymous compilation: error=%v diagnostics=%v", compileErr, analysis.Diagnostics)
					}
					if _, err := runner.execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		})
	}
}

const emailRecipientNativeControlSource = `@IsTest
private class EmailRecipientNativeControlTest {
    @IsTest static void setterCopiesInput() {
        Messaging.SingleEmailMessage m=new Messaging.SingleEmailMessage();
        List<String> xs=new List<String>{'one@example.invalid'};
        m.setToAddresses(xs);
        xs.add('two@example.invalid');
        System.assertEquals(1,m.getToAddresses().size());
    }
    @IsTest static void getterAliasesStoredList() {
        Messaging.SingleEmailMessage m=new Messaging.SingleEmailMessage();
        m.setToAddresses(new List<String>{'one@example.invalid'});
        List<String> xs=m.getToAddresses();
        xs.clear();
        System.assertEquals(0,m.getToAddresses().size());
    }
    @IsTest static void missingRecipientRejected() {
        Messaging.SendEmailResult result=Messaging.sendEmail(new List<Messaging.Email>{new Messaging.SingleEmailMessage()},false)[0];
        System.assertEquals(false,result.isSuccess());
        System.assertEquals('REQUIRED_FIELD_MISSING',String.valueOf(result.getErrors()[0].getStatusCode()));
        System.assertEquals('Add a recipient (To, CC, or BCC) to send an email.',result.getErrors()[0].getMessage());
    }
    @IsTest static void invalidRecipientSetterRejected() {
        Messaging.SingleEmailMessage m=new Messaging.SingleEmailMessage();
        m.setToAddresses(new List<String>{'not-an-address'});
        m.setPlainTextBody('Body');
        Messaging.SendEmailResult result=Messaging.sendEmail(new List<Messaging.Email>{m},false)[0];
        System.assertEquals(false,result.isSuccess());
        System.assertEquals('INVALID_EMAIL_ADDRESS',String.valueOf(result.getErrors()[0].getStatusCode()));
        System.assertEquals('Email address is invalid: not-an-address',result.getErrors()[0].getMessage());
    }
    @IsTest static void emptyRecipientSetterRejected() {
        Messaging.SingleEmailMessage m=new Messaging.SingleEmailMessage();
        m.setToAddresses(new List<String>{''});
        m.setPlainTextBody('Body');
        Messaging.SendEmailResult result=Messaging.sendEmail(new List<Messaging.Email>{m},false)[0];
        System.assertEquals(false,result.isSuccess());
        System.assertEquals('INVALID_EMAIL_ADDRESS',String.valueOf(result.getErrors()[0].getStatusCode()));
        System.assertEquals('Email address is invalid: ',result.getErrors()[0].getMessage());
    }
    @IsTest static void nullRecipientSetterRejected() {
        Messaging.SingleEmailMessage m=new Messaging.SingleEmailMessage();
        m.setToAddresses(new List<String>{null});
        m.setPlainTextBody('Body');
        Messaging.SendEmailResult result=Messaging.sendEmail(new List<Messaging.Email>{m},false)[0];
        System.assertEquals(false,result.isSuccess());
        System.assertEquals('INVALID_EMAIL_ADDRESS',String.valueOf(result.getErrors()[0].getStatusCode()));
        System.assertEquals('Email address is invalid: null',result.getErrors()[0].getMessage());
    }
}
`
