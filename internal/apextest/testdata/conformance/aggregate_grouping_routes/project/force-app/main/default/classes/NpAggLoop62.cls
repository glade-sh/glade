@IsTest private class NpAggLoop62 {
 // Pa-cf-g@listSObject
 @IsTest static void q00(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) c, Pick__c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { List<SObject> rows=Database.query(q); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-cf-g@listSObject|'+out);
 }
 // Pa-cf-g@listAgg
 @IsTest static void q01(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) c, Pick__c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { List<AggregateResult> rows=Database.query(q); for(AggregateResult r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-cf-g@listAgg|'+out);
 }
 // Pa-cf-g@forAgg
 @IsTest static void q02(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) c, Pick__c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { for(AggregateResult r:Database.query(q)) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-cf-g@forAgg|'+out);
 }
 // Pa-cf-g@forSObject
 @IsTest static void q03(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) c, Pick__c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { for(SObject r:Database.query(q)) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-cf-g@forSObject|'+out);
 }
 // Pa-cf-g@forChunkAgg
 @IsTest static void q04(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) c, Pick__c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { for(List<AggregateResult> ch:Database.query(q)) for(AggregateResult r:ch) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-cf-g@forChunkAgg|'+out);
 }
 // Pa-cf-g@forChunkSObject
 @IsTest static void q05(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) c, Pick__c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { for(List<SObject> ch:Database.query(q)) for(SObject r:ch) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-cf-g@forChunkSObject|'+out);
 }
 // Pa-cf-g@sizeExpr
 @IsTest static void q06(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) c, Pick__c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { Integer n=Database.query(q).size(); s.add('size='+n); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-cf-g@sizeExpr|'+out);
 }
 // Pa-fc-g@listSObject
 @IsTest static void q07(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT Pick__c, COUNT(Id) c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { List<SObject> rows=Database.query(q); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-fc-g@listSObject|'+out);
 }
 // Pa-fc-g@listAgg
 @IsTest static void q08(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT Pick__c, COUNT(Id) c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { List<AggregateResult> rows=Database.query(q); for(AggregateResult r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-fc-g@listAgg|'+out);
 }
 // Pa-fc-g@forAgg
 @IsTest static void q09(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT Pick__c, COUNT(Id) c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { for(AggregateResult r:Database.query(q)) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-fc-g@forAgg|'+out);
 }
 // Pa-fc-g@forSObject
 @IsTest static void q10(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT Pick__c, COUNT(Id) c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { for(SObject r:Database.query(q)) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-fc-g@forSObject|'+out);
 }
 // Pa-fc-g@forChunkAgg
 @IsTest static void q11(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT Pick__c, COUNT(Id) c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { for(List<AggregateResult> ch:Database.query(q)) for(AggregateResult r:ch) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-fc-g@forChunkAgg|'+out);
 }
 // Pa-fc-g@forChunkSObject
 @IsTest static void q12(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT Pick__c, COUNT(Id) c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { for(List<SObject> ch:Database.query(q)) for(SObject r:ch) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-fc-g@forChunkSObject|'+out);
 }
 // Pa-fc-g@sizeExpr
 @IsTest static void q13(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT Pick__c, COUNT(Id) c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { Integer n=Database.query(q).size(); s.add('size='+n); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-fc-g@sizeExpr|'+out);
 }
 // Ta-cf-g@listSObject
 @IsTest static void q14(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) c, Txt__c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { List<SObject> rows=Database.query(q); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Ta-cf-g@listSObject|'+out);
 }
 // Ta-cf-g@listAgg
 @IsTest static void q15(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) c, Txt__c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { List<AggregateResult> rows=Database.query(q); for(AggregateResult r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Ta-cf-g@listAgg|'+out);
 }
 // Ta-cf-g@forAgg
 @IsTest static void q16(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) c, Txt__c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { for(AggregateResult r:Database.query(q)) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Ta-cf-g@forAgg|'+out);
 }
 // Ta-cf-g@forSObject
 @IsTest static void q17(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) c, Txt__c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { for(SObject r:Database.query(q)) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Ta-cf-g@forSObject|'+out);
 }
 // Ta-cf-g@forChunkAgg
 @IsTest static void q18(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) c, Txt__c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { for(List<AggregateResult> ch:Database.query(q)) for(AggregateResult r:ch) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Ta-cf-g@forChunkAgg|'+out);
 }
 // Ta-cf-g@forChunkSObject
 @IsTest static void q19(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) c, Txt__c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { for(List<SObject> ch:Database.query(q)) for(SObject r:ch) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Ta-cf-g@forChunkSObject|'+out);
 }
 // Ta-cf-g@sizeExpr
 @IsTest static void q20(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) c, Txt__c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { Integer n=Database.query(q).size(); s.add('size='+n); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Ta-cf-g@sizeExpr|'+out);
 }
 // Ta-fc-g@listSObject
 @IsTest static void q21(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT Txt__c, COUNT(Id) c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { List<SObject> rows=Database.query(q); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Ta-fc-g@listSObject|'+out);
 }
 // Ta-fc-g@listAgg
 @IsTest static void q22(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT Txt__c, COUNT(Id) c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { List<AggregateResult> rows=Database.query(q); for(AggregateResult r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Ta-fc-g@listAgg|'+out);
 }
 // Ta-fc-g@forAgg
 @IsTest static void q23(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT Txt__c, COUNT(Id) c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { for(AggregateResult r:Database.query(q)) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Ta-fc-g@forAgg|'+out);
 }
 // Ta-fc-g@forSObject
 @IsTest static void q24(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT Txt__c, COUNT(Id) c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { for(SObject r:Database.query(q)) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Ta-fc-g@forSObject|'+out);
 }
 // Ta-fc-g@forChunkAgg
 @IsTest static void q25(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT Txt__c, COUNT(Id) c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { for(List<AggregateResult> ch:Database.query(q)) for(AggregateResult r:ch) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Ta-fc-g@forChunkAgg|'+out);
 }
 // Ta-fc-g@forChunkSObject
 @IsTest static void q26(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT Txt__c, COUNT(Id) c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { for(List<SObject> ch:Database.query(q)) for(SObject r:ch) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Ta-fc-g@forChunkSObject|'+out);
 }
 // Ta-fc-g@sizeExpr
 @IsTest static void q27(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT Txt__c, COUNT(Id) c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { Integer n=Database.query(q).size(); s.add('size='+n); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Ta-fc-g@sizeExpr|'+out);
 }
 // Pn-cf-g@listSObject
 @IsTest static void q28(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id), Pick__c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { List<SObject> rows=Database.query(q); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pn-cf-g@listSObject|'+out);
 }
 // Pn-cf-g@listAgg
 @IsTest static void q29(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id), Pick__c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { List<AggregateResult> rows=Database.query(q); for(AggregateResult r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pn-cf-g@listAgg|'+out);
 }
 // Pn-cf-g@forAgg
 @IsTest static void q30(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id), Pick__c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { for(AggregateResult r:Database.query(q)) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pn-cf-g@forAgg|'+out);
 }
 // Pn-cf-g@forSObject
 @IsTest static void q31(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id), Pick__c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { for(SObject r:Database.query(q)) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pn-cf-g@forSObject|'+out);
 }
 // Pn-cf-g@forChunkAgg
 @IsTest static void q32(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id), Pick__c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { for(List<AggregateResult> ch:Database.query(q)) for(AggregateResult r:ch) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pn-cf-g@forChunkAgg|'+out);
 }
 // Pn-cf-g@forChunkSObject
 @IsTest static void q33(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id), Pick__c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { for(List<SObject> ch:Database.query(q)) for(SObject r:ch) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pn-cf-g@forChunkSObject|'+out);
 }
 // Pn-cf-g@sizeExpr
 @IsTest static void q34(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id), Pick__c FROM NpAgg__c GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { Integer n=Database.query(q).size(); s.add('size='+n); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pn-cf-g@sizeExpr|'+out);
 }
 // Pa-cf-n@listSObject
 @IsTest static void q35(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) c, Pick__c FROM NpAgg__c'; List<String> s=new List<String>(); String out;
  try { List<SObject> rows=Database.query(q); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-cf-n@listSObject|'+out);
 }
 // Pa-cf-n@listAgg
 @IsTest static void q36(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) c, Pick__c FROM NpAgg__c'; List<String> s=new List<String>(); String out;
  try { List<AggregateResult> rows=Database.query(q); for(AggregateResult r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-cf-n@listAgg|'+out);
 }
 // Pa-cf-n@forAgg
 @IsTest static void q37(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) c, Pick__c FROM NpAgg__c'; List<String> s=new List<String>(); String out;
  try { for(AggregateResult r:Database.query(q)) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-cf-n@forAgg|'+out);
 }
 // Pa-cf-n@forSObject
 @IsTest static void q38(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) c, Pick__c FROM NpAgg__c'; List<String> s=new List<String>(); String out;
  try { for(SObject r:Database.query(q)) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-cf-n@forSObject|'+out);
 }
 // Pa-cf-n@forChunkAgg
 @IsTest static void q39(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) c, Pick__c FROM NpAgg__c'; List<String> s=new List<String>(); String out;
  try { for(List<AggregateResult> ch:Database.query(q)) for(AggregateResult r:ch) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-cf-n@forChunkAgg|'+out);
 }
 // Pa-cf-n@forChunkSObject
 @IsTest static void q40(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) c, Pick__c FROM NpAgg__c'; List<String> s=new List<String>(); String out;
  try { for(List<SObject> ch:Database.query(q)) for(SObject r:ch) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-cf-n@forChunkSObject|'+out);
 }
 // Pa-cf-n@sizeExpr
 @IsTest static void q41(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) c, Pick__c FROM NpAgg__c'; List<String> s=new List<String>(); String out;
  try { Integer n=Database.query(q).size(); s.add('size='+n); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-cf-n@sizeExpr|'+out);
 }
 // Pa-cf-ge@listSObject
 @IsTest static void q42(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) c, Pick__c FROM NpAgg__c WHERE Id IN :empty GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { List<SObject> rows=Database.query(q); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-cf-ge@listSObject|'+out);
 }
 // Pa-cf-ge@listAgg
 @IsTest static void q43(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) c, Pick__c FROM NpAgg__c WHERE Id IN :empty GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { List<AggregateResult> rows=Database.query(q); for(AggregateResult r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-cf-ge@listAgg|'+out);
 }
 // Pa-cf-ge@forAgg
 @IsTest static void q44(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) c, Pick__c FROM NpAgg__c WHERE Id IN :empty GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { for(AggregateResult r:Database.query(q)) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-cf-ge@forAgg|'+out);
 }
 // Pa-cf-ge@forSObject
 @IsTest static void q45(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) c, Pick__c FROM NpAgg__c WHERE Id IN :empty GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { for(SObject r:Database.query(q)) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-cf-ge@forSObject|'+out);
 }
 // Pa-cf-ge@forChunkAgg
 @IsTest static void q46(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) c, Pick__c FROM NpAgg__c WHERE Id IN :empty GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { for(List<AggregateResult> ch:Database.query(q)) for(AggregateResult r:ch) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-cf-ge@forChunkAgg|'+out);
 }
 // Pa-cf-ge@forChunkSObject
 @IsTest static void q47(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) c, Pick__c FROM NpAgg__c WHERE Id IN :empty GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { for(List<SObject> ch:Database.query(q)) for(SObject r:ch) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-cf-ge@forChunkSObject|'+out);
 }
 // Pa-cf-ge@sizeExpr
 @IsTest static void q48(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) c, Pick__c FROM NpAgg__c WHERE Id IN :empty GROUP BY Other__c'; List<String> s=new List<String>(); String out;
  try { Integer n=Database.query(q).size(); s.add('size='+n); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-cf-ge@sizeExpr|'+out);
 }
 // R213@listSObject
 @IsTest static void q49(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT Name,COUNT(Id) v FROM Account WHERE Id IN :sids GROUP BY BillingCity'; List<String> s=new List<String>(); String out;
  try { List<SObject> rows=Database.query(q); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|R213@listSObject|'+out);
 }
 // R213@listAgg
 @IsTest static void q50(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT Name,COUNT(Id) v FROM Account WHERE Id IN :sids GROUP BY BillingCity'; List<String> s=new List<String>(); String out;
  try { List<AggregateResult> rows=Database.query(q); for(AggregateResult r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|R213@listAgg|'+out);
 }
 // R213@forAgg
 @IsTest static void q51(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT Name,COUNT(Id) v FROM Account WHERE Id IN :sids GROUP BY BillingCity'; List<String> s=new List<String>(); String out;
  try { for(AggregateResult r:Database.query(q)) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|R213@forAgg|'+out);
 }
 // R213@forSObject
 @IsTest static void q52(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT Name,COUNT(Id) v FROM Account WHERE Id IN :sids GROUP BY BillingCity'; List<String> s=new List<String>(); String out;
  try { for(SObject r:Database.query(q)) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|R213@forSObject|'+out);
 }
 // R213@forChunkAgg
 @IsTest static void q53(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT Name,COUNT(Id) v FROM Account WHERE Id IN :sids GROUP BY BillingCity'; List<String> s=new List<String>(); String out;
  try { for(List<AggregateResult> ch:Database.query(q)) for(AggregateResult r:ch) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|R213@forChunkAgg|'+out);
 }
 // R213@forChunkSObject
 @IsTest static void q54(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT Name,COUNT(Id) v FROM Account WHERE Id IN :sids GROUP BY BillingCity'; List<String> s=new List<String>(); String out;
  try { for(List<SObject> ch:Database.query(q)) for(SObject r:ch) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|R213@forChunkSObject|'+out);
 }
 // R213@sizeExpr
 @IsTest static void q55(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT Name,COUNT(Id) v FROM Account WHERE Id IN :sids GROUP BY BillingCity'; List<String> s=new List<String>(); String out;
  try { Integer n=Database.query(q).size(); s.add('size='+n); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|R213@sizeExpr|'+out);
 }
 // R212@listSObject
 @IsTest static void q56(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT BillingCity,COUNT(Id) v FROM Account WHERE Id IN :sids'; List<String> s=new List<String>(); String out;
  try { List<SObject> rows=Database.query(q); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|R212@listSObject|'+out);
 }
 // R212@listAgg
 @IsTest static void q57(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT BillingCity,COUNT(Id) v FROM Account WHERE Id IN :sids'; List<String> s=new List<String>(); String out;
  try { List<AggregateResult> rows=Database.query(q); for(AggregateResult r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|R212@listAgg|'+out);
 }
 // R212@forAgg
 @IsTest static void q58(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT BillingCity,COUNT(Id) v FROM Account WHERE Id IN :sids'; List<String> s=new List<String>(); String out;
  try { for(AggregateResult r:Database.query(q)) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|R212@forAgg|'+out);
 }
 // R212@forSObject
 @IsTest static void q59(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT BillingCity,COUNT(Id) v FROM Account WHERE Id IN :sids'; List<String> s=new List<String>(); String out;
  try { for(SObject r:Database.query(q)) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|R212@forSObject|'+out);
 }
 // R212@forChunkAgg
 @IsTest static void q60(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT BillingCity,COUNT(Id) v FROM Account WHERE Id IN :sids'; List<String> s=new List<String>(); String out;
  try { for(List<AggregateResult> ch:Database.query(q)) for(AggregateResult r:ch) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|R212@forChunkAgg|'+out);
 }
 // R212@forChunkSObject
 @IsTest static void q61(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT BillingCity,COUNT(Id) v FROM Account WHERE Id IN :sids'; List<String> s=new List<String>(); String out;
  try { for(List<SObject> ch:Database.query(q)) for(SObject r:ch) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|R212@forChunkSObject|'+out);
 }
 // R212@sizeExpr
 @IsTest static void q62(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT BillingCity,COUNT(Id) v FROM Account WHERE Id IN :sids'; List<String> s=new List<String>(); String out;
  try { Integer n=Database.query(q).size(); s.add('size='+n); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|R212@sizeExpr|'+out);
 }
 // AI-cf-gw@listSObject
 @IsTest static void q63(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) v,Industry FROM Account WHERE Id IN :sids GROUP BY Phone'; List<String> s=new List<String>(); String out;
  try { List<SObject> rows=Database.query(q); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|AI-cf-gw@listSObject|'+out);
 }
 // AI-cf-gw@listAgg
 @IsTest static void q64(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) v,Industry FROM Account WHERE Id IN :sids GROUP BY Phone'; List<String> s=new List<String>(); String out;
  try { List<AggregateResult> rows=Database.query(q); for(AggregateResult r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|AI-cf-gw@listAgg|'+out);
 }
 // AI-cf-gw@forAgg
 @IsTest static void q65(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) v,Industry FROM Account WHERE Id IN :sids GROUP BY Phone'; List<String> s=new List<String>(); String out;
  try { for(AggregateResult r:Database.query(q)) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|AI-cf-gw@forAgg|'+out);
 }
 // AI-cf-gw@forSObject
 @IsTest static void q66(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) v,Industry FROM Account WHERE Id IN :sids GROUP BY Phone'; List<String> s=new List<String>(); String out;
  try { for(SObject r:Database.query(q)) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|AI-cf-gw@forSObject|'+out);
 }
 // AI-cf-gw@forChunkAgg
 @IsTest static void q67(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) v,Industry FROM Account WHERE Id IN :sids GROUP BY Phone'; List<String> s=new List<String>(); String out;
  try { for(List<AggregateResult> ch:Database.query(q)) for(AggregateResult r:ch) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|AI-cf-gw@forChunkAgg|'+out);
 }
 // AI-cf-gw@forChunkSObject
 @IsTest static void q68(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) v,Industry FROM Account WHERE Id IN :sids GROUP BY Phone'; List<String> s=new List<String>(); String out;
  try { for(List<SObject> ch:Database.query(q)) for(SObject r:ch) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|AI-cf-gw@forChunkSObject|'+out);
 }
 // AI-cf-gw@sizeExpr
 @IsTest static void q69(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String q='SELECT COUNT(Id) v,Industry FROM Account WHERE Id IN :sids GROUP BY Phone'; List<String> s=new List<String>(); String out;
  try { Integer n=Database.query(q).size(); s.add('size='+n); s.sort(); out='OK|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|AI-cf-gw@sizeExpr|'+out);
 }
}
