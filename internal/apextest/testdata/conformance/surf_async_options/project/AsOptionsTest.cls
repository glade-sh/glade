@IsTest
private class AsOptionsTest {
 @IsTest static void observe() {
  List<String> rows = new List<String>();
  AsyncOptions echo = new AsyncOptions();
  echo.MinimumQueueableDelayInMinutes = 5;
  echo.MaximumQueueableStackDepth = 2;
  echo.DuplicateSignature = QueueableDuplicateSignature.builder().addString('sig-a').build();
  rows.add('echo=' + echo.MinimumQueueableDelayInMinutes + '/' + echo.MaximumQueueableStackDepth + '/' + String.valueOf(echo.DuplicateSignature));
  AsyncOptions d = new AsyncOptions();
  rows.add('default=' + d.MinimumQueueableDelayInMinutes + '/' + d.MaximumQueueableStackDepth + '/' + String.valueOf(d.DuplicateSignature));
  Test.startTest();
  for (Integer m : new List<Integer>{0, 1, 10, 11, 60, 1440}) {
   try {
    AsyncOptions o = new AsyncOptions();
    o.MinimumQueueableDelayInMinutes = m;
    Id j = System.enqueueJob(new AsSupport.Job(), o);
    rows.add(m + '=ok');
   } catch (Exception e) { rows.add(m + '=' + e.getTypeName() + '|' + e.getMessage()); }
  }
  Test.stopTest();
  System.assert(false, 'OBS|' + String.join(rows, ';'));
 }
}
