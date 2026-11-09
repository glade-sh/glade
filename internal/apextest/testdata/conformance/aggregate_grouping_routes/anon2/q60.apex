Savepoint sp=Database.setSavepoint();
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT BillingCity,COUNT(Id) v FROM Account WHERE Id IN :sids'; List<String> s=new List<String>(); String out;
  try { for(List<AggregateResult> ch:Database.query(q)) for(AggregateResult r:ch) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  Database.rollback(sp);
  System.debug('CAPTURE|R212@forChunkAgg|'+out);
