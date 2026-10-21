public with sharing class LegacyOmissionObserver {
 public static Map<String,Object> observe(String row){
  ProjectionTile__c seed=new ProjectionTile__c(Name='Owned legacy omission',Amount__c=12,CountValue__c=4);insert seed;
  ProjectionTile__c item=LegacyOmissionQuery.fetch(seed.Id,row=='SELECTED_L');
  LegacyOmissionWrite.apply(item,row);
  Map<String,Object> reads=new Map<String,Object>();
  for(String mode:new List<String>{'direct','string','token'}){
   try {Object value=LegacyOmissionRead.lookup(item,mode);reads.put(mode,new Map<String,Object>{'ok'=>true,'value'=>value});}
   catch(Exception e){reads.put(mode,new Map<String,Object>{'ok'=>false,'type'=>e.getTypeName(),'message'=>e.getMessage()});}
  }
  return new Map<String,Object>{'outcome'=>'complete','row'=>row,'sourcePins'=>new List<Integer>{24,39,40},'amount'=>item.Amount__c,'reads'=>reads};
 }
 public static void emit(String row,Integer slot){
  String payload=EncodingUtil.base64Encode(Blob.valueOf(JSON.serialize(observe(row))));
  String digest=EncodingUtil.convertToHex(Crypto.generateDigest('SHA-256',Blob.valueOf(payload)));
  Integer total=(payload.length()+299)/300;Integer start=slot*300;
  String part=start<payload.length()?payload.substring(start,Math.min(start+300,payload.length())):'';
  System.assert(false,'LEGACY_OMISSION|'+row+'|'+slot+'|'+total+'|'+digest+'|'+part);
 }
}
