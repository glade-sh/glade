@IsTest private class MixProbe62 {
 static List<Object> res(List<Database.SaveResult> rs){ List<Object> out=new List<Object>(); for(Database.SaveResult s:rs){ List<String> errs=new List<String>(); for(Database.Error er:s.getErrors()) errs.add(String.valueOf(er.getStatusCode())+'/'+er.getFields()+'/['+er.getMessage()+']'); out.add(new List<Object>{s.isSuccess(),s.getId()!=null,errs}); } return out; }
 @IsTest static void mixed(){
  Map<String,Object> o=new Map<String,Object>();
  try { List<CapObj__c> l=new List<CapObj__c>{new CapObj__c(Name='m1',Required__c='r',Optional__c='x'),new CapObj__c(Name='m2',Required__c='r')}; o.put('insertMixed',res(Database.insert(l,false))); o.put('count',[SELECT COUNT() FROM CapObj__c WITH SYSTEM_MODE]); } catch(Exception e){o.put('insertMixed',e.getTypeName()+':'+e.getMessage());}
  try { List<CapObj__c> l=new List<CapObj__c>{new CapObj__c(Name='n1',Required__c='r'),new CapObj__c(Name='n2',Required__c='r')}; Database.insert(l,AccessLevel.SYSTEM_MODE); l[0].Optional__c='y'; l[1].Name='n2b'; o.put('updateMixed',res(Database.update(l,false)));} catch(Exception e){o.put('updateMixed',e.getTypeName()+':'+e.getMessage());}
  try { List<CapObj__c> l=new List<CapObj__c>{new CapObj__c(Name='p1',Required__c='r',Optional__c='x'),new CapObj__c(Name='p2',Required__c='r',Optional__c='x')}; o.put('insertAllDenied',res(Database.insert(l,false)));} catch(Exception e){o.put('insertAllDenied',e.getTypeName()+':'+e.getMessage());}
  try { List<CapObj__c> l=new List<CapObj__c>{new CapObj__c(Name='q1',Required__c='r',Optional__c='x'),new CapObj__c(Name='q2',Required__c='r')}; insert l; o.put('insertMixedPlain','ok');} catch(Exception e){o.put('insertMixedPlain',e.getTypeName()+':'+e.getMessage());}
  System.assert(false,'OBSERVED|mixed|'+JSON.serialize(o));
 }
}
