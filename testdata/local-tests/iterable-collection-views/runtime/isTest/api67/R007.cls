@IsTest private class CRT6774f840b34fb1R007 {
class CollectionViewProbe {
 public Object pass(Iterable<Object> values) { return values; }
}
@IsTest static void R007() {
String corpusObserved;
try {
Object r;
Object value=new CollectionViewProbe().pass(new Set<Integer>{1}); Set<String> narrowed=(Set<String>)value; r=narrowed.size();
corpusObserved = String.valueOf(r);
} catch (Exception e) {
corpusObserved = 'EXC|' + e.getTypeName() + '|' + e.getMessage();
}
System.assert(false, 'P|R007|' + corpusObserved);
}
}
