package resource

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestPaymentProviderAdapterRequiresMatchingSourceOwner(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	body := `global class ProbeAdapter implements commercepayments.PaymentGatewayAdapter { global commercepayments.GatewayResponse processRequest(commercepayments.PaymentGatewayContext context) { return null; } }`
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/ProbeAdapter.cls"), body)
	writeFile(t, filepath.Join(root, "force-app/main/default/paymentGatewayProviders/ProbeGateway.paymentGatewayProvider-meta.xml"), `<PaymentGatewayProvider><apexAdapter>ProbeAdapter</apexAdapter><masterLabel>Probe</masterLabel></PaymentGatewayProvider>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, owned := range []bool{false, true} {
		t.Run(fmt.Sprintf("owned=%v", owned), func(t *testing.T) {
			org := storage.NewOrgState()
			storage.EnsureStandardObject(&org, "ApexClass")
			object := org.Objects["ApexClass"]
			unit := "unrelated-project"
			classBody := body
			if owned {
				digest := sha256.Sum256([]byte(projectDirectoryIdentity(root)))
				unit = fmt.Sprintf("glade:project-apex-class:%x", digest)
				classBody = "old captured body"
			}
			object.Records = map[storage.ID]storage.Record{"01p000000000001": {ID: "01p000000000001", Object: "ApexClass", MetadataSourceUnit: unit, Fields: map[string]storage.Value{"Name": storage.StringValue("ProbeAdapter"), "Body": storage.StringValue(classBody)}}}
			org.Objects["ApexClass"] = object
			if err := ApplyProject(&org, p); err != nil {
				t.Fatal(err)
			}
			for _, row := range org.Objects["PaymentGatewayProvider"].Records {
				same := row.Fields["ApexAdapterId"].ID == "01p000000000001"
				if same != owned {
					t.Fatalf("adapter was bound to wrong owner: %#v", row.Fields)
				}
				if owned && org.Objects["ApexClass"].Records["01p000000000001"].Fields["Body"].String != body {
					t.Fatal("owned adapter source was not refreshed")
				}
				return
			}
			t.Fatal("missing provider")
		})
	}
}
