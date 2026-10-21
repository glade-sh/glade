class P { public List<String> rows = new List<String>();
  public void out(String id, Object v) { rows.add('P|' + id + '|' + String.valueOf(v)); }
  public void err(String id, Exception e) { rows.add('P|' + id + '|EXC|' + e.getTypeName() + '|' + e.getMessage()); }
}
P pq = new P();
try { Object r; gladens001__Ns113bFlag__mdt x=gladens001__Ns113bFlag__mdt.getInstance('Missing'); r=x==null ? 'MISSING' : x.DeveloperName+'|'+String.valueOf(x.NamespacePrefix)+'|'+x.QualifiedApiName+'|'+String.valueOf(x.gladens001__Enabled__c); pq.out('R021', r); } catch (Exception e) { pq.err('R021', e); }
class FamilyProbeTransportException extends Exception {}
throw new FamilyProbeTransportException('GLADE_FAMILY_ROWS|' + JSON.serialize(pq.rows));
