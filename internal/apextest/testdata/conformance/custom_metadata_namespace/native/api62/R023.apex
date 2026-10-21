class P { public List<String> rows = new List<String>();
  public void out(String id, Object v) { rows.add('P|' + id + '|' + String.valueOf(v)); }
  public void err(String id, Exception e) { rows.add('P|' + id + '|EXC|' + e.getTypeName() + '|' + e.getMessage()); }
}
P pq = new P();
try { Object r; List<gladens001__Ns113bFlag__mdt> xs=[SELECT DeveloperName,NamespacePrefix,QualifiedApiName FROM gladens001__Ns113bFlag__mdt ORDER BY DeveloperName]; List<String> parts=new List<String>(); for(gladens001__Ns113bFlag__mdt x:xs){parts.add(x.DeveloperName+'|'+String.valueOf(x.NamespacePrefix)+'|'+x.QualifiedApiName);} r=String.join(parts,';'); pq.out('R023', r); } catch (Exception e) { pq.err('R023', e); }
class FamilyProbeTransportException extends Exception {}
throw new FamilyProbeTransportException('GLADE_FAMILY_ROWS|' + JSON.serialize(pq.rows));
