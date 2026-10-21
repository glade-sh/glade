@IsTest private class Ns113910271390462R023 { @IsTest static void observed() {
String observation; try { Object r;
List<gladens001__Ns113bFlag__mdt> xs=[SELECT DeveloperName,NamespacePrefix,QualifiedApiName FROM gladens001__Ns113bFlag__mdt ORDER BY DeveloperName]; List<String> parts=new List<String>(); for(gladens001__Ns113bFlag__mdt x:xs){parts.add(x.DeveloperName+'|'+String.valueOf(x.NamespacePrefix)+'|'+x.QualifiedApiName);} r=String.join(parts,';');
observation=String.valueOf(r);
} catch(Exception e) { observation='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
System.assert(false,'P|R023|'+observation); } }
