@IsTest
private class T423_D13 {
  @IsTest static void run() {
    try { for (Integer i=0;i<1;i++) { String v='x'; } Database.QueryLocator ql = Database.getQueryLocator('SELECT Id FROM Account WHERE Name = :v'); } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
