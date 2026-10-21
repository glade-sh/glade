@IsTest private class Ns113910271390462R012 { @IsTest static void observed() {
String observation; try { Object r;
r=[SELECT gladens001__Enabled__c FROM gladens001__Ns113bFlag__mdt WHERE DeveloperName='LocalOff'][0].gladens001__Enabled__c;
observation=String.valueOf(r);
} catch(Exception e) { observation='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
System.assert(false,'P|R012|'+observation); } }
