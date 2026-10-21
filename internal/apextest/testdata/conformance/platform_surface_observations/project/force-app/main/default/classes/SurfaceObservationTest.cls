@IsTest private class SurfaceObservationTest {
 private class Recorder implements System.StubProvider {
  public List<String> calls=new List<String>();
  public Object handleMethodCall(Object stubbedObject,String name,Type returnType,List<Type> paramTypes,List<String> paramNames,List<Object> args) {calls.add(name);return 'answer';}
 }
 @IsTest static void stub_property() {
  Recorder recorder=new Recorder();
  PropertySurfaceTarget proxy=(PropertySurfaceTarget)Test.createStub(PropertySurfaceTarget.class,recorder);
  Map<String,Object> out=new Map<String,Object>{'plainBefore'=>proxy.plain,'fieldBefore'=>proxy.fieldValue,'explicit'=>proxy.label};
  proxy.plain='stored';proxy.fieldValue='field';
  out.put('plainAfter',proxy.plain);out.put('fieldAfter',proxy.fieldValue);out.put('calls',recorder.calls);
  System.assert(false,'OBSERVED|stub_property|'+JSON.serialize(out));
 }
 @IsTest static void serialization() {System.assert(false,'OBSERVED|serialization|'+JSON.serialize(GraphSerializationProbe.observe()));}
 @IsTest static void lead_defaults() {
  Database.LeadConvert value=new Database.LeadConvert();
  List<Boolean> initial=new List<Boolean>{value.isDoNotCreateOpportunity(),value.isOverwriteLeadSource(),value.isSendNotificationEmail()};
  value.setDoNotCreateOpportunity(true);value.setOverwriteLeadSource(true);value.setSendNotificationEmail(true);
  List<Boolean> changed=new List<Boolean>{value.isDoNotCreateOpportunity(),value.isOverwriteLeadSource(),value.isSendNotificationEmail()};
  System.assert(false,'OBSERVED|lead_defaults|'+JSON.serialize(new Map<String,Object>{'initial'=>initial,'changed'=>changed}));
 }
 @IsTest static void finalizer_predicate() {Test.startTest();PredicateContextProbe.start('context');Test.stopTest();System.assert(false,'OBSERVED|finalizer_predicate|'+PredicateContextProbe.report('context'));}
 @IsTest static void crypto_key() {
  Blob first=Crypto.generateAESKey(128),second=Crypto.generateAESKey(128);
  Blob plain=Blob.valueOf('owned plaintext');
  Blob encrypted=Crypto.encryptWithManagedIV('AES128',first,plain);
  System.assert(false,'OBSERVED|crypto_key|'+JSON.serialize(new Map<String,Object>{'length128'=>first.size(),'length192'=>Crypto.generateAESKey(192).size(),'length256'=>Crypto.generateAESKey(256).size(),'distinct'=>first!=second,'roundtrip'=>plain==Crypto.decryptWithManagedIV('AES128',first,encrypted)}));
 }
 private static Map<String,Object> reflection(String context) {
  try {
   Object value;
   if(context=='owner') value=ProtectedSurfaceProduct.ownerMake();
   else if(context=='child') value=ProtectedSurfaceChild.childMake();
   else value=ProtectedSurfaceProduct.class.newInstance();
   return new Map<String,Object>{'accepted'=>value!=null};
  }catch(Exception issue) {return new Map<String,Object>{'type'=>issue.getTypeName(),'message'=>issue.getMessage()};}
 }
 @IsTest static void protected_reflection() {System.assert(false,'OBSERVED|protected_reflection|'+JSON.serialize(new Map<String,Object>{'external'=>reflection('external'),'owner'=>reflection('owner'),'child'=>reflection('child')}));}
}
