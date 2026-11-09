List<String> out = new List<String>();
for (String k : new List<String>{'D1','D2','D3','D4','D5','D6','D7','D8','D9','L1','L2','V1','V2','C1'}) {
  for (String t : new List<String>{'Customer','Other'}) {
    try { out.add('K:' + k + '|' + t + '|' + RelProbe.run(k, t, t)); } catch (Exception e) { out.add('K:' + k + '|' + t + '|EXC|' + e.getTypeName() + ':' + e.getMessage()); }
  }
  Database.rollback(Database.setSavepoint());
}
System.assert(false, 'ROWS\n' + String.join(out, '\n'));
