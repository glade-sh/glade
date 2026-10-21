@IsTest private class Ns113910271390462R015 { @IsTest static void observed() {
String observation; try { Object r;
Ns113bLocal__mdt x=[SELECT DeveloperName,NamespacePrefix,QualifiedApiName,Enabled__c FROM Ns113bLocal__mdt WHERE DeveloperName='LocalSeed'][0]; r=x.DeveloperName+'|'+String.valueOf(x.NamespacePrefix)+'|'+x.QualifiedApiName+'|'+String.valueOf(x.Enabled__c);
observation=String.valueOf(r);
} catch(Exception e) { observation='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
System.assert(false,'P|R015|'+observation); } }
