package resource

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestPaymentProviderAdapterSourceLifecycle(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	class := filepath.Join(root, "force-app/main/default/classes/ProbeAdapter.cls")
	writeFile(t, class, `global class ProbeAdapter implements commercepayments.PaymentGatewayAdapter { global commercepayments.GatewayResponse processRequest(commercepayments.PaymentGatewayContext context) { return null; } }`)
	writeFile(t, filepath.Join(root, "force-app/main/default/paymentGatewayProviders/ProbeGateway.paymentGatewayProvider-meta.xml"), `<PaymentGatewayProvider><apexAdapter>ProbeAdapter</apexAdapter><masterLabel>Probe Gateway</masterLabel><idempotencySupported>Yes</idempotencySupported></PaymentGatewayProvider>`)
	load := func() project.Project {
		p, err := project.Load(root)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	org := storage.NewOrgState()
	if err := ApplyProject(&org, load()); err != nil {
		t.Fatal(err)
	}
	var classID storage.ID
	for _, r := range org.Objects["PaymentGatewayProvider"].Records {
		classID = r.Fields["ApexAdapterId"].ID
	}
	if classID == "" {
		t.Fatal("missing adapter")
	}
	updated := `global class ProbeAdapter implements commercepayments.PaymentGatewayAdapter { public Integer marker; global commercepayments.GatewayResponse processRequest(commercepayments.PaymentGatewayContext context) { return null; } }`
	writeFile(t, class, updated)
	if err := ApplyProject(&org, load()); err != nil {
		t.Fatal(err)
	}
	if got := org.Objects["ApexClass"].Records[classID].Fields["Body"].String; got != updated {
		t.Fatalf("stale adapter body: %q", got)
	}
	for _, bad := range []string{`public interface ProbeAdapter {}`, `public class ProbeAdapter { this is not valid Apex }`} {
		writeFile(t, class, bad)
		if err := ApplyProject(&org, load()); err != nil {
			t.Fatal(err)
		}
		if rows := org.Objects["PaymentGatewayProvider"].Records; len(rows) != 0 {
			t.Fatalf("invalid adapter projected: %#v", rows)
		}
	}
	writeFile(t, class, updated)
	if err := ApplyProject(&org, load()); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(class); err != nil {
		t.Fatal(err)
	}
	if err := ApplyProject(&org, load()); err != nil {
		t.Fatal(err)
	}
	if rows := org.Objects["PaymentGatewayProvider"].Records; len(rows) != 0 {
		t.Fatalf("removed adapter projected: %#v", rows)
	}
}

// The provider deploy rejected a class that did not implement the interface.
func TestPaymentProviderRejectsPlainAdapterClass(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/ProbeAdapter.cls"), `public class ProbeAdapter {}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/paymentGatewayProviders/ProbeGateway.paymentGatewayProvider-meta.xml"), `<PaymentGatewayProvider><apexAdapter>ProbeAdapter</apexAdapter><masterLabel>Probe Gateway</masterLabel><idempotencySupported>Yes</idempotencySupported></PaymentGatewayProvider>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	if rows := org.Objects["PaymentGatewayProvider"].Records; len(rows) != 0 {
		t.Fatalf("plain class yielded provider: %#v", rows)
	}
}

func TestPaymentProviderRejectsAdapterWithoutRequiredMethod(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/ProbeAdapter.cls"), `global class ProbeAdapter implements commercepayments.PaymentGatewayAdapter {}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/paymentGatewayProviders/ProbeGateway.paymentGatewayProvider-meta.xml"), `<PaymentGatewayProvider><apexAdapter>ProbeAdapter</apexAdapter><masterLabel>Probe Gateway</masterLabel><idempotencySupported>Yes</idempotencySupported></PaymentGatewayProvider>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	if rows := org.Objects["PaymentGatewayProvider"].Records; len(rows) != 0 {
		t.Fatalf("incomplete adapter yielded provider: %#v", rows)
	}
}
