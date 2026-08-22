package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectDefinitionFindsSymbolByName(t *testing.T) {
	root := writeInspectIntelligenceProject(t)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"inspect", "definition", "--project", root, "--symbol", "InvoiceService"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	got := stdout.String()
	for _, want := range []string{
		"Definition",
		"symbol: InvoiceService",
		"kind: apex_type",
		"file: force-app/main/default/classes/InvoiceService.cls",
		"range: 1:1-5:2",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("definition output missing %q:\n%s", want, got)
		}
	}
}

func TestInspectDefinitionFindsSymbolByLocation(t *testing.T) {
	root := writeInspectIntelligenceProject(t)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"inspect", "definition",
		"--project", root,
		"--file", "force-app/main/default/classes/Consumer.cls",
		"--line", "3",
		"--column", "9",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	got := stdout.String()
	if !strings.Contains(got, "symbol: InvoiceService") || !strings.Contains(got, "file: force-app/main/default/classes/InvoiceService.cls") {
		t.Fatalf("location definition did not point at InvoiceService:\n%s", got)
	}
}

func TestInspectReferencesTextOmitsDeclarationByDefault(t *testing.T) {
	root := writeInspectIntelligenceProject(t)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"inspect", "references", "--project", root, "--symbol", "InvoiceService.total"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	got := stdout.String()
	for _, want := range []string{
		"References",
		"symbol: InvoiceService.total",
		"count: 1",
		"force-app/main/default/classes/Consumer.cls:4:34 call",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("references output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "InvoiceService.cls:2:25 declaration") {
		t.Fatalf("references output included declaration by default:\n%s", got)
	}
}

func TestInspectReferencesJSONUsesCLIEnvelope(t *testing.T) {
	root := writeInspectIntelligenceProject(t)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"inspect", "references", "--project", root, "--symbol", "InvoiceService.total", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	var got struct {
		Command string `json:"command"`
		Status  string `json:"status"`
		Data    struct {
			Symbol     string `json:"symbol"`
			References []struct {
				File string `json:"file"`
				Kind string `json:"kind"`
			} `json:"references"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("json decode: %v\n%s", err, stdout.String())
	}
	if got.Command != "inspect references" || got.Status != "passed" {
		t.Fatalf("envelope command/status = %q/%q", got.Command, got.Status)
	}
	if got.Data.Symbol != "InvoiceService.total" || len(got.Data.References) != 1 {
		t.Fatalf("references data = %#v", got.Data)
	}
	if got.Data.References[0].File != "force-app/main/default/classes/Consumer.cls" || got.Data.References[0].Kind != "call" {
		t.Fatalf("reference row = %#v", got.Data.References[0])
	}
}

func TestInspectReferencesCanIncludeDeclaration(t *testing.T) {
	root := writeInspectIntelligenceProject(t)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"inspect", "references", "--project", root, "--symbol", "Account.Name", "--include-declaration"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	got := stdout.String()
	for _, want := range []string{
		"symbol: Account.Name",
		"count: 5",
		"force-app/main/default/objects/Account/fields/Name.field-meta.xml:1:1 declaration",
		"force-app/main/default/classes/Consumer.cls:5:24 read",
		"force-app/main/default/classes/InvoiceService.cls:3:24 read",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("include-declaration output missing %q:\n%s", want, got)
		}
	}
}

func writeInspectIntelligenceProject(t *testing.T) string {
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
