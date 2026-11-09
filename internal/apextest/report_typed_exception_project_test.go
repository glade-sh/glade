package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReportTypedExceptionsProjectNative(t *testing.T) {
	const capture = "../resource/testdata/report_rows_capture/typed-exceptions"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			base := filepath.Join(root, "force-app/main/default/classes")
			name := "B6Typed" + strings.TrimSuffix(api, ".0")
			for _, suffix := range []string{".cls", ".cls-meta.xml"} {
				data, err := os.ReadFile(filepath.Join(capture, "classes", name+suffix))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(base, name+suffix), string(data))
			}
			index := loadTestIndex(t, root)
			InvalidateRuntimeCaches()
			for _, phase := range []string{"cold", "warm"} {
				run := Run(index, Options{Parallelism: 1, SelectedClasses: []string{name}})
				if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 || summary.Errors != 0 {
					t.Fatalf("%s: %+v; %v", phase, summary, firstRunProblem(run))
				}
			}
		})
	}
}
