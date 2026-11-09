public class NativeNoSpace67 {
 public static Map<String,Object> current() {
  List<Account>recs = new List<Account>{new Account(Name='nospace one'), new Account(Name='nospace two')};
  Database.SaveResult[] r = Database.insert(recs,false);
  Map<Id,Account>byId = new Map<Id,Account>(recs);
  Database.SaveResult[] r2 = Database.update(byId.values(),false);
  Set<String>names = new Set<String>();
  for (Account a : recs) { names.add(a.Name); }
  Map<String,List<Account>>grouped = new Map<String,List<Account>>();
  grouped.put('all', recs);
  List<Account>fromMap = grouped.get('all');
  Account[]arr = new Account[]{recs[0]};
  Database.SaveResult[] r3 = Database.update(arr,false);
  List<List<Account>>nested = new List<List<Account>>{recs};
  Integer total = 0;
  Integer i = 0; List<Account>afterStatement = new List<Account>(); afterStatement.addAll(recs);
  return new Map<String,Object>{
   'insert' => r.size(), 'ok' => r[0].isSuccess(), 'byId' => byId.size(), 'update' => r2.size(),
   'names' => names.size(), 'fromMap' => fromMap.size(), 'arr' => arr.size(), 'arrUpdate' => r3.size(),
   'nested' => nested[0].size(), 'after' => afterStatement.size(),
   'param' => countParam(recs, grouped)
  };
 }
 static Integer countParam(List<Account>p, Map<String,List<Account>>q) {
  Database.SaveResult[] r = Database.update(p,false);
  Set<String>s = new Set<String>{'x'};
  return r.size() + q.get('all').size() + s.size();
 }
}
