@IsTest private class Ns113910271390462R024 { @IsTest static void observed() {
String observation; try { Object r;
r=gladens001__Ns113bFlag__mdt.SObjectType.getDescribe().getName();
observation=String.valueOf(r);
} catch(Exception e) { observation='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
System.assert(false,'P|R024|'+observation); } }
