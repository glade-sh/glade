
@IsTest private class HierarchyNativeTest62 {
 private class ProbeException extends Exception {}
 private static void grantRead(Id recordId,Id actorId){insert as system new ProbeNode__Share(ParentId=recordId,UserOrGroupId=actorId,AccessLevel='Read',RowCause=Schema.ProbeNode__Share.RowCause.Manual);}
 private static Map<String,Object> observed(ProbeNode__c leaf,ProbeNode__c middle,ProbeNode__c root,Boolean dynamicQuery){return HierarchyEntry62.observe(leaf.Id,middle.Id,root.Id,dynamicQuery);}
 private static void capture(String row){
  Id originalId=UserInfo.getUserId();User owner=[SELECT Id,UserRoleId FROM User WHERE Id=:originalId];
  Profile ordinary=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1];
  String suffix=String.valueOf(DateTime.now().getTime())+row+'62';
  User reader=new User(Alias='hprobe',Email='owned-probe@example.invalid',EmailEncodingKey='UTF-8',LastName='Probe',LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='America/Los_Angeles',ProfileId=ordinary.Id,UserName='owned-hierarchy-'+suffix+'@example.invalid');
  insert as system reader;
  Map<String,Object> out=new Map<String,Object>{'readerRoleAbsent'=>reader.UserRoleId==null,'readerIsStandardProfile'=>reader.ProfileId==ordinary.Id,'ownerIsOriginal'=>owner.Id==originalId};
  System.runAs(owner){
   ProbeNode__c root=new ProbeNode__c(Name='Root');insert as system root;
   ProbeNode__c middle=new ProbeNode__c(Name='Middle',Parent__c=root.Id);insert as system middle;
   ProbeNode__c leaf=new ProbeNode__c(Name='Leaf',Parent__c=middle.Id);insert as system leaf;
   if(row!='H05')grantRead(leaf.Id,reader.Id);
   if(row=='H01'){
    System.runAs(reader){out.put('static',observed(leaf,middle,root,false));}
   }else if(row=='H02'){
    System.runAs(reader){out.put('dynamic',observed(leaf,middle,root,true));out.put('static',observed(leaf,middle,root,false));}
   }else if(row=='H03'){
    grantRead(middle.Id,reader.Id);
    System.runAs(reader){out.put('static',observed(leaf,middle,root,false));out.put('dynamic',observed(leaf,middle,root,true));}
   }else if(row=='H04'){
    out.put('owner',observed(leaf,middle,root,false));
    grantRead(middle.Id,reader.Id);grantRead(root.Id,reader.Id);
    System.runAs(reader){out.put('allShared',observed(leaf,middle,root,false));}
    delete as system [SELECT Id FROM ProbeNode__Share WHERE UserOrGroupId=:reader.Id AND RowCause='Manual' WITH SYSTEM_MODE];
    System.runAs(reader){out.put('unshared',observed(leaf,middle,root,false));}
   }else if(row=='H05'){
    try{
     leaf.Parent__c=leaf.Id;update as system leaf;grantRead(leaf.Id,reader.Id);
     System.runAs(reader){out.put('selfStatic',observed(leaf,middle,root,false));out.put('selfDynamic',observed(leaf,middle,root,true));}
    }catch(Exception e){out.put('selfSetupError',e.getTypeName()+':'+e.getMessage());}
    leaf.Parent__c=middle.Id;update as system leaf;
    System.runAs(reader){out.put('withoutSharing',HierarchyBypass62.observe(leaf.Id,middle.Id,root.Id,false));}
   }else if(row=='H06'){
    Integer beforeQueries=Limits.getQueries();
    out.put('ownerBefore',observed(leaf,middle,root,false));
    System.runAs(reader){
     out.put('readerActor',UserInfo.getUserId()==reader.Id);out.put('readerBefore',observed(leaf,middle,root,true));
     try{System.runAs(owner){out.put('innerOwnerActor',UserInfo.getUserId()==owner.Id);out.put('innerOwner',observed(leaf,middle,root,false));throw new ProbeException('owned unwind');}}
     catch(ProbeException e){out.put('caughtOwnedException',e.getMessage()=='owned unwind');}
     out.put('readerRestoredOnUnwind',UserInfo.getUserId()==reader.Id);out.put('readerAfter',observed(leaf,middle,root,false));
    }
    out.put('ownerRestored',UserInfo.getUserId()==owner.Id);out.put('ownerAfter',observed(leaf,middle,root,true));out.put('nestedQueryDelta',Limits.getQueries()-beforeQueries);
   }
   Map<Id,ProbeNode__c> stored=new Map<Id,ProbeNode__c>([SELECT Id,Parent__c FROM ProbeNode__c WHERE Id IN :new Set<Id>{root.Id,middle.Id,leaf.Id} WITH SYSTEM_MODE]);
   out.put('sourceUnchanged',stored.size()==3&&stored.get(root.Id).Parent__c==null&&stored.get(middle.Id).Parent__c==root.Id&&stored.get(leaf.Id).Parent__c==middle.Id);
  }
  out.put('originalActorRestored',UserInfo.getUserId()==originalId);
  System.assert(false,'NATIVE_HIERARCHY|'+row+'|'+JSON.serialize(out));
 }
 @IsTest static void rowH01(){capture('H01');}
 @IsTest static void rowH02(){capture('H02');}
 @IsTest static void rowH03(){capture('H03');}
 @IsTest static void rowH04(){capture('H04');}
 @IsTest static void rowH05(){capture('H05');}
 @IsTest static void rowH06(){capture('H06');}
}
