package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRefactorRenameDryRunJSONDoesNotWrite(t *testing.T) {
	root := writeRefactorRenameProject(t)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"refactor", "rename", "--project", root, "--symbol", "InvoiceService", "--to", "BillingService", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	var got struct {
		Command string `json:"command"`
		Status  string `json:"status"`
		Data    struct {
			DryRun bool `json:"dryRun"`
			Edits  []struct {
				File        string `json:"file"`
				Replacement string `json:"replacement"`
			} `json:"edits"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("json decode: %v\n%s", err, stdout.String())
	}
	if got.Command != "refactor rename" || got.Status != "passed" || !got.Data.DryRun || len(got.Data.Edits) == 0 {
		t.Fatalf("json envelope = %#v", got)
	}
	if strings.Contains(readCLIRefactorFile(t, filepath.Join(root, "force-app/main/default/classes/InvoiceService.cls")), "BillingService") {
		t.Fatalf("dry run wrote declaration file")
	}
}

func TestRunRefactorRenameWriteByLocation(t *testing.T) {
	root := writeRefactorRenameProject(t)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"refactor", "rename",
		"--project", root,
		"--file", "force-app/main/default/classes/Consumer.cls",
		"--line", "4",
		"--column", "39",
		"--to", "netTotal",
		"--write",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	consumer := readCLIRefactorFile(t, filepath.Join(root, "force-app/main/default/classes/Consumer.cls"))
	service := readCLIRefactorFile(t, filepath.Join(root, "force-app/main/default/classes/InvoiceService.cls"))
	if !strings.Contains(consumer, "service.netTotal(account)") || !strings.Contains(service, "Decimal netTotal(Account account)") {
		t.Fatalf("write did not rename exact member refs:\nservice:\n%s\nconsumer:\n%s", service, consumer)
	}
}

func TestRunRefactorRenameProgressWritesToStderr(t *testing.T) {
	root := writeRefactorRenameProject(t)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"refactor", "rename", "--project", root, "--symbol", "InvoiceService", "--to", "BillingService", "--dry-run", "--progress"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "Rename") {
		t.Fatalf("stdout missing rename result:\n%s", stdout.String())
	}
	for _, want := range []string{"refactor rename · Loading project", "refactor rename · 2/3 Indexing Apex symbols", "done · refactor complete"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, stderr.String())
		}
	}
}

func TestRunRefactorRenameRejectsInvalidSchemaSuffix(t *testing.T) {
	root := writeRefactorRenameProject(t)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"refactor", "rename", "--project", root, "--symbol", "Account.Name", "--to", "Name__c", "--json"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("exit code = 0, want failure; stdout=%q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "schema suffix") {
		t.Fatalf("stderr = %q, want schema suffix", stderr.String())
	}
}

func TestWriteRefactorRenameTextCapsEditsUnlessFull(t *testing.T) {
	data := refactorRenameData{Count: 82}
	for i := 0; i < 82; i++ {
		data.Edits = append(data.Edits, refactorRenameEditSummary{
			File:        "force-app/main/default/classes/Consumer.cls",
			Line:        i + 1,
			Column:      3,
			Original:    "oldName",
			Replacement: "newName",
		})
	}
	var out bytes.Buffer
	writeRefactorRenameText(&out, data, false)
	got := out.String()
	if strings.Contains(got, "Consumer.cls:82:3") {
		t.Fatalf("capped output included edit 82:\n%s", got)
	}
	if !strings.Contains(got, "... 2 more edits omitted. Use --full for complete output.") {
		t.Fatalf("missing omitted-count line:\n%s", got)
	}

	out.Reset()
	writeRefactorRenameText(&out, data, true)
	if !strings.Contains(out.String(), "Consumer.cls:82:3") {
		t.Fatalf("full output omitted edit 82:\n%s", out.String())
	}
}

func writeRefactorRenameProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"65.0"}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/InvoiceService.cls"), `public class InvoiceService {
    public Decimal total(Account account) {
        return account.Name == null ? 0 : 1;
    }
}
`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/Consumer.cls"), `public class Consumer {
    public Decimal run(Account account) {
        InvoiceService service = new InvoiceService();
        Decimal amount = service.total(account);
        return account.Name == null ? amount : amount + 1;
    }
}
`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/objects/Account/Account.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Account</label><pluralLabel>Accounts</pluralLabel></CustomObject>`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fields/Name.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Name</fullName><label>Name</label><type>Text</type></CustomField>`)
	return root
}

func readCLIRefactorFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	return string(data)
}
