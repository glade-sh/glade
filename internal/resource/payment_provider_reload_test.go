package resource

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestPaymentProviderMetadataReloadAndDeletedAdapter(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/ProbeAdapter.cls"), `global class ProbeAdapter implements commercepayments.PaymentGatewayAdapter { global commercepayments.GatewayResponse processRequest(commercepayments.PaymentGatewayContext context) { return null; } }`)
	path := filepath.Join(root, "force-app/main/default/paymentGatewayProviders/ProbeGateway.paymentGatewayProvider-meta.xml")
	source := func(label string) string {
		return `<PaymentGatewayProvider><apexAdapter>ProbeAdapter</apexAdapter><masterLabel>` + label + `</masterLabel><idempotencySupported>Yes</idempotencySupported></PaymentGatewayProvider>`
	}
	writeFile(t, path, source("Before"))
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "ApexClass")
	obj := org.Objects["ApexClass"]
	obj.Records = map[storage.ID]storage.Record{"01p000000000001": {ID: "01p000000000001", Object: "ApexClass", System: storage.SystemFields{IsDeleted: true}, Fields: map[string]storage.Value{"Name": storage.StringValue("ProbeAdapter")}}}
	org.Objects["ApexClass"] = obj
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	rows := org.Objects["PaymentGatewayProvider"].Records
	if len(rows) != 1 {
		t.Fatalf("initial rows: %#v", rows)
	}
	var id storage.ID
	for key, row := range rows {
		id = key
		if row.Fields["ApexAdapterId"].ID == "01p000000000001" {
			t.Fatal("deleted adapter reused")
		}
	}
	writeFile(t, path, source("After"))
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	if got := org.Objects["PaymentGatewayProvider"].Records[id].Fields["MasterLabel"].String; got != "After" {
		t.Fatalf("reloaded label: %q", got)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	p, err = project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	if rows := org.Objects["PaymentGatewayProvider"].Records; len(rows) != 0 {
		t.Fatalf("removed metadata rows: %#v", rows)
	}
}

