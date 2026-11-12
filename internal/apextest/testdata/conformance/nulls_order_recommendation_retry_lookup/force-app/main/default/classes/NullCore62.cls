@IsTest public without sharing class NullCore62 {
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
  if((tn=='OrderItem'||tn=='ContractLineItem'||tn=='ProductConsumed'||tn=='OpportunityLineItem') && skipField!='PricebookEntryId'){List<Id> p=parents(new List<Schema.SObjectType>{g().get('pricebookentry')},1,depth+1);if(p!=null&&!p.isEmpty())s.put('PricebookEntryId',p[0]);}
  if(tn=='OrderItem') {s.put('Quantity',1);s.put('UnitPrice',1);}
  if((tn=='WorkOrder'||tn=='ServiceContract') && skipField!='Pricebook2Id') s.put('Pricebook2Id',Test.getStandardPricebookId());
  if(tn=='ProductServiceCampaignItem' && (skipField=='AssetId'||skipField=='Product2Id')) {String support=skipField=='AssetId'?'Product2Id':'AssetId'; List<Id> p=parents(new List<Schema.SObjectType>{g().get(support=='AssetId'?'asset':'product2')},1,depth+1);if(p!=null&&!p.isEmpty())s.put(support,p[0]);}
  if(tn=='Opportunity'){s.put('Name','Synthetic Opportunity');s.put('StageName','Prospecting');s.put('CloseDate',Date.today());s.put('Pricebook2Id',Test.getStandardPricebookId());}
  if(tn=='OpportunityLineItem') {s.put('Quantity',1);s.put('UnitPrice',1);}
  if(tn=='AuthorizationFormConsent' && skipField!='AuthorizationFormTextId') {List<Id> forms=parents(new List<Schema.SObjectType>{g().get('authorizationformtext')},1,depth+1);if(forms!=null&&!forms.isEmpty())s.put('AuthorizationFormTextId',forms[0]);}
  if(tn=='Image') {Map<String,Schema.SObjectField> fs=t.getDescribe().fields.getMap();for(String f:new List<String>{'Name','Title'})if(fs.containsKey(f)&&fs.get(f).getDescribe().isCreateable())s.put(f,'Synthetic');for(String f:new List<String>{'Url','ImageUrl','ExternalUrl'})if(fs.containsKey(f)&&fs.get(f).getDescribe().isCreateable())s.put(f,'https://example.invalid/synthetic.png');}
  if(tn=='Recommendation') s.put('ActionReference','SyntheticAction');
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
   if(tn=='ContentAsset'){
    try{List<ContentVersion> versions=new List<ContentVersion>();for(Integer i=0;i<n;i++)versions.add(new ContentVersion(Title='Synthetic '+i,PathOnClient='synthetic'+i+'.png',VersionData=Blob.valueOf('synthetic'),IsAssetEnabled=true));insert as system versions;Set<Id> versionIds=new Set<Id>();for(ContentVersion version:versions)versionIds.add(version.Id);Set<Id> docs=new Set<Id>();for(ContentVersion version:[SELECT ContentDocumentId FROM ContentVersion WHERE Id IN :versionIds])docs.add(version.ContentDocumentId);List<Id> ids=new List<Id>();for(ContentAsset asset:[SELECT Id FROM ContentAsset WHERE ContentDocumentId IN :docs])ids.add(asset.Id);ids.sort();if(ids.size()==n){lastParentType=tn;lastParentMode='created';return ids;}}catch(Exception e){}
    continue;
   }
   if(tn=='ContentVersion'||tn=='ContentDocument'){
    try{List<ContentVersion> versions=new List<ContentVersion>();for(Integer i=0;i<n;i++)versions.add(new ContentVersion(Title='Synthetic '+i,PathOnClient='synthetic'+i+'.txt',VersionData=Blob.valueOf('synthetic')));insert as system versions;Set<Id> vIds=new Set<Id>();for(ContentVersion version:versions)vIds.add(version.Id);List<Id> ids=new List<Id>();for(ContentVersion version:[SELECT Id,ContentDocumentId FROM ContentVersion WHERE Id IN :vIds])ids.add(tn=='ContentVersion'?version.Id:version.ContentDocumentId);ids.sort();lastParentType=tn;lastParentMode='created';return ids;}catch(Exception e){continue;}
   }
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
     List<SObject> ps=new List<SObject>(); for(Integer i=0;i<n;i++) ps.add(build(t,depth,null)); if(tn=='ContractLineItem' && ps.size()>1){Id shared=(Id)ps[0].get('ServiceContractId');for(SObject item:ps)item.put('ServiceContractId',shared);} if(tn=='WorkOrderLineItem' && ps.size()>1){Id shared=(Id)ps[0].get('WorkOrderId');for(SObject item:ps)item.put('WorkOrderId',shared);}
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
    else { System.runAs(new User(Id=UserInfo.getUserId())) { insert as system records; } }
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
 public static Map<String,Object> probeField(String objName, String field){
  Map<String,Object> o=new Map<String,Object>();
  Schema.SObjectType t=g().get(objName.toLowerCase());
  if(t==null || !t.getDescribe().fields.getMap().containsKey(field)){o.put('skip','field absent from describe');return o;}
  Schema.DescribeFieldResult d=t.getDescribe().fields.getMap().get(field).getDescribe();
  o.put('groupable',d.isGroupable()); o.put('sortable',d.isSortable()); o.put('filterable',d.isFilterable());
  o.put('cascadeDelete',d.isCascadeDelete()); o.put('restrictedDelete',d.isRestrictedDelete()); o.put('namePointing',d.isNamePointing());
  o.put('idLookup',d.isIdLookup()); o.put('custom',d.isCustom()); o.put('relationshipName',d.getRelationshipName()); o.put('relationshipOrder',d.getRelationshipOrder());
  List<String> refs=new List<String>(); for(Schema.SObjectType r:d.getReferenceTo()) refs.add(typeName(r)); o.put('referenceTo',refs);
  Savepoint sp=Database.setSavepoint();
  try{
   lastParentType=null; lastParentMode=null;
   List<Id> ps=parents(objName=='User' && field=='DelegatedApproverId'?new List<Schema.SObjectType>{User.SObjectType}:d.getReferenceTo(),2,1);
   if(ps==null||ps.size()<2){ o.put('skip','no two parents'); Database.rollback(sp); return o; }
   o.put('parentType',lastParentType); o.put('parentMode',lastParentMode);
   // reverse-name the parents when Name is updateable, so Id order and Name order disagree
   try{
    Schema.SObjectType pt=g().get(lastParentType.toLowerCase());
    Schema.SObjectField nf=pt.getDescribe().fields.getMap().get('Name');
    if(lastParentMode=='created' && nf!=null && nf.getDescribe().isUpdateable()){
     SObject lo=pt.newSObject(ps[0]); lo.put('Name','zz null probe'); SObject hi=pt.newSObject(ps[1]); hi.put('Name','aa null probe');
     update as system new List<SObject>{lo,hi}; o.put('parentNames','lo=zz,hi=aa');
    }
   }catch(Exception e){ o.put('parentNames','EXC '+e.getMessage().left(80)); }
   List<SObject> kids=new List<SObject>();
   for(Integer i=0;i<3;i++){ SObject k=build(t,1,field); k.put(field, i==0?null:ps[i-1]); if(objName=='Image' && field=='ContentDocumentId' && i>0){Map<String,Schema.SObjectField> fs=t.getDescribe().fields.getMap();for(String f:new List<String>{'Url','ImageUrl','ExternalUrl'})if(fs.containsKey(f)&&fs.get(f).getDescribe().isCreateable())k.put(f,null);} if(objName=='ContractLineItem' && field=='ParentContractLineItemId'){Id parentId=ps[0];SObject parent=Database.query('SELECT ServiceContractId FROM ContractLineItem WHERE Id = :parentId');k.put('ServiceContractId',parent.get('ServiceContractId'));} if(objName=='ProductConsumed' && field=='WorkOrderLineItemId'){Id parentId=ps[0];SObject parent=Database.query('SELECT WorkOrderId FROM WorkOrderLineItem WHERE Id = :parentId');k.put('WorkOrderId',parent.get('WorkOrderId'));} if(objName=='WorkOrderLineItem' && field=='ParentWorkOrderLineItemId'){Id parentId=ps[0];SObject parent=Database.query('SELECT WorkOrderId FROM WorkOrderLineItem WHERE Id = :parentId');k.put('WorkOrderId',parent.get('WorkOrderId'));} if(i>0 && field=='MaintenancePlanId' && (objName=='WorkOrder'||objName=='WorkOrderLineItem')) k.put('SuggestedMaintenanceDate',Date.today());
    if(i>0 && objName=='ProductServiceCampaignItem' && (field=='AssetId'||field=='Product2Id')) k.put(field=='AssetId'?'Product2Id':'AssetId',null);
    if(i>0 && objName=='ReturnOrderLineItem' && (field=='Product2Id'||field=='AssetId')) k.put(field=='Product2Id'?'AssetId':'Product2Id',null);
    if(i>0 && objName=='Partner' && (field=='AccountFromId'||field=='OpportunityId')) k.put(field=='AccountFromId'?'OpportunityId':'AccountFromId',null);
    kids.add(k); }
   insertRequired(kids,field,1);
   Id nullId=kids[0].Id; SObject saved=Database.query('SELECT '+field+' FROM '+objName+' WHERE Id = :nullId'); if(saved.get(field)!=null){o.put('skip','null input became populated');Database.rollback(sp);return o;}
   Map<Id,String> lab=new Map<Id,String>{kids[0].Id=>'N',kids[1].Id=>'L',kids[2].Id=>'H'};
   Set<Id> ids=lab.keySet();
   for(String tail:new List<String>{'NULLS FIRST','NULLS LAST','ASC','DESC','DESC NULLS LAST'}){
    List<String> got=new List<String>();
    for(SObject r:Database.query('SELECT Id FROM '+objName+' WHERE Id IN :ids ORDER BY '+field+' '+tail)) got.add(label(r.Id,lab));
    o.put(tail,String.join(got,''));
   }
  }catch(Exception e){ o.put('skip','EXC '+e.getTypeName()+': '+e.getMessage().left(160)); }
  Database.rollback(sp);
  return o;
 }
 public static String probeObject(String objName, List<String> fields){
  Map<String,Object> out=new Map<String,Object>();
  for(String f:fields){ try{ out.put(f,probeField(objName,f)); }catch(Exception e){ out.put(f,new Map<String,Object>{'skip'=>'EXC '+e.getMessage().left(160)}); } }
  return JSON.serialize(out);
 }
}
