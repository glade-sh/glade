class P { public List<String> rows = new List<String>();
  public void out(String id, Object v) { rows.add('P|' + id + '|' + String.valueOf(v)); }
  public void err(String id, Exception e) { rows.add('P|' + id + '|EXC|' + e.getTypeName() + '|' + e.getMessage()); }
}
P pq = new P();
try { Object r; Map<String,Schema.SObjectType> xs=Schema.getGlobalDescribe(); r=String.valueOf(xs.containsKey('gladens001__Ns113bFlag__mdt'))+'|'+String.valueOf(xs.containsKey('Ns113bFlag__mdt')); pq.out('R026', r); } catch (Exception e) { pq.err('R026', e); }
class FamilyProbeTransportException extends Exception {}
throw new FamilyProbeTransportException('GLADE_FAMILY_ROWS|' + JSON.serialize(pq.rows));
