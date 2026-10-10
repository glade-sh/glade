@IsTest private class CRT62ba981e9fe62cR009 {
class CollectionViewProbe {
 public Object pass(Iterable<Object> values) { return values; }
}
@IsTest static void R009() {
String corpusObserved;
try {
Object r;
Object value=new CollectionViewProbe().pass(new List<String>()); r=String.valueOf(value instanceof List<String>);
corpusObserved = String.valueOf(r);
} catch (Exception e) {
corpusObserved = 'EXC|' + e.getTypeName() + '|' + e.getMessage();
}
System.assert(false, 'P|R009|' + corpusObserved);
}
}
