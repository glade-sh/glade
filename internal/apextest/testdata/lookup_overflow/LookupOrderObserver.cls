public class LookupOrderObserver {
 public static Map<String,Object> observe(){
  Map<String,Schema.SObjectField> fields=Account.SObjectType.getDescribe().fields.getMap();
  List<String> keys=new List<String>(fields.keySet());
  List<Object> facets=new List<Object>();List<String> eligible=new List<String>();
  List<String> main=new List<String>();List<String> overflow=new List<String>();Integer ordinal=0;
  for(String key:keys){
   Schema.DescribeFieldResult facet=fields.get(key).getDescribe();List<String> targets=new List<String>();
   for(Schema.SObjectType target:facet.getReferenceTo())targets.add(target.getDescribe().getName());
   facets.add(new List<Object>{key,facet.getName(),facet.getLocalName(),String.valueOf(facet.getType()),facet.isUpdateable(),facet.getRelationshipName(),targets});
   if(facet.getType()!=Schema.DisplayType.REFERENCE||!facet.isUpdateable())continue;
   eligible.add(facet.getName());
   if(ordinal<4){main.add(facet.getName());main.add(facet.getRelationshipName()+'.Name');}
   else{overflow.add(facet.getName());overflow.add(facet.getRelationshipName()+'.Name');}
   ordinal++;
  }
  List<String> valueNames=new List<String>();for(Schema.SObjectField token:fields.values())valueNames.add(token.getDescribe().getName());
  List<Object> twins=new List<Object>();
  for(String local:new List<String>{'AlphaBridge__c','BetaBridge__c','GammaBridge__c','ZetaBridge__c'}){
   Schema.SObjectField token=fields.get(local);Schema.DescribeFieldResult facet=token.getDescribe();String full=facet.getName();
   twins.add(new Map<String,Object>{'local'=>local,'name'=>full,'localPresent'=>fields.containsKey(local),'qualifiedPresent'=>fields.containsKey(full),'sameToken'=>fields.get(local)==fields.get(full),'eligibleOrdinal'=>eligible.indexOf(full)+1});
  }
  return new Map<String,Object>{'outcome'=>'complete','declaredSourceApi'=>'CAPTURE_SOURCE_API','actorId'=>UserInfo.getUserId(),'profileId'=>UserInfo.getProfileId(),'keys'=>keys,'copyKeys'=>new List<String>(new Set<String>(fields.keySet())),'repeatKeys'=>new List<String>(Account.SObjectType.getDescribe().fields.getMap().keySet()),'facets'=>facets,'eligible'=>eligible,'main'=>main,'overflow'=>overflow,'values'=>valueNames,'twins'=>twins,'personAccounts'=>fields.containsKey('IsPersonAccount')};
 }
 public static void emit(String row,Integer slot){
  String payload=EncodingUtil.base64Encode(Blob.valueOf(JSON.serialize(observe())));
  String digest=EncodingUtil.convertToHex(Crypto.generateDigest('SHA-256',Blob.valueOf(payload)));
  Integer parts=(payload.length()+299)/300;Integer start=slot*300;
  String part=start<payload.length()?payload.substring(start,Math.min(start+300,payload.length())):'';
  System.assert(false,'LOOKUP_ORDER|'+row+'|'+slot+'|'+parts+'|'+digest+'|'+part);
 }
}
