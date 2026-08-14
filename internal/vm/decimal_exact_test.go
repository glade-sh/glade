package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestExecDecimalPreservesExactArithmeticAndStorageText(t *testing.T) {
	program, err := CompileAnonymous(`
Decimal a = Decimal.valueOf('9007199254740993');
Decimal b = Decimal.valueOf('0.01');
System.assertEquals('9007199254740993.01', (a + b).toPlainString());
System.assert(a + b > a);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecDecimalPreservesExactIntegerConversionAndUnaryText(t *testing.T) {
	program, err := CompileAnonymous(`
Decimal value = Decimal.valueOf('9007199254740993');
System.assertEquals(9007199254740993, value.longValue());
System.assertEquals('-9007199254740993', (-value).toPlainString());
Long whole = 9007199254740993L;
System.assertEquals('9007199254740993', whole.decimalValue().toPlainString());
System.assertEquals('9,007,199,254,740,993', whole.decimalValue().format());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestDecimalConstructorRetainsAuthoritativeText(t *testing.T) {
	value := Decimal(0.1)
	if value.Text != "0.1" {
		t.Fatalf("Decimal text = %q, want 0.1", value.Text)
	}
}

func TestExecDecimalCanonicalizesExactText(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals('1.20', Decimal.valueOf('0001.20').toPlainString());
System.assertEquals('0.0', Decimal.valueOf('-0.0').toPlainString());
System.assertEquals('1.20', Decimal.valueOf('+1.20').toPlainString());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecDecimalPreservesDoubleFloatSemantics(t *testing.T) {
	program, err := CompileAnonymous(`
Double doubleValue = Double.valueOf('9007199254740993');
System.assertEquals(9007199254740992, doubleValue + 1);
Decimal decimalValue = Decimal.valueOf('9007199254740993');
System.assertEquals(9007199254740992, decimalValue.doubleValue());
System.assertEquals(9007199254740992, decimalValue.doubleValue() + 1);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecDecimalPreservesStaticIntegerConversions(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(2147483647, Integer.valueOf(Decimal.valueOf('2147483647.999999999')));
System.assertEquals(9223372036854775807, Long.valueOf(Decimal.valueOf('9223372036854775807')));
Decimal decimalValue = Decimal.valueOf('9007199254740993');
Long castValue = (Long) decimalValue;
System.assertEquals(9007199254740993, castValue);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestDecimalFloatBackedMarkerIsCaseInsensitive(t *testing.T) {
	value := Decimal(9007199254740992)
	value.Text = "9007199254740993"
	value.Static = "double"
	got, err := evalBinary("+", value, Int(1))
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "9007199254740992" {
		t.Fatalf("float-backed decimal text = %q, want 9007199254740992", got.Text)
	}
	if !strings.EqualFold(got.Static, "double") {
		t.Fatalf("float-backed decimal marker = %q, want double", got.Static)
	}
}

func TestExecMathResultsRemainFloatBacked(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(9007199254740992, Math.pow(9007199254740992L, 1) + 1);
System.assertEquals(9007199254740992, Math.random() + 9007199254740992L);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecDecimalDisplayPreservesExponentText(t *testing.T) {
	program, err := CompileAnonymous(`
Decimal value = Decimal.valueOf('9.007199254740993E15');
String display = '' + value;
System.assertEquals('9007199254740993', display);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecDecimalMathOverloadsRemainExact(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals('9007199254740993', Math.max(Decimal.valueOf('9007199254740993'), Decimal.valueOf('9007199254740992')).toPlainString());
System.assertEquals('9007199254740993', Math.floor(9007199254740993.9).toPlainString());
Double doubleValue = Double.valueOf('9007199254740993');
Decimal converted = Decimal.valueOf(doubleValue);
System.assertEquals('9007199254740993', (converted + 1).toPlainString());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecDecimalPowPreservesExactText(t *testing.T) {
	program, err := CompileAnonymous(`
Decimal value = Decimal.valueOf('9007199254740993');
System.assertEquals('9007199254740993', value.pow(1).toPlainString());
System.assertEquals('1.5625', Decimal.valueOf('1.25').pow(2).toPlainString());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecDecimalMathRoundingAndSignumRemainExact(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(9007199254740993, Math.roundToLong(Decimal.valueOf('9007199254740993.4')));
System.assertEquals(2147483645, Math.round(Decimal.valueOf('2147483645.499999999')));
System.assertEquals(9007199254740993, Math.roundToLong(9007199254740993L));
System.assertEquals('1', Math.signum(Decimal.valueOf('9007199254740993')).toPlainString());
System.assertEquals('1', Math.signum(1).toPlainString());
Double doubleValue = Double.valueOf('-9007199254740993');
System.assertEquals(-1, Math.signum(doubleValue));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}

	program, err = CompileAnonymous(`Math.round(Double.valueOf('2147483648'));`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err == nil {
		t.Fatal("Math.round accepted a Double outside the Integer result range")
	}
}

func TestExecMathModRejectsDecimalOperands(t *testing.T) {
	program, err := CompileAnonymous(`
Math.mod(Decimal.valueOf('5.5'), Decimal.valueOf('2'));
`)
	if err != nil {
		return
	}
	if _, err := Execute(program, nil); err == nil {
		t.Fatal("Math.mod accepted Decimal operands despite having only Integer and Long overloads")
	}
}

func TestExecMathLongOverloadsRetainLongIdentity(t *testing.T) {
	for _, source := range []string{
		"System.assert(!(Math.roundToLong(9007199254740993L) instanceof Integer));",
		"System.assert(!(Math.max(3, 2L) instanceof Integer));",
		"System.assert(!(Math.min(3, 4L) instanceof Integer));",
		"System.assert(!(Math.mod(5, 3L) instanceof Integer));",
		"System.assert(!(Math.abs(-5L) instanceof Integer));",
		"System.assert(!(Decimal.valueOf('42').longValue() instanceof Integer));",
		"System.assert(!(Decimal.valueOf('42.1').round() instanceof Integer));",
		"System.assert(!(1L + 2L instanceof Integer));",
		"Integer integerValue = 42; System.assert(!(integerValue.longValue() instanceof Integer));",
		"Long longValue = 42L; System.assert(longValue.intValue() instanceof Integer);",
		"Long castValue = (Long) Decimal.valueOf('42'); System.assert(!(castValue instanceof Integer));",
	} {
		t.Run(source, func(t *testing.T) {
			program, err := CompileAnonymous(source)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Execute(program, nil); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestExecLongCoercionRetainsLongIdentity(t *testing.T) {
	program, err := CompileAnonymous(`
Long widened = 1;
System.assert(widened instanceof Long);
System.assert(!(widened instanceof Integer));
List<Long> values = new List<Long>{1};
System.assert(values[0] instanceof Long);
System.assert(!(values[0] instanceof Integer));
System.assert(!((widened << 1) instanceof Integer));
Long minimum = Integer.MIN_VALUE;
System.assert(!(Math.abs(minimum) instanceof Integer));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecDeclaredLongMethodReturnRetainsLongIdentity(t *testing.T) {
	returnProgram, err := CompileAnonymous(`return 1;`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`System.assert(!(LongSource.get() instanceof Integer));`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterMethod(Method{
		Name:       "LongSource.get",
		ClassName:  "LongSource",
		IsStatic:   true,
		ReturnType: "Long",
		Program:    returnProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDecimalPowUsesCaseInsensitiveSurfaceAndIntegerExponent(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals('1.953125', Decimal.valueOf('1.25').Pow(3).toPlainString());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}

	program, err = CompileAnonymous(`Decimal.valueOf('1.25').pow(3L);`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err == nil {
		t.Fatal("Decimal.pow accepted a Long exponent despite its Integer-only surface")
	}
}

func TestExecLongConstantsAndJSONPathsRetainIdentity(t *testing.T) {
	program, err := CompileAnonymous(`
System.assert(!(Long.MAX_VALUE instanceof Integer));
System.assert(!(Long.MIN_VALUE instanceof Integer));
JSONParser parser = JSON.createParser('1');
parser.nextToken();
System.assert(!(parser.getLongValue() instanceof Integer));
Object parsed = JSON.deserialize('1', Long.class);
System.assert(!(parsed instanceof Integer));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecDeclaredLongPlatformReturnsRetainIdentity(t *testing.T) {
	program, err := CompileAnonymous(`
System.assert(!(Crypto.getRandomLong() instanceof Integer));
System.assert(!(Datetime.now().getTime() instanceof Integer));
System.assert(!(System.currentTimeMillis() instanceof Integer));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestDecimalExactTextRoundTripsThroughStorageAndJSON(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Probe__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Probe__c",
			KeyPrefix: "a90",
			Fields: map[string]storage.Field{
				"Id":        {APIName: "Id", Type: storage.FieldID},
				"Name":      {APIName: "Name", Type: storage.FieldString},
				"Amount__c": {APIName: "Amount__c", Type: storage.FieldDecimal},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	program, err := CompileAnonymous(`
Decimal firstAmount = Decimal.valueOf('9007199254740993.01');
Decimal secondAmount = Decimal.valueOf('9007199254740993.02');
Probe__c first = new Probe__c(Name = 'first', Amount__c = firstAmount);
Probe__c second = new Probe__c(Name = 'second', Amount__c = secondAmount);
insert new List<Probe__c>{first, second};
Decimal threshold = Decimal.valueOf('9007199254740993.00');
List<Probe__c> rows = [SELECT Amount__c FROM Probe__c WHERE Amount__c > :threshold ORDER BY Amount__c];
System.assertEquals(2, rows.size());
System.assertEquals('9007199254740993.01', rows[0].Amount__c.toPlainString());
System.assertEquals('9007199254740993.02', rows[1].Amount__c.toPlainString());
List<AggregateResult> counts = [SELECT COUNT_DISTINCT(Amount__c) distinctCount FROM Probe__c];
System.assertEquals(2, counts[0].get('distinctCount'));
String payload = JSON.serialize(first);
Probe__c decoded = (Probe__c)JSON.deserialize(payload, Probe__c.class);
System.assertEquals('9007199254740993.01', decoded.Amount__c.toPlainString());
List<Decimal> values = new List<Decimal>{secondAmount, firstAmount};
values.sort();
System.assertEquals('9007199254740993.01', values[0].toPlainString());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDoublePathsRetainDoubleIdentity(t *testing.T) {
	program, err := CompileAnonymous(`
JSONParser parser = JSON.createParser('9007199254740993.0');
parser.nextToken();
Double parsed = parser.getDoubleValue();
System.assertEquals(9007199254740992, parsed + 1);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
	location := newLocation(Decimal(37.7749), Decimal(-122.4194))
	if !isFloatBackedDecimal(location.Fields["latitude"]) || !isFloatBackedDecimal(location.Fields["longitude"]) {
		t.Fatalf("Location coordinates = %#v, want Double-backed values", location.Fields)
	}
	distance, err := locationDistance(location, newLocation(Decimal(34.0522), Decimal(-118.2437)), "mi")
	if err != nil {
		t.Fatal(err)
	}
	if !isFloatBackedDecimal(distance) {
		t.Fatalf("Location distance = %#v, want Double-backed value", distance)
	}
	missing := Object("Location")
	latitude, _, _, handled, err := callLocationMember(missing, "getLatitude", nil)
	if err != nil || !handled || !isFloatBackedDecimal(latitude) {
		t.Fatalf("missing Location latitude = handled %v, value %#v, err %v; want Double-backed zero", handled, latitude, err)
	}
}

func TestTestLoadDataPreservesExactDecimalText(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Probe__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "Probe__c",
		Fields: map[string]storage.Field{
			"Amount__c": {APIName: "Amount__c", Type: storage.FieldDecimal},
		},
	}}
	machine := New(nil)
	machine.SetOrg(&org)
	value, err := machine.testLoadDataFieldValue("Probe__c", "Amount__c", "9007199254740993.01")
	if err != nil {
		t.Fatal(err)
	}
	if value.Text != "9007199254740993.01" {
		t.Fatalf("loaded Decimal text = %q, want exact source text", value.Text)
	}
}

func TestDecimalDivisionRejectsUntextedDecimal(t *testing.T) {
	receiver := Value{Kind: ValueDecimal, Decimal: 1}
	_, _, _, handled, err := callDecimalMember(receiver, "divide", []Value{Decimal(3), Int(2)})
	if !handled || err == nil || !strings.Contains(err.Error(), "Decimal division exact semantics are deferred") {
		t.Fatalf("untexted Decimal division = handled %v, err %v; want explicit unsupported", handled, err)
	}
}

func TestExecMathAbsRejectsIntegerOverflow(t *testing.T) {
	program, err := CompileAnonymous(`Math.abs(Integer.MIN_VALUE);`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err == nil {
		t.Fatal("Math.abs accepted an Integer result outside the Integer range")
	}
}

func TestExecDecimalDivisionIsExplicitlyUnsupported(t *testing.T) {
	for _, source := range []string{
		"Decimal result = Decimal.valueOf('9007199254740993') / Decimal.valueOf('1');",
		"Decimal result = Decimal.valueOf('9007199254740993').divide(Decimal.valueOf('1'), 0);",
	} {
		program, err := CompileAnonymous(source)
		if err != nil {
			t.Fatalf("compile Decimal division probe %q: %v", source, err)
		}
		if _, err := Execute(program, nil); err == nil {
			t.Fatalf("Decimal division unexpectedly succeeded: %s", source)
		}
	}
}

func TestExecDoubleRejectsDecimalOnlyMembers(t *testing.T) {
	for _, source := range []string{
		"Double value = Double.valueOf('1.25'); value.toPlainString();",
		"Double value = Double.valueOf('1.25'); value.setScale(1);",
		"Double value = Double.valueOf('1.25'); value.precision();",
		"Double value = Double.valueOf('-1.25'); value.abs();",
	} {
		program, err := CompileAnonymous(source)
		if err != nil {
			continue
		}
		if _, err := Execute(program, nil); err == nil {
			t.Fatalf("Double accepted a Decimal-only member: %s", source)
		}
	}
}

func TestExecDecimalListSortPreservesExactOrdering(t *testing.T) {
	program, err := CompileAnonymous(`
List<Decimal> values = new List<Decimal>{
    Decimal.valueOf('9007199254740993'),
    Decimal.valueOf('9007199254740992')
};
values.sort();
System.assertEquals('9007199254740992', values[0].toPlainString());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
