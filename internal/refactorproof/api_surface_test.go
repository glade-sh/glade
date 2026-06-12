package refactorproof

import "testing"

func TestAPISurfaceDetectsRemovedGlobalText(t *testing.T) {
	result := CheckAPISurfaceText(`global class BillingApi {
    global String total() { return '1'; }
}`, `global class BillingApi {
}`, APISurfaceOptions{})

	if result.Status != StageStatusWarn {
		t.Fatalf("status = %q, want warn: %#v", result.Status, result)
	}
	if result.Message == "" {
		t.Fatal("expected surface message")
	}
}

func TestAPISurfaceCanFailOnPublicGlobalBreaks(t *testing.T) {
	result := CheckAPISurfaceText(`public class BillingApi {
    public String total() { return '1'; }
}`, `public class BillingApi {
    public Integer total() { return 1; }
}`, APISurfaceOptions{FailOnBreak: true})

	if result.Status != StageStatusFail {
		t.Fatalf("status = %q, want fail: %#v", result.Status, result)
	}
}

func TestAPISurfaceIgnoresPrivateOnlyText(t *testing.T) {
	result := CheckAPISurfaceText(`public class BillingApi {
    private String note() { return '1'; }
}`, `public class BillingApi {
    private Integer note() { return 1; }
}`, APISurfaceOptions{FailOnBreak: true})

	if result.Status != StageStatusPass {
		t.Fatalf("status = %q, want pass: %#v", result.Status, result)
	}
}
