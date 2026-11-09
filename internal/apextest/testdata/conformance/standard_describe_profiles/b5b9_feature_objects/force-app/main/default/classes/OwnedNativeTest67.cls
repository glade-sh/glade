@IsTest private class OwnedNativeTest67 {
 @IsTest static void customMetadata() {
  System.assertEquals(true,OwnedFlag__mdt.SObjectType.getDescribe().fields.getMap().containsKey('MasterLabel'));
  System.assertEquals(false,OwnedFlag__mdt.SObjectType.getDescribe().fields.getMap().containsKey('OwnedMissing__c'));
  OwnedFlag__mdt row=[SELECT MasterLabel,Label FROM OwnedFlag__mdt WHERE DeveloperName='Example' LIMIT 1];
  System.assertEquals('Owned example',row.MasterLabel); System.assertEquals('Owned example',row.Label);
 }
 @IsTest static void signature() {
  System.QueueableDuplicateSignature.Builder b=QueueableDuplicateSignature.builder();
  System.assertEquals(0,b.getSize());System.assertEquals(32,b.getMaxSize());System.assertEquals(32,b.getRemainingSize());
  QueueableDuplicateSignature sig=b.addString('owned').addInteger(42).addId('001000000000001AAA').build();
  System.assertEquals('\'owned\'_42_001000000000001',sig.toString());
  AsyncOptions o=new AsyncOptions();o.DuplicateSignature=sig;
  System.enqueueJob(new OwnedQueue(),o);Boolean rejected=false;
  try {System.enqueueJob(new OwnedQueue(),o);}catch(System.DuplicateMessageException ex){rejected=true;}
  System.assertEquals(true,rejected);
 }
 @IsTest static void delayLower() {AsyncOptions o=new AsyncOptions();o.MinimumQueueableDelayInMinutes=-1;Boolean rejected=false;try{System.enqueueJob(new OwnedQueue(),o);}catch(System.InvalidParameterValueException ex){rejected=true;}System.assertEquals(true,rejected);}
 @IsTest static void delayUpper() {AsyncOptions o=new AsyncOptions();o.MinimumQueueableDelayInMinutes=11;Boolean rejected=false;try{System.enqueueJob(new OwnedQueue(),o);}catch(System.InvalidParameterValueException ex){rejected=true;}System.assertEquals(true,rejected);}
 @IsTest static void delayBoundary() {AsyncOptions o=new AsyncOptions();o.MinimumQueueableDelayInMinutes=10;System.assertNotEquals(null,System.enqueueJob(new OwnedQueue(),o));}
 @IsTest static void depthNegative() {AsyncOptions o=new AsyncOptions();o.MaximumQueueableStackDepth=-1;Boolean rejected=false;try{System.enqueueJob(new OwnedQueue(),o);}catch(System.InvalidParameterValueException ex){rejected=true;}System.assertEquals(true,rejected);}
 @IsTest static void depthZero() {AsyncOptions o=new AsyncOptions();o.MaximumQueueableStackDepth=0;Boolean rejected=false;try{System.enqueueJob(new OwnedQueue(),o);}catch(System.AsyncException ex){rejected=true;}System.assertEquals(true,rejected);}
 @IsTest static void depthBoundary() {AsyncOptions o=new AsyncOptions();o.MaximumQueueableStackDepth=1;System.assertNotEquals(null,System.enqueueJob(new OwnedQueue(),o));}
}
