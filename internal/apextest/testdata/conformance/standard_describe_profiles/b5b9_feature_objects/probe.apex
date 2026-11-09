System.debug('ROW custom-metadata-map '+OwnedFlag__mdt.SObjectType.getDescribe().fields.getMap().containsKey('MasterLabel'));
OwnedFlag__mdt rec = [SELECT DeveloperName, MasterLabel, Label FROM OwnedFlag__mdt WHERE DeveloperName='Example' LIMIT 1];
System.debug('ROW custom-metadata-value '+rec.MasterLabel+'|'+rec.Label);
System.debug('ROW custom-metadata-absent '+OwnedFlag__mdt.SObjectType.getDescribe().fields.getMap().containsKey('OwnedMissing__c'));
System.debug('ROW partition-count '+[SELECT COUNT() FROM PlatformCachePartition]);
System.QueueableDuplicateSignature.Builder builder=QueueableDuplicateSignature.builder();
System.debug('ROW builder-empty-size '+builder.getSize()+'|'+builder.getMaxSize()+'|'+builder.getRemainingSize());
System.QueueableDuplicateSignature sig=builder.addString('owned').addInteger(42).addId('001000000000001AAA').build();
System.debug('ROW builder-value '+sig.toString());
System.AsyncOptions dup=new System.AsyncOptions(); dup.DuplicateSignature=sig;
try { System.enqueueJob(new OwnedQueue(),dup); System.enqueueJob(new OwnedQueue(),dup); System.debug('ROW duplicate accepted'); } catch(Exception ex) { System.debug('ROW duplicate '+ex.getTypeName()+'|'+ex.getMessage()); }
for(Integer v : new List<Integer>{-1,0,1,10,11}) {
 System.AsyncOptions opt=new System.AsyncOptions(); opt.MinimumQueueableDelayInMinutes=v;
 try { System.enqueueJob(new OwnedQueue(),opt); System.debug('ROW delay-'+v+' accepted'); } catch(Exception ex) {System.debug('ROW delay-'+v+' '+ex.getTypeName()+'|'+ex.getMessage());}
}
for(Integer v : new List<Integer>{-1,0,1,2}) {
 System.AsyncOptions opt=new System.AsyncOptions(); opt.MaximumQueueableStackDepth=v;
 try { System.enqueueJob(new OwnedQueue(),opt); System.debug('ROW depth-'+v+' accepted'); } catch(Exception ex) {System.debug('ROW depth-'+v+' '+ex.getTypeName()+'|'+ex.getMessage());}
}
