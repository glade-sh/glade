package gladecli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunTestAsyncExplicitCapsContracts(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"65.0"}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/CapWorker.cls"), `
public class CapWorker implements Queueable {
  public void execute(QueueableContext context) {
    System.assertEquals(100, Limits.getLimitQueries());
    System.assertEquals(10000, Limits.getLimitCpuTime());
    insert new Account(Name = 'explicit caps checked');
  }
}
`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/AsyncCapTest.cls"), `
@isTest
private class AsyncCapTest {
  @isTest static void preservesExplicitCaps() {
    Test.startTest();
    System.enqueueJob(new CapWorker());
    Test.stopTest();
    System.assertEquals(1, [SELECT COUNT() FROM Account WHERE Name = 'explicit caps checked']);
  }
}
`)
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"profile", []string{"--limit-profile", "strict-sync"}},
		{"flags", []string{"--limit-profile", "strict-async", "--limit-queries", "100", "--limit-cpu-ms", "10000"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"test", "--project", root, "--limit-mode", "strict"}, tc.args...)
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), args, &stdout, &stderr)
			if code != 0 || !strings.Contains(stdout.String(), "1 passed") {
				t.Fatalf("exit = %d; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
			}
		})
	}
}
