
public inherited sharing class HierarchyReader62 {
 public static List<ProbeNode__c> read(Id leafId, Boolean dynamicQuery) {
  if(dynamicQuery) return Database.queryWithBinds('SELECT Id,Parent__c,Parent__r.Id,Parent__r.Parent__c,Parent__r.Parent__r.Id FROM ProbeNode__c WHERE Id=:leafId',new Map<String,Object>{'leafId'=>leafId},AccessLevel.SYSTEM_MODE);
  return [SELECT Id,Parent__c,Parent__r.Id,Parent__r.Parent__c,Parent__r.Parent__r.Id FROM ProbeNode__c WHERE Id=:leafId WITH SYSTEM_MODE];
 }
 public static String label(Id value,Id rootId,Id midId,Id leafId){
  if(value==null)return 'null';if(value==leafId)return 'leaf';if(value==midId)return 'middle';if(value==rootId)return 'root';return 'other';
 }
 public static Map<String,Object> observe(Id leafId,Id midId,Id rootId,Boolean dynamicQuery){
  Integer beforeQueries=Limits.getQueries(),beforeRows=Limits.getQueryRows();
  Map<String,Object> out=new Map<String,Object>();
  out.put('directLeaf',[SELECT count() FROM ProbeNode__c WHERE Id=:leafId WITH SYSTEM_MODE]);
  out.put('directMiddle',[SELECT count() FROM ProbeNode__c WHERE Id=:midId WITH SYSTEM_MODE]);
  out.put('directRoot',[SELECT count() FROM ProbeNode__c WHERE Id=:rootId WITH SYSTEM_MODE]);
  List<ProbeNode__c> rows=read(leafId,dynamicQuery);out.put('leafRows',rows.size());
  if(!rows.isEmpty()){
   ProbeNode__c leaf=rows[0];
   out.put('leafForeignKey',label(leaf.Parent__c,rootId,midId,leafId));
   try{out.put('parentNull',leaf.Parent__r==null);}catch(Exception e){out.put('parentNullError',e.getTypeName()+':'+e.getMessage());}
   try{out.put('parentId',label(leaf.Parent__r.Id,rootId,midId,leafId));}catch(Exception e){out.put('parentIdError',e.getTypeName()+':'+e.getMessage());}
   try{out.put('parentForeignKey',label(leaf.Parent__r.Parent__c,rootId,midId,leafId));}catch(Exception e){out.put('parentForeignKeyError',e.getTypeName()+':'+e.getMessage());}
   try{out.put('rootId',label(leaf.Parent__r.Parent__r.Id,rootId,midId,leafId));}catch(Exception e){out.put('rootIdError',e.getTypeName()+':'+e.getMessage());}
   try{out.put('getSObjectNull',leaf.getSObject('Parent__r')==null);}catch(Exception e){out.put('getSObjectError',e.getTypeName()+':'+e.getMessage());}
   try{ProbeNode__c rebound=leaf;out.put('reboundParentId',label(rebound.Parent__r.Id,rootId,midId,leafId));out.put('reboundRootId',label(rebound.Parent__r.Parent__r.Id,rootId,midId,leafId));}catch(Exception e){out.put('reboundError',e.getTypeName()+':'+e.getMessage());}
  }
  out.put('queryDelta',Limits.getQueries()-beforeQueries);out.put('queryRowsDelta',Limits.getQueryRows()-beforeRows);
  return out;
 }
}
