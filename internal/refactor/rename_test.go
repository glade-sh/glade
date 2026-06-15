package refactor_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/codeintel"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/packageartifact"
	"github.com/glade-sh/glade/internal/refactor"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestPlanRenameDryRunReturnsEditsWithoutWriting(t *testing.T) {
	root := t.TempDir()
	sourcePath := writeRenameFile(t, root, "classes/InvoiceService.cls", "public class InvoiceService {}\n")
	index := renameIndex(root, sourcePath, "InvoiceService", nil)

	plan, err := refactor.PlanRename(index, refactor.RenameOptions{Symbol: "InvoiceService", To: "BillingService"})
	if err != nil {
		t.Fatalf("PlanRename returned error: %v", err)
	}
	if len(plan.Edits) != 1 || plan.Edits[0].Replacement != "BillingService" {
		t.Fatalf("plan edits = %#v", plan.Edits)
	}
	if got := readRenameFile(t, sourcePath); !strings.Contains(got, "InvoiceService") {
		t.Fatalf("dry run wrote file:\n%s", got)
	}
}

func TestApplyRenameChangesExactReferences(t *testing.T) {
	root := t.TempDir()
	servicePath := writeRenameFile(t, root, "classes/InvoiceService.cls", "public class InvoiceService {}\n")
	consumerPath := writeRenameFile(t, root, "classes/Consumer.cls", "public class Consumer { InvoiceService svc; }\n")
	index := renameIndex(root, servicePath, "InvoiceService", []codeintel.Use{
		useAt(consumerPath, "InvoiceService", 1, 25, codeintel.ApexTypeID("", "InvoiceService")),
	})

	plan, err := refactor.PlanRename(index, refactor.RenameOptions{Symbol: "InvoiceService", To: "BillingService"})
	if err != nil {
		t.Fatalf("PlanRename returned error: %v", err)
	}
	if err := refactor.Apply(plan); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}

	if got := readRenameFile(t, servicePath); !strings.Contains(got, "public class BillingService") {
		t.Fatalf("declaration was not renamed:\n%s", got)
	}
	if got := readRenameFile(t, consumerPath); !strings.Contains(got, "BillingService svc") {
		t.Fatalf("reference was not renamed:\n%s", got)
	}
}

func TestPlanRenameAmbiguousMemberNameFails(t *testing.T) {
	root := t.TempDir()
	firstPath := writeRenameFile(t, root, "classes/InvoiceService.cls", "public class InvoiceService { public Decimal total(){ return 0; } }\n")
	secondPath := writeRenameFile(t, root, "classes/QuoteService.cls", "public class QuoteService { public Decimal total(){ return 0; } }\n")
	index := renameIndex(root, firstPath, "InvoiceService", nil)
	index.Types = append(index.Types, typesys.TypeSymbol{
		Kind:  apexast.DeclarationClass,
		Name:  "QuoteService",
		File:  secondPath,
		Range: rangeAt(1, 1, 1, 26, 0, len("public class QuoteService")),
		Members: []typesys.MemberSymbol{{
			Kind:  apexast.DeclarationMethod,
			Name:  "total",
			Type:  "Decimal",
			Range: rangeAt(1, 43, 1, 48, strings.Index(readRenameFile(t, secondPath), "total"), strings.Index(readRenameFile(t, secondPath), "total")+len("total")),
		}},
	})
	index.Types[0].Members = []typesys.MemberSymbol{{
		Kind:  apexast.DeclarationMethod,
		Name:  "total",
		Type:  "Decimal",
		Range: rangeAt(1, 45, 1, 50, strings.Index(readRenameFile(t, firstPath), "total"), strings.Index(readRenameFile(t, firstPath), "total")+len("total")),
	}}

	_, err := refactor.PlanRename(index, refactor.RenameOptions{Symbol: "total", To: "netTotal"})
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("PlanRename error = %v, want ambiguous", err)
	}
}

func TestPlanRenameSchemaFieldUpdatesApexReferencesNotMetadataXML(t *testing.T) {
	root := t.TempDir()
	apexPath := writeRenameFile(t, root, "classes/Reader.cls", "public class Reader { Decimal v = new Invoice__c(Amount__c = 1).Amount__c; }\n")
	fieldPath := writeRenameFile(t, root, "objects/Invoice__c/fields/Amount__c.field-meta.xml", `<CustomField><fullName>Amount__c</fullName></CustomField>`)
	index := schemaRenameIndex(root, apexPath, fieldPath)

	plan, err := refactor.PlanRename(index, refactor.RenameOptions{Symbol: "Invoice__c.Amount__c", To: "Total__c"})
	if err != nil {
		t.Fatalf("PlanRename returned error: %v", err)
	}
	for _, edit := range plan.Edits {
		if strings.HasSuffix(edit.File, ".xml") {
			t.Fatalf("metadata XML edit was planned: %#v", edit)
		}
	}
	if err := refactor.Apply(plan); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if got := readRenameFile(t, apexPath); strings.Count(got, "Total__c") != 2 {
		t.Fatalf("apex references were not renamed exactly:\n%s", got)
	}
	if got := readRenameFile(t, fieldPath); !strings.Contains(got, "Amount__c") {
		t.Fatalf("metadata XML was changed:\n%s", got)
	}
}

