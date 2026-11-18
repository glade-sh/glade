@IsTest
private class AsDedupTest {
 @IsTest static void observe() {
  List<String> rows = new List<String>();
  AsyncOptions a = new AsyncOptions();
  a.DuplicateSignature = QueueableDuplicateSignature.builder().addString('sig').build();
  Test.startTest();
  Id first = System.enqueueJob(new AsSupport.Job(), a);
  try {
   Id second = System.enqueueJob(new AsSupport.Job(), a);
   rows.add('second=ok:' + (second == first));
  } catch (Exception e) { rows.add('second=' + e.getTypeName() + '|' + e.getMessage()); }
  Id third = System.enqueueJob(new AsSupport.Job());
  rows.add('first=' + (first != null) + '|third=' + (third != null) + '|distinct=' + (first != third));
  System.enqueueJob(new AsSupport.DedupChild());
  Test.stopTest();
  rows.add('child=' + String.join(AsSupport.dedupSeen, ','));
  System.assert(false, 'OBS|' + String.join(rows, ';'));
 }
}
