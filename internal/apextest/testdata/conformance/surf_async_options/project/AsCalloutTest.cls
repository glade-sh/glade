@IsTest
private class AsCalloutTest {
 @IsTest static void observe() {
  List<String> rows = new List<String>();
  Test.setMock(HttpCalloutMock.class, new AsSupport.Mock());
  Test.startTest();
  System.enqueueJob(new AsSupport.NoCalloutJob());
  System.enqueueJob(new AsSupport.CalloutJob());
  Test.stopTest();
  rows.add('no=' + AsSupport.noResult);
  rows.add('yes=' + AsSupport.yesResult);
  System.assert(false, 'OBS|' + String.join(rows, ';'));
 }
}