func TestApplyRenameOverlappingEditsFail(t *testing.T) {
	root := t.TempDir()
	sourcePath := writeRenameFile(t, root, "classes/InvoiceService.cls", "public class InvoiceService {}\n")
	hash := refactorTestHash([]byte(readRenameFile(t, sourcePath)))
	plan := refactor.RenamePlan{Edits: []refactor.FileEdit{
		{File: sourcePath, Original: "InvoiceService", Replacement: "BillingService", StartOffset: 13, EndOffset: 27, OriginalHash: hash},
		{File: sourcePath, Original: "nvoiceService ", Replacement: "BillingService", StartOffset: 14, EndOffset: 28, OriginalHash: hash},
	}}

	err := refactor.Apply(plan)
	if err == nil || !strings.Contains(err.Error(), "overlap") {
		t.Fatalf("Apply error = %v, want overlap", err)
	}
}

func TestApplyRenameFailsWhenFileChangedBetweenPlanAndWrite(t *testing.T) {
	root := t.TempDir()
	sourcePath := writeRenameFile(t, root, "classes/InvoiceService.cls", "public class InvoiceService {}\n")
	index := renameIndex(root, sourcePath, "InvoiceService", nil)
	plan, err := refactor.PlanRename(index, refactor.RenameOptions{Symbol: "InvoiceService", To: "BillingService"})
	if err != nil {
		t.Fatalf("PlanRename returned error: %v", err)
	}
	writeRenameFile(t, root, "classes/InvoiceService.cls", "public class InvoiceService { Integer changed; }\n")

	err = refactor.Apply(plan)
	if err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("Apply error = %v, want changed file error", err)
	}
}

func renameIndex(root, sourcePath, typeName string, uses []codeintel.Use) typesys.Index {
	source := mustRead(sourcePath)
	start := strings.Index(source, typeName)
	return typesys.Index{
		Project: typesys.ProjectInfo{Root: root},
		Types: []typesys.TypeSymbol{{
			Kind:  apexast.DeclarationClass,
			Name:  typeName,
			File:  sourcePath,
			Range: rangeAt(1, start+1, 1, start+1+len(typeName), start, start+len(typeName)),
		}},
		CodeIntelUses: renameArtifactUses(uses),
	}
}

func schemaRenameIndex(root, apexPath, _ string) typesys.Index {
	source := mustRead(apexPath)
	first := strings.Index(source, "Amount__c")
	second := strings.LastIndex(source, "Amount__c")
	return typesys.Index{
		Project: typesys.ProjectInfo{Root: root},
		Objects: []schema.Object{{
			Name: "Invoice__c",
			Fields: []schema.Field{{
				Name: "Amount__c",
				Type: "Currency",
			}},
		}},
		CodeIntelUses: renameArtifactUses([]codeintel.Use{
			{SymbolID: codeintel.SObjectFieldID("Invoice__c", "Amount__c"), Kind: codeintel.UseWrite, Name: "Amount__c", File: apexPath, Range: rangeAt(1, first+1, 1, first+1+len("Amount__c"), first, first+len("Amount__c")), Resolved: true},
			{SymbolID: codeintel.SObjectFieldID("Invoice__c", "Amount__c"), Kind: codeintel.UseRead, Name: "Amount__c", File: apexPath, Range: rangeAt(1, second+1, 1, second+1+len("Amount__c"), second, second+len("Amount__c")), Resolved: true},
		}),
	}
}

func renameArtifactUses(uses []codeintel.Use) []packageartifact.CodeIntelUse {
	out := make([]packageartifact.CodeIntelUse, 0, len(uses))
	for _, use := range uses {
		out = append(out, packageartifact.CodeIntelUse{SymbolID: string(use.SymbolID), Kind: string(use.Kind), Name: use.Name, File: use.File, Range: use.Range, Context: string(use.Context), Resolved: use.Resolved, Metadata: use.Metadata})
	}
	return out
}

func useAt(file, name string, line, column int, id codeintel.SymbolID) codeintel.Use {
	source := mustRead(file)
	offset := strings.Index(source, name)
	return codeintel.Use{
		SymbolID: id,
		Kind:     codeintel.UseRead,
		Name:     name,
		File:     file,
		Range:    rangeAt(line, column, line, column+len(name), offset, offset+len(name)),
		Resolved: true,
	}
}

func rangeAt(startLine, startColumn, endLine, endColumn, startOffset, endOffset int) diagnostic.Range {
	return diagnostic.Range{
		Start: diagnostic.Position{Line: startLine, Column: startColumn, Offset: startOffset},
		End:   diagnostic.Position{Line: endLine, Column: endColumn, Offset: endOffset},
	}
}

func writeRenameFile(t *testing.T, root, rel, content string) string {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	return path
}

func readRenameFile(t *testing.T, path string) string {
	t.Helper()
	return mustRead(path)
}

func mustRead(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func refactorTestHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
