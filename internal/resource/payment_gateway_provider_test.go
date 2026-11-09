package resource

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestPaymentGatewayProviderMetadataStableIdentity(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "force-app/main/default/paymentGatewayProviders/ProbeGateway.paymentGatewayProvider-meta.xml")
	writeFile(t, path, `<PaymentGatewayProvider xmlns="http://soap.sforce.com/2006/04/metadata"><apexAdapter>ProbeAdapter</apexAdapter><comments>synthetic probe</comments><idempotencySupported>Yes</idempotencySupported><masterLabel>Probe Gateway</masterLabel></PaymentGatewayProvider>`)
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/ProbeAdapter.cls"), `global class ProbeAdapter implements commercepayments.PaymentGatewayAdapter { global commercepayments.GatewayResponse processRequest(commercepayments.PaymentGatewayContext context) { return null; } }`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.PaymentGatewayProviderFiles) != 1 {
		t.Fatalf("provider discovery: %v", p.PaymentGatewayProviderFiles)
	}
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "ApexClass")
	class := org.Objects["ApexClass"]
	class.Records = map[storage.ID]storage.Record{"01p000000000001": {ID: "01p000000000001", Object: "ApexClass", MetadataSourceUnit: ProjectApexClassSourceUnit(root), Fields: map[string]storage.Value{"Name": storage.StringValue("ProbeAdapter"), "NamespacePrefix": storage.NullValue(), "Body": storage.StringValue(`global class ProbeAdapter implements commercepayments.PaymentGatewayAdapter { global commercepayments.GatewayResponse processRequest(commercepayments.PaymentGatewayContext context) { return null; } }`)}}}
	org.Objects["ApexClass"] = class
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	rows := org.Objects["PaymentGatewayProvider"].Records
	if len(rows) != 1 {
		t.Fatalf("provider rows: %#v", rows)
	}
	var id storage.ID
	for key, row := range rows {
		id = key
		if row.Fields["ApexAdapterId"].ID != "01p000000000001" || row.Fields["MasterLabel"].String != "Probe Gateway" || row.Fields["IdempotencySupported"].String != "Yes" {
			t.Fatalf("provider fields: %#v", row.Fields)
		}
	}
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	if rows = org.Objects["PaymentGatewayProvider"].Records; len(rows) != 1 || rows[id].ID != id {
		t.Fatalf("repeat projection: %#v", rows)
	}
}

// The shared loader also serves routes that have not preseeded ApexClass rows.
func TestPaymentGatewayProviderMetadataLoadsWithoutPreseededClasses(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/ProbeAdapter.cls"), `global class ProbeAdapter implements commercepayments.PaymentGatewayAdapter { global commercepayments.GatewayResponse processRequest(commercepayments.PaymentGatewayContext context) { return null; } }`)
	writeFile(t, filepath.Join(root, "force-app/main/default/paymentGatewayProviders/ProbeGateway.paymentGatewayProvider-meta.xml"), `<PaymentGatewayProvider xmlns="http://soap.sforce.com/2006/04/metadata"><apexAdapter>ProbeAdapter</apexAdapter><idempotencySupported>Yes</idempotencySupported><masterLabel>Probe Gateway</masterLabel></PaymentGatewayProvider>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	rows := org.Objects["PaymentGatewayProvider"].Records
	if len(rows) != 1 {
		t.Fatalf("provider rows: %#v", rows)
	}
	for _, row := range rows {
		id := row.Fields["ApexAdapterId"].ID
		if id == "" || org.Objects["ApexClass"].Records[id].Fields["Name"].String != "ProbeAdapter" {
			t.Fatalf("adapter reference: %q", id)
		}
	}
}
