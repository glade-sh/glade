public class Probe2712 {
 public static Map<String,Object> observe(String name) {
  ViewConfig__mdt lower=(ViewConfig__mdt)JSON.deserialize('{"entity__c":"Account","entity__r":{"attributes":{"type":"EntityDefinition"},"qualifiedapiname":"Account"}}', ViewConfig__mdt.class);
  ViewGroup__mdt holder=(ViewGroup__mdt)JSON.deserialize(JSON.serialize(new Map<String,Object>{'config__r'=>lower}),ViewGroup__mdt.class);
  ViewConfig__mdt scalar=new ViewConfig__mdt(Entity__c='Account');
  Map<String,Object> out=new Map<String,Object>{'scalar'=>lower.Entity__c,'parent'=>lower.Entity__r==null?null:lower.Entity__r.QualifiedApiName,'nested'=>holder.Config__r==null?null:holder.Config__r.Entity__c,'control'=>scalar.Entity__c};
  ViewConfig__mdt exact=(ViewConfig__mdt)JSON.deserialize('{"Entity__r":{"attributes":{"type":"EntityDefinition"},"QualifiedApiName":"Account"}}',ViewConfig__mdt.class);
  ViewGroup__mdt upper=(ViewGroup__mdt)JSON.deserialize(JSON.serialize(new Map<String,Object>{'Config__r'=>exact}),ViewGroup__mdt.class);
  Contact lowerContact=(Contact)JSON.deserialize('{"account":{"attributes":{"type":"Account"},"Name":"A"}}',Contact.class);
  Contact exactContact=(Contact)JSON.deserialize('{"Account":{"attributes":{"type":"Account"},"Name":"A"}}',Contact.class);
  out.put('exact',exact.Entity__r==null?null:exact.Entity__r.QualifiedApiName);out.put('upper',upper.Config__r==null?null:upper.Config__r.Entity__r.QualifiedApiName);
  out.put('contactLower',lowerContact.Account==null?null:lowerContact.Account.Name);out.put('contactExact',exactContact.Account==null?null:exactContact.Account.Name);
  Contact mixed=(Contact)JSON.deserialize('{"account":{"Name":"lower"},"Account":{"Name":"upper"}}',Contact.class);
  try { out.put('dynamicLower',lowerContact.getSObject('account')==null?null:lowerContact.getSObject('account').get('Name')); } catch(Exception ex){ out.put('dynamicLower',ex.getTypeName()); }
  try { out.put('dynamicExact',lowerContact.getSObject('Account')==null?null:lowerContact.getSObject('Account').get('Name')); } catch(Exception ex){ out.put('dynamicExact',ex.getTypeName()); }
  out.put('mixed',mixed.Account==null?null:mixed.Account.Name);
  try { scalar.putSObject('Entity__r',null); out.put('rejection','accepted'); } catch(Exception ex) { out.put('rejection',ex.getTypeName()); }
  return out;
 }
}
