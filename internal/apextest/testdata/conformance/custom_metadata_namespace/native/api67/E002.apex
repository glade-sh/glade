class P {
  public void out(String id, Object v) { System.debug(LoggingLevel.ERROR, 'P64|' + id + '|' + EncodingUtil.base64Encode(Blob.valueOf('' + String.valueOf(v)))); }
  public void err(String id, Exception e) { System.debug(LoggingLevel.ERROR, 'P64|' + id + '|' + EncodingUtil.base64Encode(Blob.valueOf('EXC|' + e.getTypeName() + '|' + e.getMessage()))); }
}
P pq = new P();
try { Object r; Metadata.CustomMetadata x=new Metadata.CustomMetadata(); x.fullName='ns113wrong__Ns113bAbsent.Apex67'; x.label='NS113B Apex 67'; Metadata.CustomMetadataValue v=new Metadata.CustomMetadataValue(); v.field='gladens001__Enabled__c'; v.value=true; x.values.add(v); Metadata.DeployContainer d=new Metadata.DeployContainer(); d.addMetadata(x); r=Metadata.Operations.enqueueDeployment(d,null); pq.out('E002', r); } catch (Exception e) { pq.err('E002', e); }