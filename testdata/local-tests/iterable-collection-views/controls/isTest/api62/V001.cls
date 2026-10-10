@IsTest private class CRT62b1a472da9e81V001 {
private static Object pass(Iterable<Object> values) { return values; }
@IsTest static void V001() {
String corpusObserved;
try {
Object r;

Set<String> original = new Set<String>{'Alpha'};
Iterable<Object> view = original;
Object boxed = view;
Iterable<Object> emptyView = new Set<String>();
Object emptyBoxed = emptyView;
r = String.valueOf(boxed instanceof Set<String>) + '|'
    + String.valueOf(boxed instanceof Set<Integer>) + '|'
    + String.valueOf(emptyBoxed instanceof Set<String>);

corpusObserved = String.valueOf(r);
} catch (Exception e) {
corpusObserved = 'EXC|' + e.getTypeName() + '|' + e.getMessage();
}
System.assert(false, 'P|V001|' + corpusObserved);
}
}
