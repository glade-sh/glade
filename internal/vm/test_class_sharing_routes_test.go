package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// test_entry_sharing's omitted_read/without_read/with_read rows distinguish
// the actual test method from ordinary helpers, including explicit SYSTEM_MODE.
func TestIsTestDeclaredSharingNativeRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, bound := range []bool{false, true} {
			for _, keyword := range []string{"", "with sharing", "without sharing"} {
				for _, route := range []string{"direct_test", "helper_omitted", "helper_with", "helper_without"} {
					t.Run(fmt.Sprintf("%s/bound=%t/%s/%s", api, bound, keyword, route), func(t *testing.T) {
						machine := New(nil)
						org := privateExecutionPolicyOrg()
						object := org.Objects["Widget__c"]
						delete(object.Records, "a00000000000001")
						org.Objects["Widget__c"] = object
						machine.SetOrg(&org)
						machine.SetCurrentUser(storage.Record{ID: "005000000000001", Object: "User"})
						machine.EnableTestContext()
						machine.testContext.RunAsDepth = 1
						entry := Class{Name: "Entry", APIVersion: api, IsTest: true}
						if keyword != "" {
							entry.Modifiers = []string{keyword}
						}
						if err := machine.RegisterClass(entry); err != nil {
							t.Fatal(err)
						}
						want := 0
						if route == "helper_without" {
							want = 1
						}
						body := fmt.Sprintf("System.assertEquals(%d, [SELECT Id FROM Widget__c WITH SYSTEM_MODE].size());", want)
						if route != "direct_test" {
							helperKeyword := ""
							if route == "helper_with" {
								helperKeyword = "with sharing"
							} else if route == "helper_without" {
								helperKeyword = "without sharing"
							}
							helper := Class{Name: "Helper", APIVersion: api}
							if helperKeyword != "" {
								helper.Modifiers = []string{helperKeyword}
							}
							if err := machine.RegisterClass(helper); err != nil {
								t.Fatal(err)
							}
							program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							if err := machine.RegisterMethod(Method{Name: "Helper.observe", ClassName: "Helper", IsStatic: true, Access: "public", APIVersion: api, SourceContextBound: bound, SharingMode: helperKeyword, Program: program}); err != nil {
								t.Fatal(err)
							}
							body = "Helper.observe();"
						}
						program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						if err := machine.RegisterMethod(Method{Name: "Entry.observe", ClassName: "Entry", IsStatic: true, Access: "public", Modifiers: []string{"isTest"}, APIVersion: api, SourceContextBound: bound, SharingMode: keyword, Program: program}); err != nil {
							t.Fatal(err)
						}
						invoke, err := CompileAnonymousWithOptions("Entry.observe();", CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						if _, err := machine.ExecuteTestInClass(invoke, "Entry"); err != nil {
							t.Fatal(err)
						}
						if machine.entrySharingMode != "" || machine.testExecutionEntry != nil {
							t.Fatal("direct test route leaked sharing into the next invocation")
						}
					})
				}
			}
		}
	}
}
