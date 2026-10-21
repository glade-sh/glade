class P { public List<String> rows = new List<String>();
  public void out(String id, Object v) { rows.add('P|' + id + '|' + String.valueOf(v)); }
  public void err(String id, Exception e) { rows.add('P|' + id + '|EXC|' + e.getTypeName() + '|' + e.getMessage()); }
}
P pq = new P();
try { Object r; List<gladens001__Ns113bFlag__mdt> xs=[SELECT DeveloperName,NamespacePrefix,QualifiedApiName,gladens001__Enabled__c FROM gladens001__Ns113bFlag__mdt WHERE DeveloperName='LocalSeed']; r=xs.size()==1 ? xs[0].DeveloperName+'|'+String.valueOf(xs[0].NamespacePrefix)+'|'+xs[0].QualifiedApiName+'|'+String.valueOf(xs[0].gladens001__Enabled__c) : 'COUNT|'+xs.size(); pq.out('R001', r); } catch (Exception e) { pq.err('R001', e); }
class FamilyProbeTransportException extends Exception {}
throw new FamilyProbeTransportException('GLADE_FAMILY_ROWS|' + JSON.serialize(pq.rows));
