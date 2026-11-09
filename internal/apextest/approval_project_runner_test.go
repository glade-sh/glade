package apextest

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApprovalProjectRunnerAndWarmCache(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			fixture := "testdata/conformance/approval_project_capture/project"
			err := filepath.WalkDir(fixture, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				rel, err := filepath.Rel(fixture, path)
				if err != nil {
					return err
				}
				b, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(b), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/ApprovalCaptureTest.cls"), `@IsTest private class ApprovalCaptureTest {
   @IsTest static void verify() {
    Map<String,Object> actual=ApprovalCapture.observe();
    System.assertEquals('true:Pending:1',actual.get('submit'));
    System.assertEquals('false:NO_APPLICABLE_PROCESS',actual.get('missing'));
    System.assertEquals('true:true',actual.get('lock'));
    System.assertEquals('true:false',actual.get('unlock'));
   }
  }`)
			index := loadTestIndex(t, root)
			for _, phase := range []string{"cold", "warm"} {
				report := Run(index, Options{SelectedClasses: []string{"ApprovalCaptureTest"}, RuntimeRESTAPIVersion: "67.0", Parallelism: 1})
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
