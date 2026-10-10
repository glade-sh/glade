@IsTest private class CRT62b1a472da9e81V004 {
private static Object pass(Iterable<Object> values) { return values; }
@IsTest static void V004() {
String corpusObserved;
try {
Object r;

Iterable<Object> view = new Set<String>{'Alpha'};
Object boxed = view;
Set<String> narrowed = (Set<String>)boxed;
r = narrowed.size();

corpusObserved = String.valueOf(r);
} catch (Exception e) {
corpusObserved = 'EXC|' + e.getTypeName() + '|' + e.getMessage();
}
System.assert(false, 'P|V004|' + corpusObserved);
}
}
