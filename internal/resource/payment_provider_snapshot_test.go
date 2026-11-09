package resource

import (
	"fmt"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"path/filepath"
	"testing"
)

func TestPaymentProviderUsesCapturedAdapterSource(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	class := filepath.Join(root, "force-app/main/default/classes/ProbeAdapter.cls")
	writeFile(t, class, `public class ProbeAdapter {}`)
	path := filepath.Join(root, "force-app/main/default/paymentGatewayProviders/ProbeGateway.paymentGatewayProvider-meta.xml")
	metadata := `<PaymentGatewayProvider><apexAdapter>ProbeAdapter</apexAdapter><masterLabel>Probe Gateway</masterLabel><idempotencySupported>Yes</idempotencySupported></PaymentGatewayProvider>`
	writeFile(t, path, metadata)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	adapter := `global class ProbeAdapter implements commercepayments.PaymentGatewayAdapter { global commercepayments.GatewayResponse processRequest(commercepayments.PaymentGatewayContext context) { return null; } }`
	read := func(path string) ([]byte, error) {
		switch filepath.Clean(path) {
		case filepath.Clean(class):
			return []byte(adapter), nil
		case filepath.Clean(p.PaymentGatewayProviderFiles[0]):
			return []byte(metadata), nil
		}
		return nil, fmt.Errorf("unexpected path")
	}
	org := storage.NewOrgState()
	if err := ApplyProject(&org, p, read); err != nil {
		t.Fatal(err)
	}
	if rows := org.Objects["PaymentGatewayProvider"].Records; len(rows) != 1 {
		t.Fatalf("captured adapter row: %#v", rows)
	}
}

func TestPaymentProviderMissingCapturedAdapterIsError(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/ProbeAdapter.cls"), `global class ProbeAdapter implements commercepayments.PaymentGatewayAdapter { global commercepayments.GatewayResponse processRequest(commercepayments.PaymentGatewayContext context) { return null; } }`)
	path := filepath.Join(root, "force-app/main/default/paymentGatewayProviders/ProbeGateway.paymentGatewayProvider-meta.xml")
	metadata := `<PaymentGatewayProvider><apexAdapter>ProbeAdapter</apexAdapter><masterLabel>Probe Gateway</masterLabel></PaymentGatewayProvider>`
	writeFile(t, path, metadata)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	read := func(path string) ([]byte, error) {
		if filepath.Clean(path) == filepath.Clean(p.PaymentGatewayProviderFiles[0]) {
			return []byte(metadata), nil
		}
		return nil, fmt.Errorf("captured adapter missing")
	}
	org := storage.NewOrgState()
	if err := ApplyProject(&org, p, read); err == nil {
		t.Fatal("missing captured source was silently ignored")
	}
}
