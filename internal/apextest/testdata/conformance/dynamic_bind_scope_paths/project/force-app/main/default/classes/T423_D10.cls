@IsTest
private class T423_D10 {
  @IsTest static void run() {
    try { Database.QueryLocator ql = Database.getQueryLocator('SELECT Id FROM Account WHERE Name = :nope'); } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
