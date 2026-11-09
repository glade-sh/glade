List<String> out = new List<String>();
for (String k : new List<String>{'EM1','EM2','EM3'}) {
  for (String t : new List<String>{'Customer','Other'}) {
    try { out.add('K:' + k + '|' + t + '|' + RelProbe.run(k, t, t)); } catch (Exception e) { out.add('K:' + k + '|' + t + '|EXC|' + e.getTypeName() + ':' + e.getMessage()); }
  }
  Database.rollback(Database.setSavepoint());
}
System.assert(false, 'ROWS\n' + String.join(out, '\n'));