func TestPaymentProviderReloadPreservesUnrelatedRowsAndIndexes(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/ProbeAdapter.cls"), `global class ProbeAdapter implements commercepayments.PaymentGatewayAdapter { global commercepayments.GatewayResponse processRequest(commercepayments.PaymentGatewayContext context) { return null; } }`)
	path := filepath.Join(root, "force-app/main/default/paymentGatewayProviders/ProbeGateway.paymentGatewayProvider-meta.xml")
	writeFile(t, path, `<PaymentGatewayProvider><apexAdapter>ProbeAdapter</apexAdapter><masterLabel>Probe Gateway</masterLabel><idempotencySupported>Yes</idempotencySupported></PaymentGatewayProvider>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "PaymentGatewayProvider")
	obj := org.Objects["PaymentGatewayProvider"]
	obj.Definition.Indexes = []storage.IndexDefinition{{Name: "by_name", Fields: []string{"DeveloperName"}}}
	obj.Records = map[storage.ID]storage.Record{"zzz000000000001": {ID: "zzz000000000001", Object: "PaymentGatewayProvider", Fields: map[string]storage.Value{"DeveloperName": storage.StringValue("Unrelated")}}}
	org.Objects["PaymentGatewayProvider"] = obj
	storage.RebuildObjectIndexes(&org, "PaymentGatewayProvider")
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	ids, ok := storage.LookupIndex(org.Objects["PaymentGatewayProvider"], "DeveloperName", storage.StringValue("ProbeGateway"))
	if !ok || len(ids) != 1 {
		t.Fatalf("inserted index: %v, %v", ids, ok)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	p, err = project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	obj = org.Objects["PaymentGatewayProvider"]
	ids, ok = storage.LookupIndex(obj, "DeveloperName", storage.StringValue("ProbeGateway"))
	if !ok || len(ids) != 0 || len(obj.Records) != 1 || obj.Records["zzz000000000001"].Fields["DeveloperName"].String != "Unrelated" {
		t.Fatalf("removed index or unrelated row: %v %v %#v", ids, ok, obj.Records)
	}
}

func TestPaymentProviderReloadPreservesOtherProjects(t *testing.T) {
	makeProject := func(root, name string) project.Project {
		writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/ProbeAdapter.cls"), `global class ProbeAdapter implements commercepayments.PaymentGatewayAdapter { global commercepayments.GatewayResponse processRequest(commercepayments.PaymentGatewayContext context) { return null; } }`)
		writeFile(t, filepath.Join(root, "force-app/main/default/paymentGatewayProviders", name+".paymentGatewayProvider-meta.xml"), `<PaymentGatewayProvider><apexAdapter>ProbeAdapter</apexAdapter><masterLabel>Probe Gateway</masterLabel><idempotencySupported>Yes</idempotencySupported></PaymentGatewayProvider>`)
		p, err := project.Load(root)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	a := makeProject(t.TempDir(), "GatewayA")
	b := makeProject(t.TempDir(), "GatewayB")
	org := storage.NewOrgState()
	if err := ApplyProject(&org, a); err != nil {
		t.Fatal(err)
	}
	if err := ApplyProject(&org, b); err != nil {
		t.Fatal(err)
	}
	rows := org.Objects["PaymentGatewayProvider"].Records
	if len(rows) != 2 {
		t.Fatalf("independent project rows: %#v", rows)
	}
	adapters := map[storage.ID]bool{}
	for _, row := range rows {
		adapters[row.Fields["ApexAdapterId"].ID] = true
	}
	if len(adapters) != 2 {
		t.Fatalf("projects share synthesized adapter: %#v", rows)
	}
}

func TestPaymentProviderReloadAfterProjectMove(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "before")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/ProbeAdapter.cls"), `global class ProbeAdapter implements commercepayments.PaymentGatewayAdapter { global commercepayments.GatewayResponse processRequest(commercepayments.PaymentGatewayContext context) { return null; } }`)
	path := filepath.Join(root, "force-app/main/default/paymentGatewayProviders/ProbeGateway.paymentGatewayProvider-meta.xml")
	writeFile(t, path, `<PaymentGatewayProvider><apexAdapter>ProbeAdapter</apexAdapter><masterLabel>Probe Gateway</masterLabel><idempotencySupported>Yes</idempotencySupported></PaymentGatewayProvider>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(parent, "after")
	if err := os.Rename(root, moved); err != nil {
		t.Fatal(err)
	}
	p, err = project.Load(moved)
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	if classes := org.Objects["ApexClass"].Records; len(classes) != 1 {
		t.Fatalf("project move left stale adapter class: %#v", classes)
	}
	path = filepath.Join(moved, "force-app/main/default/paymentGatewayProviders/ProbeGateway.paymentGatewayProvider-meta.xml")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	p, err = project.Load(moved)
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	if rows := org.Objects["PaymentGatewayProvider"].Records; len(rows) != 0 {
		t.Fatalf("provider survived moved project deletion: %#v", rows)
	}
}

func TestPaymentProviderDoesNotTransferSameNameFromOtherProject(t *testing.T) {
	makeProject := func(root, label string) project.Project {
		writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
		writeFile(t, filepath.Join(root, "force-app/main/default/classes/ProbeAdapter.cls"), `global class ProbeAdapter implements commercepayments.PaymentGatewayAdapter { global commercepayments.GatewayResponse processRequest(commercepayments.PaymentGatewayContext context) { return null; } }`)
		writeFile(t, filepath.Join(root, "force-app/main/default/paymentGatewayProviders/ProbeGateway.paymentGatewayProvider-meta.xml"), `<PaymentGatewayProvider><apexAdapter>ProbeAdapter</apexAdapter><masterLabel>`+label+`</masterLabel><idempotencySupported>Yes</idempotencySupported></PaymentGatewayProvider>`)
		p, err := project.Load(root)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	a := makeProject(t.TempDir(), "First")
	b := makeProject(t.TempDir(), "Second")
	org := storage.NewOrgState()
	if err := ApplyProject(&org, a); err != nil {
		t.Fatal(err)
	}
	if err := ApplyProject(&org, b); err != nil {
		t.Fatal(err)
	}
	if classes := org.Objects["ApexClass"].Records; len(classes) != 1 {
		t.Fatalf("conflicting provider created an orphan adapter: %#v", classes)
	}
	for _, row := range org.Objects["PaymentGatewayProvider"].Records {
		if row.Fields["MasterLabel"].String != "First" {
			t.Fatalf("existing provider was claimed by a different project: %#v", row.Fields)
		}
		return
	}
	t.Fatal("provider absent")
}

func TestPaymentProviderMigratesPathBasedOwnership(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/ProbeAdapter.cls"), `global class ProbeAdapter implements commercepayments.PaymentGatewayAdapter { global commercepayments.GatewayResponse processRequest(commercepayments.PaymentGatewayContext context) { return null; } }`)
	path := filepath.Join(root, "force-app/main/default/paymentGatewayProviders/ProbeGateway.paymentGatewayProvider-meta.xml")
	writeFile(t, path, `<PaymentGatewayProvider><apexAdapter>ProbeAdapter</apexAdapter><masterLabel>Before</masterLabel></PaymentGatewayProvider>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(filepath.Clean(root)))
	legacy := fmt.Sprintf("glade:payment-gateway-provider:%x", digest)
	for _, kind := range []string{"PaymentGatewayProvider", "ApexClass"} {
		obj := org.Objects[kind]
		for id, row := range obj.Records {
			row.MetadataSourceUnit = legacy
			if kind == "ApexClass" {
				row.MetadataSourceUnit += ":adapter"
			}
			obj.Records[id] = row
		}
		org.Objects[kind] = obj
	}
	writeFile(t, path, `<PaymentGatewayProvider><apexAdapter>ProbeAdapter</apexAdapter><masterLabel>After</masterLabel></PaymentGatewayProvider>`)
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	for _, row := range org.Objects["PaymentGatewayProvider"].Records {
		if row.Fields["MasterLabel"].String != "After" {
			t.Fatal("legacy source not refreshed")
		}
		if row.MetadataSourceUnit == legacy {
			t.Fatal("legacy source not migrated")
		}
	}
}

func TestPaymentProviderDetachedAdapterRefreshPreservesSnapshot(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	classPath := filepath.Join(root, "force-app/main/default/classes/ProbeAdapter.cls")
	before := `global class ProbeAdapter implements commercepayments.PaymentGatewayAdapter { global commercepayments.GatewayResponse processRequest(commercepayments.PaymentGatewayContext context) { return null; } }`
	writeFile(t, classPath, before)
	providerPath := filepath.Join(root, "force-app/main/default/paymentGatewayProviders/ProbeGateway.paymentGatewayProvider-meta.xml")
	writeFile(t, providerPath, `<PaymentGatewayProvider><apexAdapter>ProbeAdapter</apexAdapter><masterLabel>Probe</masterLabel></PaymentGatewayProvider>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	var snapshot storage.Record
	for _, row := range org.Objects["ApexClass"].Records {
		if row.Fields["Name"].String == "ProbeAdapter" {
			snapshot = row
			break
		}
	}
	if snapshot.Fields == nil {
		t.Fatal("adapter missing")
	}
	oldBody := snapshot.Fields["Body"].String
	if err := os.Remove(providerPath); err != nil {
		t.Fatal(err)
	}
	writeFile(t, classPath, before+"\n// updated\n")
	p, err = project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	if got := snapshot.Fields["Body"].String; got != oldBody {
		t.Fatalf("snapshot body changed: %q", got)
	}
}
