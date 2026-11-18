@IsTest
private class T423_D11 {
  @IsTest static void run() {
    try { String nope='x'; Database.QueryLocator ql = Database.getQueryLocator('SELECT Id FROM Account WHERE Name = :nope'); } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
