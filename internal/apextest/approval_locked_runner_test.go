package apextest

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApprovalLockedMutationProjectAndWarm(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			fixture := "testdata/conformance/approval_lock_mutation/project"
			err := filepath.WalkDir(fixture, func(path string, e fs.DirEntry, err error) error {
				if err != nil || e.IsDir() {
					return err
				}
				rel, err := filepath.Rel(fixture, path)
				if err != nil {
					return err
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(data), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(root, "force-app/main/default/classes/ApprovalMutationCaptureTest.cls")
			body, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			source := strings.ReplaceAll(string(body), "System.assert(false,'MUTATE_ROWS|'+ApprovalMutationCapture.observe());", "System.assertEquals('update=false:ENTITY_IS_LOCKED;delete=false:ENTITY_IS_LOCKED;upsert=false:ENTITY_IS_LOCKED;',ApprovalMutationCapture.observe());")
			source = strings.ReplaceAll(source, "Name='Standard User'", "Name='Approval Test User'")
			writeFile(t, file, source)
			writeFile(t, filepath.Join(root, "force-app/main/default/profiles/Approval Test User.profile-meta.xml"), `<Profile xmlns="http://soap.sforce.com/2006/04/metadata"><objectPermissions><object>Account</object><allowCreate>true</allowCreate><allowRead>true</allowRead><allowEdit>true</allowEdit><allowDelete>true</allowDelete><viewAllRecords>false</viewAllRecords><modifyAllRecords>false</modifyAllRecords></objectPermissions></Profile>`)
			index := loadTestIndex(t, root)
			for _, phase := range []string{"cold", "warm"} {
				report := Run(index, Options{SelectedClasses: []string{"ApprovalMutationCaptureTest"}, SelectedMethod: "captureStandard", RuntimeRESTAPIVersion: "67.0", Parallelism: 1})
				if summary := report.Summary(); summary.Total != 1 || summary.Passed != 1 || summary.Errors != 0 {
					for _, suite := range report.Suites {
						for _, row := range suite.Cases {
							if row.Problem != nil {
								t.Logf("%s %s: %s", phase, row.Problem.Type, row.Problem.Message)
							}
						}
					}
					t.Fatalf("%s project runner: %+v", phase, summary)
				}
			}
		})
	}
}
