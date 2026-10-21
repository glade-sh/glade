class P { public List<String> rows = new List<String>();
  public void out(String id, Object v) { rows.add('P|' + id + '|' + String.valueOf(v)); }
  public void err(String id, Exception e) { rows.add('P|' + id + '|EXC|' + e.getTypeName() + '|' + e.getMessage()); }
}
P pq = new P();
try { Object r; r=[SELECT COUNT() FROM gladens001__Ns113bFlag__mdt WHERE DeveloperName='LocalSeed' AND NamespacePrefix='gladens001']; pq.out('R004', r); } catch (Exception e) { pq.err('R004', e); }
class FamilyProbeTransportException extends Exception {}
throw new FamilyProbeTransportException('GLADE_FAMILY_ROWS|' + JSON.serialize(pq.rows));
