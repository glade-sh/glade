package resource

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// Salesforce accepts a provider whose adapter inherits the interface and method:
// testdata/conformance/csnpe_capture/inherit-org.tsv:2-5.
func TestPaymentProviderInheritedAdapter(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			classes := filepath.Join(root, "force-app/main/default/classes")
			writeFile(t, filepath.Join(classes, "ProbeBaseAdapter.cls"), `global virtual class ProbeBaseAdapter implements commercepayments.PaymentGatewayAdapter { global virtual commercepayments.GatewayResponse processRequest(commercepayments.PaymentGatewayContext context) { return null; } }`)
			writeFile(t, filepath.Join(classes, "ProbeInherited.cls"), `global class ProbeInherited extends ProbeBaseAdapter {}`)
			writeFile(t, filepath.Join(root, "force-app/main/default/paymentGatewayProviders/ProbeGateway.paymentGatewayProvider-meta.xml"), `<PaymentGatewayProvider><apexAdapter>ProbeInherited</apexAdapter><masterLabel>Probe Gateway</masterLabel><idempotencySupported>Yes</idempotencySupported></PaymentGatewayProvider>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			paths, err := PaymentGatewayProviderSourceFiles(p, os.ReadFile)
			if err != nil {
				t.Fatal(err)
			}
			if len(paths) != 2 {
				t.Errorf("captured adapter source dependencies = %v, want child and parent", paths)
			}
			org := storage.NewOrgState()
			if err := ApplyProject(&org, p); err != nil {
				t.Fatal(err)
			}
			if got := len(org.Objects["PaymentGatewayProvider"].Records); got != 1 {
				t.Fatalf("inherited adapter providers = %d, want 1", got)
			}
		})
	}
}
