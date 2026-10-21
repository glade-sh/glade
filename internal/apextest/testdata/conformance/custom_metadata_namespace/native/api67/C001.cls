@IsTest private class Ns113910271390467C001 { @IsTest static void observed() {
String observation; try { Object r;
gladens001__Ns113bFlag__mdt x=new gladens001__Ns113bFlag__mdt(); x.NamespacePrefix='ns113wrong'; r=x.NamespacePrefix;
observation=String.valueOf(r);
} catch(Exception e) { observation='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
System.assert(false,'P|C001|'+observation); } }
