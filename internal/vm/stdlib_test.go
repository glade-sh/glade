package vm

import "testing"

func TestExecStringStdlibMethods(t *testing.T) {
	program, err := CompileAnonymous(`
String s = 'AbcDef';
System.assertEquals(6, s.length());
System.assert(s.contains('cD'));
System.assert(s.startsWith('Ab'));
System.assert(s.endsWith('Def'));
System.assertEquals('abcdef', s.toLowerCase());
System.assertEquals('ABCDEF', s.toUpperCase());
String lowerName = 'hello maximillian';
String upperName = 'Hello max';
System.assertEquals('Hello maximillian', lowerName.capitalize());
System.assertEquals('hello max', upperName.uncapitalize());
System.assertEquals('cDef', s.substring(2));
System.assertEquals('cD', s.substring(2, 4));
System.assert(s.containsIgnoreCase('CD'));
System.assert(s.startsWithIgnoreCase('ab'));
System.assert(s.endsWithIgnoreCase('def'));
System.assert(s.equals('AbcDef'));
System.assertEquals(-1, s.compareTo('B'));
System.assertEquals('Abc', s.left(3));
System.assertEquals('Def', s.right(3));
System.assertEquals('  AbcDef', s.leftPad(8));
System.assertEquals('xyAbcDef', s.leftPad(8, 'xy'));
System.assertEquals('AbcDef  ', s.rightPad(8));
System.assertEquals('AbcDefxy', s.rightPad(8, 'xy'));
System.assertEquals(' AbcDef ', s.center(8));
System.assertEquals('cDe', s.mid(2, 3));
System.assertEquals('feDcbA', s.reverse());
String dotted = 'Salesforce.Lightning.platform';
System.assertEquals('Lightning.platform', dotted.substringAfter('.'));
System.assertEquals('platform', dotted.substringAfterLast('.'));
System.assertEquals('Salesforce', dotted.substringBefore('.'));
System.assertEquals('Salesforce.Lightning', dotted.substringBeforeLast('.'));
String force = 'Salesforce and force.com';
System.assertEquals('Sales and .com', force.remove('force'));
System.assertEquals('and force.com', force.removeStart('Salesforce '));
System.assertEquals('and force.com', force.removeStartIgnoreCase('SALESFORCE '));
System.assertEquals('Salesforce and force', force.removeEnd('.com'));
System.assertEquals('Salesforce and force', force.removeEndIgnoreCase('.COM'));
String accent = 'ÄbcDEF';
System.assertEquals('bcDEF', accent.removeStartIgnoreCase('ä'));
System.assertEquals('Äbc', accent.removeEndIgnoreCase('def'));
String spaced = ' a b c ';
System.assertEquals('abc', spaced.deleteWhitespace());
String manySpaces = ' a   b  c ';
System.assertEquals('a b c', manySpaces.normalizeSpace());
String ab = 'ab';
System.assertEquals('ababab', ab.repeat(3));
System.assertEquals('ab|ab|ab', ab.repeat('|', 3));
System.assert(String.isEmpty(null));
System.assert(String.isEmpty(''));
System.assert(!String.isEmpty(' '));
System.assert(String.isNotEmpty('x'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringStdlibMoreMethods(t *testing.T) {
	program, err := CompileAnonymous(`
String letters = 'a b c 5 xyz';
System.assertEquals('1 1 1 5 111', letters.replaceAll('[a-zA-Z]', '1'));
String lettersFirst = 'a b c 11 xyz';
System.assertEquals('a b c 11 2z', lettersFirst.replaceFirst('[a-zA-Z]{2}', '2'));
String splitSource = 'boo:and:moo';
List<String> limitTwo = splitSource.split(':', 2);
System.assertEquals(2, limitTwo.size());
System.assertEquals('boo', limitTwo.get(0));
System.assertEquals('and:moo', limitTwo.get(1));
List<String> limitFive = splitSource.split('o', 5);
System.assertEquals(5, limitFive.size());
System.assertEquals('b', limitFive.get(0));
System.assertEquals('', limitFive.get(1));
System.assertEquals(':and:m', limitFive.get(2));
System.assertEquals('', limitFive.get(3));
System.assertEquals('', limitFive.get(4));
List<String> limitZero = splitSource.split('o', 0);
System.assertEquals(3, limitZero.size());
System.assertEquals(':and:m', limitZero.get(2));
List<String> limitNegative = splitSource.split('o', -2);
System.assertEquals(5, limitNegative.size());
System.assertEquals('', limitNegative.get(4));
String helloJaneSpace = 'Hello Jane';
System.assert(helloJaneSpace.containsWhitespace());
String helloJane = 'HelloJane';
System.assert(!helloJane.containsWhitespace());
String helloHello = 'Hello Hello';
System.assertEquals(2, helloHello.countMatches('Hello'));
String aaa = 'aaa';
System.assertEquals(0, aaa.countMatches(''));
String hello = 'hello';
System.assert(hello.containsAny('hx'));
System.assert(!hello.containsAny('xz'));
String abcde = 'abcde';
System.assert(abcde.containsNone('fg'));
System.assert(!abcde.containsNone('df'));
String abba = 'abba';
System.assert(abba.containsOnly('abcd'));
String abbaXyz = 'abba xyz';
System.assert(!abbaXyz.containsOnly('abcd'));
String oneSpace = ' ';
System.assert(oneSpace.isWhitespace());
String empty = '';
System.assert(empty.isWhitespace());
String sil80 = 'SIL80';
System.assert(!sil80.isWhitespace());
String alphaAccent = 'abcÉ';
System.assert(alphaAccent.isAlpha());
String alphaDigits = 'abc 21';
System.assert(!alphaDigits.isAlpha());
String alphaSpace = 'aA Bb';
System.assert(alphaSpace.isAlphaSpace());
String alphaDollar = 'aA$Bb';
System.assert(!alphaDollar.isAlphaSpace());
String abc021 = 'abc021';
System.assert(abc021.isAlphanumeric());
String romanNumeral = 'Ⅻ';
System.assert(!romanNumeral.isAlphanumeric());
String ae86 = 'AE 86';
System.assert(ae86.isAlphanumericSpace());
String alphaDollarDigits = 'aA$12';
System.assert(!alphaDollarDigits.isAlphanumericSpace());
String digits = '1234567890';
System.assert(digits.isNumeric());
String decimalPoint = '1.2';
System.assert(!decimalPoint.isNumeric());
String numericSpace = '1 2 3';
System.assert(numericSpace.isNumericSpace());
	String mixedCars = 'FD3S FC3S';
	System.assert(!mixedCars.isNumericSpace());
	System.assert(abcde.isAllLowerCase());
String lowerWithDigits = 'abc 123!';
System.assert(lowerWithDigits.isAllLowerCase());
	String abcDe = 'abcDe';
	System.assert(!abcDe.isAllLowerCase());
System.assert(!digits.isAllLowerCase());
System.assert(!empty.isAllLowerCase());
	String ABCDE = 'ABCDE';
	System.assert(ABCDE.isAllUpperCase());
String upperWithDigits = 'ABC 123!';
System.assert(upperWithDigits.isAllUpperCase());
	String ABCdE = 'ABCdE';
	System.assert(!ABCdE.isAllUpperCase());
System.assert(!digits.isAllUpperCase());
System.assert(!empty.isAllUpperCase());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringStdlibMoreRejectsBadRegex(t *testing.T) {
	program, err := CompileAnonymous(`String abc = 'abc';
abc.replaceAll('[', 'x');`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err == nil {
		t.Fatal("expected invalid regex error")
	}
}

func TestStringStdlibMoreRejectsBadArgumentShapes(t *testing.T) {
	tests := []struct {
		method string
		args   []Value
	}{
		{method: "replaceAll", args: []Value{String("[a]"), Int(1)}},
		{method: "replaceFirst", args: []Value{String("[a]")}},
		{method: "split", args: []Value{String(","), String("2")}},
		{method: "containsWhitespace", args: []Value{String("x")}},
		{method: "isAlpha", args: []Value{String("x")}},
	}
	for _, tc := range tests {
		if _, handled, err := callStringMember(String("abc"), tc.method, tc.args); !handled || err == nil {
			t.Fatalf("%s expected handled error, handled=%v err=%v", tc.method, handled, err)
		}
	}
}

func TestExecBlobEncodingCryptoStdlib(t *testing.T) {
	program, err := CompileAnonymous(`
Blob hello = Blob.valueOf('hello');
System.assertEquals('hello', hello.toString());
System.assertEquals(5, hello.size());
System.assertEquals('68656c6c6f', EncodingUtil.convertToHex(hello));
Blob decodedHex = EncodingUtil.convertFromHex('68656C6C6F');
System.assertEquals('hello', decodedHex.toString());
System.assertEquals('aGVsbG8=', EncodingUtil.base64Encode(hello));
Blob decodedBase64 = EncodingUtil.base64Decode('aGVsbG8=');
System.assertEquals('hello', decodedBase64.toString());
Blob md5 = Crypto.generateDigest('MD5', hello);
Blob sha1 = Crypto.generateDigest('SHA1', hello);
Blob sha256 = Crypto.generateDigest('SHA-256', hello);
Blob sha512 = Crypto.generateDigest('SHA-512', hello);
Blob sha3 = Crypto.generateDigest('SHA3-256', hello);
System.assertEquals('5d41402abc4b2a76b9719d911017c592', EncodingUtil.convertToHex(md5));
System.assertEquals('aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d', EncodingUtil.convertToHex(sha1));
System.assertEquals('2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824', EncodingUtil.convertToHex(sha256));
System.assertEquals('9b71d224bd62f3785d96d46ad3ea3d73319bfbc2890caadae2dff72519673ca72323c3d99ba5c11d7c7acc6e14b8c5da0c4663475c2e5c3adef46f73bcdec043', EncodingUtil.convertToHex(sha512));
System.assertEquals('3338be694f50c5f338814986cdf0686453a888b84f424d792af4b9202398f392', EncodingUtil.convertToHex(sha3));
Blob message = Blob.valueOf('message');
Blob key = Blob.valueOf('key');
Blob hmacMD5 = Crypto.generateMac('hmacMD5', message, key);
Blob hmacSHA1 = Crypto.generateMac('hmacSHA1', message, key);
Blob hmacSHA256 = Crypto.generateMac('HmacSHA256', message, key);
Blob hmacSHA512 = Crypto.generateMac('hmacSHA512', message, key);
System.assertEquals('4e4748e62b463521f6775fbf921234b5', EncodingUtil.convertToHex(hmacMD5));
System.assertEquals('2088df74d5f2146b48146caf4965377e9d0be3a4', EncodingUtil.convertToHex(hmacSHA1));
System.assertEquals('6e9ef29b75fffc5b7abae527d58fdadb2fe42e7219011976917343065f58ed4a', EncodingUtil.convertToHex(hmacSHA256));
System.assertEquals('e477384d7ca229dd1426e64b63ebf2d36ebd6d7e669a6735424e72ea6c01d3f8b56eb39c36d8232f5427999b8d1a3f9cd1128fc69f4d75b434216810fa367e98', EncodingUtil.convertToHex(hmacSHA512));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestBlobEncodingCryptoStdlibRejectsBadInputs(t *testing.T) {
	tests := []string{
		"Blob b = Blob.valueOf('abc'); b.size(1);",
		"EncodingUtil.base64Decode('not base64');",
		"EncodingUtil.convertFromHex('abc');",
		"EncodingUtil.convertFromHex('zz');",
		"Crypto.generateDigest('SHA-999', Blob.valueOf('x'));",
		"Crypto.generateMac('hmacSHA999', Blob.valueOf('x'), Blob.valueOf('key'));",
		"Crypto.generateMac('hmacSHA256', Blob.valueOf('x'), 'key');",
	}
	for _, source := range tests {
		program, err := CompileAnonymous(source)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Execute(program, nil); err == nil {
			t.Fatalf("expected error for %s", source)
		}
	}
}

func TestExecNumericStdlibExpansion(t *testing.T) {
	program, err := CompileAnonymous(`
Integer i = Integer.valueOf('42');
Long l = Long.valueOf('9001');
Decimal d = Decimal.valueOf('12.5');
Double x = Double.valueOf('2.25');
System.assertEquals(42, i);
System.assertEquals(9001, l);
System.assertEquals(12.5, d);
System.assertEquals(2.25, x);
System.assertEquals('42', i.format());
System.assertEquals(42.0, i.doubleValue());
System.assertEquals(12, d.intValue());
System.assertEquals(12, d.longValue());
System.assertEquals(12.5, d.doubleValue());
System.assertEquals(12.5, d.abs());
System.assertEquals(156.25, d.pow(2));
System.assertEquals('12.5', d.format());
System.assertEquals(1, Math.signum(12.5));
System.assertEquals(-1, Math.signum(-4));
System.assertEquals(0, Math.signum(0));
System.assertEquals(1, Math.mod(10, 3));
System.assertEquals(2.5, Math.mod(12.5, 5));
System.assertEquals(13, Math.roundToLong(12.5));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecNumericStdlibRejectsIntegerOverflow(t *testing.T) {
	tests := []string{
		"Decimal d = Decimal.valueOf('99999999999999999999999.5');\nInteger.valueOf(d);",
		"Decimal d = Decimal.valueOf('99999999999999999999999.5');\nd.intValue();",
		"Decimal d = Decimal.valueOf('99999999999999999999999.5');\nd.longValue();",
		"Decimal d = Decimal.valueOf('99999999999999999999999.5');\nd.round();",
		"Decimal d = Decimal.valueOf('99999999999999999999999.5');\nMath.roundToLong(d);",
	}
	for _, source := range tests {
		program, err := CompileAnonymous(source)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Execute(program, nil); err == nil {
			t.Fatalf("expected overflow error for %s", source)
		}
	}
}

func TestExecCollectionStdlibExpansion(t *testing.T) {
	program, err := CompileAnonymous(`
List<Integer> xs = new List<Integer>{1, 2};
System.assert(!xs.isEmpty());
xs.add(3);
xs.add(1, 9);
System.assertEquals(4, xs.size());
System.assertEquals(9, xs.get(1));
System.assertEquals(1, xs.indexOf(9));
System.assertEquals(-1, xs.indexOf(99));
System.assertEquals(9, xs.remove(1));
xs.set(0, 7);
List<Integer> more = new List<Integer>{8, 9};
xs.addAll(more);
System.assertEquals(7, xs.get(0));
System.assertEquals(9, xs.get(xs.size() - 1));
xs.clear();
System.assert(xs.isEmpty());
System.assertEquals(0, xs.size());

Set<String> names = new Set<String>{'a'};
System.assert(!names.isEmpty());
System.assert(names.add('b'));
System.assert(!names.add('b'));
System.assert(names.containsAll(new List<String>{'a', 'b'}));
System.assert(names.remove('a'));
System.assert(!names.contains('a'));
System.assert(names.addAll(new List<String>{'c', 'd'}));
System.assert(names.removeAll(new Set<String>{'b'}));
System.assert(names.retainAll(new List<String>{'c'}));
System.assert(names.contains('c'));
System.assertEquals(1, names.size());
names.clear();
System.assert(names.isEmpty());
System.assertEquals(0, names.size());

Map<String,Integer> counts = new Map<String,Integer>();
System.assert(counts.isEmpty());
System.assertEquals(null, counts.put('a', 1));
System.assertEquals(1, counts.put('a', 2));
System.assert(!counts.isEmpty());
System.assert(counts.containsValue(2));
Set<String> keys = counts.keySet();
System.assert(keys.contains('a'));
List<Integer> values = counts.values();
System.assert(values.contains(2));
Map<String,Integer> moreCounts = new Map<String,Integer>();
moreCounts.put('b', 3);
counts.putAll(moreCounts);
System.assertEquals(3, counts.get('b'));
System.assertEquals(2, counts.remove('a'));
System.assertEquals(null, counts.remove('missing'));
counts.clear();
System.assert(counts.isEmpty());
System.assert(!counts.containsKey('b'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
