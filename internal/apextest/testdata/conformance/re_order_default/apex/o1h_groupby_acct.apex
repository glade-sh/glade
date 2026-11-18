List<Contact> oc=[SELECT Id FROM Contact WHERE LastName LIKE 'GLC%']; if(!oc.isEmpty()){delete oc;}
List<Account> oa=[SELECT Id FROM Account WHERE Name LIKE 'GLA%']; if(!oa.isEmpty()){delete oa;}
List<String> IND=new List<String>{'Agriculture','Banking','Chemicals','Consulting','Education'};
List<Account> rs=new List<Account>();
for(Integer i=0;i<959;i++){rs.add(new Account(Name='GLA'+('0000'+String.valueOf(i)).right(4),Industry=IND[Math.mod(i,5)],AnnualRevenue=Math.mod(i*37,1000)));}
insert rs;
List<AggregateResult> a1=[SELECT Industry ind, COUNT(Id) cnt FROM Account WHERE Name LIKE 'GLA%' GROUP BY Industry];
List<String> o1=new List<String>(); for(AggregateResult r:a1){o1.add(String.valueOf(r.get('ind'))+':'+String.valueOf(r.get('cnt')));}
System.debug('O1|HGA|A1|'+String.join(o1,','));
List<AggregateResult> a2=[SELECT Industry ind, SUM(AnnualRevenue) sm FROM Account WHERE Name LIKE 'GLA%' GROUP BY Industry];
List<String> o2=new List<String>(); for(AggregateResult r:a2){o2.add(String.valueOf(r.get('ind'))+':'+String.valueOf(r.get('sm')));}
System.debug('O1|HGA|A2|'+String.join(o2,','));
List<AggregateResult> a3=[SELECT Industry ind, COUNT(Id) cnt FROM Account WHERE Name LIKE 'GLA%' GROUP BY Industry ORDER BY Industry];
List<String> o3=new List<String>(); for(AggregateResult r:a3){o3.add(String.valueOf(r.get('ind')));}
System.debug('O1|HGA|A3|'+String.join(o3,','));
System.debug('O1|HGA|ROWS|959|GROUPS|5');
