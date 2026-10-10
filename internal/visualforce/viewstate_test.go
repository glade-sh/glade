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

func TestEncodeDecodeViewStatePreservesLargeCompressiblePayload(t *testing.T) {
	// This is a local codec round-trip invariant, not a Salesforce size claim.
	// The former decoder-only response-size cap rejected this authenticated
	// graph even though its encoded state fits the existing state budget.
	value := strings.Repeat("x", 16<<20)
	payload := ViewStatePayload{PageName: "Edit", CSRF: "token", ControllerFields: map[string]string{"value": value}}
	secret := []byte("owned-compressible-state-key")
	encoded, err := EncodeViewState(payload, secret)
	if err != nil {
		t.Fatal(err)
	}
	size, err := encodedViewStateSize(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckVisualforceViewStateSize(size); err != nil {
		t.Fatalf("compressible local payload exceeds state budget: %v", err)
	}
	decoded, err := DecodeViewState(encoded, secret)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.ControllerFields["value"] != value {
		t.Fatal("authenticated compressible payload changed during round-trip")
	}
}

func TestViewStateRestoresAliasesWithoutMergingIndependentCollections(t *testing.T) {
	shared := vm.List(vm.String("PREPARED"))
	independent := vm.List(vm.String("PREPARED"))
	object := vm.Object("OwnedValue")
	object.Fields["items"] = shared
	object.Fields["self"] = object
	payload := ViewStatePayload{PageName: "Owned", CSRF: "owned-csrf", ControllerValues: map[string]vm.Value{
		"value": shared, "alias": shared, "independent": independent, "null": vm.Null, "object": object,
	}}
	encoded, err := EncodeViewState(payload, []byte("owned-view-state-test"))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeViewState(encoded, []byte("owned-view-state-test"))
	if err != nil {
		t.Fatal(err)
	}
	fields := decoded.ControllerValues
	if fields["value"].Ref == 0 || fields["value"].Ref != fields["alias"].Ref || fields["value"].Ref == fields["independent"].Ref {
		t.Fatal("shared and independent collection identities were not preserved")
	}
	if fields["object"].Fields["items"].Ref != fields["value"].Ref || fields["object"].Fields["self"].Ref != fields["object"].Ref {
		t.Fatal("nested references or cycles were not preserved")
	}
	if null, present := fields["null"]; !present || null.Kind != vm.ValueNull {
		t.Fatal("explicit persisted null was lost")
	}
	// Mutations to the shared backing collection must reach both aliases.
	fields["alias"].List[0] = vm.String("RESTORED")
	if fields["value"].List[0].Text != "RESTORED" || fields["independent"].List[0].Text != "PREPARED" {
		t.Fatal("restored shared and independent backing storage is incorrect")
	}
}

func TestViewStateRoundTripPreservesCollectionFields(t *testing.T) {
	// r_persist_list/set/map cover native collection persistence. These are
	// local codec invariants for the VM metadata carried alongside the values,
	// including __soqlQuery on SOQL lists; no native metadata format is assumed.
	for _, kind := range []vm.ValueKind{vm.ValueList, vm.ValueSet, vm.ValueMap} {
		t.Run(string(kind), func(t *testing.T) {
			record := vm.Object("Account")
			record.Fields["Name"] = vm.String("Acme")
			var collection vm.Value
			switch kind {
			case vm.ValueList:
				collection = vm.List(record)
			case vm.ValueSet:
				collection = vm.Set(record)
			case vm.ValueMap:
				collection = vm.Map()
				key := vm.String("record")
				collection.Map["string:record"] = record
				collection.MapKeys["string:record"] = key
				collection.MapOrder = []string{"string:record"}
			}
			collection.Fields = map[string]vm.Value{
				"record": record,
				"null":   vm.Null,
			}
			if kind == vm.ValueList {
				collection.Fields["__soqlQuery"] = vm.String("SELECT Name FROM Account")
			}
			collection.Fields["self"] = collection
			payload := ViewStatePayload{PageName: "Edit", CSRF: "token", ControllerValues: map[string]vm.Value{
				"value": collection, "alias": collection, "record": record,
			}}
			secret := []byte("collection-fields-round-trip")
			encoded, err := EncodeViewState(payload, secret)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := DecodeViewState(encoded, secret)
			if err != nil {
				t.Fatal(err)
			}
			got := decoded.ControllerValues["value"]
			if got.Kind != kind || got.Ref == 0 || got.Ref == collection.Ref || got.Ref != decoded.ControllerValues["alias"].Ref || got.Fields["self"].Ref != got.Ref {
				t.Fatal("collection identity, aliases or field cycle were not restored")
			}
			if value, ok := got.Fields["null"]; !ok || value.Kind != vm.ValueNull {
				t.Fatal("null collection field was lost")
			}
			if kind == vm.ValueList && got.Fields["__soqlQuery"].Text != "SELECT Name FROM Account" {
				t.Fatal("SOQL query metadata was not restored exactly")
			}
			var element vm.Value
			switch kind {
			case vm.ValueList:
				if len(got.List) != 1 {
					t.Fatalf("list length = %d, want 1", len(got.List))
				}
				element = got.List[0]
			case vm.ValueSet:
				if len(got.Set) != 1 {
					t.Fatalf("set length = %d, want 1", len(got.Set))
				}
				element = got.Set[0]
			case vm.ValueMap:
				const key = "string:record"
				if len(got.Map) != 1 || len(got.MapOrder) != 1 || got.MapOrder[0] != key || got.MapKeys[key].Text != "record" {
					t.Fatal("map entries, key or order were not restored")
				}
				element = got.Map[key]
			}
			if element.Ref != got.Fields["record"].Ref || element.Ref != decoded.ControllerValues["record"].Ref || element.Fields["Name"].Text != "Acme" {
				t.Fatal("collection element and metadata no longer share the restored record")
			}
			got.Fields["record"].Fields["Name"] = vm.String("Changed")
			if element.Fields["Name"].Text != "Changed" {
				t.Fatal("metadata mutation did not reach the shared collection element")
			}
			got.Fields["updated"] = vm.Bool(true)
			if !decoded.ControllerValues["alias"].Fields["updated"].Bool || !got.Fields["self"].Fields["updated"].Bool {
				t.Fatal("collection metadata backing map is not shared across aliases")
			}
		})
	}
}

func TestViewStateGraphRejectsInvalidCollectionFieldReference(t *testing.T) {
	for _, kind := range []vm.ValueKind{vm.ValueList, vm.ValueSet, vm.ValueMap} {
		t.Run(string(kind), func(t *testing.T) {
			data, err := json.Marshal(viewStateGraph{
				Nodes:      []viewStateNode{{Value: vm.Value{Kind: kind}, Fields: map[string]int{"metadata": 1}}},
				Controller: map[string]int{"value": 0},
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := unmarshalViewStateGraph(data); err == nil || err.Error() != "invalid state graph reference" {
				t.Fatalf("decode error = %v, want invalid state graph reference", err)
			}
		})
	}
}

func TestViewStateRoundTripRekeysIdentityMapEntries(t *testing.T) {
	// This exercises the local codec with the VM's real identity-key map
	// operations, including keys that have equal fields but distinct identity.
	for _, api := range []string{"59.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			newMachine := func() *vm.VM {
				machine := vm.New(nil)
				if err := machine.RegisterClass(vm.Class{Name: "Key", Fields: map[string]vm.Field{
					"Code": {Name: "Code", Type: "String"},
				}}); err != nil {
					t.Fatal(err)
				}
				return machine
			}
			execute := func(machine *vm.VM, source string) {
				t.Helper()
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			}
			machine := newMachine()
			for _, name := range []string{"first", "second"} {
				key := vm.Object("Key")
				key.Fields["Code"] = vm.String("same")
				machine.Globals[name] = key
			}
			values := vm.Map()
			values.Type = "Map<Key,String>"
			machine.Globals["values"] = values
			execute(machine, "values.put(first, 'left'); values.put(second, 'right');")
			payload := ViewStatePayload{PageName: "Edit", CSRF: "token", ControllerValues: map[string]vm.Value{
				"values": machine.Globals["values"], "alias": machine.Globals["values"],
				"first": machine.Globals["first"], "second": machine.Globals["second"],
			}}
			secret := []byte("identity-map-round-trip")
			encoded, err := EncodeViewState(payload, secret)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := DecodeViewState(encoded, secret)
			if err != nil {
				t.Fatal(err)
			}
			got := decoded.ControllerValues["values"]
			if len(got.MapOrder) != 2 || got.MapKeys[got.MapOrder[0]].Ref != decoded.ControllerValues["first"].Ref || got.MapKeys[got.MapOrder[1]].Ref != decoded.ControllerValues["second"].Ref {
				t.Fatal("identity map storage keys, key values or insertion order diverged")
			}
			restored := newMachine()
			for name, value := range decoded.ControllerValues {
				restored.Globals[name] = value
			}
			execute(restored, `
System.assert(values.size() == 2);
System.assert(values.containsKey(first) && values.containsKey(second));
System.assert('left'.equals((String)values.get(first)));
System.assert('right'.equals((String)values.get(second)));
values.put(first, 'updated');
System.assert(values.size() == 2);
System.assert('updated'.equals((String)values.remove(first)));
System.assert(values.size() == 1 && !values.containsKey(first));
System.assert('right'.equals((String)values.get(second)));
System.assert(alias.size() == 1);
`)
		})
	}
}

