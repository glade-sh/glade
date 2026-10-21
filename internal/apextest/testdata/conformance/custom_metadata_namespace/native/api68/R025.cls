@IsTest private class Ns113910271390468R025 { @IsTest static void observed() {
String observation; try { Object r;
r=gladens001__Ns113bFlag__mdt.gladens001__Enabled__c.getDescribe().getName();
observation=String.valueOf(r);
} catch(Exception e) { observation='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
System.assert(false,'P|R025|'+observation); } }
