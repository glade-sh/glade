package vm

import (
	"strings"
	"testing"
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

func TestExecDecimalPreservesDoubleFloatSemantics(t *testing.T) {
	program, err := CompileAnonymous(`
Double doubleValue = Double.valueOf('9007199254740993');
System.assertEquals('9007199254740992', (doubleValue + 1).toPlainString());
Decimal decimalValue = Decimal.valueOf('9007199254740993');
System.assertEquals('9007199254740992', decimalValue.doubleValue().toPlainString());
System.assertEquals('9007199254740992', (decimalValue.doubleValue() + 1).toPlainString());
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
System.assertEquals('9007199254740992', (Math.pow(9007199254740992L, 1) + 1).toPlainString());
System.assertEquals('9007199254740992', (Math.random() + 9007199254740992L).toPlainString());
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

func TestExecDecimalMathRoundingAndSignumRemainExact(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(9007199254740993, Math.roundToLong(Decimal.valueOf('9007199254740993.4')));
System.assertEquals(2147483645, Math.round(Decimal.valueOf('2147483645.499999999')));
System.assertEquals('1', Math.signum(Decimal.valueOf('9007199254740993')).toPlainString());
Double doubleValue = Double.valueOf('-9007199254740993');
System.assertEquals('-1', Math.signum(doubleValue).toPlainString());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
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
