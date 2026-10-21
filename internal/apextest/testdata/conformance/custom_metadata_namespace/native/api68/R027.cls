@IsTest private class Ns113910271390468R027 { @IsTest static void observed() {
String observation; try { Object r;
Metadata.CustomMetadata x=new Metadata.CustomMetadata(); x.fullName='gladens001__Ns113bFlag.DtoOnly'; x.label='NS113B DTO'; Metadata.CustomMetadataValue v=new Metadata.CustomMetadataValue(); v.field='gladens001__Enabled__c'; v.value=true; x.values.add(v); r=x.fullName+'|'+x.values[0].field+'|'+String.valueOf(x.values[0].value);
observation=String.valueOf(r);
} catch(Exception e) { observation='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
System.assert(false,'P|R027|'+observation); } }
