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
