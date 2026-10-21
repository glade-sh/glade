@IsTest private class Ns113910271390468R002 { @IsTest static void observed() {
String observation; try { Object r;
List<gladens001__Ns113bFlag__mdt> xs=[SELECT DeveloperName,NamespacePrefix,QualifiedApiName,gladens001__Enabled__c FROM gladens001__Ns113bFlag__mdt WHERE DeveloperName='ProviderSeed']; r=xs.size()==1 ? xs[0].DeveloperName+'|'+String.valueOf(xs[0].NamespacePrefix)+'|'+xs[0].QualifiedApiName+'|'+String.valueOf(xs[0].gladens001__Enabled__c) : 'COUNT|'+xs.size();
observation=String.valueOf(r);
} catch(Exception e) { observation='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
System.assert(false,'P|R002|'+observation); } }
