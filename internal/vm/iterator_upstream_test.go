package vm

import "testing"

func TestIteratorSnapshotAndCachedCollectionControls(t *testing.T) {
	program, err := CompileAnonymous(`
List<String> values = new List<String>{'north', 'south'};
Map<String, List<String>> cache = new Map<String, List<String>>();
cache.put('route', values);
System.assertEquals('north', cache.get('route').iterator().next());
System.assertEquals('north', cache.get('route').iterator().next(), 'a fresh iterator starts at zero');
Iterator<String> cursor = cache.get('route').iterator();
System.assert(cursor.hasNext());
System.assert(cursor.hasNext(), 'hasNext does not advance');
System.assertEquals('north', cursor.next());
System.assertEquals('south', cursor.next());
System.assert(!cursor.hasNext());
Boolean exhausted = false;
try { cursor.next(); } catch (NoSuchElementException e) { exhausted = true; }
System.assert(exhausted);
List<String> emptyValues = new List<String>();
System.assert(!emptyValues.iterator().hasNext());
Boolean emptyThrows = false;
try { emptyValues.iterator().next(); } catch (NoSuchElementException e) { emptyThrows = true; }
System.assert(emptyThrows, 'an empty source must not fabricate an element');
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
