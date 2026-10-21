package sema

import (
	"strings"
	"testing"
)

func TestCartDeliveryGroupMethodListNativeAddPoints(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
		"CartDeliveryGroupMethodListProbe.cls": `
public class CartDeliveryGroupMethodListProbe {
  public void addFromDeliveryGroup(CartExtension.CartDeliveryGroup deliveryGroup, CartExtension.CartDeliveryGroupMethod method) {
    deliveryGroup.getCartDeliveryGroupMethods().clear();
    deliveryGroup.getCartDeliveryGroupMethods().add(method);
  }

  public void addFromTypedCollection(CartExtension.CartDeliveryGroupMethodList methods, CartExtension.CartDeliveryGroupMethod method) {
    methods.add(method);
  }

  public void standardListControl(CartExtension.CartDeliveryGroupMethod method) {
    List<CartExtension.CartDeliveryGroupMethod> methods = new List<CartExtension.CartDeliveryGroupMethod>();
    methods.add(method);
  }
}
`,
	}, "65.0")
	if result.HasErrors() {
		t.Fatalf("native CartDeliveryGroupMethodList.add points were rejected: %#v", result.Diagnostics)
	}
}

func TestCartDeliveryGroupMethodListKeepsNativeTypeBoundaries(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
		"CartDeliveryGroupMethodListBoundaryProbe.cls": `
public class CartDeliveryGroupMethodListBoundaryProbe {
  public void wrongAddType(CartExtension.CartDeliveryGroupMethodList methods) {
    methods.add('wrong');
  }

  public void cannotAssignToList(CartExtension.CartDeliveryGroup deliveryGroup) {
    List<CartExtension.CartDeliveryGroupMethod> methods = deliveryGroup.getCartDeliveryGroupMethods();
  }

  public void cannotAssignToIterable(CartExtension.CartDeliveryGroup deliveryGroup) {
    Iterable<CartExtension.CartDeliveryGroupMethod> methods = deliveryGroup.getCartDeliveryGroupMethods();
  }

  public void cannotIterate(CartExtension.CartDeliveryGroup deliveryGroup) {
    for (CartExtension.CartDeliveryGroupMethod method : deliveryGroup.getCartDeliveryGroupMethods()) {
    }
  }

  public void cannotCaptureAddReturn(CartExtension.CartDeliveryGroupMethodList methods, CartExtension.CartDeliveryGroupMethod item) {
    Object result = methods.add(item);
  }
}
`,
	}, "65.0")
	var wrongAdd, listAssignment, iterableAssignment, foreach, voidReturn bool
	for _, diag := range result.Diagnostics {
		if strings.Contains(diag.Message, `"add"`) || strings.Contains(diag.Message, "add(") {
			wrongAdd = true
		}
		if strings.Contains(diag.Message, "initializes List<CartExtension.CartDeliveryGroupMethod> local") && strings.Contains(diag.Message, "with CartExtension.CartDeliveryGroupMethodList") {
			listAssignment = true
		}
		if strings.Contains(diag.Message, "initializes Iterable<CartExtension.CartDeliveryGroupMethod> local") && strings.Contains(diag.Message, "with CartExtension.CartDeliveryGroupMethodList") {
			iterableAssignment = true
		}
		if strings.Contains(diag.Message, "enhanced-for iterates non-collection type CartExtension.CartDeliveryGroupMethodList") {
			foreach = true
		}
		if strings.Contains(diag.Message, "initializes Object local") && strings.Contains(diag.Message, "with void") {
			voidReturn = true
		}
	}
	if !wrongAdd || !listAssignment || !iterableAssignment || !foreach || !voidReturn {
		t.Fatalf("native collection boundary diagnostics missing (wrong add=%t list assignment=%t iterable assignment=%t foreach=%t void return=%t): %#v", wrongAdd, listAssignment, iterableAssignment, foreach, voidReturn, result.Diagnostics)
	}
}
