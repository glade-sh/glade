@IsTest private class CRT6773696491f316V008 {
private static Object pass(Iterable<Object> values) { return values; }
@IsTest static void V008() {
String corpusObserved;
try {
Object r;

Object result = pass(new Set<Integer>{1});
Set<String> narrowed = (Set<String>)result;
r = narrowed.size();

corpusObserved = String.valueOf(r);
} catch (Exception e) {
corpusObserved = 'EXC|' + e.getTypeName() + '|' + e.getMessage();
}
System.assert(false, 'P|V008|' + corpusObserved);
}
}
