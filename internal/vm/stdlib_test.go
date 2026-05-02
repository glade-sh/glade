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
System.assertEquals('cDef', s.substring(2));
System.assertEquals('cD', s.substring(2, 4));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecCollectionStdlibExpansion(t *testing.T) {
	program, err := CompileAnonymous(`
List<Integer> xs = new List<Integer>{1, 2};
System.assert(!xs.isEmpty());
xs.clear();
System.assert(xs.isEmpty());
System.assertEquals(0, xs.size());

Set<String> names = new Set<String>{'a'};
System.assert(!names.isEmpty());
names.clear();
System.assert(names.isEmpty());
System.assertEquals(0, names.size());

Map<String,Integer> counts = new Map<String,Integer>();
System.assert(counts.isEmpty());
counts.put('a', 1);
System.assert(!counts.isEmpty());
System.assertEquals(1, counts.remove('a'));
System.assertEquals(null, counts.remove('missing'));
System.assert(counts.isEmpty());
counts.put('b', 2);
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
