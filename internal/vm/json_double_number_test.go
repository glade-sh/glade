package vm

import "testing"

// Native rows: testdata/conformance/double_json_serialization (org.tsv,
// org-extra.tsv). JSON writes a Double exactly as String.valueOf does.
func TestExecJSONSerializeDoubleKeepsFractionalMarker(t *testing.T) {
	program, err := CompileAnonymous(`
Double five = 5.0;
Integer whole = 5;
Double fromInteger = whole;
System.assertEquals('{"d":5.0}', JSON.serialize(new Map<String,Object>{'d' => five}));
System.assertEquals('{"i":5}', JSON.serialize(new Map<String,Object>{'i' => whole}));
System.assertEquals('{"x":5.0}', JSON.serialize(new Map<String,Object>{'x' => fromInteger}));
System.assertEquals('[5.0,5.0]', JSON.serialize(new List<Double>{five, fromInteger}));
System.assertEquals('[5.0,5.0]', JSON.serialize(new List<Object>{five, five}));
System.assertEquals('[ 5.0 ]', JSON.serializePretty(new List<Double>{five}));
System.assertEquals('{"v":5.0}', JSON.serialize(new Map<String,Double>{'v' => 5}));
System.assertEquals('1.0E20', JSON.serialize(Double.valueOf('1e20')));
System.assertEquals('1.0E-4', JSON.serialize(Double.valueOf('0.0001')));
System.assertEquals('0.001', JSON.serialize(Double.valueOf('0.001')));
System.assertEquals('1.0E7', JSON.serialize(Double.valueOf('1e7')));
System.assertEquals('-0.0', JSON.serialize(Double.valueOf('-0.0')));
System.assertEquals('4.9E-324', JSON.serialize(Double.valueOf('4.9E-324')));
System.assertEquals('[5.0]', JSON.serialize(new List<Double>{Decimal.valueOf('5').doubleValue()}));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONSerializeDecimalKeepsScaleAndExponent(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals('5', JSON.serialize(Decimal.valueOf('5')));
System.assertEquals('5.0', JSON.serialize(Decimal.valueOf('5.0')));
System.assertEquals('5.50', JSON.serialize(Decimal.valueOf('5.50')));
System.assertEquals('1E-7', JSON.serialize(Decimal.valueOf('1E-7')));
System.assertEquals('1.0E-7', JSON.serialize(Decimal.valueOf('1.0E-7')));
System.assertEquals('1E+20', JSON.serialize(Decimal.valueOf('1E20')));
System.assertEquals('100000000000000000000', JSON.serialize(Decimal.valueOf('100000000000000000000')));
System.assertEquals('0.000001', JSON.serialize(Decimal.valueOf('0.000001')));
System.assertEquals('[1E-7]', JSON.serialize(new List<Decimal>{Decimal.valueOf('0.0000001')}));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONSerializeNonFiniteDouble(t *testing.T) {
	program, err := CompileAnonymous(`
Double inf = Double.valueOf('Infinity');
Double nan = Double.valueOf('NaN');
System.assertEquals('Infinity', JSON.serialize(inf));
System.assertEquals('{"v":-Infinity}', JSON.serialize(new Map<String,Object>{'v' => Double.valueOf('-Infinity')}));
System.assertEquals('[NaN,NaN]', JSON.serialize(new List<Double>{nan, nan}));
System.assertEquals('[ Infinity ]', JSON.serializePretty(new List<Object>{inf}));
JSONGenerator g = JSON.createGenerator(false);
g.writeStartArray();
g.writeNumber(inf);
g.writeObject(nan);
g.writeEndArray();
System.assertEquals('["Infinity","NaN"]', g.getAsString());
try {
  Decimal d = inf;
  System.assert(false, 'expected MathException');
} catch (MathException e) {
  System.assertEquals('Cannot represent Double \'Infinity\' as a Decimal', e.getMessage());
}
System.assertEquals('∞', inf.format());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDecimalFormatRoundsHalfEvenToThreeDigits(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals('0', Decimal.valueOf('0.0005').format());
System.assertEquals('0.002', Decimal.valueOf('0.0015').format());
System.assertEquals('0.002', Decimal.valueOf('0.0025').format());
System.assertEquals('1.234', Decimal.valueOf('1.2345').format());
System.assertEquals('1', Decimal.valueOf('0.9995').format());
System.assertEquals('-0', Decimal.valueOf('-0.0005').format());
System.assertEquals('0.333', (Decimal.valueOf(1)/3).format());
System.assertEquals('1,234,567.892', Decimal.valueOf('1234567.8915').format());
System.assertEquals('-1,234.568', Decimal.valueOf('-1234.5678').format());
System.assertEquals('123,000', Decimal.valueOf('1.23E+5').format());
System.assertEquals('5.5', Decimal.valueOf('5.50').format());
System.assertEquals('0', Decimal.valueOf('1E-7').format());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}
