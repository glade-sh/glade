@IsTest private class CRT6773696491f316V002 {
private static Object pass(Iterable<Object> values) { return values; }
@IsTest static void V002() {
String corpusObserved;
try {
Object r;

List<String> original = new List<String>{'Alpha'};
Iterable<Object> view = original;
Object boxed = view;
Iterable<Object> emptyView = new List<String>();
Object emptyBoxed = emptyView;
r = String.valueOf(boxed instanceof List<String>) + '|'
    + String.valueOf(boxed instanceof List<Integer>) + '|'
    + String.valueOf(emptyBoxed instanceof List<String>);

corpusObserved = String.valueOf(r);
} catch (Exception e) {
corpusObserved = 'EXC|' + e.getTypeName() + '|' + e.getMessage();
}
System.assert(false, 'P|V002|' + corpusObserved);
}
}
