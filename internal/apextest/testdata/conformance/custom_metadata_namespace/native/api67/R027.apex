class P { public List<String> rows = new List<String>();
  public void out(String id, Object v) { rows.add('P|' + id + '|' + String.valueOf(v)); }
  public void err(String id, Exception e) { rows.add('P|' + id + '|EXC|' + e.getTypeName() + '|' + e.getMessage()); }
}
P pq = new P();
try { Object r; Metadata.CustomMetadata x=new Metadata.CustomMetadata(); x.fullName='gladens001__Ns113bFlag.DtoOnly'; x.label='NS113B DTO'; Metadata.CustomMetadataValue v=new Metadata.CustomMetadataValue(); v.field='gladens001__Enabled__c'; v.value=true; x.values.add(v); r=x.fullName+'|'+x.values[0].field+'|'+String.valueOf(x.values[0].value); pq.out('R027', r); } catch (Exception e) { pq.err('R027', e); }
class FamilyProbeTransportException extends Exception {}
throw new FamilyProbeTransportException('GLADE_FAMILY_ROWS|' + JSON.serialize(pq.rows));
