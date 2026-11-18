List<Ord__c> oo=[SELECT Id FROM Ord__c]; if(!oo.isEmpty()){delete oo;}
List<Ord__c> rs=new List<Ord__c>();
for(Integer i=0;i<959;i++){rs.add(new Ord__c(Name='N'+('000'+String.valueOf(Math.mod(i,137))).right(3)));}
insert rs;
List<AggregateResult> a1=[SELECT Name nm, COUNT(Id) cnt FROM Ord__c GROUP BY Name];
List<String> o1=new List<String>(); for(AggregateResult r:a1){o1.add(String.valueOf(r.get('nm'))+':'+String.valueOf(r.get('cnt')));}
System.debug('O1|HGO|A1|'+String.join(o1,','));
List<AggregateResult> a2=[SELECT Name nm, COUNT(Id) cnt FROM Ord__c GROUP BY Name ORDER BY Name];
List<String> o2=new List<String>(); for(AggregateResult r:a2){o2.add(String.valueOf(r.get('nm')));}
System.debug('O1|HGO|A2|'+String.join(o2,','));
System.debug('O1|HGO|ROWS|959|NAMES|137');
