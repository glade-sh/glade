@IsTest private class NpAgg62 {
 // Pa-cf-g
 @IsTest static void q00(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT COUNT(Id) c, Pick__c FROM NpAgg__c GROUP BY Other__c'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-cf-g|'+out);
 }
 // Pa-fc-g
 @IsTest static void q01(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT Pick__c, COUNT(Id) c FROM NpAgg__c GROUP BY Other__c'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-fc-g|'+out);
 }
 // Pa-cf-n
 @IsTest static void q02(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT COUNT(Id) c, Pick__c FROM NpAgg__c'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-cf-n|'+out);
 }
 // Pa-fc-n
 @IsTest static void q03(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT Pick__c, COUNT(Id) c FROM NpAgg__c'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-fc-n|'+out);
 }
 // Pn-cf-g
 @IsTest static void q04(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT COUNT(Id), Pick__c FROM NpAgg__c GROUP BY Other__c'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pn-cf-g|'+out);
 }
 // Pn-fc-g
 @IsTest static void q05(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT Pick__c, COUNT(Id) FROM NpAgg__c GROUP BY Other__c'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pn-fc-g|'+out);
 }
 // Pn-cf-n
 @IsTest static void q06(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT COUNT(Id), Pick__c FROM NpAgg__c'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pn-cf-n|'+out);
 }
 // Pn-fc-n
 @IsTest static void q07(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT Pick__c, COUNT(Id) FROM NpAgg__c'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pn-fc-n|'+out);
 }
 // Pa-cf-gw
 @IsTest static void q08(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT COUNT(Id) c, Pick__c FROM NpAgg__c WHERE Id IN :ids GROUP BY Other__c'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-cf-gw|'+out);
 }
 // Pa-fc-gw
 @IsTest static void q09(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT Pick__c, COUNT(Id) c FROM NpAgg__c WHERE Id IN :ids GROUP BY Other__c'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-fc-gw|'+out);
 }
 // Pa-cf-ge
 @IsTest static void q10(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT COUNT(Id) c, Pick__c FROM NpAgg__c WHERE Id IN :empty GROUP BY Other__c'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Pa-cf-ge|'+out);
 }
 // Ta-cf-g
 @IsTest static void q11(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT COUNT(Id) c, Txt__c FROM NpAgg__c GROUP BY Other__c'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Ta-cf-g|'+out);
 }
 // Ta-fc-g
 @IsTest static void q12(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT Txt__c, COUNT(Id) c FROM NpAgg__c GROUP BY Other__c'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Ta-fc-g|'+out);
 }
 // Ta-cf-n
 @IsTest static void q13(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT COUNT(Id) c, Txt__c FROM NpAgg__c'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Ta-cf-n|'+out);
 }
 // Ta-fc-n
 @IsTest static void q14(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT Txt__c, COUNT(Id) c FROM NpAgg__c'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Ta-fc-n|'+out);
 }
 // Tn-cf-g
 @IsTest static void q15(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT COUNT(Id), Txt__c FROM NpAgg__c GROUP BY Other__c'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Tn-cf-g|'+out);
 }
 // Tn-fc-g
 @IsTest static void q16(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT Txt__c, COUNT(Id) FROM NpAgg__c GROUP BY Other__c'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Tn-fc-g|'+out);
 }
 // Tn-cf-n
 @IsTest static void q17(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT COUNT(Id), Txt__c FROM NpAgg__c'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Tn-cf-n|'+out);
 }
 // Tn-fc-n
 @IsTest static void q18(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT Txt__c, COUNT(Id) FROM NpAgg__c'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Tn-fc-n|'+out);
 }
 // Ta-cf-gw
 @IsTest static void q19(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT COUNT(Id) c, Txt__c FROM NpAgg__c WHERE Id IN :ids GROUP BY Other__c'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Ta-cf-gw|'+out);
 }
 // Ta-fc-gw
 @IsTest static void q20(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT Txt__c, COUNT(Id) c FROM NpAgg__c WHERE Id IN :ids GROUP BY Other__c'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Ta-fc-gw|'+out);
 }
 // Ta-cf-ge
 @IsTest static void q21(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT COUNT(Id) c, Txt__c FROM NpAgg__c WHERE Id IN :empty GROUP BY Other__c'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|Ta-cf-ge|'+out);
 }
 // AN-fc-gw
 @IsTest static void q22(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT Name,COUNT(Id) v FROM Account WHERE Id IN :sids GROUP BY Phone'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|AN-fc-gw|'+out);
 }
 // AN-cf-gw
 @IsTest static void q23(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT COUNT(Id) v,Name FROM Account WHERE Id IN :sids GROUP BY Phone'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|AN-cf-gw|'+out);
 }
 // AN-fc-nw
 @IsTest static void q24(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT Name,COUNT(Id) v FROM Account WHERE Id IN :sids'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|AN-fc-nw|'+out);
 }
 // AN-cf-nw
 @IsTest static void q25(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT COUNT(Id) v,Name FROM Account WHERE Id IN :sids'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|AN-cf-nw|'+out);
 }
 // AI-fc-gw
 @IsTest static void q26(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT Industry,COUNT(Id) v FROM Account WHERE Id IN :sids GROUP BY Phone'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|AI-fc-gw|'+out);
 }
 // AI-cf-gw
 @IsTest static void q27(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT COUNT(Id) v,Industry FROM Account WHERE Id IN :sids GROUP BY Phone'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|AI-cf-gw|'+out);
 }
 // AI-fc-nw
 @IsTest static void q28(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT Industry,COUNT(Id) v FROM Account WHERE Id IN :sids'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|AI-fc-nw|'+out);
 }
 // AI-cf-nw
 @IsTest static void q29(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT COUNT(Id) v,Industry FROM Account WHERE Id IN :sids'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|AI-cf-nw|'+out);
 }
 // AB-fc-gw
 @IsTest static void q30(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT BillingCity,COUNT(Id) v FROM Account WHERE Id IN :sids GROUP BY Phone'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|AB-fc-gw|'+out);
 }
 // AB-cf-gw
 @IsTest static void q31(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT COUNT(Id) v,BillingCity FROM Account WHERE Id IN :sids GROUP BY Phone'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|AB-cf-gw|'+out);
 }
 // AB-fc-nw
 @IsTest static void q32(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT BillingCity,COUNT(Id) v FROM Account WHERE Id IN :sids'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|AB-fc-nw|'+out);
 }
 // AB-cf-nw
 @IsTest static void q33(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT COUNT(Id) v,BillingCity FROM Account WHERE Id IN :sids'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|AB-cf-nw|'+out);
 }
 // R213
 @IsTest static void q34(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT Name,COUNT(Id) v FROM Account WHERE Id IN :sids GROUP BY BillingCity'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|R213|'+out);
 }
 // R213-cf
 @IsTest static void q35(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT COUNT(Id) v,Name FROM Account WHERE Id IN :sids GROUP BY BillingCity'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|R213-cf|'+out);
 }
 // R212
 @IsTest static void q36(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT BillingCity,COUNT(Id) v FROM Account WHERE Id IN :sids'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|R212|'+out);
 }
 // R213-e
 @IsTest static void q37(){
  List<NpAgg__c> nx=new List<NpAgg__c>{new NpAgg__c(Name='n1',Pick__c='Alpha',Txt__c='x',Other__c='o1'),new NpAgg__c(Name='n2',Pick__c='Beta',Txt__c='y',Other__c='o1'),new NpAgg__c(Name='n3',Other__c='o2')};
  insert nx; Set<Id> ids=new Map<Id,NpAgg__c>(nx).keySet(); Set<Id> empty=new Set<Id>();
  List<Account> ax=new List<Account>{new Account(Name='agg1',BillingCity='North',Phone='1',Industry='Banking'),new Account(Name='agg2',BillingCity='North',Phone='1',Industry='Energy'),new Account(Name='agg3',BillingCity='South',Phone='2')};
  insert ax; Set<Id> sids=new Map<Id,Account>(ax).keySet();
  String out;
  try { List<SObject> rows=Database.query('SELECT Name,COUNT(Id) v FROM Account WHERE Id IN :empty GROUP BY BillingCity'); List<String> s=new List<String>(); for(SObject r:rows) s.add(String.valueOf(r.getPopulatedFieldsAsMap())); s.sort(); out='OK|'+rows.size()+'|'+String.join(s,';'); }
  catch(Exception e){ out='EXC|'+e.getTypeName()+'|'+e.getMessage(); }
  System.assert(false,'CAPTURE|R213-e|'+out);
 }
}
