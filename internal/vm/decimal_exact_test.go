package vm

import "testing"

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
