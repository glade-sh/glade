@IsTest
private class AsCapTest {
 @IsTest static void observe() {
  List<String> rows = new List<String>();
  AsSupport.hits.clear();
  AsyncOptions o = new AsyncOptions();
  o.MaximumQueueableStackDepth = 11;
  o.MinimumQueueableDelayInMinutes = 0;
  o.DuplicateSignature = QueueableDuplicateSignature.builder().addString('combo').build();
  rows.add('opts=' + o.MinimumQueueableDelayInMinutes + '/' + o.MaximumQueueableStackDepth + '/' + String.valueOf(o.DuplicateSignature));
  Test.startTest();
  Id root = System.enqueueJob(new AsSupport.Chain(1, 4), o);
  rows.add('root=' + (root != null) + '|jobs=' + Limits.getQueueableJobs());
  Test.stopTest();
  rows.add('hits=' + String.join(AsSupport.hits, ','));
  System.assert(false, 'OBS|' + String.join(rows, ';'));
 }
}
