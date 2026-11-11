@IsTest public without sharing class NullCore67 {
 static Map<String,Schema.SObjectType> gd;
 static Map<String,Schema.SObjectType> g(){ if(gd==null) gd=Schema.getGlobalDescribe(); return gd; }
 static Integer seq=0;
 static Map<String,List<Id>> existing=new Map<String,List<Id>>();
 public static String typeName(Schema.SObjectType t){ return String.valueOf(t); }
 static Object filler(Schema.DescribeFieldResult d, Integer depth){
  Schema.DisplayType ty=d.getType(); seq++;
  if(ty==Schema.DisplayType.STRING||ty==Schema.DisplayType.TEXTAREA||ty==Schema.DisplayType.ENCRYPTEDSTRING){ String s='NP'+seq; Integer l=d.getLength(); return l>0&&s.length()>l?s.left(l):s; }
  if(ty==Schema.DisplayType.EMAIL) return 'np'+seq+'@example.invalid';
  if(ty==Schema.DisplayType.PHONE) return '555'+seq;
  if(ty==Schema.DisplayType.URL) return 'https://example.invalid/'+seq;
  if(ty==Schema.DisplayType.PICKLIST||ty==Schema.DisplayType.MULTIPICKLIST){ for(Schema.PicklistEntry p:d.getPicklistValues()) if(p.isActive()) return p.getValue(); return null; }
  if(ty==Schema.DisplayType.BOOLEAN) return false;
  if(ty==Schema.DisplayType.DATE) return Date.today();
  if(ty==Schema.DisplayType.DATETIME) return System.now();
  if(ty==Schema.DisplayType.TIME) return Time.newInstance(1,0,0,0);
  if(ty==Schema.DisplayType.INTEGER) return 1;
  if(ty==Schema.DisplayType.DOUBLE||ty==Schema.DisplayType.CURRENCY||ty==Schema.DisplayType.PERCENT) return 1;
  if(ty==Schema.DisplayType.REFERENCE){ List<Id> p=parents(d.getReferenceTo(),1,depth+1); return p==null||p.isEmpty()?null:p[0]; }
  return null;
 }
 public static SObject build(Schema.SObjectType t, Integer depth, String skipField){
  SObject s=t.newSObject(null,true);
  for(Schema.SObjectField f:t.getDescribe().fields.getMap().values()){
   Schema.DescribeFieldResult d=f.getDescribe();
   if(!d.isCreateable()||d.isNillable()||d.isDefaultedOnCreate()) continue;
   if(d.getName()==skipField) continue;
   if(s.get(d.getName())!=null) continue;
   Object v=filler(d,depth); if(v!=null) s.put(d.getName(),v);
  }
  String tn=typeName(t);
  if(tn=='Shift' || tn=='TimeSlot') {
   Map<String,Schema.SObjectField> fs=t.getDescribe().fields.getMap();
   if(fs.containsKey('StartTime') && fs.get('StartTime').getDescribe().isCreateable()) {
    if(fs.get('StartTime').getDescribe().getType()==Schema.DisplayType.DATETIME) {s.put('StartTime',System.now());s.put('EndTime',System.now().addHours(1));}
    else if(fs.get('StartTime').getDescribe().getType()==Schema.DisplayType.TIME) {s.put('StartTime',Time.newInstance(1,0,0,0));s.put('EndTime',Time.newInstance(2,0,0,0));}
   }
  }
  if(tn=='ServiceResource' && skipField!='RelatedRecordId') { List<Id> users=parents(new List<Schema.SObjectType>{User.SObjectType},2,depth+1); if(users!=null&&!users.isEmpty()) s.put('RelatedRecordId',users[0]); }
  if(tn=='Order') {s.put('EffectiveDate',Date.today());s.put('Status','Draft');s.put('Pricebook2Id',Test.getStandardPricebookId()); if(skipField!='AccountId' && s.get('AccountId')==null){List<Id> p=parents(new List<Schema.SObjectType>{g().get('account')},1,depth+1);if(p!=null&&!p.isEmpty())s.put('AccountId',p[0]);}}
  if((tn=='OrderItem'||tn=='ContractLineItem'||tn=='ProductConsumed') && skipField!='PricebookEntryId'){List<Id> p=parents(new List<Schema.SObjectType>{g().get('pricebookentry')},1,depth+1);if(p!=null&&!p.isEmpty())s.put('PricebookEntryId',p[0]);}
  if(tn=='OrderItem') {s.put('Quantity',1);s.put('UnitPrice',1);}
  if(tn=='ServiceAppointment' && skipField!='ParentRecordId' && s.get('ParentRecordId')==null){List<Id> p=parents(new List<Schema.SObjectType>{g().get('account')},1,depth+1);if(p!=null&&!p.isEmpty())s.put('ParentRecordId',p[0]);}
  if(tn=='TimeSheet' && skipField!='ServiceResourceId' && s.get('ServiceResourceId')==null){List<Id> p=parents(new List<Schema.SObjectType>{g().get('serviceresource')},1,depth+1);if(p!=null&&!p.isEmpty())s.put('ServiceResourceId',p[0]);}
  if(tn=='Location') { Map<String,Schema.SObjectField> fs=t.getDescribe().fields.getMap(); for(String flag:new List<String>{'IsInventoryLocation','IsMobile'}) if(fs.containsKey(flag)&&fs.get(flag).getDescribe().isCreateable()) s.put(flag,true); }
  if(tn=='TimeSheetEntry') { List<Id> sheets=parents(new List<Schema.SObjectType>{g().get('timesheet')},1,depth+1); if(sheets!=null&&!sheets.isEmpty()) s.put('TimeSheetId',sheets[0]); }
  if(tn=='ServiceTerritory') s.put('IsActive',true);
  if(tn=='FinanceTransaction') s.put('ReferenceEntityType','Other');
  if(tn=='Payment') s.put('Type','Capture');
  if(tn=='Payment'||tn=='Refund'||tn=='PaymentAuthorization') { if(t.getDescribe().fields.getMap().containsKey('ProcessingMode')) s.put('ProcessingMode','External'); }
  if(tn=='ServiceResource') s.put('IsActive',true);
  if(tn=='MaintenanceWorkRule') s.put('RecurrencePattern','FREQ=DAILY;INTERVAL=1');
  if(tn=='MaintenanceWorkRule' && skipField!='ParentMaintenancePlanId') {
   List<Id> p=parents(new List<Schema.SObjectType>{g().get('maintenanceplan')},1,depth+1);
   if(p!=null&&!p.isEmpty()) s.put('ParentMaintenancePlanId',p[0]);
  }

  if(tn=='ReturnOrderLineItem') {
   s.put('QuantityReturned',1);
   String support=skipField=='Product2Id'?'AssetId':'Product2Id';
   List<Id> p=parents(new List<Schema.SObjectType>{g().get(support=='AssetId'?'asset':'product2')},1,depth+1);
   if(p!=null&&!p.isEmpty()) s.put(support,p[0]);
  }
  if(tn=='Partner' && (skipField=='AccountFromId' || skipField=='OpportunityId')) {
   String support=skipField=='AccountFromId'?'OpportunityId':'AccountFromId';
   List<Id> p=parents(new List<Schema.SObjectType>{g().get(support=='AccountFromId'?'account':'opportunity')},1,depth+1);
   if(p!=null&&!p.isEmpty()) s.put(support,p[0]);
  }

  if(tn=='ContentDistribution') {ContentVersion cv=new ContentVersion(Title='Synthetic',PathOnClient='synthetic.txt',VersionData=Blob.valueOf('synthetic'));insert as system cv;s.put('ContentVersionId',cv.Id);}
  if(tn=='Contact' && s.get('AccountId')==null && skipField!='AccountId') { List<Id> p=parents(new List<Schema.SObjectType>{g().get('account')},1,depth+1); if(p!=null&&!p.isEmpty()) s.put('AccountId',p[0]); }
  if(tn=='Asset') {
   String support=skipField=='AccountId'?'ContactId':'AccountId';
   if(s.get(support)==null) { List<Id> p=parents(new List<Schema.SObjectType>{g().get(support=='ContactId'?'contact':'account')},1,depth+1); if(p!=null&&!p.isEmpty()) s.put(support,p[0]); }
  }
  if(tn=='AssetWarranty' && s.get('AssetId')==null) { List<Id> p=parents(new List<Schema.SObjectType>{g().get('asset')},1,depth+1); if(p!=null&&!p.isEmpty()) s.put('AssetId',p[0]); }
  if(tn=='User') {
   seq++;s.put('Username','lookup'+seq+'.'+UserInfo.getOrganizationId()+'.'+System.now().getTime()+'@example.invalid');
   s.put('Alias','look'+Math.mod(seq,1000));s.put('LastName','Lookup');s.put('Email','lookup'+seq+'@example.invalid');
   s.put('EmailEncodingKey','UTF-8');s.put('LanguageLocaleKey','en_US');s.put('LocaleSidKey','en_US');s.put('TimeZoneSidKey','GMT');s.put('ProfileId',UserInfo.getProfileId());
  }
  return s;
 }
 // returns n parent ids of one of the target types, sorted ascending
 public static String lastParentType; public static String lastParentMode;
 public static List<Id> parents(List<Schema.SObjectType> targets, Integer n, Integer depth){
  if(depth>3) return null;
  List<Schema.SObjectType> order=new List<Schema.SObjectType>();
  for(Schema.SObjectType t:targets) if(typeName(t)=='Account') order.add(t);
  for(Schema.SObjectType t:targets) if(typeName(t)!='Account') order.add(t);
  for(Schema.SObjectType t:order){
   String tn=typeName(t);
   if(tn=='PricebookEntry'){
    try{List<Product2> products=new List<Product2>();for(Integer i=0;i<n;i++) products.add(new Product2(Name='Synthetic '+i,IsActive=true));insert as system products;List<PricebookEntry> entries=new List<PricebookEntry>();for(Product2 product:products)entries.add(new PricebookEntry(Pricebook2Id=Test.getStandardPricebookId(),Product2Id=product.Id,UnitPrice=1,IsActive=true));insert as system entries;List<Id> ids=new List<Id>();for(PricebookEntry entry:entries)ids.add(entry.Id);ids.sort();lastParentType=tn;lastParentMode='created';return ids;}catch(Exception e){continue;}
   }
   if(tn=='User'){
    if(n==1) { lastParentType=tn; lastParentMode='existing'; return new List<Id>{UserInfo.getUserId()}; }
    List<User> us=new List<User>();
    for(Integer i=0;i<n;i++){ seq++; us.add(new User(Alias='np'+seq,LastName='NullProbe'+seq,Email='np'+seq+'@example.invalid',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='GMT',ProfileId=UserInfo.getProfileId(),Username='np'+seq+'.'+UserInfo.getOrganizationId()+'.'+System.now().getTime()+'@example.invalid')); }
    try{ System.runAs(new User(Id=UserInfo.getUserId())){ insert as system us; } List<Id> ids=new List<Id>(); for(User u:us) ids.add(u.Id); ids.sort(); lastParentType=tn; lastParentMode='created'; return ids; }catch(Exception e){}
    continue;
   }
   Schema.DescribeSObjectResult dr=t.getDescribe();
   if(dr.isCreateable() && depth<=3){
    try{
     List<SObject> ps=new List<SObject>(); for(Integer i=0;i<n;i++) ps.add(build(t,depth,null));
     insertRequired(ps,null,depth); List<Id> ids=new List<Id>(); for(SObject p:ps) ids.add(p.Id); ids.sort(); lastParentType=tn; lastParentMode='created'; return ids;
    }catch(Exception e){}
   }
   if(dr.isQueryable()){
    try{
     List<Id> ids=new List<Id>(); for(SObject p:Database.query('SELECT Id FROM '+tn+' ORDER BY Id LIMIT '+n)) ids.add(p.Id);
     if(ids.size()==n){ lastParentType=tn; lastParentMode='existing'; return ids; }
    }catch(Exception e){}
   }
  }
  return null;
 }
 public static String label(Id x, Map<Id,String> m){ return m.containsKey(x)?m.get(x):'?'; }
 static void insertRequired(List<SObject> records, String skipField, Integer depth) {
  for(Integer attempt=0;attempt<5;attempt++) {
   try {
    if(typeName(records[0].getSObjectType())=='User') { System.runAs(new User(Id=UserInfo.getUserId())) { insert as system records; } }
    else { insert as system records; }
    return;
   } catch(DmlException ex) {
    if(ex.getNumDml()==0 || String.valueOf(ex.getDmlType(0))!='REQUIRED_FIELD_MISSING') throw ex;
    List<String> names=ex.getDmlFieldNames(0); if(names==null || names.isEmpty()) throw ex;
    Boolean changed=false; Map<String,Schema.SObjectField> fmap=records[0].getSObjectType().getDescribe().fields.getMap();
    for(String name:names) {
     if(name==skipField || !fmap.containsKey(name)) continue;
     Schema.DescribeFieldResult fd=fmap.get(name).getDescribe(); if(!fd.isCreateable()) continue;
     Object value=filler(fd,depth+1); if(value==null) continue;
     for(SObject record:records) if(record.get(name)==null) {record.put(name,value);changed=true;}
    }
    if(!changed || attempt==4) throw ex;
   }
  }
 }
 public static Map<String,Object> probeField(String objName,String field){Map<String,Object> o=new Map<String,Object>();Savepoint sp=Database.setSavepoint();try{Schema.SObjectType t=g().get(objName.toLowerCase());if(t==null||!t.getDescribe().fields.getMap().containsKey(field)){o.put('skip','field absent');Database.rollback(sp);return o;}Schema.DescribeFieldResult d=t.getDescribe().fields.getMap().get(field).getDescribe();o.put('updateable',d.isUpdateable());if(!d.isUpdateable()){o.put('skip','field not updateable');Database.rollback(sp);return o;}List<Id> ps=parents(d.getReferenceTo(),2,1);if(ps==null||ps.size()!=2){o.put('skip','no two parents');Database.rollback(sp);return o;}List<SObject> kids=new List<SObject>();for(Integer i=0;i<2;i++){SObject k=build(t,1,field);k.put(field,ps[i]);kids.add(k);}List<Database.SaveResult> rs=Database.insert(kids,false,AccessLevel.SYSTEM_MODE);List<String> results=new List<String>();for(Database.SaveResult r:rs)results.add(r.isSuccess()?'OK':String.valueOf(r.getErrors()[0].getStatusCode())+':'+r.getErrors()[0].getMessage().left(140));o.put('inserts',results);if(!rs[0].isSuccess()||!rs[1].isSuccess()){Database.rollback(sp);return o;}SObject patch=t.newSObject(rs[0].getId());patch.put(field,null);Database.SaveResult clear=Database.update(patch,false,AccessLevel.SYSTEM_MODE);o.put('clear',clear.isSuccess()?'OK':String.valueOf(clear.getErrors()[0].getStatusCode())+':'+clear.getErrors()[0].getMessage().left(140));if(!clear.isSuccess()){Database.rollback(sp);return o;}Set<Id> ids=new Set<Id>{rs[0].getId(),rs[1].getId()};List<SObject> saved=Database.query('SELECT Id,'+field+' FROM '+objName+' WHERE Id IN :ids');Map<Id,String> labels=new Map<Id,String>();for(SObject x:saved)labels.put((Id)x.get('Id'),x.get(field)==null?'N':'P');o.put('savedShape',labels.values());if(labels.values().contains('N')&&labels.values().contains('P'))for(String tail:new List<String>{'ASC','DESC','ASC NULLS FIRST','ASC NULLS LAST','DESC NULLS FIRST','DESC NULLS LAST'}){String order='';for(SObject x:Database.query('SELECT Id FROM '+objName+' WHERE Id IN :ids ORDER BY '+field+' '+tail))order+=labels.get((Id)x.get('Id'));o.put(tail,order);}}catch(Exception e){o.put('skip','EXC '+e.getTypeName()+': '+e.getMessage().left(180));}Database.rollback(sp);return o;}
 public static String probeObject(String objName, List<String> fields){
  Map<String,Object> out=new Map<String,Object>();
  for(String f:fields){ try{ out.put(f,probeField(objName,f)); }catch(Exception e){ out.put(f,new Map<String,Object>{'skip'=>'EXC '+e.getMessage().left(500)}); } }
  return JSON.serialize(out);
 }
}
