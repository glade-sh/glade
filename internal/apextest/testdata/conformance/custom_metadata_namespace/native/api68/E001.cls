@IsTest private class Ns113b910271390468E001 { @IsTest static void observed() {
String observation; try { Object r;
Metadata.CustomMetadata x=new Metadata.CustomMetadata(); x.fullName='gladens001__Ns113bFlag.Apex68'; x.label='NS113B Apex 68'; Metadata.CustomMetadataValue v=new Metadata.CustomMetadataValue(); v.field='gladens001__Enabled__c'; v.value=true; x.values.add(v); Metadata.DeployContainer d=new Metadata.DeployContainer(); d.addMetadata(x); r=Metadata.Operations.enqueueDeployment(d,null);
observation=String.valueOf(r);
} catch(Exception e) { observation='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
System.assert(false,'P|E001|'+observation); } }
