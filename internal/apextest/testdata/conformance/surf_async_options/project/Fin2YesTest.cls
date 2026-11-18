@IsTest
private class Fin2YesTest {
 @IsTest static void observe() {
  Test.setMock(HttpCalloutMock.class, new FinSupport.Mock());
  Test.startTest();
  System.enqueueJob(new FinSupport.YesParent());
  Test.stopTest();
  System.assert(false, 'OBS|res=' + FinSupport.yesRes + ';ctx=' + FinSupport.yesCtx + ';count=' + FinSupport.yesCount);
 }
}
