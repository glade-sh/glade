@IsTest private class CRT67bd64fb5daa6fR008 {

private static String pageName(String address) {
    return address.replaceFirst('(?i)^/apex/(?:[A-Za-z][A-Za-z0-9_]*__)?', '');
}
private static List<SObject> lookup(Set<String> selectedNames) {
    return Database.query('SELECT Id, Name FROM ApexPage WHERE Name IN :selectedNames');
}

@IsTest static void R008() {
String corpusObserved;
try {
Object r;
r=pageName('/APEX/probe__ResPage67_f48cd82f84b648a89f0148cd');
corpusObserved = String.valueOf(r);
} catch (Exception e) {
corpusObserved = 'EXC|' + e.getTypeName() + '|' + e.getMessage();
}
System.assert(false, 'P|R008|' + corpusObserved);
}
}
