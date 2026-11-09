Map<String,Object> o=new Map<String,Object>();
Schema.SObjectType st=Schema.getGlobalDescribe().get('CapObj__c');
Schema.SObjectField fld=st.getDescribe().fields.getMap().get('Optional__c');
o.put('fieldInMap',fld!=null);
if(fld!=null){Schema.DescribeFieldResult f=fld.getDescribe(); o.put('describe',new List<Boolean>{st.getDescribe().isAccessible(),st.getDescribe().isCreateable(),f.isAccessible(),f.isCreateable(),f.isUpdateable()});}
SObject base=st.newSObject(); base.put('Name','base'); base.put('Required__c','r'); insert base;
try { SObject r=st.newSObject(); r.put('Name','a'); r.put('Required__c','r'); r.put('Optional__c','x'); insert r; o.put('insert','ok'); } catch(Exception e) {o.put('insert',e.getTypeName()+':'+e.getMessage());}
try { base.put('Optional__c','y'); update base; o.put('update','ok'); } catch(Exception e) {o.put('update',e.getTypeName()+':'+e.getMessage());}
try { SObject r=st.newSObject(); r.put('Name','b'); r.put('Required__c','r'); r.put('Optional__c','x'); Database.SaveResult s=Database.insert(r,false); List<String> errs=new List<String>(); for(Database.Error er:s.getErrors()) errs.add(String.valueOf(er.getStatusCode())+'/'+er.getFields()); o.put('dbInsertFalse',new List<Object>{s.isSuccess(),errs}); } catch(Exception e) {o.put('dbInsertFalse',e.getTypeName()+':'+e.getMessage());}
try { List<SObject> l=Database.query('SELECT Id, Optional__c FROM CapObj__c'); o.put('soql','ok:'+l.size()); } catch(Exception e) {o.put('soql',e.getTypeName()+':'+e.getMessage());}
try { List<SObject> l=Database.query('SELECT Id, Optional__c FROM CapObj__c WITH USER_MODE'); o.put('soqlUserMode','ok:'+l.size()); } catch(Exception e) {o.put('soqlUserMode',e.getTypeName()+':'+e.getMessage());}
try { List<SObject> l=Database.query('SELECT Id, Optional__c FROM CapObj__c WITH SECURITY_ENFORCED'); o.put('soqlEnforced','ok:'+l.size()); } catch(Exception e) {o.put('soqlEnforced',e.getTypeName()+':'+e.getMessage());}
System.debug('OBSERVED|anonymous|'+JSON.serialize(o));
