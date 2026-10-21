@IsTest private class Ns113910271390468R022 { @IsTest static void observed() {
String observation; try { Object r;
Map<String,gladens001__Ns113bFlag__mdt> xs=gladens001__Ns113bFlag__mdt.getAll(); List<String> keys=new List<String>(xs.keySet()); keys.sort(); List<String> parts=new List<String>(); for(String k:keys){gladens001__Ns113bFlag__mdt x=xs.get(k); parts.add(k+':'+x.DeveloperName+'|'+String.valueOf(x.NamespacePrefix)+'|'+x.QualifiedApiName);} r=String.join(parts,';');
observation=String.valueOf(r);
} catch(Exception e) { observation='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
System.assert(false,'P|R022|'+observation); } }
