@IsTest private class CRT62ba981e9fe62cR003 {
class CollectionViewProbe {
 public Object pass(Iterable<Object> values) { return values; }
}
@IsTest static void R003() {
String corpusObserved;
try {
Object r;
Account row=new Account(Name='Alpha'); insert row; Object wanted=new CollectionViewProbe().pass(new Set<String>{'Alpha'}); Map<String,Object> bindings=new Map<String,Object>{'wanted'=>wanted}; r=Database.queryWithBinds('SELECT Id FROM Account WHERE Name IN :wanted',bindings,AccessLevel.USER_MODE).size();
corpusObserved = String.valueOf(r);
} catch (Exception e) {
corpusObserved = 'EXC|' + e.getTypeName() + '|' + e.getMessage();
}
System.assert(false, 'P|R003|' + corpusObserved);
}
}
