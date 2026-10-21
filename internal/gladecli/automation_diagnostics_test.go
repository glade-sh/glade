package gladecli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
)

func TestRunCheckReportsUnparsableFlowFileAsWarning(t *testing.T) {
	t.Cleanup(checkSemanticResults.Reset)
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/A.cls"), `public class A {}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/flows/Broken_Flow.flow-meta.xml"), `<Flow xmlns="http://soap.sforce.com/2006/04/metadata"><status>Active</status><start><object>Account</object>`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"check", "--project", root, "--json", "--no-progress", "--no-cache"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 for a warning; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	out := stdout.String()
	if !strings.Contains(out, `"code": "GLADEAUTO003"`) || !strings.Contains(out, "Broken_Flow.flow-meta.xml") || !strings.Contains(out, `"severity": "warning"`) {
		t.Fatalf("stdout = %s", out)
	}
}

func TestRunCheckAllValidFlowsAddNoAutomationDiagnostics(t *testing.T) {
	t.Cleanup(checkSemanticResults.Reset)
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/A.cls"), `public class A {}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/flows/Account_Status.flow-meta.xml"), `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <processType>AutoLaunchedFlow</processType>
  <status>Active</status>
  <start>
    <object>Account</object>
    <triggerType>RecordBeforeSave</triggerType>
    <connector><targetReference>Set_Status</targetReference></connector>
  </start>
  <assignments>
    <name>Set_Status</name>
    <assignmentItems>
      <assignToReference>$Record.Description</assignToReference>
      <operator>Assign</operator>
      <value><stringValue>flowed</stringValue></value>
    </assignmentItems>
  </assignments>
</Flow>`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"check", "--project", root, "--json", "--no-progress", "--no-cache"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if strings.Contains(stdout.String(), "GLADEAUTO") {
		t.Fatalf("stdout = %s", stdout.String())
	}
}

func TestAutomationWarningMessagesNameAllDiagnostics(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "project")
	got := automationWarningMessages(root, []diagnostic.Diagnostic{
		{Severity: diagnostic.Warning, Code: "GLADEAUTO003", Message: "Broken_Flow: flow file could not be loaded and will not run: XML syntax error on line 1: unexpected EOF", File: filepath.Join(root, "flows", "Broken_Flow.flow-meta.xml")},
		{Severity: diagnostic.Warning, Code: "GLADEAUTO002", Message: "Other: screen node \"S\" is not supported", File: filepath.Join(root, "flows", "Other.flow-meta.xml")},
		{Severity: diagnostic.Warning, Code: "GLADEAUTO004", Message: "Invalid: unsupported entry condition", File: filepath.Join(root, "flows", "Invalid.flow-meta.xml")},
	})
	if len(got) != 3 {
		t.Fatalf("messages = %#v", got)
	}
	if !strings.Contains(got[0], filepath.Join("flows", "Broken_Flow.flow-meta.xml")) || strings.Contains(got[0], root+string(filepath.Separator)) || !strings.Contains(got[0], "XML syntax error") {
		t.Fatalf("load failure message = %q", got[0])
	}
	for i, want := range []string{
		"GLADEAUTO002: Other: screen node \"S\" is not supported",
		"GLADEAUTO004: Invalid: unsupported entry condition",
	} {
		if !strings.Contains(got[i+1], want) || !strings.Contains(got[i+1], filepath.Join("flows", []string{"Other.flow-meta.xml", "Invalid.flow-meta.xml"}[i])) {
			t.Fatalf("diagnostic message = %q, want details %q", got[i+1], want)
		}
	}
	if msgs := automationWarningMessages(root, nil); len(msgs) != 0 {
		t.Fatalf("messages for no diagnostics = %#v", msgs)
	}
}
