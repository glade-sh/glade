@IsTest private class Ns113910271390462R019 { @IsTest static void observed() {
String observation; try { Object r;
gladens001__Ns113bFlag__mdt x=gladens001__Ns113bFlag__mdt.getInstance('ProviderSeed'); r=x==null ? 'MISSING' : x.DeveloperName+'|'+String.valueOf(x.NamespacePrefix)+'|'+x.QualifiedApiName+'|'+String.valueOf(x.gladens001__Enabled__c);
observation=String.valueOf(r);
} catch(Exception e) { observation='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
System.assert(false,'P|R019|'+observation); } }
