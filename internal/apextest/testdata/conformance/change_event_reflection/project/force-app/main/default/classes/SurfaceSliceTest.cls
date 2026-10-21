@IsTest private class SurfaceSliceTest {
 @IsTest static void changeEventReflection() {
  Type eventType=Type.forName('AccountChangeEvent');
  Map<String,Object> result=new Map<String,Object>{'resolved'=>eventType!=null,'staticName'=>AccountChangeEvent.SObjectType.getDescribe().getName()};
  if(eventType!=null)result.put('instanceName',((SObject)eventType.newInstance()).getSObjectType().getDescribe().getName());
  for(String name:new List<String>{'ContactChangeEvent','LeadChangeEvent','OpportunityChangeEvent','CaseChangeEvent','BusinessHoursChangeEvent','NoSuchChangeEvent','Schema.AccountChangeEvent'}) {
   Type token=Type.forName(name);
   result.put(name,token==null?null:((SObject)token.newInstance()).getSObjectType().getDescribe().getName());
  }
  System.assert(false,'OBSERVED|changeEventReflection|'+JSON.serialize(result));
 }
}
