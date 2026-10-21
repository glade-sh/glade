@IsTest private class Ns113910271390467R010 { @IsTest static void observed() {
String observation; try { Object r;
r=[SELECT COUNT() FROM gladens001__Ns113bFlag__mdt WHERE DeveloperName='ProviderSeed' AND QualifiedApiName='gladens001__ProviderSeed'];
observation=String.valueOf(r);
} catch(Exception e) { observation='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
System.assert(false,'P|R010|'+observation); } }
