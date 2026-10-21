@IsTest private class Ns113910271390462C003 { @IsTest static void observed() {
String observation; try { Object r;
Metadata.CustomMetadata x=new Metadata.CustomMetadata(); x.namespacePrefix='ns113wrong'; r=x.namespacePrefix;
observation=String.valueOf(r);
} catch(Exception e) { observation='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
System.assert(false,'P|C003|'+observation); } }
