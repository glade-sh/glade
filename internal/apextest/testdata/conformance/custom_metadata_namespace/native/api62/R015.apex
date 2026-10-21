class P { public List<String> rows = new List<String>();
  public void out(String id, Object v) { rows.add('P|' + id + '|' + String.valueOf(v)); }
  public void err(String id, Exception e) { rows.add('P|' + id + '|EXC|' + e.getTypeName() + '|' + e.getMessage()); }
}
P pq = new P();
try { Object r; Ns113bLocal__mdt x=[SELECT DeveloperName,NamespacePrefix,QualifiedApiName,Enabled__c FROM Ns113bLocal__mdt WHERE DeveloperName='LocalSeed'][0]; r=x.DeveloperName+'|'+String.valueOf(x.NamespacePrefix)+'|'+x.QualifiedApiName+'|'+String.valueOf(x.Enabled__c); pq.out('R015', r); } catch (Exception e) { pq.err('R015', e); }
class FamilyProbeTransportException extends Exception {}
throw new FamilyProbeTransportException('GLADE_FAMILY_ROWS|' + JSON.serialize(pq.rows));
