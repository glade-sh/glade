@IsTest
private class Fin1NoTest {
 @IsTest static void observe() {
  Test.setMock(HttpCalloutMock.class, new FinSupport.Mock());
  Test.startTest();
  System.enqueueJob(new FinSupport.NoParent());
  Test.stopTest();
  System.assert(false, 'OBS|res=' + FinSupport.noRes + ';ctx=' + FinSupport.noCtx + ';count=' + FinSupport.noCount);
 }
}
