@IsTest private class CRT6773696491f316V005 {
private static Object pass(Iterable<Object> values) { return values; }
@IsTest static void V005() {
String corpusObserved;
try {
Object r;

Object result = pass(new Set<String>{'Alpha'});
r = String.valueOf(result instanceof Set<String>) + '|'
    + String.valueOf(result instanceof Set<Integer>);

corpusObserved = String.valueOf(r);
} catch (Exception e) {
corpusObserved = 'EXC|' + e.getTypeName() + '|' + e.getMessage();
}
System.assert(false, 'P|V005|' + corpusObserved);
}
}
