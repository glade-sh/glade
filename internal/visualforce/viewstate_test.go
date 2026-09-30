package visualforce

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/vm"
)

func TestEncodeDecodeViewStateRoundTrip(t *testing.T) {
	payload := ViewStatePayload{
		PageName:         "Edit",
		ControllerType:   "AccountController",
		ControllerValues: map[string]vm.Value{"count": vm.Int(7), "active": vm.Bool(true)},
		ControllerFields: map[string]string{"name": "Acme"},
		CSRF:             "token",
	}
	encoded, err := EncodeViewState(payload, nil)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeViewState(encoded, nil)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.PageName != "Edit" || decoded.ControllerFields["name"] != "Acme" {
		t.Fatalf("decoded = %#v", decoded)
	}
	if decoded.ControllerValues["count"].Kind != vm.ValueInt || decoded.ControllerValues["count"].Int != 7 {
		t.Fatalf("typed count = %#v", decoded.ControllerValues["count"])
	}
	if decoded.ControllerValues["active"].Kind != vm.ValueBool || !decoded.ControllerValues["active"].Bool {
		t.Fatalf("typed active = %#v", decoded.ControllerValues["active"])
	}
	if decoded.Version != CurrentViewStateVersion {
		t.Fatalf("decoded version = %d, want %d", decoded.Version, CurrentViewStateVersion)
	}
}

func TestEncodeViewStateDoesNotExposeControllerFieldPlaintext(t *testing.T) {
	const sentinel = "VF-PRIVATE-CONTROLLER-FIELD-9f4a2d"
	secret := []byte("task-11.6-explicit-test-key")
	payload := ViewStatePayload{
		PageName:         "PrivateEdit",
		CSRF:             "fixed-csrf-token",
		Timestamp:        time.Now().Unix(),
		ControllerFields: map[string]string{"privateValue": sentinel},
	}
	encoded, err := EncodeViewState(payload, secret)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("encoded view state is not base64: %v", err)
	}
	if bytes.Contains(decoded, []byte(sentinel)) {
		t.Fatal("base64-decoded view state exposes controller field plaintext")
	}
	state, err := DecodeViewState(encoded, secret)
	if err != nil {
		t.Fatalf("decode encrypted view state: %v", err)
	}
	if got := state.ControllerFields["privateValue"]; got != sentinel {
		t.Fatalf("decoded privateValue = %q, want original value", got)
	}
}

