@IsTest private class Ns113910271390467R008 { @IsTest static void observed() {
String observation; try { Object r;
r=[SELECT COUNT() FROM gladens001__Ns113bFlag__mdt WHERE DeveloperName='LocalSeed' AND QualifiedApiName='LocalSeed'];
observation=String.valueOf(r);
} catch(Exception e) { observation='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
System.assert(false,'P|R008|'+observation); } }
