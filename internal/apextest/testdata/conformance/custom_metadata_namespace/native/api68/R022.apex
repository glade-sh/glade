class P { public List<String> rows = new List<String>();
  public void out(String id, Object v) { rows.add('P|' + id + '|' + String.valueOf(v)); }
  public void err(String id, Exception e) { rows.add('P|' + id + '|EXC|' + e.getTypeName() + '|' + e.getMessage()); }
}
P pq = new P();
try { Object r; Map<String,gladens001__Ns113bFlag__mdt> xs=gladens001__Ns113bFlag__mdt.getAll(); List<String> keys=new List<String>(xs.keySet()); keys.sort(); List<String> parts=new List<String>(); for(String k:keys){gladens001__Ns113bFlag__mdt x=xs.get(k); parts.add(k+':'+x.DeveloperName+'|'+String.valueOf(x.NamespacePrefix)+'|'+x.QualifiedApiName);} r=String.join(parts,';'); pq.out('R022', r); } catch (Exception e) { pq.err('R022', e); }
class FamilyProbeTransportException extends Exception {}
throw new FamilyProbeTransportException('GLADE_FAMILY_ROWS|' + JSON.serialize(pq.rows));
