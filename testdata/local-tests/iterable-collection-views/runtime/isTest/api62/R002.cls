@IsTest private class CRT62ba981e9fe62cR002 {
class CollectionViewProbe {
 public Object pass(Iterable<Object> values) { return values; }
}
@IsTest static void R002() {
String corpusObserved;
try {
Object r;
Object value=new CollectionViewProbe().pass(new List<String>{'Alpha'}); r=String.valueOf(value instanceof List<String>);
corpusObserved = String.valueOf(r);
} catch (Exception e) {
corpusObserved = 'EXC|' + e.getTypeName() + '|' + e.getMessage();
}
System.assert(false, 'P|R002|' + corpusObserved);
}
}
