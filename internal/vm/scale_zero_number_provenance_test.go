package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/storage"
)

func numericProvenanceTestOrg() storage.OrgState {
	org := testDataOrg()
	org.Objects["ProbeGauge__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "ProbeGauge__c", KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Id":           {APIName: "Id", Type: storage.FieldID},
				"Name":         {APIName: "Name", Type: storage.FieldString},
				"TickValue__c": {APIName: "TickValue__c", Type: storage.FieldDecimal, DisplayType: "DOUBLE", Precision: 18, ScaleSpecified: true},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	return org
}

// The synthetic Object bridge exercises the shared parameter/return and field
// paths. These assertions check internal provenance, not Salesforce acceptance
// of casts. number_token_provenance C01-C04/C17/C18 now capture these routes.
// Captured controls: scale_zero_number the captured rows (token identity),
// 003/009 (String/typed Decimal views), at API 62/67/68 on both native routes.
func TestScaleZeroNumberIntegerProvenanceBridge(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, isTest := range []bool{false, true} {
			for _, parameter := range []string{"Integer", "Object"} {
				for _, withID := range []bool{false, true} {
					name := fmt.Sprintf("%s/isTest=%t/%s/Id=%t", api, isTest, parameter, withID)
					t.Run(name, func(t *testing.T) {
						org := numericProvenanceTestOrg()
						machine := New(nil)
						machine.SetOrg(&org)
						if isTest {
							machine.EnableTestContext()
						}
						for _, method := range []struct {
							name, result, source string
							params               []Param
						}{
							{"writeBox", "void", "row.put(key, input);", []Param{{Name: "row", Type: "SObject"}, {Name: "key", Type: "Schema.SObjectField"}, {Name: "input", Type: parameter}}},
							{"readBox", "Object", "return row.get(key);", []Param{{Name: "row", Type: "SObject"}, {Name: "key", Type: "Schema.SObjectField"}}},
						} {
							program, err := CompileAnonymousWithOptions(method.source, CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							if err := machine.RegisterMethod(Method{Name: "NumericCaptureBridge." + method.name, ClassName: "NumericCaptureBridge", ReturnType: method.result, Params: method.params, IsStatic: true, Program: program}); err != nil {
								t.Fatal(err)
							}
						}
						idSource := ""
						if withID {
							idSource = "row.Id = 'a00000000000001';"
						}
						program, err := CompileAnonymousWithOptions(`
ProbeGauge__c row = new ProbeGauge__c();
`+idSource+`
NumericCaptureBridge.writeBox(row, ProbeGauge__c.TickValue__c, 7);
Object actual = NumericCaptureBridge.readBox(row, ProbeGauge__c.TickValue__c);
Decimal typed = row.TickValue__c;
Object stringRead = row.get('TickValue__c');
Map<Schema.SObjectField,Object> bucket = new Map<Schema.SObjectField,Object>();
bucket.put(ProbeGauge__c.TickValue__c, actual);
Object mapped = bucket.get(ProbeGauge__c.TickValue__c);
`, CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						if _, err := machine.Execute(program); err != nil {
							t.Fatal(err)
						}
						for _, name := range []string{"actual", "mapped"} {
							value := machine.Globals[name]
							if value.Kind != ValueInt || isLongIntValue(value) || value.Int != 7 || value.Static != "Object" {
								t.Fatalf("%s lost Integer provenance across Object bridge: %#v", name, value)
							}
						}
						for _, name := range []string{"typed", "stringRead"} {
							if value := machine.Globals[name]; value.Kind != ValueDecimal || value.String() != "7" {
								t.Fatalf("%s lost Decimal view: %#v", name, value)
							}
						}
					})
				}
			}
		}
	}
}

// These controls replay captured behavior (the captured rows).
// Untouched query values and String reads remain Decimal.
func TestScaleZeroNumberProvenanceControls(t *testing.T) {
	program, err := CompileAnonymous(`
ProbeGauge__c row = new ProbeGauge__c(Name='probe');
row.put(ProbeGauge__c.TickValue__c, 7);
System.assertEquals(7, (Integer)row.get(ProbeGauge__c.TickValue__c));
Object typed = row.TickValue__c;
try { Integer bad = (Integer)typed; System.assert(false); }
catch (TypeException e) { System.assertEquals('Invalid conversion from runtime type Decimal to Integer', e.getMessage()); }
row.put('TickValue__c', 7);
row.put('TickValue__c', 7.0);
Object rewritten = row.get('TickValue__c');
try { Integer bad = (Integer)rewritten; System.assert(false); }
catch (TypeException e) { System.assertEquals('Invalid conversion from runtime type Decimal to Integer', e.getMessage()); }
row.put(ProbeGauge__c.TickValue__c, 7L);
Object longValue = row.get(ProbeGauge__c.TickValue__c);
try { Integer bad = (Integer)longValue; System.assert(false); }
catch (TypeException e) { System.assertEquals('Invalid conversion from runtime type Long to Integer', e.getMessage()); }
Object boxed = 7;
row.put('TickValue__c', boxed);
Object stringValue = row.get('TickValue__c');
try { Integer bad = (Integer)stringValue; System.assert(false); }
catch (TypeException e) { System.assertEquals('Invalid conversion from runtime type Decimal to Integer', e.getMessage()); }
insert row;
ProbeGauge__c queried = [SELECT TickValue__c FROM ProbeGauge__c WHERE Id=:row.Id];
Object queryValue = queried.get('TickValue__c');
try { Integer bad = (Integer)queryValue; System.assert(false); }
catch (TypeException e) { System.assertEquals('Invalid conversion from runtime type Decimal to Integer', e.getMessage()); }
queried.put('TickValue__c', 8);
Object queryRewrite = queried.get('TickValue__c');
try { Integer bad = (Integer)queryRewrite; System.assert(false); }
catch (TypeException e) { System.assertEquals('Invalid conversion from runtime type Decimal to Integer', e.getMessage()); }
`)
	if err != nil {
		t.Fatal(err)
	}
	org := numericProvenanceTestOrg()
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

// Scope checks retain unprobed Long/String limits; C15 confirms boxed Integer
// String puts on new Id-less records, and PF_A/C confirm materialized token puts.
func TestScaleZeroNumberProvenanceScope(t *testing.T) {
	boxed := Int(7)
	boxed.Static = "Object"
	long := Int(7)
	long.Type = "Long"
	boxedLong := long
	boxedLong.Static = "Object"
	token := sObjectFieldToken("ProbeGauge__c", "TickValue__c")
	for _, tc := range []struct {
		name     string
		input    Value
		selector Value
		withID   bool
		setup    string
		kind     ValueKind
	}{
		{"boxedLong", boxedLong, token, false, "", ValueDecimal},
		{"longWithId", long, token, true, "", ValueDecimal},
		{"boxedStringPut", boxed, String("TickValue__c"), false, "", ValueInt},
		{"stringPutWithId", Int(7), String("TickValue__c"), true, "", ValueDecimal},
		{"dmlCaller", boxed, token, false, "insert row;", ValueInt},
		{"queriedRewrite", boxed, token, false, "insert row; row = [SELECT TickValue__c FROM ProbeGauge__c WHERE Id=:row.Id];", ValueInt},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := numericProvenanceTestOrg()
			machine := New(nil)
			machine.SetOrg(&org)
			program, err := CompileAnonymous("ProbeGauge__c row = new ProbeGauge__c(Name='probe');" + tc.setup)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
			row := machine.Globals["row"]
			if tc.withID {
				setExplicitSObjectField(&row, "Id", String("a00000000000001"))
			}
			if _, handled, err := machine.callSObjectMember(row, "put", []Value{tc.selector, tc.input}); err != nil || !handled {
				t.Fatalf("put: handled=%t err=%v", handled, err)
			}
			value, handled, err := machine.callSObjectMember(row, "get", []Value{token})
			if err != nil || !handled || value.Kind != tc.kind || value.Static != "Object" || value.String() != "7" {
				t.Fatalf("token read escaped pending scope: handled=%t err=%v value=%#v", handled, err, value)
			}
		})
	}
}

// number_token_provenance PF_A-E and their null/Long/Decimal/mixed-case twins
// capture the DML/token and dynamic-query/setter routes on both native routes.
// String views remain backed by the captured rows. String-origin token
// reads on queried records remain unprobed and retain the original exclusion.
func TestScaleZeroNumberMaterializedTokenProvenance(t *testing.T) {
	const querySetup = `
ProbeGauge__c r = new ProbeGauge__c(Name='probe', TickValue__c=1);
insert r;
Id rid = r.Id;
SObject q = Database.query('SELECT Id,TickValue__c FROM ProbeGauge__c WHERE Id=:rid');
`
	const integerSetter = `Integer n = Integer.valueOf((Decimal)q.get('TickValue__c'));`
	const tokenRewrite = integerSetter + `NumericRouteBridge.writeValue(q,n);`
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, isTest := range []bool{false, true} {
			for _, tc := range []struct {
				name, source, receiver string
				kind                   ValueKind
			}{
				{"A_mapDML", `
ProbeGauge__c r = new ProbeGauge__c(Name='probe');
Map<Schema.SObjectField,Object> m = new Map<Schema.SObjectField,Object>{ProbeGauge__c.TickValue__c=>1};
NumericRouteBridge.writeValue(r, m.get(ProbeGauge__c.TickValue__c));
Object before = NumericRouteBridge.readValue(r);
insert r;
Object afterInsert = NumericRouteBridge.readValue(r);
update r;
`, "r", ValueInt},
				{"B_untouchedQuery", querySetup, "q", ValueDecimal},
				{"C_ObjectSetter", querySetup + integerSetter + `
SObject snapshot = q.clone(true,true,false,false);
NumericRouteBridge.writeValue(q,n);
Object snapshotValue = NumericRouteBridge.readValue(snapshot);
`, "q", ValueInt},
				{"D_IntegerSetter", querySetup + integerSetter + `q.put(ProbeGauge__c.TickValue__c,n);`, "q", ValueInt},
				{"E_StringSetter", querySetup + integerSetter + `q.put('TickValue__c',n);`, "q", ValueDecimal},
				{"C_null", querySetup + `NumericRouteBridge.writeValue(q,null);`, "q", ValueNull},
				{"C_Long", querySetup + `NumericRouteBridge.writeValue(q,1L);`, "q", ValueInt},
				{"C_Decimal", querySetup + `NumericRouteBridge.writeValue(q,1.0);`, "q", ValueDecimal},
				{"E_mixedCase", querySetup + integerSetter + `q.pUt('tIcKvAlUe__c',n);`, "q", ValueDecimal},
				{"overwrite_Long", querySetup + tokenRewrite + `NumericRouteBridge.writeValue(q,1L);`, "q", ValueInt},
				{"overwrite_Decimal", querySetup + tokenRewrite + `NumericRouteBridge.writeValue(q,1.0);`, "q", ValueDecimal},
				{"overwrite_null", querySetup + tokenRewrite + `NumericRouteBridge.writeValue(q,null);`, "q", ValueNull},
				{"overwrite_String", querySetup + tokenRewrite + `q.put('TickValue__c',n);`, "q", ValueDecimal},
			} {
				t.Run(fmt.Sprintf("%s/isTest=%t/%s", api, isTest, tc.name), func(t *testing.T) {
					org := numericProvenanceTestOrg()
					machine := New(nil)
					machine.SetOrg(&org)
					if isTest {
						machine.EnableTestContext()
					}
					compile := func(source string) ir.Program {
						program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						return program
					}
					for _, method := range []Method{
						{Name: "NumericRouteBridge.writeValue", ClassName: "NumericRouteBridge", ReturnType: "void", IsStatic: true, Params: []Param{{Name: "r", Type: "SObject"}, {Name: "x", Type: "Object"}}, Program: compile(`r.put(ProbeGauge__c.TickValue__c,x);`)},
						{Name: "NumericRouteBridge.readValue", ClassName: "NumericRouteBridge", ReturnType: "Object", IsStatic: true, Params: []Param{{Name: "r", Type: "SObject"}}, Program: compile(`return r.get(ProbeGauge__c.TickValue__c);`)},
					} {
						if err := machine.RegisterMethod(method); err != nil {
							t.Fatal(err)
						}
					}
					program := compile(tc.source + fmt.Sprintf(`
Object actual = NumericRouteBridge.readValue(%s);
Object stringRead = %s.gEt('TICKVALUE__C');
ProbeGauge__c typedRow = (ProbeGauge__c)%s;
Decimal typed = typedRow.TickValue__c;
`, tc.receiver, tc.receiver, tc.receiver))
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
					value := machine.Globals["actual"]
					if value.Kind != tc.kind || value.Static != "Object" || (tc.kind == ValueInt && (isLongIntValue(value) != (tc.name == "C_Long" || tc.name == "overwrite_Long") || value.Int != 1)) {
						t.Fatalf("candidate token provenance: want %s, got %#v", tc.kind, value)
					}
					if tc.kind != ValueNull && !value.Equal(Int(1)) {
						t.Fatalf("candidate token numeric value changed: %#v", value)
					}
					for _, name := range []string{"stringRead", "typed", "snapshotValue"} {
						value, exists := machine.Globals[name]
						if !exists {
							continue
						}
						want := ValueDecimal
						if tc.kind == ValueNull {
							want = ValueNull
						}
						if value.Kind != want || (want == ValueDecimal && !value.Equal(Int(1))) {
							t.Fatalf("%s view changed: want %s, got %#v", name, want, value)
						}
					}
					for _, name := range []string{"before", "afterInsert"} {
						if value, exists := machine.Globals[name]; exists && (value.Kind != ValueInt || value.Int != 1) {
							t.Fatalf("%s lost caller-owned provenance: %#v", name, value)
						}
					}
				})
			}
		}
	}
}

