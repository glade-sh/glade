Map<String, Schema.SObjectField> pf = Schema.SObjectType.Profile.fields.getMap();
List<String> names = new List<String>();
for (String n : pf.keySet()) { if (n.startsWith('permissions')) names.add(pf.get(n).getDescribe().getName()); }
names.sort();
Id pid = [SELECT ProfileId FROM User WHERE Id = :UserInfo.getUserId()].ProfileId;
Profile p = Database.query('SELECT Name, ' + String.join(names, ',') + ' FROM Profile WHERE Id = :pid');
List<String> trues = new List<String>(); Integer nullCount = 0;
for (String n : names) { Object v = p.get(n); if (v == null) nullCount++; else if ((Boolean) v) trues.add(n); }
System.debug('R5|profile|name=' + p.Name + '|count=' + names.size() + '|nullCount=' + nullCount + '|trues=' + String.join(trues, ','));
Map<String, Schema.SObjectField> sf = Schema.SObjectType.PermissionSet.fields.getMap();
List<String> snames = new List<String>();
for (String n : sf.keySet()) { if (n.startsWith('permissions')) snames.add(sf.get(n).getDescribe().getName()); }
snames.sort();
PermissionSet ps = Database.query('SELECT Name, IsOwnedByProfile, ' + String.join(snames, ',') + ' FROM PermissionSet WHERE ProfileId = :pid LIMIT 1');
List<String> st = new List<String>(); Integer sn = 0;
for (String n : snames) { Object v = ps.get(n); if (v == null) sn++; else if ((Boolean) v) st.add(n); }
System.debug('R5|permset|owned=' + ps.IsOwnedByProfile + '|count=' + snames.size() + '|nullCount=' + sn + '|trues=' + String.join(st, ','));
