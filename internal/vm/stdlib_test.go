package vm

import (
	"strings"
	"testing"

	"github.com/open-aer/oaer/internal/storage"
)

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

func TestExecStringStdlibCompletionMethods(t *testing.T) {
	program, err := CompileAnonymous(`
String csv = 'a,"b",c';
System.assertEquals('"a,""b"",c"', csv.escapeCsv());
String escapedCsv = csv.escapeCsv();
System.assertEquals(csv, escapedCsv.unescapeCsv());
String html = '<tag attr=''x''>&';
System.assertEquals('&lt;tag attr=&#39;x&#39;&gt;&amp;', html.escapeHtml4());
String escapedHtml = html.escapeHtml4();
System.assertEquals(html, escapedHtml.unescapeHtml4());
System.assertEquals('&lt;tag attr=&apos;x&apos;&gt;&amp;', html.escapeXml());
String escapedXml = html.escapeXml();
System.assertEquals(html, escapedXml.unescapeXml());
String slash = 'a/b';
System.assertEquals('a\/b', slash.escapeEcmaScript());
String escapedSlash = slash.escapeEcmaScript();
System.assertEquals(slash, escapedSlash.unescapeEcmaScript());
String quoted = 'He said "hi"';
System.assertEquals('He said \"hi\"', quoted.escapeJava());
String escapedQuoted = quoted.escapeJava();
System.assertEquals(quoted, escapedQuoted.unescapeJava());
String omega = 'AΩ';
System.assertEquals('A\u03A9', omega.escapeUnicode());
String escapedOmega = omega.escapeUnicode();
System.assertEquals(omega, escapedOmega.unescapeUnicode());
System.assertEquals('Bob\''s', String.escapeSingleQuotes('Bob''s'));
List<String> formatArgs = new List<String>();
formatArgs.add('Ada');
formatArgs.add('Lovelace');
System.assertEquals('Hello Ada Lovelace', String.format('Hello {0} {1}', formatArgs));
String alphabet = 'abcdefghijklmnopqrstuvwxyz';
System.assertEquals('abcdefg...', alphabet.abbreviate(10));
System.assertEquals('...ijklmn...', alphabet.abbreviate(8, 12));
String machine = 'i am a machine';
System.assertEquals('robot', machine.difference('i am a robot'));
String interstate = 'interstate';
System.assertEquals('interst', interstate.commonPrefix('interstellar'));
List<String> prefixes = new List<String>();
prefixes.add('flower');
prefixes.add('flow');
prefixes.add('flight');
System.assertEquals('fl', String.getCommonPrefix(prefixes));
String kitten = 'kitten';
System.assertEquals(3, kitten.getLevenshteinDistance('sitting'));
System.assertEquals(3, String.getLevenshteinDistance('kitten', 'sitting'));
String chars = 'AΩ';
System.assertEquals('A', chars.charAt(0));
System.assertEquals(937, chars.codePointAt(1));
System.assertEquals(65, chars.codePointBefore(1));
System.assertEquals(2, chars.codePointCount(0, 2));
List<Integer> charCodes = chars.getChars();
System.assertEquals(2, charCodes.size());
System.assertEquals(65, charCodes.get(0));
System.assertEquals(937, charCodes.get(1));
System.assertEquals(chars, String.fromCharArray(charCodes));
String printable = 'AZ 19~';
String nonPrintable = 'Snow Ω';
System.assert(printable.isAsciiPrintable());
System.assert(!nonPrintable.isAsciiPrintable());
String typeSource = 'ab12 CD';
List<String> splitType = typeSource.splitByCharacterType();
System.assertEquals(4, splitType.size());
System.assertEquals('ab', splitType.get(0));
System.assertEquals('12', splitType.get(1));
System.assertEquals(' ', splitType.get(2));
System.assertEquals('CD', splitType.get(3));
String camelSource = 'HTTPServer42';
List<String> camel = camelSource.splitByCharacterTypeCamelCase();
System.assertEquals(3, camel.size());
System.assertEquals('HTTP', camel.get(0));
System.assertEquals('Server', camel.get(1));
System.assertEquals('42', camel.get(2));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestStringStdlibCompletionRejectsBadArguments(t *testing.T) {
	tests := []struct {
		method string
		args   []Value
	}{
		{method: "escapeCsv", args: []Value{String("x")}},
		{method: "abbreviate", args: []Value{Int(3)}},
		{method: "charAt", args: []Value{Int(-1)}},
		{method: "codePointAt", args: []Value{Int(9)}},
		{method: "codePointBefore", args: []Value{Int(0)}},
		{method: "codePointCount", args: []Value{Int(1), Int(0)}},
		{method: "splitByCharacterType", args: []Value{String("x")}},
	}
	for _, tc := range tests {
		if _, handled, err := callStringMember(String("abc"), tc.method, tc.args); !handled || err == nil {
			t.Fatalf("%s expected handled error, handled=%v err=%v", tc.method, handled, err)
		}
	}
	if _, err := stringStatic("String.format", []Value{String("{0}"), String("x")}); err == nil {
		t.Fatal("String.format expected bad argument error")
	}
	if _, err := stringStatic("String.fromCharArray", []Value{List(String("x"))}); err == nil {
		t.Fatal("String.fromCharArray expected bad argument error")
	}
	if _, _, err := callStringMember(String(`\u00ZZ`), "unescapeUnicode", nil); err == nil {
		t.Fatal("String.unescapeUnicode expected bad escape error")
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

func TestExecCoreTypeIDURLObjectStdlib(t *testing.T) {
	program, err := CompileAnonymous(`
Type accountType = Type.forName('Account');
Type accountTypeAgain = Type.forName('Account');
Type contactType = Type.forName('Contact');
String accountName = 'Account';
System.assertEquals('Account', accountType.getName());
System.assertEquals('Account', accountType.toString());
System.assert(accountType.equals(accountTypeAgain));
System.assert(!accountType.equals(contactType));
System.assertEquals(accountName.hashCode(), accountType.hashCode());

Id valid = Id.valueOf('001B000001DVM9t');
Id same = Id.valueOf('001B000001DVM9t', false);
Id restored = Id.valueOf('001b000001dvm9tIAH', true);
System.assert(valid.equals(same));
System.assertEquals('001B000001DVM9t', valid.toString());
System.assertEquals('001B000001DVM9t', valid.to15());
System.assertEquals('001B000001DVM9tIAH', valid.to18());
Id longId = Id.valueOf('001B000001DVM9tIAH');
System.assertEquals('001B000001DVM9t', longId.to15());
System.assertEquals('001B000001DVM9tIAH', longId.to18());
System.assertEquals('001B000001DVM9tIAH', restored.toString());

String text = 'trail';
System.assert(text.equals('trail'));
System.assert(!text.equals('ridge'));
System.assertEquals('trail', text.toString());
String sameText = 'trail';
System.assertEquals(sameText.hashCode(), text.hashCode());
Integer count = 7;
System.assert(count.equals(7));
System.assertEquals('7', count.toString());

URL base = URL.getOrgDomainUrl();
System.assertEquals('https://local.oaer.example', base.toExternalForm());
System.assertEquals('https', base.getProtocol());
System.assertEquals('local.oaer.example', base.getHost());
System.assertEquals(443, base.getDefaultPort());
System.assertEquals(-1, base.getPort());
URL detailed = new URL('https://example.test:8443/apex/Page?id=001#top');
URL detailedAgain = new URL('https://example.test:8443/apex/Page?id=001#top');
System.assert(detailed.equals(detailedAgain));
System.assertEquals(detailed.hashCode(), detailedAgain.hashCode());
System.assertEquals('example.test', detailed.getHost());
System.assertEquals('example.test:8443', detailed.getAuthority());
System.assertEquals('/apex/Page', detailed.getPath());
System.assertEquals('id=001', detailed.getQuery());
System.assertEquals('top', detailed.getRef());
System.assertEquals('/apex/Page?id=001', detailed.getFile());
System.assertEquals(8443, detailed.getPort());
URL protocolHost = new URL('https', 'example.test', '/trail');
System.assertEquals('https://example.test/trail', protocolHost.toExternalForm());
URL protocolHostPort = new URL('https', 'example.test', 8443, '/ridge');
System.assertEquals('https://example.test:8443/ridge', protocolHostPort.toExternalForm());
URL relative = new URL(detailed, '../Other?x=1');
System.assertEquals('https://example.test:8443/Other?x=1', relative.toExternalForm());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecCoreSystemTimeAndDebugStdlib(t *testing.T) {
	program, err := CompileAnonymous(`
Date today = System.today();
System.assertEquals('2026-05-02', today.format(), 'System.today should use the VM clock');
Datetime now = System.now();
System.assertEquals('2026-05-02T12:00:00Z', now.format());
System.assertEquals(1777723200000, System.currentTimeMillis());
System.debug(LoggingLevel.INFO, 'logged with level');
System.debug('logged without level');
`)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Execute(program, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(result.Debug), 2; got != want {
		t.Fatalf("debug lines = %d, want %d: %#v", got, want, result.Debug)
	}
	if result.Debug[0] != "logged with level" || result.Debug[1] != "logged without level" {
		t.Fatalf("debug lines = %#v", result.Debug)
	}
}

func TestExecCoreExceptionStdlibMethods(t *testing.T) {
	program, err := CompileAnonymous(`
Exception constructed = new DmlException('blocked');
System.assertEquals('blocked', constructed.getMessage());
System.assertEquals('DmlException', constructed.getTypeName());
System.assertEquals(0, constructed.getLineNumber());
System.assertEquals('', constructed.getStackTraceString());
System.assertEquals('System.DmlException: blocked', constructed.toString());
Exception noMessage = new DmlException();
System.assertEquals(null, noMessage.getMessage());

String caught = '';
try {
	throw new QueryException('bad query');
} catch (Exception e) {
	caught = e.getTypeName() + ':' + e.getMessage();
	System.assert(e.getLineNumber() > 0, 'caught exceptions should carry a line number');
	String stackTrace = e.getStackTraceString();
	System.assert(stackTrace != '', 'caught exceptions should carry a stack trace');
	System.assertEquals('System.QueryException: bad query', e.toString());
}
System.assertEquals('QueryException:bad query', caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecTypeIsAssignableFrom(t *testing.T) {
	program, err := CompileAnonymous(`
Type exceptionType = Type.forName('Exception');
Type dmlType = Type.forName('DmlException');
System.assert(exceptionType.isAssignableFrom(dmlType));
System.assert(!dmlType.isAssignableFrom(exceptionType));

Type markerType = Type.forName('Marker');
Type childType = Type.forName('Child');
Type parentType = Type.forName('Parent');
System.assert(markerType.isAssignableFrom(childType));
System.assert(parentType.isAssignableFrom(childType));
System.assert(!childType.isAssignableFrom(parentType));
System.assert(childType.isAssignableFrom(childType));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Marker", IsInterface: true}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Parent"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Child", SuperClass: "Parent", Interfaces: []string{"Marker"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
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

func TestExecTypeNewInstanceRunsZeroArgConstructor(t *testing.T) {
	constructorProgram, err := CompileAnonymous(`this.Name = 'built';`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Type thingType = Type.forName('Thing');
Thing thing = thingType.newInstance();
System.assertEquals('built', thing.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Thing",
		Fields: map[string]Field{
			"Name": {Name: "Name", Type: "String"},
		},
		FieldOrder: []string{"Name"},
		Constructors: []Method{
			{Name: "Thing.<init>", ClassName: "Thing", Program: constructorProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCoreIDValueOfRejectsInvalidAndRestoreCasing(t *testing.T) {
	tests := []string{
		`Id.valueOf('short');`,
		`Id.valueOf('001B000001DVM9!');`,
		`Id.valueOf('001B000001DVM9tIAA');`,
		`Id.valueOf('001B000001DVM9t999', true);`,
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

func TestExecIDGetSObjectType(t *testing.T) {
	program, err := CompileAnonymous(`
Id accountId = Id.valueOf('001B000001DVM9t');
Object accountType = accountId.getSObjectType();
Object accountDescribe = accountType.getDescribe();
System.assertEquals('Account', accountDescribe.getName());
Id contactId = Id.valueOf('003B000001DVM9tIAH');
Object contactType = contactId.getSObjectType();
Object contactDescribe = contactType.getDescribe();
System.assertEquals('Contact', contactDescribe.getName());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Contact",
			KeyPrefix: "003",
			Fields: map[string]storage.Field{
				"LastName": {APIName: "LastName", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecURLCurrentRequestUrlUnsupported(t *testing.T) {
	program, err := CompileAnonymous(`URL.getCurrentRequestUrl();`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Execute(program, nil)
	if err == nil {
		t.Fatal("expected URL.getCurrentRequestUrl to be unsupported")
	}
	if got := err.Error(); got != `unsupported call "URL.getCurrentRequestUrl"` {
		t.Fatalf("error = %q", got)
	}
}

func TestExecNumericStdlibExpansion(t *testing.T) {
	program, err := CompileAnonymous(`
Integer i = Integer.valueOf('42');
Long l = Long.valueOf('9001');
Decimal d = Decimal.valueOf('12.5');
Double x = Double.valueOf('2.25');
Decimal bigLong = Decimal.valueOf('3000000000');
System.assertEquals(42, i);
System.assertEquals(9001, l);
System.assertEquals(12.5, d);
System.assertEquals(2.25, x);
System.assertEquals('42', i.format());
System.assertEquals(42.0, i.doubleValue());
System.assertEquals(12, d.intValue());
System.assertEquals(12, d.longValue());
System.assertEquals(3000000000, bigLong.longValue());
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
System.assertEquals(3.0, Math.ceil(2.1));
System.assertEquals(2.0, Math.floor(2.9));
System.assertEquals(3.0, Math.round(2.5));
System.assertEquals(7, Math.max(3, 7));
System.assertEquals(3, Math.min(3, 7));
System.assertEquals(3.0, Math.sqrt(9));
System.assertEquals(8.0, Math.pow(2, 3));
System.assertEquals(2147483647, Integer.MAX_VALUE);
System.assertEquals(-2147483648, Integer.MIN_VALUE);
System.assert(Long.MAX_VALUE > 0);
System.assert(Long.MIN_VALUE < 0);
System.assert(Math.abs(Math.PI - 3.141592653589793) < 0.000000000000001);
System.assert(Math.abs(Math.E - 2.718281828459045) < 0.000000000000001);
System.assert(Math.abs(Math.sin(Math.PI / 2) - 1) < 0.000000000001);
System.assert(Math.abs(Math.cos(0) - 1) < 0.000000000001);
System.assert(Math.abs(Math.tan(0)) < 0.000000000001);
System.assert(Math.abs(Math.acos(1)) < 0.000000000001);
System.assert(Math.abs(Math.asin(1) - (Math.PI / 2)) < 0.000000000001);
System.assert(Math.abs(Math.atan(1) - (Math.PI / 4)) < 0.000000000001);
System.assert(Math.abs(Math.atan2(1, 1) - (Math.PI / 4)) < 0.000000000001);
System.assert(Math.abs(Math.exp(1) - Math.E) < 0.000000000001);
System.assert(Math.abs(Math.log(Math.E) - 1) < 0.000000000001);
System.assert(Math.abs(Math.log10(1000) - 3) < 0.000000000001);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecNumericStdlibRejectsInvalidInputs(t *testing.T) {
	tests := []string{
		"Integer.valueOf('not an integer');",
		"Integer.valueOf('2147483648');",
		"Long.valueOf('9223372036854775808');",
		"Decimal.valueOf('1e309');",
		"Double.valueOf('NaN');",
		"Decimal d = Decimal.valueOf('3000000000');\nd.intValue();",
		"Math.acos(2);",
		"Math.asin(-2);",
		"Math.log(0);",
		"Math.log10(-1);",
		"Math.exp(1000);",
	}
	for _, source := range tests {
		program, err := CompileAnonymous(source)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Execute(program, nil); err == nil {
			t.Fatalf("expected numeric stdlib error for %s", source)
		}
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

func TestExecCollectionStdlibMoreMethods(t *testing.T) {
	program, err := CompileAnonymous(`
List<Integer> source = new List<Integer>{3, 1, 2};
List<Integer> copied = new List<Integer>(source);
source.set(0, 9);
System.assertEquals(3, copied.size());
System.assertEquals(3, copied.get(0));
copied.sort();
System.assertEquals(1, copied.get(0));
System.assertEquals(3, copied.get(2));
List<Integer> cloned = copied.clone();
cloned.set(0, 7);
System.assertEquals(1, copied.get(0));
System.assertEquals(7, cloned.get(0));
List<Integer> deep = copied.deepClone();
deep.set(1, 8);
System.assertEquals(2, copied.get(1));
System.assertEquals(8, deep.get(1));

List<String> words = new List<String>{'delta', 'alpha', 'charlie'};
words.sort();
System.assertEquals('alpha', words.get(0));
System.assertEquals('delta', words.get(2));

Set<String> fromList = new Set<String>(new List<String>{'b', 'a', 'b'});
System.assertEquals(2, fromList.size());
Set<String> setClone = fromList.clone();
setClone.add('c');
System.assertEquals(2, fromList.size());
System.assertEquals(3, setClone.size());
Set<String> setDeep = fromList.deepClone();
setDeep.remove('a');
System.assert(fromList.contains('a'));
System.assert(!setDeep.contains('a'));

Map<String,Integer> counts = new Map<String,Integer>();
counts.put('b', 2);
counts.put('a', 1);
Map<String,Integer> copiedCounts = new Map<String,Integer>(counts);
System.assertEquals(counts, copiedCounts);
System.assertEquals('Map{a=1, b=2}', copiedCounts.toString());
List<Integer> orderedValues = copiedCounts.values();
System.assertEquals(1, orderedValues.get(0));
System.assertEquals(2, orderedValues.get(1));
Map<String,Integer> clonedCounts = copiedCounts.clone();
clonedCounts.put('a', 9);
System.assertEquals(1, copiedCounts.get('a'));
System.assertEquals(9, clonedCounts.get('a'));
Map<String,Integer> deepCounts = copiedCounts.deepClone();
deepCounts.put('b', 8);
System.assertEquals(2, copiedCounts.get('b'));
System.assertEquals(8, deepCounts.get('b'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCollectionStdlibMoreRejectsUnsupportedSortValues(t *testing.T) {
	values := []Value{Map()}
	err := sortComparableValues(values)
	if err == nil || !strings.Contains(err.Error(), "List.sort supports only primitive comparable values") {
		t.Fatalf("err = %v, want primitive comparable sort error", err)
	}
}
