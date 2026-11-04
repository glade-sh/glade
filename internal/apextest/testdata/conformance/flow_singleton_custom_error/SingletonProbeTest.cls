@IsTest private class SingletonProbeTest {
 @IsTest static void observe() {
  Database.SaveResult r = Database.insert(new Account(Name='control'), false);
  String observation = r.isSuccess() ? 'SUCCESS' : String.valueOf(r.getErrors()[0].getStatusCode()) + '|' + r.getErrors()[0].getMessage();
  System.assertEquals('REPORT', observation);
 }
}
