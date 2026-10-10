package visualforce

import (
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestVisualforceFormControllerReferenceRoundTrip(t *testing.T) {
	// The native extension controls retain the primary model across postbacks.
	// An independent instance of the same type must not become that model.
	controller := vm.Object("FormController")
	controller.Fields["actions"] = vm.Int(0)
	independent := vm.Object("FormController")
	independent.Fields["actions"] = vm.Int(0)
	fields := []map[string]vm.Value{{"model": controller, "independent": independent}}
	payload := ViewStatePayload{
		PageName:                "Form",
		ControllerType:          controller.Type,
		ControllerValues:        controller.Fields,
		ExtensionValues:         fields,
		ExtensionControllerRefs: extensionControllerReferences(controller, fields),
		CSRF:                    "form-controller-reference-test",
	}
	secret := []byte("form-controller-reference-test-key")
	encoded, err := EncodeViewState(payload, secret)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeViewState(encoded, secret)
	if err != nil {
		t.Fatal(err)
	}
	restored := vm.Object("FormController")
	applyValueFields(&restored, decoded.ControllerValues)
	extension := vm.Object("FormExtension")
	applyValueFields(&extension, decoded.ExtensionValues[0])
	restoreExtensionControllerReferences(&extension, decoded.ExtensionControllerRefs[0], restored)
	model := extension.Fields["model"]
	model.Fields["actions"] = vm.Int(1)
	if restored.Fields["actions"].Int != 1 || model.Ref != restored.Ref {
		t.Fatal("extension action did not update the restored primary controller")
	}
	other := extension.Fields["independent"]
	if other.Fields["actions"].Int != 0 {
		t.Fatal("independent same-type controller became an alias")
	}
	if refs := extensionControllerReferences(controller, []map[string]vm.Value{{"independent": independent}}); refs != nil {
		t.Fatal("view state added alias metadata for an independent object")
	}
}