func TestDecodeViewStateRejectsWrongSecret(t *testing.T) {
	payload := ViewStatePayload{PageName: "Edit", CSRF: "fixed-csrf-token", Timestamp: time.Now().Unix()}
	encoded, err := EncodeViewState(payload, []byte("task-11.6-correct-key"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeViewState(encoded, []byte("task-11.6-wrong-key")); !errors.Is(err, ErrViewStateTampered) {
		t.Fatalf("wrong-key error = %v, want %v", err, ErrViewStateTampered)
	}
}

func TestEncodeViewStateUsesFreshNonce(t *testing.T) {
	secret := []byte("task-11.6-explicit-test-key")
	payload := ViewStatePayload{
		PageName:     "Edit",
		CSRF:         "fixed-csrf-token",
		Timestamp:    time.Now().Unix(),
		PageMessages: []string{"same input"},
	}
	first, err := EncodeViewState(payload, secret)
	if err != nil {
		t.Fatal(err)
	}
	second, err := EncodeViewState(payload, secret)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("encoding identical view-state input twice must use fresh nonce material")
	}
	for _, encoded := range []string{first, second} {
		if _, err := DecodeViewState(encoded, secret); err != nil {
			t.Fatalf("decode independently encoded state: %v", err)
		}
	}
}

type failedViewStateEntropy struct{}

func (failedViewStateEntropy) Read([]byte) (int, error) {
	return 0, errors.New("injected entropy failure")
}

func TestViewStateKeyCacheReturnsStableProcessKey(t *testing.T) {
	var cache viewStateKeyCache
	first, err := cache.get(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	second, err := cache.get(failedViewStateEntropy{})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 32 || !bytes.Equal(first, second) {
		t.Fatalf("cached process key length=%d stable=%t; want stable 32-byte key", len(first), bytes.Equal(first, second))
	}
}

func TestViewStateKeyCacheFailsClosedOnEntropyFailure(t *testing.T) {
	var cache viewStateKeyCache
	key, err := cache.get(failedViewStateEntropy{})
	if key != nil || !errors.Is(err, ErrViewStateEntropy) {
		t.Fatalf("keyPresent=%t error=%v, want no key and secure-randomness error", key != nil, err)
	}
	key, err = cache.get(rand.Reader)
	if key != nil || !errors.Is(err, ErrViewStateEntropy) {
		t.Fatalf("retryKeyPresent=%t error=%v, want cached fail-closed error", key != nil, err)
	}
}

func TestEncodeViewStateFailsClosedWhenNonceEntropyFails(t *testing.T) {
	payload := ViewStatePayload{PageName: "Edit", CSRF: "fixed-csrf-token", Timestamp: time.Now().Unix()}
	encoded, err := encodeViewStateWithRandom(payload, []byte("explicit-test-key"), failedViewStateEntropy{})
	if encoded != "" || !errors.Is(err, ErrViewStateEntropy) {
		t.Fatalf("encoded=%q error=%v, want no envelope and secure-randomness error", encoded, err)
	}
}

func TestViewStateRejectsExplicitEmptySecret(t *testing.T) {
	payload := ViewStatePayload{PageName: "Edit", CSRF: "fixed-csrf-token"}
	if encoded, err := EncodeViewState(payload, []byte{}); encoded != "" || !errors.Is(err, ErrViewStateInvalid) {
		t.Fatalf("encoded=%q error=%v, want empty-secret rejection", encoded, err)
	}
	encoded, err := EncodeViewState(payload, []byte("explicit-test-key"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeViewState(encoded, []byte{}); !errors.Is(err, ErrViewStateInvalid) {
		t.Fatalf("empty-secret decode error = %v, want %v", err, ErrViewStateInvalid)
	}
}

func TestDecodeViewStateRejectsMalformedEnvelope(t *testing.T) {
	unknownVersion := []byte(viewStateEnvelopeHeader)
	unknownVersion[len(unknownVersion)-1] = 2
	unknownVersion = append(unknownVersion, make([]byte, 64)...)
	for name, encoded := range map[string]string{
		"invalid base64":  "not-base64%",
		"short envelope":  base64.StdEncoding.EncodeToString([]byte(viewStateEnvelopeHeader)),
		"unknown version": base64.StdEncoding.EncodeToString(unknownVersion),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeViewState(encoded, []byte("explicit-test-key")); !errors.Is(err, ErrViewStateInvalid) {
				t.Fatalf("decode error = %v, want %v", err, ErrViewStateInvalid)
			}
		})
	}
}

func TestDecodeViewStateRejectsLegacyPlaintextEnvelope(t *testing.T) {
	secret := []byte("legacy-test-key")
	payload := ViewStatePayload{PageName: "Edit", CSRF: "fixed-csrf-token", Timestamp: time.Now().Unix()}
	plaintext, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(plaintext)
	legacy := append(append([]byte(nil), plaintext...), mac.Sum(nil)...)
	encoded := base64.StdEncoding.EncodeToString(legacy)
	if _, err := DecodeViewState(encoded, secret); !errors.Is(err, ErrViewStateInvalid) {
		t.Fatalf("legacy-envelope error = %v, want %v", err, ErrViewStateInvalid)
	}
}

func TestApplyViewStateValuesRestoresTypedControllerFields(t *testing.T) {
	controller := vm.Object("CounterController")
	applyValueFields(&controller, map[string]vm.Value{
		"count":  vm.Int(9),
		"active": vm.Bool(true),
		"label":  vm.String("ready"),
	})
	if controller.Fields["count"].Kind != vm.ValueInt || controller.Fields["count"].Int != 9 {
		t.Fatalf("count = %#v", controller.Fields["count"])
	}
	if controller.Fields["active"].Kind != vm.ValueBool || !controller.Fields["active"].Bool {
		t.Fatalf("active = %#v", controller.Fields["active"])
	}
	if controller.Fields["label"].Kind != vm.ValueString || controller.Fields["label"].Text != "ready" {
		t.Fatalf("label = %#v", controller.Fields["label"])
	}
}

func TestRenderPageRejectsViewStateForDifferentPageOrController(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root+"/sfdx-project.json", `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, root+"/force-app/main/default/pages/Edit.page", `<apex:page controller="EditController"><apex:outputText value="{!name}"/></apex:page>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}

	_, err = RenderPage(PageRenderRequest{
		Project:   p,
		VFIndex:   idx,
		Machine:   vm.New(nil),
		PageName:  "Edit",
		ViewState: &ViewStatePayload{PageName: "Other", ControllerType: "EditController", ControllerValues: map[string]vm.Value{"name": vm.String("leaked")}},
	})
	if err == nil || !strings.Contains(err.Error(), "view state page mismatch") {
		t.Fatalf("err = %v, want page mismatch", err)
	}

	_, err = RenderPage(PageRenderRequest{
		Project:   p,
		VFIndex:   idx,
		Machine:   vm.New(nil),
		PageName:  "Edit",
		ViewState: &ViewStatePayload{PageName: "Edit", ControllerType: "OtherController", ControllerValues: map[string]vm.Value{"name": vm.String("leaked")}},
	})
	if err == nil || !strings.Contains(err.Error(), "view state controller mismatch") {
		t.Fatalf("err = %v, want controller mismatch", err)
	}
}

func TestDecodeViewStateRejectsTamperedPayload(t *testing.T) {
	payload := ViewStatePayload{PageName: "Edit", CSRF: "token"}
	encoded, err := EncodeViewState(payload, nil)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	decoded[len(decoded)-1] ^= 1
	tampered := base64.StdEncoding.EncodeToString(decoded)
	if _, err := DecodeViewState(tampered, nil); !errors.Is(err, ErrViewStateTampered) {
		t.Fatalf("tampered view-state error = %v, want %v", err, ErrViewStateTampered)
	}
}

func TestVerifyViewStateCSRFRejectsEmptyPayloadToken(t *testing.T) {
	if err := VerifyViewStateCSRF(ViewStatePayload{}, "token"); err == nil {
		t.Fatal("expected missing payload CSRF to be rejected")
	}
}

func TestRenderPageInjectsCSRFFieldForLocalClients(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root+"/sfdx-project.json", `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, root+"/force-app/main/default/pages/Edit.page", `<apex:page><apex:form><apex:outputText value="ready"/></apex:form></apex:page>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	result, err := RenderPage(PageRenderRequest{Project: p, VFIndex: idx, Machine: vm.New(nil), PageName: "Edit"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, `name="__vf_csrf"`) {
		t.Fatalf("html missing csrf field: %s", result.HTML)
	}
}

func TestInjectViewStateAddsFieldToEveryForm(t *testing.T) {
	rendered := InjectViewState(`<form id="a"></form><form id="b"></form>`, "state")
	if got := strings.Count(rendered, ViewStateFormFieldName()); got != 2 {
		t.Fatalf("view state fields = %d html=%s", got, rendered)
	}
}

func TestRenderRepeatAndDataTable(t *testing.T) {
	markup := `<apex:page><apex:repeat value="{!items}" var="item"><apex:outputText value="{!item}" /></apex:repeat><apex:dataTable value="{!items}" var="row"><apex:column value="{!row}" header="Name" /></apex:dataTable></apex:page>`
	tree, err := ParseMarkupTree(markup)
	if err != nil {
		t.Fatal(err)
	}
	runner := testRunner(t)
	items := vmList("Alpha", "Beta")
	controller := vmObject("Controller")
	controller.Fields["items"] = items
	ctx := RenderContext{
		VM:         runner,
		PageName:   "List",
		Expression: &ExpressionContext{VM: runner, Controller: controller},
		Scope:      NewScopeStack(),
		Metrics:    &RenderMetrics{ComponentCounts: map[string]int{}},
	}
	rendered, err := RenderMarkupTree(tree, &ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered, "Alpha") || !strings.Contains(rendered, "Beta") {
		t.Fatalf("rendered = %q", rendered)
	}
	if !strings.Contains(rendered, "<table") {
		t.Fatalf("rendered = %q", rendered)
	}
}

func TestRenderCustomComponentFallback(t *testing.T) {
	markup := `<apex:page><c:MissingBadge value="x" /></apex:page>`
	tree, err := ParseMarkupTree(markup)
	if err != nil {
		t.Fatal(err)
	}
	runner := testRunner(t)
	idx := Index{}
	ctx := RenderContext{
		VM:         runner,
		PageName:   "Demo",
		VFIndex:    &idx,
		Expression: &ExpressionContext{VM: runner},
	}
	rendered, err := RenderMarkupTree(tree, &ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered, "customComponentMissing") {
		t.Fatalf("rendered = %q", rendered)
	}
}

func TestRenderDynamicComponentFallback(t *testing.T) {
	markup := `<apex:page><apex:dynamicComponent componentValue="c:Missing" /></apex:page>`
	tree, err := ParseMarkupTree(markup)
	if err != nil {
		t.Fatal(err)
	}
	runner := testRunner(t)
	ctx := RenderContext{
		VM:         runner,
		Expression: &ExpressionContext{VM: runner},
	}
	rendered, err := RenderMarkupTree(tree, &ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered, "dynamicComponentFallback") {
		t.Fatalf("rendered = %q", rendered)
	}
}
