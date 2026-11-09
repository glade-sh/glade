package apextest

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestPaymentProviderMetadataGenerationRetainsCapturedBytes(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	path := filepath.Join(root, "force-app/main/default/paymentGatewayProviders/ProbeGateway.paymentGatewayProvider-meta.xml")
	const original = `<PaymentGatewayProvider><masterLabel>Probe Gateway</masterLabel></PaymentGatewayProvider>`
	writeFile(t, path, original)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	sources := newSourceCache()
	sources.seedProjectSnapshot(&p)
	generation := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
	if err := sources.captureProjectRuntimeMetadata(typesys.Index{Project: typesys.ProjectInfo{Root: root}}, &generation); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, "changed after capture")
	data, err := sources.readCapturedProjectMetadata(path)
	if err != nil || string(data) != original || !generation.inputs[path].Present {
		t.Fatalf("metadata snapshot: %q, %v", data, err)
	}
}

func TestPaymentProviderGenerationCapturesAdapterSource(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	class := filepath.Join(root, "force-app/main/default/classes/ProbeAdapter.cls")
	const source = `global class ProbeAdapter implements commercepayments.PaymentGatewayAdapter { global commercepayments.GatewayResponse processRequest(commercepayments.PaymentGatewayContext context) { return null; } }`
	writeFile(t, class, source)
	writeFile(t, filepath.Join(root, "force-app/main/default/paymentGatewayProviders/ProbeGateway.paymentGatewayProvider-meta.xml"), `<PaymentGatewayProvider><apexAdapter>ProbeAdapter</apexAdapter><masterLabel>Probe Gateway</masterLabel></PaymentGatewayProvider>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	sources := newSourceCache()
	sources.seedProjectSnapshot(&p)
	generation := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
	if err := sources.captureProjectRuntimeMetadata(typesys.Index{Project: typesys.ProjectInfo{Root: root}}, &generation); err != nil {
		t.Fatal(err)
	}
	writeFile(t, class, `public class ProbeAdapter {}`)
	body, err := sources.readCapturedProjectMetadata(class)
	if err != nil || string(body) != source || !generation.inputs[class].Present {
		t.Fatalf("adapter snapshot: %q %v", body, err)
	}
}

func TestPaymentProviderGenerationUsesOneMetadataVersion(t *testing.T) {
	p := project.Project{PaymentGatewayProviderFiles: []string{"provider.paymentGatewayProvider-meta.xml"}, ApexFiles: []string{"First.cls", "Second.cls"}}
	bodies := map[string]string{}
	generation := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
	reads := 0
	read := func(path string) ([]byte, error) {
		switch path {
		case p.PaymentGatewayProviderFiles[0]:
			reads++
			if reads == 1 {
				return []byte(`<PaymentGatewayProvider><apexAdapter>First</apexAdapter></PaymentGatewayProvider>`), nil
			}
			return []byte(`<PaymentGatewayProvider><apexAdapter>Second</apexAdapter></PaymentGatewayProvider>`), nil
		case "First.cls":
			return []byte("first source"), nil
		case "Second.cls":
			return []byte("second source"), nil
		}
		return nil, fmt.Errorf("unexpected path")
	}
	if err := captureProviderAdapterGeneration(p, []string{p.PaymentGatewayProviderFiles[0]}, bodies, &generation, read); err != nil {
		t.Fatal(err)
	}
	if reads != 1 || bodies["First.cls"] != "first source" || bodies["Second.cls"] != "" {
		t.Fatalf("inconsistent metadata snapshot: reads=%d bodies=%v", reads, bodies)
	}
}

func TestPaymentProviderGenerationCapturesInheritedAdapter(t *testing.T) {
	p := project.Project{PaymentGatewayProviderFiles: []string{"provider.paymentGatewayProvider-meta.xml"}, ApexFiles: []string{"Child.cls", "Parent.cls"}}
	bodies := map[string]string{}
	generation := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
	inputs := map[string]string{
		p.PaymentGatewayProviderFiles[0]: `<PaymentGatewayProvider><apexAdapter>Child</apexAdapter></PaymentGatewayProvider>`,
		"Child.cls":                      `global class Child extends Parent {}`,
		"Parent.cls":                     `global virtual class Parent implements commercepayments.PaymentGatewayAdapter { global virtual commercepayments.GatewayResponse processRequest(commercepayments.PaymentGatewayContext context) { return null; } }`,
	}
	read := func(path string) ([]byte, error) {
		if body, ok := inputs[path]; ok {
			return []byte(body), nil
		}
		return nil, fmt.Errorf("unexpected path")
	}
	if err := captureProviderAdapterGeneration(p, p.PaymentGatewayProviderFiles, bodies, &generation, read); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"Child.cls", "Parent.cls"} {
		if bodies[path] != inputs[path] || !generation.inputs[path].Present {
			t.Fatalf("source dependency %q not captured", path)
		}
	}
}
