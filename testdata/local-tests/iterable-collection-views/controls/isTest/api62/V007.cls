@IsTest private class CRT62b1a472da9e81V007 {
private static Object pass(Iterable<Object> values) { return values; }
@IsTest static void V007() {
String corpusObserved;
try {
Object r;

r = String.valueOf(pass(new Set<String>()) instanceof Set<String>) + '|'
    + String.valueOf(pass(new List<String>()) instanceof List<String>);

corpusObserved = String.valueOf(r);
} catch (Exception e) {
corpusObserved = 'EXC|' + e.getTypeName() + '|' + e.getMessage();
}
System.assert(false, 'P|V007|' + corpusObserved);
}
}
