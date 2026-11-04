@IsTest(SeeAllData=false)
private without sharing class DootSoqlQ7TestB1031 {
 @IsTest static void baseline(){System.assertEquals(2,1+1,'BASELINE|arithmetic');}

 private static Map<String,Object> failure(String stage,Exception e){
  Map<String,Object> x=new Map<String,Object>{'stage'=>stage,'type'=>e.getTypeName(),'message'=>e.getMessage()};
  if(e instanceof QueryException)x.put('inaccessible',((QueryException)e).getInaccessibleFields());
  if(e instanceof DmlException){DmlException d=(DmlException)e;List<Object> rows=new List<Object>();for(Integer i=0;i<d.getNumDml();i++)rows.add(new Map<String,Object>{'i'=>d.getDmlIndex(i),'code'=>String.valueOf(d.getDmlStatusCode(i)),'message'=>d.getDmlMessage(i),'fields'=>d.getDmlFieldNames(i)});x.put('dml',rows);}return x;
 }
 private static Object compactValue(Object v){if(v instanceof SObject){Map<String,Object> m=new Map<String,Object>(),raw=((SObject)v).getPopulatedFieldsAsMap();for(String k:raw.keySet())m.put(k,compactValue(raw.get(k)));return m;}if(v instanceof List<SObject>)return compact((List<SObject>)v);return v;}
 private static List<Object> compact(List<SObject> rows){List<Object> xs=new List<Object>();for(SObject r:rows)xs.add(compactValue(r));return xs;}
 private static void emit(String id,Map<String,Object> o){
  o.put('locale',UserInfo.getLocale());o.put('tz',UserInfo.getTimeZone().getID());
  String j=JSON.serialize(o);if(j.length()>620)j=JSON.serialize(new Map<String,Object>{'status'=>'inconclusive_overlength','originalLength'=>j.length()});
  System.assert(false,'CAPTURE|'+id+'|'+j);
 }
 @IsTest static void valid(){
  Map<String,Object> o=new Map<String,Object>();String stage='fixture';
  try{Account a=new Account(Name='Q7',Phone='555');insert as system a;Task t=new Task(Subject='Q7',WhatId=a.Id);insert as system t;Id tid=t.Id;System.assertEquals(a.Id,[SELECT WhatId FROM Task WHERE Id=:tid WITH SYSTEM_MODE].WhatId,'fixture WhatId');stage='query';o.put('rows',compact(Database.query('SELECT Id,TYPEOF What WHEN Account THEN Phone END FROM Task WHERE Id=:tid WITH SYSTEM_MODE'))); o.put('status','observed');o.put('stage',stage);}
  catch(Exception e){o.put('exception',failure(stage,e));}
  emit('Q7.valid',o);
 }
 @IsTest static void overlap(){
  Map<String,Object> o=new Map<String,Object>();String stage='fixture';
  try{Account a=new Account(Name='Q7',Phone='555');insert as system a;Task t=new Task(Subject='Q7',WhatId=a.Id);insert as system t;Id tid=t.Id;System.assertEquals(a.Id,[SELECT WhatId FROM Task WHERE Id=:tid WITH SYSTEM_MODE].WhatId,'fixture WhatId');stage='query';o.put('rows',compact(Database.query('SELECT Id,What.Name,TYPEOF What WHEN Account THEN Phone END FROM Task WHERE Id=:tid WITH SYSTEM_MODE'))); o.put('status','observed');o.put('stage',stage);}
  catch(Exception e){o.put('exception',failure(stage,e));}
  emit('Q7.overlap',o);
 }
 @IsTest static void formatFunction(){
  Map<String,Object> o=new Map<String,Object>();String stage='fixture';
  try{Account a=new Account(Name='Q7',Phone='555');insert as system a;Task t=new Task(Subject='Q7',WhatId=a.Id);insert as system t;Id tid=t.Id;System.assertEquals(a.Id,[SELECT WhatId FROM Task WHERE Id=:tid WITH SYSTEM_MODE].WhatId,'fixture WhatId');stage='query';o.put('rows',compact(Database.query('SELECT Id,FORMAT(CreatedDate),TYPEOF What WHEN Account THEN Phone END FROM Task WHERE Id=:tid WITH SYSTEM_MODE'))); o.put('status','observed');o.put('stage',stage);}
  catch(Exception e){o.put('exception',failure(stage,e));}
  emit('Q7.formatFunction',o);
 }
 @IsTest static void typeofFirstCount(){
  Map<String,Object> o=new Map<String,Object>();String stage='fixture';
  try{Account a=new Account(Name='Q7',Phone='555');insert as system a;Task t=new Task(Subject='Q7',WhatId=a.Id);insert as system t;Id tid=t.Id;System.assertEquals(a.Id,[SELECT WhatId FROM Task WHERE Id=:tid WITH SYSTEM_MODE].WhatId,'fixture WhatId');stage='query';o.put('rows',compact(Database.query('SELECT TYPEOF What WHEN Account THEN Phone END,COUNT() FROM Task WHERE Id=:tid WITH SYSTEM_MODE'))); o.put('status','observed');o.put('stage',stage);}
  catch(Exception e){o.put('exception',failure(stage,e));}
  emit('Q7.typeofFirstCount',o);
 }
 @IsTest static void countFirstTypeof(){
  Map<String,Object> o=new Map<String,Object>();String stage='fixture';
  try{Account a=new Account(Name='Q7',Phone='555');insert as system a;Task t=new Task(Subject='Q7',WhatId=a.Id);insert as system t;Id tid=t.Id;System.assertEquals(a.Id,[SELECT WhatId FROM Task WHERE Id=:tid WITH SYSTEM_MODE].WhatId,'fixture WhatId');stage='query';o.put('rows',compact(Database.query('SELECT COUNT(),TYPEOF What WHEN Account THEN Phone END FROM Task WHERE Id=:tid WITH SYSTEM_MODE'))); o.put('status','observed');o.put('stage',stage);}
  catch(Exception e){o.put('exception',failure(stage,e));}
  emit('Q7.countFirstTypeof',o);
 }
}
