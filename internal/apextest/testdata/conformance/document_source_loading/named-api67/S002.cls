@IsTest(SeeAllData=true) private class CRT67bd64fb5daa6fS002 {

private static String pageName(String address) {
    return address.replaceFirst('(?i)^/apex/(?:[A-Za-z][A-Za-z0-9_]*__)?', '');
}
private static List<SObject> lookup(Set<String> selectedNames) {
    return Database.query('SELECT Id, Name FROM ApexPage WHERE Name IN :selectedNames');
}

@IsTest static void S002() {
String corpusObserved;
try {
Object r;
Document d=[SELECT Name,DeveloperName,Type,BodyLength FROM Document WHERE DeveloperName='ResPic67_f48cd82f84b648a89f0148cd' LIMIT 1]; r=d.Name+'|'+d.DeveloperName+'|'+d.Type+'|'+String.valueOf(d.BodyLength>0);
corpusObserved = String.valueOf(r);
} catch (Exception e) {
corpusObserved = 'EXC|' + e.getTypeName() + '|' + e.getMessage();
}
System.assert(false, 'P|S002|' + corpusObserved);
}
}
