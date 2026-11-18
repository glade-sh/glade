@IsTest
private class AsInfoTest {
 @IsTest static void observe() {
  AsSupport.depthSeen.clear();
  Test.startTest();
  AsyncOptions o = new AsyncOptions();
  o.MaximumQueueableStackDepth = 3;
  o.MinimumQueueableDelayInMinutes = 0;
  System.enqueueJob(new AsSupport.DepthJob(), o);
  System.enqueueJob(new AsSupport.DepthJob());
  Test.stopTest();
  System.assert(false, 'OBS|' + String.join(AsSupport.depthSeen, ';'));
 }
}
