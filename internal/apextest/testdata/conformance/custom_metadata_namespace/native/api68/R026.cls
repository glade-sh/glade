@IsTest private class Ns113910271390468R026 { @IsTest static void observed() {
String observation; try { Object r;
Map<String,Schema.SObjectType> xs=Schema.getGlobalDescribe(); r=String.valueOf(xs.containsKey('gladens001__Ns113bFlag__mdt'))+'|'+String.valueOf(xs.containsKey('Ns113bFlag__mdt'));
observation=String.valueOf(r);
} catch(Exception e) { observation='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
System.assert(false,'P|R026|'+observation); } }