func TestViewStateRoundTripRetainsValueMapStorageKeys(t *testing.T) {
	// Custom hashCode buckets (and collisions) retain their insertion hashes,
	// including after the key mutates. They must not be rehashed by the codec.
	values := vm.Map()
	for i, name := range []string{"object:Key:hash:7", "object:Key:hash:7\x00collision:1", "string:object:Key:ref:123"} {
		key := vm.Object("Key")
		key.Fields["hash"] = vm.Int(99)
		if i == 2 {
			key = vm.String("object:Key:ref:123")
		}
		values.Map[name] = vm.Int(int64(i))
		values.MapKeys[name] = key
		values.MapOrder = append(values.MapOrder, name)
	}
	encoded, err := EncodeViewState(ViewStatePayload{PageName: "Edit", CSRF: "token", ControllerValues: map[string]vm.Value{"values": values}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeViewState(encoded, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := decoded.ControllerValues["values"]
	if len(got.Map) != len(values.Map) || len(got.MapOrder) != len(values.MapOrder) {
		t.Fatal("value-key map entries or insertion order were lost")
	}
	for i, name := range values.MapOrder {
		if got.MapOrder[i] != name || got.Map[name].Kind != vm.ValueInt || got.Map[name].Int != int64(i) || got.MapKeys[name].Kind != values.MapKeys[name].Kind {
			t.Fatalf("stored value-key entry %q changed", name)
		}
	}
}

func TestViewStatePublicCSRFCannotFallBackToCompatibilityToken(t *testing.T) {
	payload := ViewStatePayload{CSRF: "owned-csrf"}
	for _, public := range []string{"", "wrong-token"} {
		values := map[string]string{viewStateVersionFieldName: "1", "__vf_csrf": payload.CSRF}
		if public != "" {
			values[viewStateCSRFFieldName] = public
		}
		if !errors.Is(VerifyViewStateFormCSRF(payload, values), ErrViewStateCSRF) {
			t.Fatal("removed or modified public CSRF token used the legacy fallback")
		}
	}
	if err := VerifyViewStateFormCSRF(payload, map[string]string{viewStateCSRFFieldName: payload.CSRF}); err != nil {
		t.Fatal(err)
	}
	if err := VerifyViewStateFormCSRF(payload, map[string]string{"__vf_csrf": payload.CSRF}); err != nil {
		t.Fatal(err)
	}
}

func TestEncodeViewStateDoesNotExposeControllerFieldPlaintext(t *testing.T) {
	const sentinel = "VF-PRIVATE-CONTROLLER-FIELD-9f4a2d"
	secret := []byte("viewstate-explicit-test-key")
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
	encoded, err := EncodeViewState(payload, []byte("viewstate-correct-key"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeViewState(encoded, []byte("viewstate-wrong-key")); !errors.Is(err, ErrViewStateTampered) {
		t.Fatalf("wrong-key error = %v, want %v", err, ErrViewStateTampered)
	}
}

func TestEncodeViewStateUsesFreshNonce(t *testing.T) {
	secret := []byte("viewstate-explicit-test-key")
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

func TestDecodeViewStatePreservesAuthenticatedUncompressedFormats(t *testing.T) {
	secret := []byte("owned-uncompressed-state-key")
	payload := ViewStatePayload{PageName: "Edit", CSRF: "owned-csrf", ControllerValues: map[string]vm.Value{"null": vm.Null, "list": vm.List(vm.String("OWNED"))}}
	flat, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := marshalViewStateGraph(payload)
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{flat, graph} {
		decoded, err := DecodeViewState(ownedAuthenticatedViewState(t, data, secret), secret)
		if err != nil {
			t.Fatal(err)
		}
		if null, present := decoded.ControllerValues["null"]; !present || null.Kind != vm.ValueNull || decoded.ControllerValues["list"].List[0].Text != "OWNED" {
			t.Fatal("authenticated uncompressed state did not restore its values")
		}
	}
}

func TestDecodeViewStateRejectsInvalidAuthenticatedCompressedPayload(t *testing.T) {
	secret := []byte("owned-invalid-compressed-state-key")
	encoded := ownedAuthenticatedViewState(t, []byte(viewStateCompressedHeader+"invalid-zlib"), secret)
	if _, err := DecodeViewState(encoded, secret); !errors.Is(err, ErrViewStateInvalid) {
		t.Fatalf("decode error = %v, want %v", err, ErrViewStateInvalid)
	}
}

func ownedAuthenticatedViewState(t *testing.T, data, secret []byte) string {
	t.Helper()
	aead, err := viewStateAEAD(secret)
	if err != nil {
		t.Fatal(err)
	}
	header := []byte(viewStateEnvelopeHeader)
	nonce := make([]byte, aead.NonceSize())
	envelope := append(append([]byte(nil), header...), nonce...)
	envelope = append(envelope, aead.Seal(nil, nonce, data, header)...)
	return base64.StdEncoding.EncodeToString(envelope)
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

func TestRenderPageRestoresOmittedRootNullFields(t *testing.T) {
	// Select controls binding_selectList_none_runtime/binding_selectRadio_none_runtime
	// observe raw null selections; View state r_persist_null/list_null/account_null
	// retain null across requests despite non-null constructor defaults. This
	// local integration regression also checks the omitted-root representation;
	// it adds no native observations to either family's denominator.
	for _, extension := range []bool{false, true} {
		name := "controller"
		if extension {
			name = "extension"
		}
		t.Run(name, func(t *testing.T) {
			const host = "NullStateHost"
			const ext = "NullStateExtension"
			target := host
			attributes := `controller="` + host + `"`
			if extension {
				target = ext
				attributes += ` extensions="` + ext + `"`
			}
			p, index := expressionContractPage(t, "NullState", `<apex:page `+attributes+`><apex:form><apex:commandButton action="{!prepare}" value="Prepare"/><apex:commandButton action="{!observe}" value="Observe"/></apex:form></apex:page>`)
			constructor, err := vm.CompileAnonymous(`this.scalar = 'CONSTRUCTOR_INITIAL';`)
			if err != nil {
				t.Fatal(err)
			}
			prepare, err := vm.CompileAnonymous(`this.scalar = null; this.items = null; this.record = null; this.secret = null; return null;`)
			if err != nil {
				t.Fatal(err)
			}
			observe, err := vm.CompileAnonymous(`this.scalarWasNull = this.scalar == null; this.itemsWasNull = this.items == null; this.recordWasNull = this.record == null; this.secretWasReset = this.secret == 'SECRET_INITIAL'; return null;`)
			if err != nil {
				t.Fatal(err)
			}
			newMachine := func() *vm.VM {
				machine := testRunner(t)
				if extension {
					if err := machine.RegisterClass(vm.Class{Name: host}); err != nil {
						t.Fatal(err)
					}
				}
				record := vm.Object("Account")
				record.Fields["Name"] = vm.String("INITIAL")
				nested := vm.Object("Account")
				nested.Fields["Name"], nested.Fields["Description"] = vm.String("KEPT"), vm.Null
				carrier := vm.List(nested)
				carrier.Fields = map[string]vm.Value{"__soqlQuery": vm.String("SELECT Name, Description FROM Account"), "null": vm.Null}
				if err := machine.RegisterClass(vm.Class{
					Name: target,
					Fields: map[string]vm.Field{
						"scalar":         {Name: "scalar", Type: "String", InitialValue: vm.String("INITIAL")},
						"items":          {Name: "items", Type: "List<String>", InitialValue: vm.List(vm.String("INITIAL"))},
						"record":         {Name: "record", Type: "Account", InitialValue: record},
						"secret":         {Name: "secret", Type: "String", Modifiers: []string{"transient"}, InitialValue: vm.String("SECRET_INITIAL")},
						"carrier":        {Name: "carrier", Type: "List<Account>", InitialValue: carrier},
						"scalarWasNull":  {Name: "scalarWasNull", Type: "Boolean", InitialValue: vm.Bool(false)},
						"itemsWasNull":   {Name: "itemsWasNull", Type: "Boolean", InitialValue: vm.Bool(false)},
						"recordWasNull":  {Name: "recordWasNull", Type: "Boolean", InitialValue: vm.Bool(false)},
						"secretWasReset": {Name: "secretWasReset", Type: "Boolean", InitialValue: vm.Bool(false)},
					},
					Constructors: []vm.Method{{Name: target + ".<init>", ClassName: target, IsConstructor: true, Program: constructor}},
					Methods: map[string]vm.Method{
						"prepare": {Name: target + ".prepare", ClassName: target, ReturnType: "PageReference", Program: prepare},
						"observe": {Name: target + ".observe", ClassName: target, ReturnType: "PageReference", Program: observe},
					},
				}); err != nil {
					t.Fatal(err)
				}
				return machine
			}
			secret := []byte("owned-null-state-integration")
			render := func(saved *ViewStatePayload, action string) ViewStatePayload {
				result, err := RenderPage(PageRenderRequest{
					Project: p, VFIndex: index, Machine: newMachine(), PageName: "NullState", PageURL: "/apex/NullState",
					ViewState: saved, Action: action, ViewStateSecret: secret,
				})
				if err != nil {
					t.Fatal(err)
				}
				decoded, err := DecodeViewState(result.ViewState, secret)
				if err != nil {
					t.Fatal(err)
				}
				return decoded
			}
			checkNullProjection := func(payload ViewStatePayload) map[string]vm.Value {
				t.Helper()
				values, nullFields := payload.ControllerValues, payload.ControllerNullFields
				if extension {
					if len(payload.ExtensionValues) != 1 || len(payload.ExtensionNullFields) != 1 {
						t.Fatalf("extension state lengths = %d values, %d null lists", len(payload.ExtensionValues), len(payload.ExtensionNullFields))
					}
					values, nullFields = payload.ExtensionValues[0], payload.ExtensionNullFields[0]
				}
				if strings.Join(nullFields, ",") != "items,record,scalar" {
					t.Errorf("root null fields = %v, want only items, record, scalar", nullFields)
				}
				for _, field := range []string{"scalar", "items", "record", "secret"} {
					if _, present := values[field]; present {
						t.Errorf("omitted root field %s remains in value projection", field)
					}
				}
				carrier := values["carrier"]
				if carrier.Kind != vm.ValueList || carrier.Fields["__soqlQuery"].Text != "SELECT Name, Description FROM Account" || len(carrier.List) != 1 {
					t.Fatalf("collection contents or metadata changed: %#v", carrier)
				}
				if null, present := carrier.Fields["null"]; !present || null.Kind != vm.ValueNull {
					t.Error("collection metadata null was omitted")
				}
				if null, present := carrier.List[0].Fields["Description"]; !present || null.Kind != vm.ValueNull {
					t.Error("nested object null was omitted")
				}
				return values
			}
			prepared := render(nil, "{!prepare}")
			checkNullProjection(prepared)
			restored := render(&prepared, "{!observe}")
			values := checkNullProjection(restored)
			for _, field := range []string{"scalarWasNull", "itemsWasNull", "recordWasNull", "secretWasReset"} {
				if value := values[field]; value.Kind != vm.ValueBool || !value.Bool {
					t.Errorf("fresh-request action %s = %#v, want true", field, value)
				}
			}
		})
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
	// View state native rows include Version and MAC fields with the same prefix.
	// Count the state field's exact name, not all three prefixed names.
	if got := strings.Count(rendered, `name="`+ViewStateFormFieldName()+`"`); got != 2 {
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
