@IsTest private class CRT62b1a472da9e81V009 {
private static Object pass(Iterable<Object> values) { return values; }
@IsTest static void V009() {
String corpusObserved;
try {
Object r;

Object result = pass(new List<Object>{'Alpha'});
r = String.valueOf(result instanceof List<String>);

corpusObserved = String.valueOf(r);
} catch (Exception e) {
corpusObserved = 'EXC|' + e.getTypeName() + '|' + e.getMessage();
}
System.assert(false, 'P|V009|' + corpusObserved);
}
}
