@IsTest private class Ns113910271390468R003 { @IsTest static void observed() {
String observation; try { Object r;
r=[SELECT COUNT() FROM gladens001__Ns113bFlag__mdt WHERE DeveloperName='LocalSeed' AND NamespacePrefix=null];
observation=String.valueOf(r);
} catch(Exception e) { observation='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
System.assert(false,'P|R003|'+observation); } }
