@IsTest private class CRT6773696491f316V006 {
private static Object pass(Iterable<Object> values) { return values; }
@IsTest static void V006() {
String corpusObserved;
try {
Object r;

Object result = pass(new List<String>{'Alpha'});
r = String.valueOf(result instanceof List<String>) + '|'
    + String.valueOf(result instanceof List<Integer>);

corpusObserved = String.valueOf(r);
} catch (Exception e) {
corpusObserved = 'EXC|' + e.getTypeName() + '|' + e.getMessage();
}
System.assert(false, 'P|V006|' + corpusObserved);
}
}
