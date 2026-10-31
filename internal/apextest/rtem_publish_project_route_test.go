package apextest

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// The method containing the captured publish call is compiled but not run:
// the native rows establish deployment behavior, not event delivery.
func TestRTEMPublishProjectRunnerCompile(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "ProbeTest.cls")
			writeFile(t, path, `@IsTest private class ProbeTest {
                @IsTest static void publishCompile() { EventBus.publish(new FileEvent()); EventBus.publish(new FileEvent(), null); }
                @IsTest static void control() { System.assert(true); }
            }`)
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
			result := Run(index, Options{SelectedClasses: []string{"ProbeTest"}, SelectedMethod: "control", NoDiskCache: true, Parallelism: 1})
			if len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 || result.Suites[0].Cases[0].Problem != nil {
				t.Fatalf("expected compiled control test to pass: %+v", result)
			}
		})
	}
}
