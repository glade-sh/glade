@IsTest private class CRT62ba981e9fe62cR001 {
class CollectionViewProbe {
 public Object pass(Iterable<Object> values) { return values; }
}
@IsTest static void R001() {
String corpusObserved;
try {
Object r;
Object value=new CollectionViewProbe().pass(new Set<String>{'Alpha'}); r=String.valueOf(value instanceof Set<String>);
corpusObserved = String.valueOf(r);
} catch (Exception e) {
corpusObserved = 'EXC|' + e.getTypeName() + '|' + e.getMessage();
}
System.assert(false, 'P|R001|' + corpusObserved);
}
}