// number_token_provenance C10 distinguishes the Double token view from the
// Decimal String/typed views, including the two boxed integral cast errors.
func TestScaleZeroNumberDoubleTokenProvenance(t *testing.T) {
	org := numericProvenanceTestOrg()
	machine := New(nil)
	machine.SetOrg(&org)
	program, err := CompileAnonymous(`ProbeGauge__c r = new ProbeGauge__c(); Double x=7.0; r.put(ProbeGauge__c.TickValue__c,x); Object actual=r.get(ProbeGauge__c.TickValue__c);`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	value := machine.Globals["actual"]
	if !isFloatBackedDecimal(value) || !value.scaleZeroNumberTokenDouble {
		entry, _ := scaleZeroNumberDynamicInput(machine.Globals["r"], "TickValue__c")
		t.Fatalf("Double identity lost: value=%#v input=%#v declared=%#v", value, entry, machine.Globals["x"])
	}
	for _, target := range []string{"Integer", "Long"} {
		_, err := machine.coerceCastForOperand(target, value, false)
		message := "Invalid conversion from runtime type Double to " + target
		if err == nil || err.Error() != message {
			t.Fatalf("native boxed %s rejection: %v", target, err)
		}
	}
	// Internal scope guard: the captured read authority must not spread to
	// arbitrary boxed Doubles whose Long-cast behavior this packet did not probe.
	ordinary := decimalAsDouble(Decimal(7))
	ordinary.Static = "Object"
	if scaleZeroNumberBoxedLongCastRejected("Long", ordinary) {
		t.Fatal("token-only Double authority escaped to an ordinary value")
	}
}
