@IsTest
private class T423_D12 {
  @IsTest static void run() {
    try { if (true) { String v='x'; } Database.QueryLocator ql = Database.getQueryLocator('SELECT Id FROM Account WHERE Name = :v'); } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
