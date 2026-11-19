@IsTest(SeeAllData=true) public class NullExisting62 {
 public static Map<String,Object> observe(String objectName,String fieldName){
  Map<String,Object> result=new Map<String,Object>();
  try{
   Schema.SObjectType type=Schema.getGlobalDescribe().get(objectName);
   if(type==null || !type.getDescribe().fields.getMap().containsKey(fieldName)){result.put('skip','field absent from describe');return result;}
   List<SObject> ns=Database.query('SELECT Id FROM '+objectName+' WHERE '+fieldName+' = null LIMIT 1');
   if(ns.isEmpty()){result.put('skip','no existing null child');return result;}
   List<SObject> children=Database.query('SELECT Id,'+fieldName+' FROM '+objectName+' WHERE '+fieldName+' != null LIMIT 100');
   Map<Id,Id> childByParent=new Map<Id,Id>();for(SObject child:children){Id parent=(Id)child.get(fieldName);if(!childByParent.containsKey(parent))childByParent.put(parent,child.Id);}
   if(childByParent.size()<2){result.put('skip','fewer than two existing distinct parents');return result;}
   List<Id> parentIds=new List<Id>(childByParent.keySet());parentIds.sort();Id nullId=ns[0].Id;Id low=childByParent.get(parentIds[0]);Id high=childByParent.get(parentIds[1]);Set<Id> ids=new Set<Id>{nullId,low,high};Map<Id,String> labels=new Map<Id,String>{nullId=>'N',low=>'L',high=>'H'};
   for(String tail:new List<String>{'NULLS FIRST','NULLS LAST','ASC','DESC','DESC NULLS LAST'}){List<String> order=new List<String>();for(SObject child:Database.query('SELECT Id FROM '+objectName+' WHERE Id IN :ids ORDER BY '+fieldName+' '+tail))order.add(labels.get(child.Id));result.put(tail,String.join(order,''));}
  }catch(Exception e){result.put('skip','EXC '+e.getTypeName()+': '+e.getMessage().left(160));}return result;
 }
}
