public class GroupProbe62 {
 public static String run() {
  List<List<String>> sets = new List<List<String>>{
   new List<String>{'abc','ABC'}, new List<String>{'ABC','abc'},
   new List<String>{'aBc','ABC','abc'}, new List<String>{'abc','aBc','ABC'},
   new List<String>{'éclair','ÉCLAIR','eclair','ECLAIR'},
   new List<String>{'ECLAIR','eclair','ÉCLAIR','éclair'},
   new List<String>{'mix','MIX','Mix','mIx'}
  };
  List<String> out = new List<String>{'locale='+UserInfo.getLocale()};
  for (Integer k=0; k<sets.size(); k++) {
   List<Account> rows = new List<Account>();
   for (String s : sets[k]) rows.add(new Account(Name=s));
   insert rows;
   Set<Id> ids = new Map<Id,Account>(rows).keySet();
   List<AggregateResult> plain = [SELECT Name FROM Account WHERE Id IN :ids GROUP BY Name];
   List<AggregateResult> counted = [SELECT Name, COUNT(Id) n FROM Account WHERE Id IN :ids GROUP BY Name];
   List<String> a=new List<String>(); List<String> b=new List<String>();
   for (AggregateResult r:plain) a.add((String)r.get('Name'));
   for (AggregateResult r:counted) b.add((String)r.get('Name')+':'+String.valueOf(r.get('n')));
   a.sort(); b.sort(); out.add(k+'='+String.join(a,',')+'/'+String.join(b,','));
   if (k==0 || k==1) {
    List<Account> ordered=[SELECT Id,Name FROM Account WHERE Id IN :ids ORDER BY Id ASC];
    out.add('id'+k+'='+ordered[0].Name+','+ordered[1].Name);
    String temp=ordered[0].Name; ordered[0].Name=ordered[1].Name; ordered[1].Name=temp; update ordered;
    List<AggregateResult> swapped=[SELECT Name,COUNT(Id) n FROM Account WHERE Id IN :ids GROUP BY Name];
    out.add('swap'+k+'='+String.valueOf(swapped[0].get('Name'))+':'+String.valueOf(swapped[0].get('n')));
   }
  }
  return String.join(out,'|');
 }
}