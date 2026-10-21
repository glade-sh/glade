class P { public List<String> rows = new List<String>();
  public void out(String id, Object v) { rows.add('P|' + id + '|' + String.valueOf(v)); }
  public void err(String id, Exception e) { rows.add('P|' + id + '|EXC|' + e.getTypeName() + '|' + e.getMessage()); }
}
P pq = new P();
Object r;
Metadata.CustomMetadata x=new Metadata.CustomMetadata(); x.namespacePrefix='ns113wrong'; r=x.namespacePrefix;
class FamilyProbeTransportException extends Exception {}
throw new FamilyProbeTransportException('GLADE_FAMILY_ROWS|' + JSON.serialize(pq.rows));
