@IsTest
private class T423_D21 {
  @IsTest static void run() {
    try { Account acc = new Account(Name='x'); Database.QueryLocator ql = Database.getQueryLocator('SELECT Id FROM Account WHERE Name = :acc.Name'); } catch (Exception e) { System.assert(false, 'R|' + e.getTypeName() + ': ' + e.getMessage()); }
  }
}
