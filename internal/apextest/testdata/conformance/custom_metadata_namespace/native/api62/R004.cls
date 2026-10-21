@IsTest private class Ns113910271390462R004 { @IsTest static void observed() {
String observation; try { Object r;
r=[SELECT COUNT() FROM gladens001__Ns113bFlag__mdt WHERE DeveloperName='LocalSeed' AND NamespacePrefix='gladens001'];
observation=String.valueOf(r);
} catch(Exception e) { observation='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
System.assert(false,'P|R004|'+observation); } }
