@IsTest private class Ns113910271390468C002 { @IsTest static void observed() {
String observation; try { Object r;
gladens001__Ns113bFlag__mdt x=new gladens001__Ns113bFlag__mdt(); x.QualifiedApiName='ns113wrong__LocalSeed'; r=x.QualifiedApiName;
observation=String.valueOf(r);
} catch(Exception e) { observation='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
System.assert(false,'P|C002|'+observation); } }
