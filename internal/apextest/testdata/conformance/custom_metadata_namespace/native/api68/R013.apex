class P { public List<String> rows = new List<String>();
  public void out(String id, Object v) { rows.add('P|' + id + '|' + String.valueOf(v)); }
  public void err(String id, Exception e) { rows.add('P|' + id + '|EXC|' + e.getTypeName() + '|' + e.getMessage()); }
}
P pq = new P();
try { Object r; r=[SELECT gladens001__Enabled__c FROM gladens001__Ns113bFlag__mdt WHERE DeveloperName='LocalDefault'][0].gladens001__Enabled__c; pq.out('R013', r); } catch (Exception e) { pq.err('R013', e); }
class FamilyProbeTransportException extends Exception {}
throw new FamilyProbeTransportException('GLADE_FAMILY_ROWS|' + JSON.serialize(pq.rows));
